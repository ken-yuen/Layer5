// 狀態聚合：掃描專案、解析事實帳本、推導信任等級、讀收據。
package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"ykc/internal/borrow"
	"ykc/internal/claimview"
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
	// L5 借用幾何分析視圖（judge 落盤; sha256 對賬帳本 borrow.analysis 事實）
	L5 *borrow.AnalysisReport `json:"l5,omitempty"`
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

// discoverProjects 找出含 .ykc/ledger.jsonl 的專案（根 + 至多 depth 層子目錄）。
func discoverProjects(root string, extra []string, depth int) []string {
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
	walkDepth(root, depth, func(d string) {
		if strings.HasPrefix(filepath.Base(d), ".") {
			return
		}
		add(d)
	})
	sort.Strings(out)
	return out
}

// walkDepth 深度優先遍歷至多 depth 層子目錄（depth 0 = 僅根，不進子目錄）。
func walkDepth(root string, depth int, fn func(string)) {
	if depth <= 0 {
		return
	}
	ents, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range ents {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		d := filepath.Join(root, e.Name())
		fn(d)
		walkDepth(d, depth-1, fn)
	}
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

// maxL5Explain 是面板展示的 L5 解釋文本上限（完整文本以 sha256 對賬帳本）。
const maxL5Explain = 64 * 1024

// ── D7：觀察端快取 ─────────────────────────────────────────────
// 大專案下 /api/state 每 poll 全量 ReadAll+VerifyChain 是 O(n)×頻度。
// 快取鍵 = (帳本 mtime, 帳本 size, 事件檔數)：帳本 append-only，
// 三者不變 ⇒ 狀態必然不變，直接回傳快取（零重讀）。
type stateKey struct {
	ledgerModTime time.Time
	ledgerSize    int64
	eventFiles    int
	l5ModTime     time.Time // L5 report.json mtime（零值 = 不存在）
}

type stateCacheEntry struct {
	key   stateKey
	state ProjectState
}

var (
	stateCacheMu sync.Mutex
	stateCache   = map[string]stateCacheEntry{}
)

func computeStateKey(dir string) (stateKey, bool) {
	st, err := os.Stat(filepath.Join(dir, ".ykc", "ledger.jsonl"))
	if err != nil {
		return stateKey{}, false
	}
	key := stateKey{ledgerModTime: st.ModTime(), ledgerSize: st.Size(), eventFiles: countEventFiles(filepath.Join(dir, ".ykc", "events"))}
	// L5 報告獨立於帳本更新/清除（如 RemoveReport 路徑），mtime 入鍵防陳舊快取
	if l5st, err := os.Stat(filepath.Join(dir, ".ykc", "l5", "report.json")); err == nil {
		key.l5ModTime = l5st.ModTime()
	}
	return key, true
}

func countEventFiles(dir string) int {
	n := 0
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".json") {
			n++
		}
		return nil
	})
	return n
}

func projectState(dir string) ProjectState {
	key, ok := computeStateKey(dir)
	if !ok {
		return computeProjectState(dir)
	}
	stateCacheMu.Lock()
	if e, hit := stateCache[dir]; hit && e.key == key {
		stateCacheMu.Unlock()
		return e.state
	}
	stateCacheMu.Unlock()
	ps := computeProjectState(dir)
	stateCacheMu.Lock()
	stateCache[dir] = stateCacheEntry{key, ps}
	stateCacheMu.Unlock()
	return ps
}

func computeProjectState(dir string) ProjectState {
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
		// 信任事實解碼經 internal/claimview——舊（扁平）與新（bridge 信封）格式通用
		if v, ok := claimview.Parse(f); ok {
			switch v.Kind {
			case claimview.KindClaimVerdict:
				ps.Verdicts = append(ps.Verdicts, VerdictView{v.AgentID, v.ClaimID, v.Text, v.Verdict, v.Evidence, v.Severity})
			case claimview.KindTrustEvent:
				ps.TrustEvents = append(ps.TrustEvents, TrustEventView{v.AgentID, v.KindLabel, v.Intent, v.Action, v.From, v.To})
				ps.Agents[v.AgentID] = v.To
			case claimview.KindTrustReset:
				ps.Agents[v.AgentID] = v.To
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
	ps.L5 = borrow.ReadReport(dir, maxL5Explain)
	return ps
}

func collectState(root string, extra []string, depth int) GlobalState {
	gs := GlobalState{ServerTime: time.Now().UTC().Format(time.RFC3339), Projects: []ProjectState{}}
	for _, d := range discoverProjects(root, extra, depth) {
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
// discoverCargoProjects 找出可作為「運行目標」的專案（含 Cargo.toml；根 + 至多 depth 層）。
func discoverCargoProjects(root string, extra []string, depth int) []string {
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
	walkDepth(root, depth, func(d string) {
		if strings.HasPrefix(filepath.Base(d), ".") {
			return
		}
		add(d)
	})
	sort.Strings(out)
	return out
}

func rawHandler(root string, extra []string, depth int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dir := r.URL.Query().Get("project")
		if dir == "" {
			http.Error(w, "缺少 ?project= 參數", http.StatusBadRequest)
			return
		}
		resolved := ""
		for _, d := range discoverProjects(root, extra, depth) {
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
