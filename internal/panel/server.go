// Package panel 是 YKC Trust Console 的唯一實作（cmd/ykc-panel 與 cmd/ykc-serve 共用）。
//
// 兩大表面，職責分明：
//
//	觀察（唯讀）：/api/state、/api/raw —— 直接讀事實帳本，不產生、不改寫任何資料。
//	控制（人類觸發）：/api/projects、/api/jobs —— 揀專案、啟動/停止 YKC 動作、實時日誌。
//
// 安全邊界（S1 修復，自 cmd/ykc-panel 遷移，語義不變）：
//
//   - 預設綁 127.0.0.1（本機）；要暴露到網路必須顯式 -addr 0.0.0.0:PORT。
//   - 控制端點（POST /api/jobs、/api/jobs/stop）可要求 Bearer token；未設定
//     token 時暴露到非本機位址會打印醒目警告。
//   - 任務的 project 參數必須在「已發現的 Cargo 專案白名單」內（任意路徑拒收）。
//   - claims 檔必須在專案目錄或面板根之內（防任意檔讀取）。
//   - 請求體限長（MaxBytesReader 1MB）。
//
// 端點：
//
//	GET  /                     人類面板（內嵌 HTML，零外部依賴）
//	GET  /api/state            全量觀察狀態 JSON（供 AI agent / 外部系統）
//	GET  /api/raw?project=X    單一專案的原始事實與收據
//	GET  /api/know/*           Rust 知識庫（唯讀、無 token）
//	GET  /api/projects         可運行目標（含 Cargo.toml 的專案）
//	GET  /api/jobs             任務清單（含實時日誌）
//	POST /api/jobs             啟動任務 {action, project, claims}
//	POST /api/jobs/stop        停止任務 {id}
//	GET  /healthz              存活探針
package panel

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed dashboard.html
var dashboardFS embed.FS

const maxBodyBytes = 1 << 20 // 1MB 請求體上限

// Options 是面板配置（cmd/ykc-panel 與 cmd/ykc-serve 各自解析旗標後傳入）。
type Options struct {
	Root      string   // 掃描根目錄（觀察：含 .ykc 的專案；控制：含 Cargo.toml 的專案）
	ExtraDirs []string // 額外專案目錄（絕對路徑）
	BinDir    string   // ykc 二進制目錄（jobs 用）
	Token     string   // 控制端點 Bearer token（空 = 不驗證）
	Depth     int      // 專案發現掃描深度（0=僅根目錄）
}

func decodeJSON(r io.Reader, dst any) error {
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

// BuildMux 組裝面板全部端點；extra 允許宿主（ykc-serve）附加自己的端點
// （如 /api/claims、/api/watch——掛在面板同一個 mux、共用同一套 token 邊界）。
func BuildMux(o Options, extra map[string]http.HandlerFunc) *http.ServeMux {
	if o.Depth < 0 {
		o.Depth = 0
	}
	jm := NewJobManagerWithDiscovery(o.BinDir, o.Root, o.ExtraDirs, o.Depth)
	return BuildMuxWith(o, jm, extra)
}

// BuildMuxWith 是 BuildMux 的可注入版本（serve 以既有 JobManager 構建，避免雙實例）。
func BuildMuxWith(o Options, jm *JobManager, extra map[string]http.HandlerFunc) *http.ServeMux {
	if o.Depth < 0 {
		o.Depth = 0
	}
	if jm == nil {
		jm = NewJobManagerWithDiscovery(o.BinDir, o.Root, o.ExtraDirs, o.Depth)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		b, _ := dashboardFS.ReadFile("dashboard.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		WriteJSON(w, collectState(o.Root, o.ExtraDirs, o.Depth))
	})
	mux.HandleFunc("/api/raw", rawHandler(o.Root, o.ExtraDirs, o.Depth))
	mux.Handle("/api/know/", KnowledgeHandler())
	mux.HandleFunc("/api/projects", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		WriteJSON(w, map[string]any{"projects": DiscoverCargoProjects(o.Root, o.ExtraDirs, o.Depth)})
	})
	mux.HandleFunc("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			WriteJSON(w, map[string]any{"jobs": jm.List()})
		case http.MethodPost:
			if !Authed(w, r, o.Token) {
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			var req struct {
				Action  string `json:"action"`
				Project string `json:"project"`
				Claims  string `json:"claims"`
			}
			if decodeJSON(r.Body, &req) != nil || req.Action == "" || req.Project == "" {
				http.Error(w, "需要單一 JSON object，且包含 action 與 project", http.StatusBadRequest)
				return
			}
			j, err := jm.Start(req.Action, req.Project, req.Claims)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			WriteJSON(w, j.snapshot())
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/jobs/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !Authed(w, r, o.Token) {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		var req struct {
			ID string `json:"id"`
		}
		if decodeJSON(r.Body, &req) != nil || req.ID == "" {
			http.Error(w, "需要單一 JSON object，且包含 id", http.StatusBadRequest)
			return
		}
		if err := jm.Stop(req.ID); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		WriteJSON(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	for pattern, h := range extra {
		mux.HandleFunc(pattern, h)
	}
	return mux
}

// Run 以 BuildMux 起一個獨立 HTTP 伺服器（cmd/ykc-panel 的行為，遷移自舊 main）。
func Run(o Options, addr string, port int) error {
	EnsureToolchainPath()
	listen := addr
	if _, _, err := net.SplitHostPort(listen); err != nil {
		listen = net.JoinHostPort(listen, strconv.Itoa(port))
	}
	if o.Token == "" && !isLoopback(listen) {
		log.Printf("⚠️  安全警告：面板綁定 %s（非本機）且未設定 token——LAN 內任何主機可觸發任務。建議 -token <密鑰> 或 -addr 127.0.0.1", listen)
	}
	log.Printf("YKC Trust Console listening on %s (root=%s, bindir=%s, depth=%d)", listen, o.Root, o.BinDir, o.Depth)
	mux := BuildMux(o, nil)
	srv := &http.Server{Addr: listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return srv.ListenAndServe()
}

// Authed 驗證 Bearer token（timing-safe）；token 為空時放行。
func Authed(w http.ResponseWriter, r *http.Request, token string) bool {
	if token == "" {
		return true
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func isLoopback(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		host = listen
	}
	if host == "" || host == "::" || host == "0.0.0.0" || host == "[::]" {
		return false // 全部介面
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// WriteJSON 輸出 JSON 回應（面板與 serve 附加端點共用）。
func WriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

// EnsureToolchainPath 讓面板/serve 無論以何種方式啟動（launch.sh / make / 直接執行），
// 都能為子行程（cargo / go / rust-analyzer）補齊工具鏈環境：PATH + RUSTUP_HOME + CARGO_HOME。
func EnsureToolchainPath() {
	ykcHome := os.Getenv("YKC_HOME")
	if ykcHome == "" {
		if home, err := os.UserHomeDir(); err == nil {
			ykcHome = filepath.Join(home, ".ykc")
		}
	}
	// 定位 rustup home：多個常見位置，取第一個存在者
	rustupHome := os.Getenv("RUSTUP_HOME")
	cargoHome := os.Getenv("CARGO_HOME")
	if rustupHome == "" || cargoHome == "" {
		for _, base := range []string{ykcHome, "/usr/local/ykc"} {
			r := filepath.Join(base, "rustup")
			c := filepath.Join(base, "cargo")
			if rustupHome == "" {
				if fi, err := os.Stat(r); err == nil && fi.IsDir() {
					rustupHome = r
				}
			}
			if cargoHome == "" {
				if fi, err := os.Stat(c); err == nil && fi.IsDir() {
					cargoHome = c
				}
			}
		}
	}
	if rustupHome != "" {
		os.Setenv("RUSTUP_HOME", rustupHome)
	}
	if cargoHome != "" {
		os.Setenv("CARGO_HOME", cargoHome)
	}

	cands := []string{
		filepath.Join(ykcHome, "bin"),
		filepath.Join(ykcHome, "go", "bin"),
		filepath.Join(cargoHome, "bin"),
		"/usr/local/ykc/bin",
		"/usr/local/ykc/cargo/bin",
		"/usr/local/go/bin",
	}
	var prepend []string
	for _, d := range cands {
		if d == "" {
			continue
		}
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			prepend = append(prepend, d)
		}
	}
	if len(prepend) > 0 {
		cur := os.Getenv("PATH")
		os.Setenv("PATH", strings.Join(append(prepend, cur), string(os.PathListSeparator)))
	}
}
