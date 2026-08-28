// Package serve 是 YKC 的常駐進程（YKC_14：合併 atom+judge+guard+panel 的
// 「運行時監督面」為單一 ykc serve）。
//
// 職責分工（與四個既有 CLI 的關係——合併的是運行時，不是功能）：
//
//	atom   → serve 的監看事件流（fsnotify/inotify + 去抖 + 事件序列化進帳本）
//	judge  → serve 經 panel.JobManager 觸發（人工 /api/jobs 或 -auto-judge 存檔觸發）
//	precompile → serve 內建的主動 Rust 預譯（啟動時全專案一次；每個 .rs/.toml/.lock
//	             變更去抖後再跑，單一專案單飛並保留最後一次請求）
//	guard  → serve 的 /api/claims（聲明評估 = datalog 護欄 + 執行 + 帳本）
//	panel  → serve 直接內嵌 panel.BuildMuxWith（同一套端點與安全邊界）
//	（ykc-guard 的 MCP 與信任棘輪 CLI 維持獨立進程——stdio 協定不屬於 HTTP 常駐。）
//
// 單一寫者紀律：serve 與 judge 子行程共用專案的 .ykc/ledger.jsonl，
// 兩者皆以 flock 短暫持鎖；serve 遇 ErrLocked 以指數退避重試（有界）。
//
// 安全邊界（承襲 panel S1）：
//   - /api/claims 與 /api/jobs 同受 Bearer token 保護與專案白名單限制；
//   - 監看事件只上拋 .rs/.toml/.lock 且必排 .ykc（防自身寫入回環）；
//   - 預譯一律使用 auto sandbox（gVisor/runsc 或 bwrap）；無隔離能力時只落
//     unsupported 證據，不偷偷降級到 native；native 必須由操作者明示允許；
//   - run_smoke 需顯式請求（不在聲明評估後自動執行 cargo——控制權在人）。
package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ykc/internal/atomicfile"
	"ykc/internal/datalog"
	"ykc/internal/domain"
	"ykc/internal/enforcement"
	"ykc/internal/eventledger"
	"ykc/internal/guardrail"
	"ykc/internal/ledger"
	"ykc/internal/panel"
	"ykc/internal/precompile"
	"ykc/internal/sandbox"
	"ykc/internal/smoke"
	"ykc/internal/toolchain"
	"ykc/internal/watch"

	"ykc/core"
)

// Config 是 ykc serve 的配置。
type Config struct {
	Root         string        // 掃描根目錄（專案發現 + 面板觀察）
	ExtraDirs    []string      // 額外專案目錄
	Addr         string        // 監聽位址（預設 127.0.0.1）
	Port         int           // 監聽埠（預設 8080）
	BinDir       string        // ykc 二進制目錄（judge 等 jobs 用）
	Token        string        // 控制端點 Bearer token
	Depth        int           // 專案發現深度
	Debounce     time.Duration // 去抖窗口（零值 300ms）
	PollInterval time.Duration // 輪詢後端間隔（零值 250ms）
	RulesPath    string        // 附加 datalog 規則（.dl 檔或目錄；附加不取代）
	AutoJudge    bool          // .rs 變更批次後自動觸發 judge 任務（需 BinDir 有 ykc-judge）
	// NoAutoPrecompile 是唯一的明示退出開關。零值代表主動預譯開啟：
	// serve 啟動時先掃每個專案，之後每個 .rs/.toml/.lock 批次都會重跑。
	NoAutoPrecompile bool
	// 預譯預設選擇真正的 sandbox（auto → runsc/bwrap）；不會自動 native。
	PrecompileBackend     sandbox.Backend
	PrecompileAllowNative bool          // 僅可信本地專案的明示例外
	PrecompileTimeout     time.Duration // 零值 3 分鐘/每 stage
	PrecompileImage       string        // container backend image，零值 rust:1.98
	// PrecompileRun 可注入回放/測試執行器；零值使用 internal/precompile.Run。
	PrecompileRun func(context.Context, precompile.Options) (precompile.Report, error)
	Actor         string // 帳本 actor（零值 ykc-serve）
	StateDir      string // 專案內狀態目錄名（零值 .ykc）
	// ToolchainPolicy（T-21c）：strict = 工具鏈缺席/失配拒絕啟動；
	// degrade（零值預設）= 照常啟動並在帳本/日誌標記降級。
	ToolchainPolicy string
	// Toolchain 可注入（測試用 replay/unavailable）；零值 = native。
	Toolchain core.RustToolchain
}

// Server 是常駐服務的運行實例。
type Server struct {
	cfg   Config
	jm    *panel.JobManager
	extra []*datalog.Rule
	tc    core.RustToolchain // 工具鏈 port（T-21a/c）

	mu          sync.Mutex
	bridges     map[string]*eventledger.Bridge // 專案目錄 → bridge
	projects    []string                       // 已登記監看的專案（排序）
	watcher     *watch.Watcher
	lastBatches map[string][]watch.Event // 專案 → 最近一次去抖批次
	ruleSource  string                   // 附加規則來源描述
	httpSrv     *http.Server

	// precompileMu protects the active-precompile coordinator. A project has at
	// most one running precompile; edits arriving while it runs set pending and
	// cause exactly one follow-up run against the newest workspace state.
	precompileMu     sync.Mutex
	precompileStates map[string]*precompileJobState
	precompileRun    func(context.Context, precompile.Options) (precompile.Report, error)
}

// precompileJobState is deliberately small: the full, auditable report lives
// at .ykc/precompile/report.json and in the precompile.report event. This state
// only drives live status and single-flight/coalescing.
type precompileJobState struct {
	enabled       bool
	running       bool
	pending       bool
	requestCount  uint64
	runCount      uint64
	lastTrigger   string
	lastOverall   precompile.Status
	lastStarted   time.Time
	lastFinished  time.Time
	lastSandbox   sandbox.Capability
	lastError     string
	lastReport    string
	lastStageCnt  int
	lastErrorCnt  int
	lastWarnCount int
}

// precompileStateView is the bounded live view returned by /api/watch. It does
// not expose compiler output; clients can inspect the path or ledger event for
// the complete auditable report.
type precompileStateView struct {
	Enabled       bool                `json:"enabled"`
	Running       bool                `json:"running"`
	Pending       bool                `json:"pending"`
	RequestCount  uint64              `json:"request_count"`
	RunCount      uint64              `json:"run_count"`
	LastTrigger   string              `json:"last_trigger,omitempty"`
	LastOverall   precompile.Status   `json:"last_overall,omitempty"`
	LastStarted   string              `json:"last_started,omitempty"`
	LastFinished  string              `json:"last_finished,omitempty"`
	LastSandbox   *sandbox.Capability `json:"last_sandbox,omitempty"`
	LastError     string              `json:"last_error,omitempty"`
	LastReport    string              `json:"last_report,omitempty"`
	LastStageCnt  int                 `json:"last_stage_count,omitempty"`
	LastErrorCnt  int                 `json:"last_error_count,omitempty"`
	LastWarnCount int                 `json:"last_warning_count,omitempty"`
}

// New 驗證配置並載入附加規則（尚未啟動）。
func New(cfg Config) (*Server, error) {
	if cfg.Root == "" {
		return nil, fmt.Errorf("serve: root is required")
	}
	abs, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, err
	}
	cfg.Root = abs
	if resolved, err := filepath.EvalSymlinks(cfg.Root); err == nil {
		cfg.Root = filepath.Clean(resolved)
	}
	for i, dir := range cfg.ExtraDirs {
		if a, err := filepath.Abs(dir); err == nil {
			dir = a
		}
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		}
		cfg.ExtraDirs[i] = filepath.Clean(dir)
	}
	if cfg.Debounce <= 0 {
		cfg.Debounce = 300 * time.Millisecond
	}
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if cfg.Actor == "" {
		cfg.Actor = "ykc-serve"
	}
	if cfg.StateDir == "" {
		cfg.StateDir = ".ykc"
	}
	if cfg.Depth < 0 {
		cfg.Depth = 0
	}
	if cfg.PrecompileBackend == "" {
		cfg.PrecompileBackend = sandbox.BackendAuto
	}
	if cfg.PrecompileTimeout <= 0 {
		cfg.PrecompileTimeout = 3 * time.Minute
	}
	if cfg.PrecompileRun == nil {
		cfg.PrecompileRun = precompile.Run
	}
	switch cfg.ToolchainPolicy {
	case "":
		cfg.ToolchainPolicy = PolicyDegrade
	case PolicyStrict, PolicyDegrade:
	default:
		return nil, fmt.Errorf("serve: toolchain policy 只接受 strict|degrade，收到 %q", cfg.ToolchainPolicy)
	}
	if cfg.Toolchain == nil {
		cfg.Toolchain = toolchain.NewNative()
	}
	s := &Server{
		cfg:              cfg,
		tc:               cfg.Toolchain,
		bridges:          map[string]*eventledger.Bridge{},
		lastBatches:      map[string][]watch.Event{},
		precompileStates: map[string]*precompileJobState{},
		precompileRun:    cfg.PrecompileRun,
	}
	if cfg.RulesPath != "" {
		rules, err := guardrail.LoadRules(cfg.RulesPath)
		if err != nil {
			return nil, fmt.Errorf("serve: load rules: %w", err)
		}
		s.extra = rules
		s.ruleSource = cfg.RulesPath
	}
	return s, nil
}

// discoverProjects 回傳要監看的專案清單（Cargo 專案發現 + 額外目錄，排序去重）。
func (s *Server) discoverProjects() []string {
	seen := map[string]bool{}
	var out []string
	add := func(d string) {
		if abs, err := filepath.Abs(d); err == nil {
			d = abs
		}
		d = filepath.Clean(d)
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	for _, d := range s.cfg.ExtraDirs {
		add(d)
	}
	for _, d := range panel.DiscoverCargoProjects(s.cfg.Root, nil, s.cfg.Depth) {
		add(d)
	}
	// precompile also supports single-file rustc mode. Keep a source-only root
	// as an active project instead of requiring a synthetic Cargo.toml.
	if len(out) == 0 && !hasCargoManifest(s.cfg.Root) && hasRustSource(s.cfg.Root) {
		add(s.cfg.Root)
	}
	sort.Strings(out)
	return out
}

func hasCargoManifest(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "Cargo.toml"))
	return err == nil
}

func hasRustSource(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".ykc", "target":
				if path != dir {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".rs") {
			found = true
		}
		return nil
	})
	return found
}

// stateDirOf 回傳專案的 .ykc 狀態目錄（絕對路徑）。
func (s *Server) stateDirOf(project string) string {
	return filepath.Join(project, s.cfg.StateDir)
}

// bridgeOf 取得（或開啟）專案的 eventledger bridge。
func (s *Server) bridgeOf(project string) (*eventledger.Bridge, error) {
	s.mu.Lock()
	b, ok := s.bridges[project]
	s.mu.Unlock()
	if ok {
		return b, nil
	}
	b, err := eventledger.Open(s.stateDirOf(project), s.cfg.Actor)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.bridges[project] = b
	s.mu.Unlock()
	return b, nil
}

// Start 啟動常駐服務：監看 + 面板 HTTP + 事件循環；阻塞直到 ctx 取消或 HTTP 出錯。
func (s *Server) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	panel.EnsureToolchainPath()

	s.jm = panel.NewJobManagerWithDiscovery(s.cfg.BinDir, s.cfg.Root, s.cfg.ExtraDirs, s.cfg.Depth)
	projects := s.discoverProjects()
	if len(projects) == 0 {
		return fmt.Errorf("serve: no cargo projects discovered under %s (depth=%d); use -dir to add one", s.cfg.Root, s.cfg.Depth)
	}
	s.mu.Lock()
	s.projects = projects
	s.mu.Unlock()
	for _, p := range projects {
		if _, err := s.bridgeOf(p); err != nil {
			return fmt.Errorf("serve: open ledger bridge for %s: %w", p, err)
		}
	}

	// T-21c 工具鏈握手：版本指紋入帳本；strict 失配即拒絕啟動。
	if err := s.attestAll(runCtx, projects); err != nil {
		return err
	}

	w, err := watch.NewWatcher(watch.Config{
		Roots:        projects,
		Debounce:     s.cfg.Debounce,
		PollInterval: s.cfg.PollInterval,
	})
	if err != nil {
		return fmt.Errorf("serve: watcher: %w", err)
	}
	s.mu.Lock()
	s.watcher = w
	s.mu.Unlock()
	defer func() {
		_ = w.Close()
		s.mu.Lock()
		if s.watcher == w {
			s.watcher = nil
		}
		s.mu.Unlock()
	}()

	mux := panel.BuildMuxWith(panel.Options{
		Root:      s.cfg.Root,
		ExtraDirs: s.cfg.ExtraDirs,
		BinDir:    s.cfg.BinDir,
		Token:     s.cfg.Token,
		Depth:     s.cfg.Depth,
	}, s.jm, map[string]http.HandlerFunc{
		"/api/claims":    s.handleClaims,
		"/api/watch":     s.handleWatchState,
		"/api/rules":     s.handleRules,
		"/api/toolchain": s.handleToolchain,
	})

	listen := net.JoinHostPort(s.cfg.Addr, strconv.Itoa(s.cfg.Port))
	if s.cfg.Token == "" && !isLoopbackAddr(listen) {
		log.Printf("⚠️  安全警告：serve 綁定 %s（非本機）且未設定 token——LAN 內任何主機可觸發任務/聲明評估。建議 -token <密鑰>", listen)
	}
	srv := &http.Server{Addr: listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	s.mu.Lock()
	s.httpSrv = srv
	s.mu.Unlock()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("ykc serve listening on %s (root=%s, projects=%d, backend=%s, debounce=%s, rules=+%d, auto_judge=%v, auto_precompile=%v)",
			listen, s.cfg.Root, len(projects), w.BackendName(), s.cfg.Debounce, len(s.extra), s.cfg.AutoJudge, s.precompileEnabled())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	go s.eventLoop(runCtx)
	if s.precompileEnabled() {
		// Initial sweep makes the feature active even before the first editor
		// event. Each project is independently single-flight/coalesced.
		for _, project := range projects {
			s.requestPrecompile(runCtx, project, "startup")
		}
	}

	select {
	case <-runCtx.Done():
		shutCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutCtx)
		return nil
	case err := <-errCh:
		cancel()
		return err
	}
}

// eventLoop 消費去抖批次 → file.change 帳本事件 + 主動預譯（+可選 auto-judge）。
func (s *Server) eventLoop(ctx context.Context) {
	w := s.currentWatcher()
	if w == nil {
		return
	}
	batches := w.Batches()
	watchErrors := w.Errors()
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-watchErrors:
			if !ok {
				watchErrors = nil
				continue
			}
			if err != nil {
				log.Printf("serve: watcher error: %v", err)
				// An overflow explicitly means the event stream is incomplete.
				// Re-run every project rather than pretending the incremental
				// trigger set was complete.
				if strings.Contains(strings.ToLower(err.Error()), "overflow") && s.precompileEnabled() {
					s.mu.Lock()
					projects := append([]string(nil), s.projects...)
					s.mu.Unlock()
					for _, project := range projects {
						s.requestPrecompile(ctx, project, "watch-overflow")
					}
				}
			}
		case batch, ok := <-batches:
			if !ok {
				return
			}
			s.handleBatch(ctx, batch)
		}
	}
}

func (s *Server) currentWatcher() *watch.Watcher {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.watcher
}

func (s *Server) precompileEnabled() bool { return !s.cfg.NoAutoPrecompile }

func (s *Server) ensurePrecompileStateLocked(project string) *precompileJobState {
	if s.precompileStates == nil {
		s.precompileStates = map[string]*precompileJobState{}
	}
	st := s.precompileStates[project]
	if st == nil {
		st = &precompileJobState{}
		s.precompileStates[project] = st
	}
	st.enabled = s.precompileEnabled()
	if st.lastReport == "" {
		st.lastReport = filepath.Join(s.stateDirOf(project), "precompile", "report.json")
	}
	return st
}

// requestPrecompile schedules an active precompile without blocking the file
// event loop. Requests are coalesced per project: while cargo/rustc is running,
// any number of edits become one pending run against the newest workspace.
func (s *Server) requestPrecompile(ctx context.Context, project, trigger string) {
	if project == "" || !s.precompileEnabled() || ctx == nil || ctx.Err() != nil {
		return
	}
	s.precompileMu.Lock()
	st := s.ensurePrecompileStateLocked(project)
	st.pending = true
	st.requestCount++
	if trigger != "" {
		st.lastTrigger = trigger
	}
	if st.running {
		s.precompileMu.Unlock()
		return
	}
	st.running = true
	s.precompileMu.Unlock()
	go s.precompileLoop(ctx, project)
}

func (s *Server) precompileLoop(ctx context.Context, project string) {
	for {
		s.precompileMu.Lock()
		st := s.ensurePrecompileStateLocked(project)
		trigger := st.lastTrigger
		st.pending = false
		st.runCount++
		st.lastStarted = time.Now().UTC()
		s.precompileMu.Unlock()

		rep, runErr := s.runPrecompile(ctx, project, trigger)
		var problems []string
		if runErr != nil {
			problems = append(problems, runErr.Error())
		}
		if err := s.writePrecompileReport(project, rep); err != nil {
			problems = append(problems, "write report: "+err.Error())
		}
		// A failed compile is still a useful, auditable report. Only an
		// infrastructure failure is surfaced as last_error; rep.overall carries
		// compiler failure/unsupported semantics.
		if err := s.appendProjectEvent(project, domain.EventPrecompileReport, rep); err != nil {
			problems = append(problems, "append report event: "+err.Error())
		}

		s.precompileMu.Lock()
		st = s.ensurePrecompileStateLocked(project)
		st.lastFinished = time.Now().UTC()
		st.lastOverall = rep.Overall
		st.lastSandbox = rep.Sandbox
		st.lastError = strings.Join(problems, "; ")
		st.lastStageCnt = len(rep.Stages)
		st.lastErrorCnt = 0
		st.lastWarnCount = 0
		for _, stage := range rep.Stages {
			st.lastErrorCnt += stage.Diagnostics.ErrorCount
			st.lastWarnCount += stage.Diagnostics.WarningCount
		}
		again := st.pending && ctx.Err() == nil
		if !again {
			st.running = false
			if ctx.Err() != nil {
				st.pending = false
			}
		}
		s.precompileMu.Unlock()
		if !again {
			return
		}
	}
}

func (s *Server) runPrecompile(ctx context.Context, project, trigger string) (precompile.Report, error) {
	opt := precompile.DefaultOptions(project)
	opt.Backend = s.cfg.PrecompileBackend
	opt.AllowNative = s.cfg.PrecompileAllowNative
	opt.Timeout = s.cfg.PrecompileTimeout
	opt.Image = s.cfg.PrecompileImage
	opt.StateDir = s.cfg.StateDir
	runner := s.precompileRun
	if runner == nil {
		runner = precompile.Run
	}
	rep, err := runner(ctx, opt)
	if rep.ProjectDir == "" {
		rep.ProjectDir = project
	}
	if rep.CreatedAt.IsZero() {
		rep.CreatedAt = time.Now().UTC()
	}
	rep.Trigger = trigger
	if rep.Overall == "" {
		if err != nil {
			rep.Overall = precompile.StatusFailed
		} else {
			rep.Overall = precompile.StatusUnsupported
		}
	}
	return rep, err
}

func (s *Server) writePrecompileReport(project string, rep precompile.Report) error {
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.stateDirOf(project), "precompile", "report.json")
	return atomicfile.WriteFileSync(path, append(b, '\n'), 0o644)
}

func (s *Server) precompileStateView(project string) precompileStateView {
	s.precompileMu.Lock()
	st := s.ensurePrecompileStateLocked(project)
	view := precompileStateView{
		Enabled:       st.enabled,
		Running:       st.running,
		Pending:       st.pending,
		RequestCount:  st.requestCount,
		RunCount:      st.runCount,
		LastTrigger:   st.lastTrigger,
		LastOverall:   st.lastOverall,
		LastReport:    st.lastReport,
		LastStageCnt:  st.lastStageCnt,
		LastErrorCnt:  st.lastErrorCnt,
		LastWarnCount: st.lastWarnCount,
	}
	if !st.lastStarted.IsZero() {
		view.LastStarted = st.lastStarted.Format(time.RFC3339)
	}
	if !st.lastFinished.IsZero() {
		view.LastFinished = st.lastFinished.Format(time.RFC3339)
	}
	if st.lastSandbox.Backend != "" {
		cap := st.lastSandbox
		view.LastSandbox = &cap
	}
	view.LastError = st.lastError
	s.precompileMu.Unlock()
	return view
}

// projectOf 以最長前綴把事件路徑歸屬到專案；無歸屬回傳 ""。
func (s *Server) projectOf(path string) string {
	s.mu.Lock()
	projects := append([]string(nil), s.projects...)
	s.mu.Unlock()
	best := ""
	for _, p := range projects {
		if (path == p || strings.HasPrefix(path, p+string(os.PathSeparator))) && len(p) > len(best) {
			best = p
		}
	}
	return best
}

// handleBatch 是批次處理單元（測試可直接注入批次）：
// ①按專案分組 ②FileChange 事件入帳本 ③主動預譯最新工作區
// ④（可選）.rs 變更觸發 judge。
func (s *Server) handleBatch(ctx context.Context, batch []watch.Event) {
	if len(batch) == 0 {
		return
	}
	byProject := map[string][]watch.Event{}
	for _, e := range batch {
		if p := s.projectOf(e.Path); p != "" {
			byProject[p] = append(byProject[p], e)
		}
	}
	var projects []string
	for p := range byProject {
		projects = append(projects, p)
	}
	sort.Strings(projects)
	for _, p := range projects {
		events := byProject[p]
		// 批次內按路徑排序——帳本事實與上游事件到達順序無關（可重放對賬）
		sort.Slice(events, func(i, j int) bool { return events[i].Path < events[j].Path })
		s.mu.Lock()
		s.lastBatches[p] = events
		s.mu.Unlock()

		payload := domain.FileChangeBatch{Backend: s.backendName()}
		for _, e := range events {
			rel, err := filepath.Rel(p, e.Path)
			if err != nil {
				continue
			}
			payload.Files = append(payload.Files, domain.FileChangeFile{Path: filepath.ToSlash(rel), Op: string(e.Op)})
		}
		if len(payload.Files) == 0 {
			continue
		}
		if err := s.appendProjectEvent(p, domain.EventFileChange, payload); err != nil {
			log.Printf("⚠️ serve: file.change 帳本寫入失敗（%s）: %v", p, err)
		}
		if s.precompileEnabled() && ctx != nil && ctx.Err() == nil {
			s.requestPrecompile(ctx, p, "file-change")
		}
		if s.cfg.AutoJudge && s.hasRs(events) && ctx != nil && ctx.Err() == nil {
			s.autoJudge(p)
		}
	}
}

func (s *Server) backendName() string {
	if w := s.currentWatcher(); w != nil {
		return w.BackendName()
	}
	return ""
}

func (s *Server) hasRs(events []watch.Event) bool {
	for _, e := range events {
		if strings.HasSuffix(e.Path, ".rs") && e.Op != watch.OpRemove {
			return true
		}
	}
	return false
}

// autoJudge 在無運行中任務時觸發 judge（單飛：同專案不重疊）。
func (s *Server) autoJudge(project string) {
	jobs := s.jm.List() // []panel.Job 內含 mutex：一律索引存取，不複製
	for i := range jobs {
		if jobs[i].Project == project && jobs[i].Status == "running" {
			return
		}
	}
	if _, err := s.jm.Start("judge", project, ""); err != nil {
		log.Printf("serve: auto-judge 觸發失敗（%s）: %v", project, err)
	}
}

// appendProjectEvent 組裝 envelope 並以重試寫入（ErrLocked 退避；有界）。
func (s *Server) appendProjectEvent(project string, kind domain.EventKind, payload any) error {
	b, err := s.bridgeOf(project)
	if err != nil {
		return err
	}
	history, err := b.ReplayEvents()
	if err != nil {
		return err
	}
	env, err := domain.NewEnvelope(kind, domain.LatestSnapshotEpoch(history), project, payload)
	if err != nil {
		return err
	}
	return appendWithRetry(b, env, 6)
}

// appendWithRetry：帳本被其他 YKC 進程持鎖（judge 子行程寫收據）時退避重試。
func appendWithRetry(b *eventledger.Bridge, env domain.Envelope, attempts int) error {
	var err error
	backoff := 40 * time.Millisecond
	for i := 0; i < attempts; i++ {
		_, err = b.Append(env)
		if err == nil || !errors.Is(err, ledger.ErrLocked) {
			return err
		}
		time.Sleep(backoff)
		if backoff < 640*time.Millisecond {
			backoff *= 2
		}
	}
	return fmt.Errorf("ledger still locked after %d attempts: %w", attempts, err)
}

// ---------- HTTP 端點 ----------

// claimsRequest 是 POST /api/claims 的請求體。
type claimsRequest struct {
	Project  string `json:"project"`
	Kind     string `json:"kind"` // work_done|tests_passed|build_passed|no_errors
	Text     string `json:"text"`
	RunSmoke bool   `json:"run_smoke"` // 聲明被駁回時是否立即執行煙測接管（預設否——控制權在人）
}

func decodeSingleJSON(r io.Reader, dst any) error {
	dec := json.NewDecoder(r)
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

// handleClaims：聲明評估閉環（atom claim 的常駐版）：
// append claim → datalog 護欄評估 → append decision → enforcement 落盤 →（可選）煙測。
func (s *Server) handleClaims(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !panel.Authed(w, r, s.cfg.Token) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req claimsRequest
	if err := decodeSingleJSON(r.Body, &req); err != nil || req.Project == "" || req.Kind == "" {
		http.Error(w, "需要單一 JSON object，且包含 project 與 kind", http.StatusBadRequest)
		return
	}
	project, err := s.jm.ValidateProject(req.Project)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	decision, claimID, err := s.evaluateClaim(r.Context(), project, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	panel.WriteJSON(w, map[string]any{
		"project":   project,
		"claim_id":  claimID,
		"decision":  decision,
		"state_dir": s.stateDirOf(project),
		"smoke_ran": req.RunSmoke && decision.RunSmoke,
	})
}

// evaluateClaim 是聲明評估的唯一路徑（HTTP 與測試共用）。
func (s *Server) evaluateClaim(ctx context.Context, project string, req claimsRequest) (guardrail.Decision, string, error) {
	b, err := s.bridgeOf(project)
	if err != nil {
		return guardrail.Decision{}, "", err
	}
	history, err := b.ReplayEvents()
	if err != nil {
		return guardrail.Decision{}, "", err
	}
	epoch := domain.LatestSnapshotEpoch(history)
	claim := domain.AgentClaim{Kind: domain.AgentClaimKind(req.Kind), Text: req.Text}
	claimEnv, err := domain.NewEnvelope(domain.EventAgentClaim, epoch, project, claim)
	if err != nil {
		return guardrail.Decision{}, "", err
	}
	if err := appendWithRetry(b, claimEnv, 6); err != nil {
		return guardrail.Decision{}, "", err
	}
	history = append(history, claimEnv)

	decision := guardrail.EvaluateClaimWithRules(guardrail.StrictPolicy(), history, claimEnv, s.extraRules())

	decEnv, err := domain.NewEnvelope(domain.EventGuardrailDecision, epoch, project, decision)
	if err != nil {
		return decision, claimEnv.ID, err
	}
	if err := appendWithRetry(b, decEnv, 6); err != nil {
		return decision, claimEnv.ID, err
	}
	if err := enforcement.ApplyDecision(s.stateDirOf(project), decision); err != nil {
		return decision, claimEnv.ID, err
	}
	if decision.RunSmoke && req.RunSmoke {
		report, err := smoke.Runner{FailFast: true}.Run(ctx, smoke.DefaultRustSmoke(project))
		if err != nil {
			return decision, claimEnv.ID, err
		}
		for _, res := range report.Results {
			resEnv, err := domain.NewEnvelope(domain.EventCommandResult, epoch, project, res)
			if err != nil {
				return decision, claimEnv.ID, fmt.Errorf("create smoke result event: %w", err)
			}
			if err := appendWithRetry(b, resEnv, 6); err != nil {
				return decision, claimEnv.ID, fmt.Errorf("append smoke result event: %w", err)
			}
		}
		if err := enforcement.ApplySmokeReport(s.stateDirOf(project), report); err != nil {
			return decision, claimEnv.ID, err
		}
	}
	return decision, claimEnv.ID, nil
}

func (s *Server) extraRules() []*datalog.Rule {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.extra
}

// handleWatchState：監看狀態（後端、專案、最近批次）。
func (s *Server) handleWatchState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	projects := append([]string(nil), s.projects...)
	last := make(map[string][]watch.Event, len(s.lastBatches))
	for k, v := range s.lastBatches {
		last[k] = append([]watch.Event(nil), v...)
	}
	s.mu.Unlock()
	type projView struct {
		Dir        string              `json:"dir"`
		LastBatch  []watch.Event       `json:"last_batch,omitempty"`
		Precompile precompileStateView `json:"precompile"`
	}
	projViews := make([]projView, 0, len(projects))
	for _, p := range projects {
		projViews = append(projViews, projView{Dir: p, LastBatch: last[p], Precompile: s.precompileStateView(p)})
	}
	watchError, watchErrorCount := "", uint64(0)
	if w := s.currentWatcher(); w != nil {
		watchError, watchErrorCount = w.LastError()
	}
	panel.WriteJSON(w, map[string]any{
		"backend":            s.backendName(),
		"watch_error":        watchError,
		"watch_error_count":  watchErrorCount,
		"debounce_ms":        s.cfg.Debounce.Milliseconds(),
		"projects":           projViews,
		"running_jobs":       s.runningJobs(),
		"extra_rules":        len(s.extraRules()),
		"auto_judge":         s.cfg.AutoJudge,
		"auto_precompile":    s.precompileEnabled(),
		"precompile_backend": s.cfg.PrecompileBackend,
		"server_time":        time.Now().UTC().Format(time.RFC3339),
	})
}

// jobView 是 job 的輕量視圖（panel.Job 內含 mutex，不跨包複製）。
type jobView struct {
	ID      string    `json:"id"`
	Project string    `json:"project"`
	Action  string    `json:"action"`
	Status  string    `json:"status"`
	Started time.Time `json:"started"`
}

func (s *Server) runningJobs() []jobView {
	if s.jm == nil {
		return nil
	}
	jobs := s.jm.List()
	var out []jobView
	for i := range jobs {
		if jobs[i].Status == "running" {
			out = append(out, jobView{ID: jobs[i].ID, Project: jobs[i].Project, Action: jobs[i].Action, Status: jobs[i].Status, Started: jobs[i].Started})
		}
	}
	return out
}

// handleRules：規則透明度——預設規則全文 + 附加規則來源（裁判可審計）。
func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	panel.WriteJSON(w, map[string]any{
		"default_rules": guardrail.DefaultRulesSource(),
		"extra_source":  s.ruleSourceLocked(),
		"extra_count":   len(s.extraRules()),
	})
}

func (s *Server) ruleSourceLocked() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ruleSource
}

// Handler 構建 HTTP handler（測試用；與 Start 同一路由）。
func (s *Server) Handler() http.Handler {
	s.jm = panel.NewJobManagerWithDiscovery(s.cfg.BinDir, s.cfg.Root, s.cfg.ExtraDirs, s.cfg.Depth)
	return panel.BuildMuxWith(panel.Options{
		Root:      s.cfg.Root,
		ExtraDirs: s.cfg.ExtraDirs,
		BinDir:    s.cfg.BinDir,
		Token:     s.cfg.Token,
		Depth:     s.cfg.Depth,
	}, s.jm, map[string]http.HandlerFunc{
		"/api/claims": s.handleClaims,
		"/api/watch":  s.handleWatchState,
		"/api/rules":  s.handleRules,
	})
}

func isLoopbackAddr(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		host = listen
	}
	if host == "" || host == "::" || host == "0.0.0.0" || host == "[::]" {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
