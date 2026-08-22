// Package serve 是 YKC 的常駐進程（YKC_14：合併 atom+judge+guard+panel 的
// 「運行時監督面」為單一 ykc serve）。
//
// 職責分工（與四個既有 CLI 的關係——合併的是運行時，不是功能）：
//
//	atom   → serve 的監看事件流（fsnotify/inotify + 去抖 + 事件序列化進帳本）
//	judge  → serve 經 panel JobManager 觸發（人工 /api/jobs 或 -auto-judge 存檔觸發）
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
//   - run_smoke 需顯式請求（不在聲明評估後自動執行 cargo——控制權在人）。
package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

	"ykc/internal/datalog"
	"ykc/internal/domain"
	"ykc/internal/enforcement"
	"ykc/internal/eventledger"
	"ykc/internal/guardrail"
	"ykc/internal/ledger"
	"ykc/internal/panel"
	"ykc/internal/smoke"
	"ykc/internal/watch"
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
	Actor        string        // 帳本 actor（零值 ykc-serve）
	StateDir     string        // 專案內狀態目錄名（零值 .ykc）
}

// Server 是常駐服務的運行實例。
type Server struct {
	cfg   Config
	jm    *panel.JobManager
	extra []*datalog.Rule

	mu          sync.Mutex
	bridges     map[string]*eventledger.Bridge // 專案目錄 → bridge
	projects    []string                       // 已登記監看的專案（排序）
	watcher     *watch.Watcher
	lastBatches map[string][]watch.Event // 專案 → 最近一次去抖批次
	ruleSource  string                   // 附加規則來源描述
	httpSrv     *http.Server
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
	s := &Server{
		cfg:         cfg,
		bridges:     map[string]*eventledger.Bridge{},
		lastBatches: map[string][]watch.Event{},
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
	sort.Strings(out)
	return out
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
	panel.EnsureToolchainPath()

	s.jm = panel.NewJobManager(s.cfg.BinDir, s.cfg.Root)
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
	defer w.Close()

	mux := panel.BuildMuxWith(panel.Options{
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
		log.Printf("ykc serve listening on %s (root=%s, projects=%d, backend=%s, debounce=%s, rules=+%d, auto_judge=%v)",
			listen, s.cfg.Root, len(projects), w.BackendName(), s.cfg.Debounce, len(s.extra), s.cfg.AutoJudge)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	go s.eventLoop(ctx)

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		return nil
	case err := <-errCh:
		return err
	}
}

// eventLoop 消費去抖批次 → 帳本事件（+可選 auto-judge）。
func (s *Server) eventLoop(ctx context.Context) {
	batches := s.currentWatcher().Batches()
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-batches:
			s.handleBatch(ctx, batch)
		}
	}
}

func (s *Server) currentWatcher() *watch.Watcher {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.watcher
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
// ①按專案分組 ②FileChange 事件入帳本 ③（可選）.rs 變更觸發 judge。
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
		if s.cfg.AutoJudge && s.hasRs(events) && ctx.Err() == nil {
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
	env, err := domain.NewEnvelope(kind, latestEpoch(history), project, payload)
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

// latestEpoch 與 ykc-atom 同邏輯（最新 workspace.snapshot 的 epoch；空史回傳 ""）。
// 刻意本地實作：cmd 套件無法被 internal 匯入；邏輯經 serve 測試鎖定。
func latestEpoch(events []domain.Envelope) string {
	var epoch string
	var at time.Time
	for _, e := range events {
		if e.Kind == domain.EventWorkspaceSnapshot && e.At.After(at) {
			epoch = e.Epoch
			at = e.At
		}
	}
	return epoch
}

// ---------- HTTP 端點 ----------

// claimsRequest 是 POST /api/claims 的請求體。
type claimsRequest struct {
	Project  string `json:"project"`
	Kind     string `json:"kind"` // work_done|tests_passed|build_passed|no_errors
	Text     string `json:"text"`
	RunSmoke bool   `json:"run_smoke"` // 聲明被駁回時是否立即執行煙測接管（預設否——控制權在人）
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Project == "" || req.Kind == "" {
		http.Error(w, "需要 project 與 kind", http.StatusBadRequest)
		return
	}
	project, err := s.jm.ValidateProject(req.Project)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	decision, claimID, err := s.evaluateClaim(context.Background(), project, req)
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
	epoch := latestEpoch(history)
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
			if err == nil {
				_ = appendWithRetry(b, resEnv, 6)
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
	s.mu.Lock()
	projects := append([]string(nil), s.projects...)
	last := make(map[string][]watch.Event, len(s.lastBatches))
	for k, v := range s.lastBatches {
		last[k] = append([]watch.Event(nil), v...)
	}
	s.mu.Unlock()
	type projView struct {
		Dir       string        `json:"dir"`
		LastBatch []watch.Event `json:"last_batch,omitempty"`
	}
	projViews := make([]projView, 0, len(projects))
	for _, p := range projects {
		projViews = append(projViews, projView{Dir: p, LastBatch: last[p]})
	}
	panel.WriteJSON(w, map[string]any{
		"backend":      s.backendName(),
		"debounce_ms":  s.cfg.Debounce.Milliseconds(),
		"projects":     projViews,
		"running_jobs": s.runningJobs(),
		"extra_rules":  len(s.extraRules()),
		"auto_judge":   s.cfg.AutoJudge,
		"server_time":  time.Now().UTC().Format(time.RFC3339),
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
	s.jm = panel.NewJobManager(s.cfg.BinDir, s.cfg.Root)
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
