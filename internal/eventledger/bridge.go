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

// Append 提交事件並投影到 hash 鏈帳本。
// 去重檢查在取得帳本寫鎖（flock）**之後**執行——消除「兩程序同時判定
// 未投影 → 重複投影」的 TOCTOU 窗口。
func (b *Bridge) Append(e domain.Envelope) (AppendResult, error) {
	if b == nil || b.Events == nil {
		return AppendResult{}, errors.New("nil event ledger bridge")
	}
	committed, err := b.Events.Append(e) // store 會為空 ID 自動產生
	if err != nil {
		return AppendResult{}, err
	}
	if committed.ID == "" {
		return AppendResult{}, errors.New("cannot project event without id")
	}
	led, err := ledger.Open(b.LedgerPath)
	if err != nil {
		return AppendResult{Event: committed}, err
	}
	defer led.Close()
	if hasProjectedID(ledger.ReadAll(b.LedgerPath), committed.ID) {
		ok, head, _, verr := ledger.VerifyAnchored(b.LedgerPath)
		if verr != nil {
			return AppendResult{Event: committed}, fmt.Errorf("ledger verification before duplicate projection: %w", verr)
		}
		if !ok {
			return AppendResult{Event: committed}, errors.New("ledger verification before duplicate projection failed")
		}
		return AppendResult{Event: committed, LedgerHash: head, AlreadySeen: true}, nil
	}
	seq, err := led.Append(FactType(committed.Kind), b.Actor, NewBridgePayload(committed))
	if err != nil {
		return AppendResult{Event: committed}, err
	}
	return AppendResult{Event: committed, LedgerSeq: seq, LedgerHash: led.Head()}, nil
}

func (b *Bridge) ReplayEvents() ([]domain.Envelope, error) {
	if b == nil || b.Events == nil {
		return nil, errors.New("nil event ledger bridge")
	}
	return b.Events.Replay()
}

// SyncMissing 把「已提交但未投影」的事件批量補進帳本。
// 效能修正（D3）：舊版對每個事件各做一次全量 ReadAll + 全量 Open（O(n²)）；
// 現在只讀一次建索引、只開一次帳本（持鎖批量 append），整體 O(n)。
func (b *Bridge) SyncMissing() (SyncResult, error) {
	if b == nil || b.Events == nil {
		return SyncResult{}, errors.New("nil event ledger bridge")
	}
	events, err := b.Events.Replay()
	if err != nil {
		return SyncResult{}, err
	}
	res := SyncResult{EventsSeen: len(events)}
	led, err := ledger.Open(b.LedgerPath)
	if err != nil {
		return res, err
	}
	defer led.Close()
	seen := projectedIDs(ledger.ReadAll(b.LedgerPath))
	for _, e := range events {
		if seen[e.ID] {
			res.AlreadyProjected++
			continue
		}
		if _, err := led.Append(FactType(e.Kind), b.Actor, NewBridgePayload(e)); err != nil {
			res.MissingFailed = append(res.MissingFailed, e.ID+": "+err.Error())
			continue
		}
		seen[e.ID] = true
		res.Projected++
	}
	if ok, head, _, verr := ledger.VerifyAnchored(b.LedgerPath); verr == nil && ok {
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
	ok, head, _, err := ledger.VerifyAnchored(b.LedgerPath)
	return ok, head, err
}

// Close 釋放橋接持有的資源（目前帳本採「開-寫-關」每次 Append 自含，
// 此方法為呼叫端 defer 語意保留；可安全重複呼叫）。
func (b *Bridge) Close() error {
	return nil
}

// hasProjectedID：線性檢查（單事件 append 路徑用；批量路徑用 projectedIDs 索引）。
func hasProjectedID(facts []ledger.Fact, eventID string) bool {
	for _, f := range facts {
		if !strings.HasPrefix(f.Type, FactPrefix) {
			continue
		}
		var p struct {
			EventID string `json:"event_id"`
		}
		if json.Unmarshal(f.Payload, &p) == nil && p.EventID == eventID {
			return true
		}
	}
	return false
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

// projectedIDs：一次讀全表、建已投影事件 id 索引（批量補償路徑用）。
func projectedIDs(facts []ledger.Fact) map[string]bool {
	out := make(map[string]bool, len(facts))
	for _, f := range facts {
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
