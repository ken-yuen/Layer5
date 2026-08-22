package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain：若被 Manager 以子行程身份啟動（YKC_FAKE_LSP_SERVER=1），
// 本測試二進制轉身變成一個「假 rust-analyzer」stdio LSP server——
// 零 rust-analyzer 依賴即可測 framing / 握手 / 診斷 / 看門狗。
func TestMain(m *testing.M) {
	if os.Getenv("YKC_FAKE_LSP_SERVER") == "1" {
		fakeLSPServer()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// fakeLSPServer 講最小 LSP：initialize → 回應；didOpen → 依內容推診斷。
// YKC_FAKE_LSP_CRASH=1 時 initialize 回應後立即退出（看門狗場景）。
func fakeLSPServer() {
	in := bufio.NewReader(os.Stdin)
	for {
		raw, err := ReadMessage(in)
		if err != nil {
			return
		}
		var m Msg
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		switch m.Method {
		case "initialize":
			_ = Send(os.Stdout, Msg{ID: m.ID, Result: json.RawMessage(`{"capabilities":{}}`)})
			if os.Getenv("YKC_FAKE_LSP_CRASH") == "1" {
				return // 模擬 r-a 崩潰
			}
		case "initialized":
			// 通知，無回應
		case "textDocument/didOpen":
			var p struct {
				TextDocument struct {
					URI  string `json:"uri"`
					Text string `json:"text"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(m.Params, &p)
			diags := `[]`
			if strings.Contains(p.TextDocument.Text, "v.push") {
				diags = `[{"range":{"start":{"line":3,"character":4},"end":{"line":3,"character":14}},` +
					`"severity":1,"code":"E0502",` +
					`"message":"cannot borrow \u0060v\u0060 as mutable because it is also borrowed as immutable"}]`
			}
			params, _ := json.Marshal(json.RawMessage(
				`{"uri":"` + p.TextDocument.URI + `","diagnostics":` + diags + `}`))
			_ = Send(os.Stdout, Msg{Method: "textDocument/publishDiagnostics", Params: params})
		}
	}
}

func TestFramingRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	msg := Msg{ID: json.RawMessage("7"), Method: "initialize", Params: json.RawMessage(`{"a":1}`)}
	if err := Send(&buf, msg); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "Content-Length: ") {
		t.Fatalf("缺 Content-Length 框定: %q", buf.String())
	}
	raw, err := ReadMessage(bufio.NewReader(&buf))
	if err != nil {
		t.Fatal(err)
	}
	var got Msg
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Method != "initialize" || string(got.ID) != "7" || got.JSONRPC != "2.0" {
		t.Fatalf("round-trip 失真: %+v", got)
	}
}

func TestReadMessageRejectsMissingLength(t *testing.T) {
	if _, err := ReadMessage(bufio.NewReader(strings.NewReader("\r\n\r\n"))); err == nil {
		t.Fatal("無 Content-Length 應報錯")
	}
}

// writeRustProject 建立含 Cargo.toml 的假專案與 .rs 檔。
func writeRustProject(t *testing.T, dirty bool) (root, file string) {
	t.Helper()
	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[package]\nname=\"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "fn main() {}\n"
	if dirty {
		src = "fn main() {\n    let mut v = vec![1];\n    let first = &v[0];\n    v.push(2);\n    println!(\"{}\", first);\n}\n"
	}
	file = filepath.Join(root, "main.rs")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, file
}

func newFakeManager(t *testing.T) *Manager {
	t.Helper()
	t.Setenv("YKC_FAKE_LSP_SERVER", "1")
	m := NewManager(os.Args[0]) // 測試二進制自身 = 假 LSP server
	m.InitTimeout = 10 * time.Second
	m.DiagTimeout = 10 * time.Second
	m.QuietWindow = 150 * time.Millisecond // 假 server 單次推送，小窗即可
	t.Cleanup(m.Close)
	return m
}

func TestManagerDiagnosticsDirtyFile(t *testing.T) {
	m := newFakeManager(t)
	_, file := writeRustProject(t, true)
	diags, err := m.Diagnostics(context.Background(), file)
	if err != nil {
		t.Fatalf("diagnostics: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("diags = %d, want 1", len(diags))
	}
	d := diags[0]
	if d.Code != "E0502" || d.Severity != 1 {
		t.Fatalf("診斷內容: %+v", d)
	}
	if d.Line != 4 || d.Col != 5 {
		t.Fatalf("LSP 0-based → 1-based 轉換錯: line=%d col=%d", d.Line, d.Col)
	}
}

func TestManagerDiagnosticsCleanFileAndSessionReuse(t *testing.T) {
	m := newFakeManager(t)
	_, file := writeRustProject(t, false)
	diags, err := m.Diagnostics(context.Background(), file)
	if err != nil {
		t.Fatalf("diagnostics: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("乾淨檔應零診斷: %+v", diags)
	}
	// 第二次呼叫必須復用同一 session（initialize 只一次）
	if _, err := m.Diagnostics(context.Background(), file); err != nil {
		t.Fatalf("session 復用失敗: %v", err)
	}
	if m.Restarts != 0 {
		t.Fatalf("不應有重啟: %d", m.Restarts)
	}
}

func TestManagerWatchdogRestartsCrashedServer(t *testing.T) {
	m := newFakeManager(t)
	_, file := writeRustProject(t, true)

	t.Setenv("YKC_FAKE_LSP_CRASH", "1") // server 於 initialize 後崩潰
	if _, err := m.Diagnostics(context.Background(), file); err == nil {
		t.Fatal("崩潰 server 應回錯誤")
	}
	if m.Restarts != 1 {
		t.Fatalf("看門狗應重啟 1 次，實際 %d", m.Restarts)
	}

	t.Setenv("YKC_FAKE_LSP_CRASH", "") // 復原後自動重建 session
	diags, err := m.Diagnostics(context.Background(), file)
	if err != nil {
		t.Fatalf("重啟後應恢復: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("重啟後診斷: %+v", diags)
	}
}

func TestManagerAvailableHonest(t *testing.T) {
	m := NewManager("definitely-not-a-real-lsp-server-binary")
	if ok, why := m.Available(); ok || why == "" {
		t.Fatal("缺席 server 必須如實申報")
	}
	if _, err := m.Diagnostics(context.Background(), "/tmp/x.rs"); err == nil {
		t.Fatal("缺席 server 的 Diagnostics 應報錯供降級")
	}
}
