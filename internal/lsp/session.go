package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// publishDiagnosticsParams 是 textDocument/publishDiagnostics 的最小投影。
type publishDiagnosticsParams struct {
	URI         string `json:"uri"`
	Diagnostics []struct {
		Range struct {
			Start struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"start"`
		} `json:"range"`
		Severity int             `json:"severity"`
		Code     json.RawMessage `json:"code"`
		Message  string          `json:"message"`
	} `json:"diagnostics"`
}

// Diagnostic 是回傳給呼叫方的診斷（1-based 行列，對齊 rustc 習慣）。
type Diagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Severity int    `json:"severity"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message"`
}

// Session 是一個常駐 LSP server 行程（每個專案 root 一個）。
// initialize 一次；之後 didOpen/didChange 增量取診斷。非執行緒安全，由 Manager 串行化。
type Session struct {
	ServerCmd string   // 預設 rust-analyzer
	Args      []string // 額外引數
	Root      string   // 專案根（rootUri）
	// Quiet 是「空集後等待後續推送」的寬限窗；零值 = 自適應
	//（冷 session 10s、熱 session 2s——r-a flycheck 首輪常需 3–10 秒）。
	Quiet time.Duration

	cmd     *exec.Cmd
	stdin   io.WriteCloser
	out     *bufio.Reader
	msgs    chan []byte   // 常駐 reader goroutine 推入（跨呼叫零丟失）
	exited  chan struct{} // cmd.Wait 完成即關閉（Alive 判準）
	nextID  int64
	started time.Time
	version int
}

// Start 啟動 server 行程並完成 initialize / initialized 握手。
func (s *Session) Start(ctx context.Context) error {
	if s.ServerCmd == "" {
		s.ServerCmd = "rust-analyzer"
	}
	cmd := exec.Command(s.ServerCmd, s.Args...)
	cmd.Dir = s.Root
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("啟動 %s 失敗: %w", s.ServerCmd, err)
	}
	s.cmd, s.stdin, s.out = cmd, stdin, bufio.NewReader(stdout)
	s.started = time.Now()
	s.exited = make(chan struct{})
	s.msgs = make(chan []byte, 64)
	go func() {
		_ = cmd.Wait()
		close(s.exited)
	}()
	// 常駐 reader：唯一從 stdout 讀訊息的 goroutine，杜絕競讀丟失。
	go func() {
		for {
			raw, err := ReadMessage(s.out)
			if err != nil {
				close(s.msgs)
				return
			}
			s.msgs <- raw
		}
	}()

	initParams, _ := json.Marshal(map[string]any{
		"processId": nil,
		"rootUri":   "file://" + s.Root,
		"capabilities": map[string]any{
			"textDocument": map[string]any{"publishDiagnostics": map[string]any{}},
		},
	})
	id := s.reqID()
	if err := Send(s.stdin, Msg{ID: id, Method: "initialize", Params: initParams}); err != nil {
		s.Kill()
		return err
	}
	// 等 initialize 回應（尊重 ctx deadline）
	if _, err := s.waitResponse(ctx, string(id)); err != nil {
		s.Kill()
		return fmt.Errorf("initialize 未完成: %w", err)
	}
	if err := Send(s.stdin, Msg{Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		s.Kill()
		return err
	}
	return nil
}

// Alive 回報行程是否仍在。
func (s *Session) Alive() bool {
	if s.cmd == nil || s.exited == nil {
		return false
	}
	select {
	case <-s.exited:
		return false
	default:
		return true
	}
}

// Age 回報 session 存活時長（Manager 的最大壽命重啟用）。
func (s *Session) Age() time.Duration { return time.Since(s.started) }

// AliveAfter 給行程回收一個寬限窗再判存活（EOF 常比 Wait 完成先到）。
func (s *Session) AliveAfter(grace time.Duration) bool {
	if s.exited == nil {
		return false
	}
	select {
	case <-s.exited:
		return false
	case <-time.After(grace):
		return s.Alive()
	}
}

// Kill 終止行程（Wait 由 Start 的 goroutine 負責回收）。
func (s *Session) Kill() {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	if s.exited != nil {
		select {
		case <-s.exited:
		case <-time.After(3 * time.Second):
		}
	}
}

// Diagnostics 對單一檔案 didOpen 並等待診斷：
//   - 收到非空集即返（可行動訊號，低延遲優先）；
//   - 收到空集不立即信——r-a 常先推初始空集、flycheck 完成後才推真診斷，
//     故在寬限窗（quiet）內續等後續推送，窗內無更新才確認「乾淨」。
//
// 完成後 didClose，讓 session 可對同檔重複查詢。
func (s *Session) Diagnostics(ctx context.Context, absFile string) ([]Diagnostic, error) {
	content, err := os.ReadFile(absFile)
	if err != nil {
		return nil, err
	}
	s.version++
	dp, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{
			"uri":        "file://" + absFile,
			"languageId": "rust",
			"version":    s.version,
			"text":       string(content),
		},
	})
	if err := Send(s.stdin, Msg{Method: "textDocument/didOpen", Params: dp}); err != nil {
		return nil, fmt.Errorf("didOpen 送出失敗: %w", err)
	}
	defer func() {
		cp, _ := json.Marshal(map[string]any{
			"textDocument": map[string]any{"uri": "file://" + absFile},
		})
		_ = Send(s.stdin, Msg{Method: "textDocument/didClose", Params: cp})
	}()

	want := "file://" + absFile
	var last []Diagnostic
	seen := false
	// 空集後的寬限窗（等 flycheck 後續推送）：冷 session 首查時 cargo check
	// 尚未跑完（常見 3–10 秒），窗自適應加長；熱 session 維持低延遲。
	quiet := s.Quiet
	if quiet <= 0 {
		quiet = 2 * time.Second
		if s.Age() < 30*time.Second {
			quiet = 10 * time.Second
		}
	}

	for {
		rctx := ctx
		var cancel context.CancelFunc
		if seen {
			rctx, cancel = context.WithTimeout(ctx, quiet)
		}
		raw, err := s.readWithDeadline(rctx)
		if cancel != nil {
			cancel()
		}
		if err != nil {
			// 已收到空集後的任何 deadline（寬限窗或整體上限）＝靜默確認空集；
			// EOF / 取消一律上拋（觸發看門狗）。
			if seen && errors.Is(err, context.DeadlineExceeded) {
				return last, nil
			}
			return nil, err
		}
		var m Msg
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		if m.Method != "textDocument/publishDiagnostics" {
			continue
		}
		var p publishDiagnosticsParams
		if json.Unmarshal(m.Params, &p) != nil || p.URI != want {
			continue
		}
		out := make([]Diagnostic, 0, len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			out = append(out, Diagnostic{
				File:     absFile,
				Line:     d.Range.Start.Line + 1,
				Col:      d.Range.Start.Character + 1,
				Severity: d.Severity,
				Code:     codeString(d.Code),
				Message:  d.Message,
			})
		}
		if len(out) > 0 {
			return out, nil // 非空 = 可行動訊號，立即返回
		}
		last, seen = out, true // 空集：記下並續等寬限窗
	}
}

// waitResponse 阻塞等待指定 id 的回應。
func (s *Session) waitResponse(ctx context.Context, id string) (json.RawMessage, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		raw, err := s.readWithDeadline(ctx)
		if err != nil {
			return nil, err
		}
		var m Msg
		if json.Unmarshal(raw, &m) == nil && string(m.ID) == id {
			return m.Result, nil
		}
	}
}

// readWithDeadline 自常駐 reader 通道取一則訊息，尊重 ctx 取消。
func (s *Session) readWithDeadline(ctx context.Context) ([]byte, error) {
	select {
	case raw, ok := <-s.msgs:
		if !ok {
			return nil, io.EOF
		}
		return raw, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Session) reqID() json.RawMessage {
	s.nextID++
	return json.RawMessage(strconv.FormatInt(s.nextID, 10))
}

func codeString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var str string
	if json.Unmarshal(raw, &str) == nil {
		return str
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return strings.Trim(string(raw), `"`)
}

// Manager 是常駐 session 池 + 看門狗（core.DiagnosticsProvider 的實作）。
//   - 每個專案 root 一個 session，惰性啟動；
//   - 連續失敗 ≥ MaxFailures 或壽命 > MaxAge → 殺掉重啟（事件由呼叫方記帳本）；
//   - 串行化對單一 session 的存取（r-a stdio 無多路複用）。
type Manager struct {
	ServerCmd   string
	InitTimeout time.Duration // initialize 上限（零值 30s）
	DiagTimeout time.Duration // 單次診斷上限（零值 15s）
	MaxFailures int           // 連續失敗即重啟（零值 3）
	MaxAge      time.Duration // session 最大壽命（零值 2h）
	QuietWindow time.Duration // 傳給 Session.Quiet（零值 = 自適應；測試注入小窗）

	mu       sync.Mutex
	sessions map[string]*Session
	failures map[string]int
	Restarts int // 看門狗重啟計數（觀測用）
}

// NewManager 建立 session 池。
func NewManager(serverCmd string) *Manager {
	return &Manager{
		ServerCmd: serverCmd,
		sessions:  map[string]*Session{},
		failures:  map[string]int{},
	}
}

// Available 如實申報 server 是否可啟動（borrow.Analyzer 範式）。
func (m *Manager) Available() (bool, string) {
	name := m.ServerCmd
	if name == "" {
		name = "rust-analyzer"
	}
	if _, err := exec.LookPath(name); err != nil {
		return false, name + " 不在 PATH（rustup component add rust-analyzer 或載入 ykc-pack-rust-toolchain）"
	}
	return true, ""
}

// Diagnostics 取單檔診斷；session 惰性啟動，失敗觸發看門狗。
func (m *Manager) Diagnostics(ctx context.Context, absFile string) ([]Diagnostic, error) {
	if ok, why := m.Available(); !ok {
		return nil, fmt.Errorf("%s", why)
	}
	root := rootOf(absFile)

	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.sessionLocked(ctx, root)
	if err != nil {
		return nil, err
	}
	dctx, cancel := context.WithTimeout(ctx, m.diagTimeout())
	defer cancel()
	diags, err := s.Diagnostics(dctx, absFile)
	if err != nil {
		m.failures[root]++
		if m.failures[root] >= m.maxFailures() || !s.AliveAfter(200*time.Millisecond) {
			m.restartLocked(root)
		}
		return nil, err
	}
	m.failures[root] = 0
	return diags, nil
}

// sessionLocked 取得（或建立）root 的 session；過壽命則先重啟。
func (m *Manager) sessionLocked(ctx context.Context, root string) (*Session, error) {
	if s, ok := m.sessions[root]; ok {
		if s.Alive() && s.Age() < m.maxAge() {
			return s, nil
		}
		m.restartLocked(root)
	}
	s := &Session{ServerCmd: m.ServerCmd, Root: root, Quiet: m.QuietWindow}
	ictx, cancel := context.WithTimeout(ctx, m.initTimeout())
	defer cancel()
	if err := s.Start(ictx); err != nil {
		return nil, err
	}
	m.sessions[root] = s
	m.failures[root] = 0
	return s, nil
}

// restartLocked 殺掉並移除 session（下次呼叫惰性重建）。
func (m *Manager) restartLocked(root string) {
	if s, ok := m.sessions[root]; ok {
		s.Kill()
		delete(m.sessions, root)
		m.Restarts++
	}
}

// Close 終止全部 session。
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for root, s := range m.sessions {
		s.Kill()
		delete(m.sessions, root)
	}
}

func (m *Manager) initTimeout() time.Duration {
	if m.InitTimeout > 0 {
		return m.InitTimeout
	}
	return 30 * time.Second
}

func (m *Manager) diagTimeout() time.Duration {
	if m.DiagTimeout > 0 {
		return m.DiagTimeout
	}
	return 15 * time.Second
}

func (m *Manager) maxFailures() int {
	if m.MaxFailures > 0 {
		return m.MaxFailures
	}
	return 3
}

func (m *Manager) maxAge() time.Duration {
	if m.MaxAge > 0 {
		return m.MaxAge
	}
	return 2 * time.Hour
}

// rootOf 從檔案往上找 Cargo.toml 作 session root；找不到用檔案目錄。
func rootOf(absFile string) string {
	dir := filepath.Dir(absFile)
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, "Cargo.toml")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return dir
		}
		d = parent
	}
}
