// Package eventledger bridges YKC's atomic event store with the legacy
// hash-chained ledger. The event store is the durable outbox; the ledger is the
// tamper-evident audit projection. If ledger append fails after an event is
// committed, SyncMissing can rebuild the missing projection deterministically.
package eventledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ykc/internal/domain"
	"ykc/internal/eventstore"
	"ykc/internal/ledger"
)

const (
	DefaultActor = "ykc-event-bridge"
	FactPrefix   = "event."
)

type Bridge struct {
	Events     *eventstore.Store
	LedgerPath string
	Actor      string
}

type BridgePayload struct {
	BridgeVersion int              `json:"bridge_version"`
	EventID       string           `json:"event_id"`
	EventKind     domain.EventKind `json:"event_kind"`
	EventAt       time.Time        `json:"event_at"`
	Epoch         string           `json:"epoch,omitempty"`
	WorkspaceRoot string           `json:"workspace_root,omitempty"`
	PayloadSHA256 string           `json:"payload_sha256"`
	Envelope      domain.Envelope  `json:"envelope"`
}

type AppendResult struct {
	Event       domain.Envelope `json:"event"`
	LedgerSeq   uint64          `json:"ledger_seq,omitempty"`
	LedgerHash  string          `json:"ledger_hash,omitempty"`
	AlreadySeen bool            `json:"already_seen"`
}

type SyncResult struct {
	EventsSeen       int      `json:"events_seen"`
	AlreadyProjected int      `json:"already_projected"`
	Projected        int      `json:"projected"`
	LedgerHead       string   `json:"ledger_head,omitempty"`
	MissingFailed    []string `json:"missing_failed,omitempty"`
}

func Open(stateDir string, actor string) (*Bridge, error) {
	if stateDir == "" {
		return nil, errors.New("state directory is required")
	}
	if actor == "" {
		actor = DefaultActor
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, err
	}
	events, err := eventstore.New(filepath.Join(stateDir, "events"))
	if err != nil {
		return nil, err
	}
	return &Bridge{Events: events, LedgerPath: filepath.Join(stateDir, "ledger.jsonl"), Actor: actor}, nil
}

func (b *Bridge) Append(e domain.Envelope) (AppendResult, error) {
	if b == nil || b.Events == nil {
		return AppendResult{}, errors.New("nil event ledger bridge")
	}
	committed, err := b.Events.Append(e)
	if err != nil {
		return AppendResult{}, err
	}
	seq, head, already, err := b.projectIfMissing(committed)
	if err != nil {
		return AppendResult{Event: committed}, err
	}
	return AppendResult{Event: committed, LedgerSeq: seq, LedgerHash: head, AlreadySeen: already}, nil
}

func (b *Bridge) ReplayEvents() ([]domain.Envelope, error) {
	if b == nil || b.Events == nil {
		return nil, errors.New("nil event ledger bridge")
	}
	return b.Events.Replay()
}

func (b *Bridge) SyncMissing() (SyncResult, error) {
	if b == nil || b.Events == nil {
		return SyncResult{}, errors.New("nil event ledger bridge")
	}
	events, err := b.Events.Replay()
	if err != nil {
		return SyncResult{}, err
	}
	seen := b.projectedEventIDs()
	res := SyncResult{EventsSeen: len(events)}
	for _, e := range events {
		if seen[e.ID] {
			res.AlreadyProjected++
			continue
		}
		_, _, _, err := b.projectIfMissing(e)
		if err != nil {
			res.MissingFailed = append(res.MissingFailed, e.ID+": "+err.Error())
			continue
		}
		seen[e.ID] = true
		res.Projected++
	}
	_, head, err := ledger.VerifyChain(b.LedgerPath)
	if err == nil {
		res.LedgerHead = head
	}
	if len(res.MissingFailed) > 0 {
		return res, fmt.Errorf("failed to project %d event(s)", len(res.MissingFailed))
	}
	return res, nil
}

func (b *Bridge) VerifyLedger() (bool, string, error) {
	if b == nil {
		return false, "", errors.New("nil event ledger bridge")
	}
	return ledger.VerifyChain(b.LedgerPath)
}

func (b *Bridge) projectIfMissing(e domain.Envelope) (seq uint64, head string, already bool, err error) {
	if e.ID == "" {
		return 0, "", false, errors.New("cannot project event without id")
	}
	if b.projectedEventIDs()[e.ID] {
		_, head, _ := ledger.VerifyChain(b.LedgerPath)
		return 0, head, true, nil
	}
	led, err := ledger.Open(b.LedgerPath)
	if err != nil {
		return 0, "", false, err
	}
	defer led.Close()
	payload := NewBridgePayload(e)
	seq, err = led.Append(FactType(e.Kind), b.Actor, payload)
	if err != nil {
		return 0, "", false, err
	}
	return seq, led.Head(), false, nil
}

func NewBridgePayload(e domain.Envelope) BridgePayload {
	sum := sha256.Sum256(e.Payload)
	return BridgePayload{
		BridgeVersion: 1,
		EventID:       e.ID,
		EventKind:     e.Kind,
		EventAt:       e.At,
		Epoch:         e.Epoch,
		WorkspaceRoot: e.WorkspaceRoot,
		PayloadSHA256: hex.EncodeToString(sum[:]),
		Envelope:      e,
	}
}

func FactType(kind domain.EventKind) string {
	k := strings.TrimSpace(string(kind))
	if k == "" {
		k = "unknown"
	}
	return FactPrefix + k
}

func IsBridgeFact(f ledger.Fact) bool {
	if !strings.HasPrefix(f.Type, FactPrefix) {
		return false
	}
	var p struct {
		BridgeVersion int    `json:"bridge_version"`
		EventID       string `json:"event_id"`
	}
	if err := json.Unmarshal(f.Payload, &p); err != nil {
		return false
	}
	return p.BridgeVersion > 0 && p.EventID != ""
}

func (b *Bridge) projectedEventIDs() map[string]bool {
	out := map[string]bool{}
	for _, f := range ledger.ReadAll(b.LedgerPath) {
		if !strings.HasPrefix(f.Type, FactPrefix) {
			continue
		}
		var p struct {
			EventID string `json:"event_id"`
		}
		if json.Unmarshal(f.Payload, &p) == nil && p.EventID != "" {
			out[p.EventID] = true
		}
	}
	return out
}
