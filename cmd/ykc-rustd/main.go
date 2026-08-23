// ykc-rustd — rust-analyzer 常駐 daemon（T-21d，見 YKC_22 §2.4）。
//
// 取代「每次 spawn 冷啟動等 20 秒」：每個專案一個常駐 r-a session，
// initialize 一次，之後毫秒級取診斷；看門狗於連續失敗/崩潰/超壽命時重啟。
//
// 角色定位（不可越界）：LSP 診斷只是低延遲「前哨」訊號——判定權不轉移，
// 裁判定論仍以 cargo check --message-format=json（ykc-judge）為準。
//
// 用法：
//
//	ykc-rustd [-addr 127.0.0.1] [-port 8093] [-server rust-analyzer] [-token <secret>]
//
// 端點（唯讀，無副作用）：
//
//	GET /healthz               → 存活 + rust-analyzer 可用性 + 重啟計數
//	GET /diagnostics?file=<abs .rs> → 該檔診斷（JSON）
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ykc/internal/lsp"
	"ykc/internal/panel"
)

func main() {
	addr := flag.String("addr", "127.0.0.1", "綁定位址")
	port := flag.Int("port", 8093, "監聽埠")
	server := flag.String("server", "rust-analyzer", "LSP server 命令")
	token := flag.String("token", os.Getenv("YKC_RUSTD_TOKEN"), "Bearer token（綁定非本機位址時必填）")
	diagTimeout := flag.Duration("diag-timeout", 15*time.Second, "單次診斷上限")
	initTimeout := flag.Duration("init-timeout", 30*time.Second, "session initialize 上限")
	maxAge := flag.Duration("max-age", 2*time.Hour, "session 最大壽命（防 r-a 記憶體膨脹）")
	flag.Parse()

	panel.EnsureToolchainPath()

	m := lsp.NewManager(*server)
	m.DiagTimeout = *diagTimeout
	m.InitTimeout = *initTimeout
	m.MaxAge = *maxAge
	defer m.Close()

	if ok, why := m.Available(); !ok {
		log.Printf("⚠️  %s（daemon 照常啟動，端點回 503 —— 如實申報，不假裝）", why)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		if !panel.Authed(w, r, *token) {
			return
		}
		ok, why := m.Available()
		panel.WriteJSON(w, map[string]any{
			"ok": ok, "reason": why, "restarts": m.RestartCount(), "server": *server,
		})
	})
	mux.HandleFunc("/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		if !panel.Authed(w, r, *token) {
			return
		}
		file := r.URL.Query().Get("file")
		if file == "" || !strings.HasSuffix(file, ".rs") {
			http.Error(w, "需要 ?file=<絕對路徑 .rs>", http.StatusBadRequest)
			return
		}
		abs, err := filepath.Abs(file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if ok, why := m.Available(); !ok {
			http.Error(w, why, http.StatusServiceUnavailable)
			return
		}
		start := time.Now()
		diags, err := m.Diagnostics(r.Context(), abs)
		if err != nil {
			http.Error(w, "diagnostics: "+err.Error(), http.StatusBadGateway)
			return
		}
		panel.WriteJSON(w, map[string]any{
			"schema":      "ykc-rustd/v1",
			"file":        abs,
			"elapsed_ms":  time.Since(start).Milliseconds(),
			"diagnostics": diags,
			"authority":   "advisory", // 前哨訊號；判定權在 cargo check（ykc-judge）
		})
	})

	listen := net.JoinHostPort(*addr, strconv.Itoa(*port))
	if *token == "" && !isLoopbackListen(listen) {
		log.Fatalf("refusing to expose rustd on %s without -token/YKC_RUSTD_TOKEN", listen)
	}
	srv := &http.Server{Addr: listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()

	fmt.Printf("ykc-rustd 常駐於 http://%s（server=%s）\n", listen, *server)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func isLoopbackListen(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		host = listen
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
