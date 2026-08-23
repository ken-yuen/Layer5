// ykc-serve — YKC 常駐進程（YKC_14）：atom（監看）+ judge（觸發）+ guard（聲明評估）
// + panel（Trust Console）合併為單一運行時。實作在 internal/serve。
//
// 用法：
//
//	ykc serve 而言：
//	  ./bin/ykc-serve -root ~/code -addr 127.0.0.1 -port 8080 [-token <密鑰>]
//	                    [-rules ./my-rules.d] [-auto-judge] [-debounce 300ms]
//	  啟動後會主動預譯所有已發現專案，並在每個 Rust 工作區檔案變更後重跑；
//	  只有 -no-auto-precompile 才會關閉此行為。
//
// 與既有 CLI 的關係：四個 CLI 全部保留（git 閘門、MCP、一次性除錯等場景）；
// serve 是「常駐監督面」的合一入口。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"ykc/internal/sandbox"
	"ykc/internal/serve"
)

func main() {
	root := flag.String("root", ".", "掃描根目錄（專案發現 + 面板觀察）")
	dirs := flag.String("dir", "", "額外專案目錄（逗號分隔）")
	addr := flag.String("addr", "127.0.0.1", "綁定位址（預設 127.0.0.1；暴露網路須顯式 0.0.0.0 並建議 -token）")
	port := flag.Int("port", 8080, "監聽埠")
	bindir := flag.String("bindir", "./bin", "ykc 二進制目錄（judge 等 jobs 用）")
	token := flag.String("token", os.Getenv("YKC_PANEL_TOKEN"), "控制端點 Bearer token")
	depth := flag.Int("depth", 1, "專案發現掃描深度")
	debounce := flag.Duration("debounce", 300*time.Millisecond, "檔案事件去抖窗口")
	poll := flag.Duration("poll", 250*time.Millisecond, "輪詢後端間隔（非 inotify 平台用）")
	rules := flag.String("rules", "", "附加 datalog 護欄規則（.dl 檔或目錄；附加不取代預設）")
	autoJudge := flag.Bool("auto-judge", false, ".rs 變更批次後自動觸發 judge 任務（單飛）")
	noAutoPrecompile := flag.Bool("no-auto-precompile", false, "停用主動 Rust 預譯（預設啟用；僅在明示需要時使用）")
	precompileSandbox := flag.String("precompile-sandbox", "auto", "主動預譯 sandbox：auto|docker-runsc|podman-runsc|bwrap|native|docker|podman")
	precompileAllowNative := flag.Bool("precompile-allow-native", false, "允許主動預譯降級到 native（僅可信本地專案）")
	precompileTimeout := flag.Duration("precompile-timeout", 3*time.Minute, "主動預譯每個 stage 的 timeout")
	precompileImage := flag.String("precompile-image", "rust:1.98", "container sandbox 使用的 Rust image")
	tcPolicy := flag.String("toolchain-policy", "degrade", "工具鏈握手政策：strict=缺席/失配拒絕啟動；degrade=標記降級照常啟動")
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
	if abs, err := filepath.Abs(*bindir); err == nil {
		*bindir = abs
	}

	cfg := serve.Config{
		Root:                  *root,
		ExtraDirs:             extra,
		Addr:                  *addr,
		Port:                  *port,
		BinDir:                *bindir,
		Token:                 *token,
		Depth:                 *depth,
		Debounce:              *debounce,
		PollInterval:          *poll,
		RulesPath:             *rules,
		AutoJudge:             *autoJudge,
		NoAutoPrecompile:      *noAutoPrecompile,
		PrecompileBackend:     sandbox.NormalizeBackend(*precompileSandbox),
		PrecompileAllowNative: *precompileAllowNative,
		PrecompileTimeout:     *precompileTimeout,
		PrecompileImage:       *precompileImage,
		ToolchainPolicy:       *tcPolicy,
	}
	srv, err := serve.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ykc-serve:", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.Start(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "ykc-serve:", err)
		os.Exit(1)
	}
}
