package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ykc/internal/ledger"
	"ykc/internal/watch"
)

// makeProject 建一個含 Cargo.toml 的臨時專案並回傳其目錄。
func makeProject(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \""+name+"\"\nversion = \"0.1.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "main.rs"), []byte("fn main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func newTestServer(t *testing.T, root string, mutate func(*Config)) *Server {
	t.Helper()
	cfg := Config{Root: root, BinDir: filepath.Join(root, "bin"), Depth: 1, Debounce: 50 * time.Millisecond}
	if mutate != nil {
		mutate(&cfg)
	}
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestDiscoverProjects：Cargo 專案發現 + 排序。
func TestDiscoverProjects(t *testing.T) {
	root := t.TempDir()
	a := makeProject(t, root, "proj-a")
	makeProject(t, root, "proj-b")
	os.MkdirAll(filepath.Join(root, "not-a-project"), 0o755)
	s := newTestServer(t, root, nil)
	// Handler() 需先初始化 jm；discover 不依賴它
	got := s.discoverProjects()
	if len(got) != 2 {
		t.Fatalf("expected 2 projects, got %v", got)
	}
	if got[0] != a && got[1] != a {
		t.Fatalf("proj-a missing: %v", got)
	}
}

// TestHandleBatchWritesFileChangeEvent：去抖批次 → 帳本 file.change 投影 + 事件庫。
func TestHandleBatchWritesFileChangeEvent(t *testing.T) {
	root := t.TempDir()
	proj := makeProject(t, root, "demo")
	s := newTestServer(t, root, nil)
	s.mu.Lock()
	s.projects = []string{proj}
	s.mu.Unlock()

	batch := []watch.Event{
		{Op: watch.OpWrite, Path: filepath.Join(proj, "src", "main.rs")},
		{Op: watch.OpCreate, Path: filepath.Join(proj, "Cargo.lock")},
	}
	s.handleBatch(context.Background(), batch)

	ledPath := filepath.Join(proj, ".ykc", "ledger.jsonl")
	if _, err := os.Stat(ledPath); err != nil {
		t.Fatalf("ledger not created: %v", err)
	}
	facts := ledger.ReadAll(ledPath)
	var sawFileChange bool
	for _, f := range facts {
		if f.Type == "event.file.change" {
			sawFileChange = true
			var p struct {
				Envelope struct {
					Payload struct {
						Files []struct {
							Path string `json:"path"`
							Op   string `json:"op"`
						} `json:"files"`
					} `json:"payload"`
				} `json:"envelope"`
			}
			if err := json.Unmarshal(f.Payload, &p); err != nil {
				t.Fatalf("decode bridge payload: %v", err)
			}
			if len(p.Envelope.Payload.Files) != 2 {
				t.Fatalf("expected 2 files in batch, got %v", p.Envelope.Payload.Files)
			}
			if p.Envelope.Payload.Files[0].Path != "Cargo.lock" {
				t.Fatalf("files should be path-sorted: %v", p.Envelope.Payload.Files)
			}
		}
	}
	if !sawFileChange {
		t.Fatal("expected event.file.change fact in ledger")
	}
	// 事件庫可重放
	b, err := s.bridgeOf(proj)
	if err != nil {
		t.Fatal(err)
	}
	events, err := b.ReplayEvents()
	if err != nil || len(events) != 1 {
		t.Fatalf("expected 1 event in store, got %d (err=%v)", len(events), err)
	}
}

// TestClaimsEndpointDatalogLoop：POST /api/claims（無證據 tests_passed）
// → datalog 護欄駁回 → 帳本雙事件 → enforcement 落盤。
func TestClaimsEndpointDatalogLoop(t *testing.T) {
	root := t.TempDir()
	proj := makeProject(t, root, "demo")
	s := newTestServer(t, root, nil)
	s.mu.Lock()
	s.projects = []string{proj}
	s.mu.Unlock()

	// 專案需在 Cargo 發現白名單內（serve 用 panel 白名單驗證 claims）
	_ = s.jm // Handler() 內建 jm；直接用 Handler
	h := s.Handler()
	srv := httptest.NewServer(h)
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{"project": proj, "kind": "tests_passed", "text": "tests pass"})
	resp, err := http.Post(srv.URL+"/api/claims", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("claims status = %d", resp.StatusCode)
	}
	var out struct {
		Decision struct {
			Mode        string `json:"mode"`
			BlockWrites bool   `json:"block_writes"`
			Violations  []struct {
				Code string `json:"code"`
			} `json:"violations"`
		} `json:"decision"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.Decision.BlockWrites || out.Decision.Mode != "evidence_only_smoke_takeover" {
		t.Fatalf("expected smoke takeover decision, got %+v", out.Decision)
	}
	if len(out.Decision.Violations) != 1 || out.Decision.Violations[0].Code != "fake_test_claim" {
		t.Fatalf("expected fake_test_claim, got %+v", out.Decision.Violations)
	}
	// enforcement 落盤
	blockPath := filepath.Join(proj, ".ykc", "enforcement", "AGENT_WRITES_BLOCKED")
	if _, err := os.Stat(blockPath); err != nil {
		t.Fatal("AGENT_WRITES_BLOCKED not written")
	}
	// 帳本：agent.claim + guardrail.decision
	facts := ledger.ReadAll(filepath.Join(proj, ".ykc", "ledger.jsonl"))
	kinds := map[string]int{}
	for _, f := range facts {
		kinds[f.Type]++
	}
	if kinds["event.agent.claim"] != 1 || kinds["event.guardrail.decision"] != 1 {
		t.Fatalf("expected claim+decision facts, got %v", kinds)
	}
	// 帳本鏈完整
	if ok, _, err := ledger.VerifyChain(filepath.Join(proj, ".ykc", "ledger.jsonl")); err != nil || !ok {
		t.Fatalf("ledger chain broken: ok=%v err=%v", ok, err)
	}
}

// TestClaimsHonorsFreshEvidence：先注入成功 test 事件 → 聲明通過。
func TestClaimsHonorsFreshEvidence(t *testing.T) {
	root := t.TempDir()
	proj := makeProject(t, root, "demo")
	s := newTestServer(t, root, nil)
	s.mu.Lock()
	s.projects = []string{proj}
	s.mu.Unlock()

	// 注入新鮮成功證據（直接經事件庫 append —— serve 的 evaluateClaim 重放歷史）
	if _, err := s.bridgeOf(proj); err != nil {
		t.Fatal(err)
	}
	// epoch 由 snapshot 事件確立
	snap := map[string]string{"digest": "d1"}
	if err := s.appendProjectEvent(proj, "workspace.snapshot", snap); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	res := map[string]any{
		"command_id": "t1", "class": "test", "name": "cargo",
		"started_at": now, "finished_at": now, "exit_code": 0,
	}
	if err := s.appendProjectEvent(proj, "command.result", res); err != nil {
		t.Fatal(err)
	}

	h := s.Handler()
	srv := httptest.NewServer(h)
	defer srv.Close()
	body, _ := json.Marshal(map[string]any{"project": proj, "kind": "tests_passed"})
	resp, err := http.Post(srv.URL+"/api/claims", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Decision struct {
			BlockWrites bool `json:"block_writes"`
		} `json:"decision"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.Decision.BlockWrites {
		t.Fatal("fresh successful evidence should allow the claim")
	}
}

// TestWatchStateEndpoint：/api/watch 狀態視圖 + /api/rules 透明度。
func TestWatchAndRulesEndpoints(t *testing.T) {
	root := t.TempDir()
	proj := makeProject(t, root, "demo")
	s := newTestServer(t, root, nil)
	s.mu.Lock()
	s.projects = []string{proj}
	s.lastBatches[proj] = []watch.Event{{Op: watch.OpWrite, Path: filepath.Join(proj, "src", "main.rs")}}
	s.mu.Unlock()

	h := s.Handler()
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/watch")
	if err != nil {
		t.Fatal(err)
	}
	var wv struct {
		DebounceMS int `json:"debounce_ms"`
		Projects   []struct {
			Dir       string `json:"dir"`
			LastBatch []struct {
				Op   string `json:"op"`
				Path string `json:"path"`
			} `json:"last_batch"`
		} `json:"projects"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&wv)
	resp.Body.Close()
	if wv.DebounceMS != 50 || len(wv.Projects) != 1 || len(wv.Projects[0].LastBatch) != 1 {
		t.Fatalf("watch view mismatch: %+v", wv)
	}

	resp2, err := http.Get(srv.URL + "/api/rules")
	if err != nil {
		t.Fatal(err)
	}
	var rv struct {
		DefaultRules string `json:"default_rules"`
		ExtraCount   int    `json:"extra_count"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&rv)
	resp2.Body.Close()
	if !strings.Contains(rv.DefaultRules, "fake_test_claim") || rv.ExtraCount != 0 {
		t.Fatalf("rules view mismatch: %d extra, has fake_test_claim=%v", rv.ExtraCount, strings.Contains(rv.DefaultRules, "fake_test_claim"))
	}
}

// TestAutoJudgeSingleFlight：.rs 批次觸發 judge 任務請求；無二進制時不 panic、不重疊。
func TestAutoJudgeSingleFlight(t *testing.T) {
	root := t.TempDir()
	proj := makeProject(t, root, "demo")
	s := newTestServer(t, root, func(c *Config) { c.AutoJudge = true })
	s.mu.Lock()
	s.projects = []string{proj}
	s.mu.Unlock()
	_ = s.Handler() // 初始化 jm

	s.handleBatch(context.Background(), []watch.Event{{Op: watch.OpWrite, Path: filepath.Join(proj, "src", "main.rs")}})
	s.handleBatch(context.Background(), []watch.Event{{Op: watch.OpWrite, Path: filepath.Join(proj, "src", "main.rs")}})
	// 無 bin/ykc-judge → Start 報錯被記錄；關鍵是不 panic、批次照樣入帳本
	if _, err := os.Stat(filepath.Join(proj, ".ykc", "ledger.jsonl")); err != nil {
		t.Fatalf("ledger should still be written: %v", err)
	}
}

// TestProjectOfLongestPrefix：事件路徑歸屬（最長前綴勝）。
func TestProjectOfLongestPrefix(t *testing.T) {
	root := t.TempDir()
	a := makeProject(t, root, "a")
	nested := makeProject(t, a, "nested")
	s := newTestServer(t, root, nil)
	s.mu.Lock()
	s.projects = []string{a, nested}
	s.mu.Unlock()
	if got := s.projectOf(filepath.Join(nested, "src", "main.rs")); got != nested {
		t.Fatalf("nested project should win, got %s", got)
	}
	if got := s.projectOf(filepath.Join(a, "Cargo.toml")); got != a {
		t.Fatalf("outer project expected, got %s", got)
	}
	if got := s.projectOf("/elsewhere/x.rs"); got != "" {
		t.Fatalf("unknown path should map to empty, got %s", got)
	}
}
