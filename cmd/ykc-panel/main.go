// YKC Trust Console — 控制 + 觀察台（人類監督 + 機器可讀）
//
// 兩大表面，職責分明：
//
//	觀察（唯讀）：/api/state、/api/raw —— 直接讀事實帳本，不產生、不改寫任何資料。
//	控制（人類觸發）：/api/projects、/api/jobs —— 揀專案、啟動/停止 YKC 動作、實時日誌。
//
// 安全邊界（S1 修復）：
//
//   - 預設綁 127.0.0.1（本機）；要暴露到網路必須顯式 -addr 0.0.0.0:PORT。
//   - 控制端點（POST /api/jobs、/api/jobs/stop）可要求 Bearer token（-token
//     或 YKC_PANEL_TOKEN）；未設定 token 時暴露到非本機位址會打印醒目警告。
//   - 任務的 project 參數必須在「已發現的 Cargo 專案白名單」內（任意路徑拒收）。
//   - 請求體限長（MaxBytesReader）。
//
// 端點：
//
//	GET  /                     人類面板（內嵌 HTML，零外部依賴）
//	GET  /api/state            全量觀察狀態 JSON（供 AI agent / 外部系統）
//	GET  /api/raw?project=X    單一專案的原始事實與收據
//	GET  /api/projects         可運行目標（含 Cargo.toml 的專案）
//	GET  /api/jobs             任務清單（含實時日誌）
//	POST /api/jobs             啟動任務 {action, project, claims}
//	POST /api/jobs/stop        停止任務 {id}
//	GET  /healthz              存活探針
package main

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"flag"
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

func main() {
	root := flag.String("root", ".", "掃描根目錄（觀察：含 .ykc 的專案；控制：含 Cargo.toml 的專案）")
	dirs := flag.String("dir", "", "額外專案目錄（逗號分隔）")
	port := flag.Int("port", 8080, "監聽埠")
	addr := flag.String("addr", "127.0.0.1", "綁定位址（預設 127.0.0.1 本機；暴露到網路請顯式 0.0.0.0 並建議 -token）")
	bindir := flag.String("bindir", "./bin", "ykc 二進制目錄")
	token := flag.String("token", os.Getenv("YKC_PANEL_TOKEN"), "控制端點 Bearer token（空 = 不驗證；暴露到網路時強烈建議設定）")
	depth := flag.Int("depth", 1, "專案發現掃描深度（0=僅根目錄，1=根+一層子目錄，預設）")
	flag.Parse()

	extra := []string{}
	if *dirs != "" {
		for _, d := range strings.Split(*dirs, ",") {
			if d = strings.TrimSpace(d); d != "" {
				if abs, err := filepath.Abs(d); err == nil {
					d = abs
				}
				extra = append(extra, d)
			}
		}
	}
	if abs, err := filepath.Abs(*root); err == nil {
		*root = abs
	}
	if abs, err := filepath.Abs(*bindir); err == nil {
		*bindir = abs
	}
	if *depth < 0 {
		*depth = 0
	}

	ensureToolchainPath()

	jm := newJobManager(*bindir, *root)

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
		writeJSON(w, collectState(*root, extra, *depth))
	})
	mux.HandleFunc("/api/raw", rawHandler(*root, extra, *depth))
	mux.HandleFunc("/api/projects", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"projects": discoverCargoProjects(*root, extra, *depth)})
	})
	mux.HandleFunc("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, map[string]any{"jobs": jm.List()})
		case http.MethodPost:
			if !authed(w, r, *token) {
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			var req struct {
				Action  string `json:"action"`
				Project string `json:"project"`
				Claims  string `json:"claims"`
			}
			if json.NewDecoder(r.Body).Decode(&req) != nil || req.Action == "" || req.Project == "" {
				http.Error(w, "需要 action 與 project", http.StatusBadRequest)
				return
			}
			j, err := jm.Start(*root, req.Action, req.Project, req.Claims)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, j.snapshot())
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/jobs/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authed(w, r, *token) {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		var req struct {
			ID string `json:"id"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.ID == "" {
			http.Error(w, "需要 id", http.StatusBadRequest)
			return
		}
		if err := jm.Stop(req.ID); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	listen := *addr
	if _, _, err := net.SplitHostPort(listen); err != nil {
		// 未帶埠號（含空字串）→ 補上 -port
		listen = net.JoinHostPort(listen, strconv.Itoa(*port))
	}
	// 安全警告：暴露到非本機位址且無 token = 任何人都能觸發任務
	if *token == "" && !isLoopback(listen) {
		log.Printf("⚠️  安全警告：面板綁定 %s（非本機）且未設定 token——LAN 內任何主機可觸發任務。建議 -token <密鑰> 或 -addr 127.0.0.1", listen)
	}
	log.Printf("YKC Trust Console listening on %s (root=%s, bindir=%s, depth=%d)", listen, *root, *bindir, *depth)
	srv := &http.Server{Addr: listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// authed 驗證 Bearer token（timing-safe）；token 為空時放行。
func authed(w http.ResponseWriter, r *http.Request, token string) bool {
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

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

// ensureToolchainPath 讓面板無論以何種方式啟動（launch.sh / make panel / 直接 ./bin/ykc-panel），
// 都能為子行程（cargo / go / rust-analyzer）補齊工具鏈環境：PATH + RUSTUP_HOME + CARGO_HOME。
func ensureToolchainPath() {
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
