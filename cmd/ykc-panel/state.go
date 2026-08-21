// 狀態聚合：掃描專案、解析事實帳本、推導信任等級、讀收據。
package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ykc/internal/eventledger"
	"ykc/internal/eventstore"
	"ykc/internal/ledger"
)

// FactView 是展示用的一條事實（Payload 保留原始 JSON）。
type FactView struct {
	Seq     uint64          `json:"seq"`
	Type    string          `json:"type"`
	Actor   string          `json:"actor"`
	Ts      string          `json:"ts,omitempty"`
	Payload json.RawMessage `json:"payload"`
	Hash    string          `json:"hash"`
}

// VerdictView 是 claim.verdict 事實的結構化視圖。
type VerdictView struct {
	AgentID  string `json:"agent_id"`
	ClaimID  string `json:"claim_id"`
	Text     string `json:"text"`
	Verdict  string `json:"verdict"`
	Evidence string `json:"evidence"`
	Severity int    `json:"severity"`
}

// TrustEventView 是 trust.event 事實的結構化視圖。
type TrustEventView struct {
	AgentID string `json:"agent_id"`
	Kind    string `json:"kind"`
	Intent  string `json:"intent"`
	Action  string `json:"action"`
	From    int    `json:"from"`
	To      int    `json:"to"`
}

// ReceiptView 是收據（judge 或 smoke 產出）的結構化視圖。
type ReceiptView struct {
	Engine    string `json:"engine"`
	Overall   string `json:"overall"`
	Timestamp string `json:"timestamp"`
	ChainHash string `json:"chain_hash"`
	Signature string `json:"signature"`
}

// ProjectState 是單一專案的聚合狀態。
type ProjectState struct {
	Name               string           `json:"name"`
	Dir                string           `json:"dir"`
	Integrity          string           `json:"integrity"` // verified | tampered | empty | error
	ChainHead          string           `json:"chain_head"`
	FactCount          int              `json:"fact_count"`
	EventCount         int              `json:"event_count"`
	ProjectedEvents    int              `json:"projected_events"`
	MissingProjections int              `json:"missing_projections"`
	LastFact           *FactView        `json:"last_fact,omitempty"`
	Receipt            *ReceiptView     `json:"receipt,omitempty"`
	Verdicts           []VerdictView    `json:"verdicts"`
	TrustEvents        []TrustEventView `json:"trust_events"`
	Agents             map[string]int   `json:"agents"` // agentID → 信任等級 0..3
	Facts              []FactView       `json:"facts"`
}

// GlobalState 是全系統聚合狀態。
type GlobalState struct {
	ServerTime         string         `json:"server_time"`
	TotalProjects      int            `json:"total_projects"`
	TotalFacts         int            `json:"total_facts"`
	TotalEvents        int            `json:"total_events"`
	MissingProjections int            `json:"missing_projections"`
	Tampered           int            `json:"tampered"`
	Projects           []ProjectState `json:"projects"`
}

// discoverProjects 找出根目錄及其一層子目錄中含 .ykc/ledger.jsonl 的專案。
func discoverProjects(root string, extra []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(d string) {
		d = filepath.Clean(d)
		if seen[d] {
			return
		}
		if _, err := os.Stat(filepath.Join(d, ".ykc", "ledger.jsonl")); err == nil {
			seen[d] = true
			out = append(out, d)
		}
	}
	for _, d := range extra {
		add(d)
	}
	add(root)
	if ents, err := os.ReadDir(root); err == nil {
		for _, e := range ents {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				add(filepath.Join(root, e.Name()))
			}
		}
	}
	sort.Strings(out)
	return out
}

func readReceipt(dir string) *ReceiptView {
	for _, name := range []string{".ykc/receipt.json", "ykc-receipt.json"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var r struct {
			Engine    string `json:"engine"`
			Overall   string `json:"overall"`
			Timestamp string `json:"timestamp"`
			ChainHash string `json:"chain_hash"`
			Signature string `json:"signature"`
		}
		if json.Unmarshal(b, &r) == nil {
			return &ReceiptView{r.Engine, r.Overall, r.Timestamp, r.ChainHash, r.Signature}
		}
	}
	return nil
}

func projectState(dir string) ProjectState {
	ps := ProjectState{Dir: dir, Name: filepath.Base(dir), Agents: map[string]int{}}
	path := filepath.Join(dir, ".ykc", "ledger.jsonl")
	facts := ledger.ReadAll(path)
	ps.FactCount = len(facts)
	if es, err := eventstore.New(filepath.Join(dir, ".ykc", "events")); err == nil {
		if events, err := es.Replay(); err == nil {
			ps.EventCount = len(events)
		}
	}

	ok, head, err := ledger.VerifyChain(path)
	switch {
	case len(facts) == 0:
		ps.Integrity = "empty"
	case err != nil:
		ps.Integrity = "error"
	case ok:
		ps.Integrity = "verified"
	default:
		ps.Integrity = "tampered"
	}
	ps.ChainHead = head

	projected := map[string]bool{}
	for _, f := range facts {
		ps.Facts = append(ps.Facts, FactView{Seq: f.Seq, Type: f.Type, Actor: f.Actor, Ts: f.Ts, Payload: f.Payload, Hash: f.Hash})
		if eventledger.IsBridgeFact(f) {
			var bp struct {
				EventID string `json:"event_id"`
			}
			if json.Unmarshal(f.Payload, &bp) == nil && bp.EventID != "" {
				projected[bp.EventID] = true
			}
		}
		switch f.Type {
		case "claim.verdict":
			var p struct {
				AgentID  string `json:"agent_id"`
				ClaimID  string `json:"claim_id"`
				Text     string `json:"text"`
				Verdict  string `json:"verdict"`
				Evidence string `json:"evidence"`
				Severity int    `json:"severity"`
			}
			if json.Unmarshal(f.Payload, &p) == nil {
				ps.Verdicts = append(ps.Verdicts, VerdictView{p.AgentID, p.ClaimID, p.Text, p.Verdict, p.Evidence, p.Severity})
			}
		case "trust.event":
			var p struct {
				AgentID string `json:"agent_id"`
				Kind    string `json:"kind"`
				Intent  string `json:"intent"`
				Action  string `json:"action"`
				From    int    `json:"from"`
				To      int    `json:"to"`
			}
			if json.Unmarshal(f.Payload, &p) == nil {
				ps.TrustEvents = append(ps.TrustEvents, TrustEventView{p.AgentID, p.Kind, p.Intent, p.Action, p.From, p.To})
				ps.Agents[p.AgentID] = p.To
			}
		case "trust.reset":
			var p struct {
				AgentID string `json:"agent_id"`
				To      int    `json:"to"`
			}
			if json.Unmarshal(f.Payload, &p) == nil {
				ps.Agents[p.AgentID] = p.To
			}
		}
	}
	sort.Slice(ps.Facts, func(i, j int) bool { return ps.Facts[i].Seq > ps.Facts[j].Seq })
	if len(ps.Facts) > 0 {
		ps.LastFact = &ps.Facts[0]
	}
	ps.ProjectedEvents = len(projected)
	if ps.EventCount > ps.ProjectedEvents {
		ps.MissingProjections = ps.EventCount - ps.ProjectedEvents
	}
	ps.Receipt = readReceipt(dir)
	return ps
}

func collectState(root string, extra []string) GlobalState {
	gs := GlobalState{ServerTime: time.Now().UTC().Format(time.RFC3339), Projects: []ProjectState{}}
	for _, d := range discoverProjects(root, extra) {
		ps := projectState(d)
		gs.Projects = append(gs.Projects, ps)
		gs.TotalFacts += ps.FactCount
		gs.TotalEvents += ps.EventCount
		gs.MissingProjections += ps.MissingProjections
		gs.TotalProjects++
		if ps.Integrity == "tampered" {
			gs.Tampered++
		}
	}
	return gs
}

// discoverCargoProjects 找出可作為「運行目標」的專案（含 Cargo.toml）。
func discoverCargoProjects(root string, extra []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(d string) {
		d = filepath.Clean(d)
		if seen[d] {
			return
		}
		if _, err := os.Stat(filepath.Join(d, "Cargo.toml")); err == nil {
			seen[d] = true
			out = append(out, d)
		}
	}
	for _, d := range extra {
		add(d)
	}
	add(root)
	if ents, err := os.ReadDir(root); err == nil {
		for _, e := range ents {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				add(filepath.Join(root, e.Name()))
			}
		}
	}
	sort.Strings(out)
	return out
}

func rawHandler(root string, extra []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dir := r.URL.Query().Get("project")
		if dir == "" {
			http.Error(w, "缺少 ?project= 參數", http.StatusBadRequest)
			return
		}
		resolved := ""
		for _, d := range discoverProjects(root, extra) {
			if d == dir || filepath.Base(d) == dir {
				resolved = d
				break
			}
		}
		if resolved == "" {
			http.Error(w, "找不到專案", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		facts := ledger.ReadAll(filepath.Join(resolved, ".ykc", "ledger.jsonl"))
		out := map[string]any{"dir": resolved, "facts": facts}
		if rc := readReceipt(resolved); rc != nil {
			out["receipt"] = rc
		}
		_ = json.NewEncoder(w).Encode(out)
	}
}
