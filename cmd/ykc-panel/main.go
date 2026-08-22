// ykc-panel — YKC Trust Console 獨立入口（薄殼；實作在 internal/panel）。
//
// 自 YKC_14 起，面板實作與 cmd/ykc-serve 共用 internal/panel（唯一實作）；
// 本檔只保留旗標解析與啟動。
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"ykc/internal/panel"
)

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

	opts := panel.Options{
		Root:      *root,
		ExtraDirs: extra,
		BinDir:    *bindir,
		Token:     *token,
		Depth:     *depth,
	}
	if err := panel.Run(opts, *addr, *port); err != nil {
		log.Fatal(err)
	}
}
