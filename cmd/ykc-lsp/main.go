// YKC LSP 客戶端（T-09 命脈驗證）— 驅動 rust-analyzer 於編輯器外取診斷。
//
// T-21d 後，一次性 CLI 與 ykc-rustd 共用 internal/lsp 的 Manager、Session、
// framing、握手與 watchdog。裁判定論仍以 cargo check 為準；LSP 只提供 advisory
// 的低延遲診斷前哨。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ykc/internal/lsp"
)

const cliTimeout = 20 * time.Second

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ykc-lsp <file.rs> [server]")
		os.Exit(1)
	}
	file := os.Args[1]
	server := "rust-analyzer"
	if len(os.Args) > 2 {
		server = os.Args[2]
	}
	absFile, err := filepath.Abs(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "解析檔案路徑失敗:", err)
		os.Exit(1)
	}

	manager := lsp.NewManager(server)
	manager.InitTimeout = cliTimeout
	manager.DiagTimeout = cliTimeout
	// A one-shot CLI should wait as long as its advertised diagnostic budget for
	// a delayed non-empty publishDiagnostics message after an initial empty one.
	manager.QuietWindow = cliTimeout
	defer manager.Close()

	diags, err := manager.Diagnostics(context.Background(), absFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "LSP diagnostics 失敗:", err)
		os.Exit(1)
	}
	fmt.Println("✅ LSP initialize 成功（共用 internal/lsp session）")
	fmt.Printf("== LSP publishDiagnostics：%d 條診斷 ==\n", len(diags))
	b, err := json.MarshalIndent(map[string]any{
		"file":        absFile,
		"diagnostics": diags,
		"authority":   "advisory",
	}, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "序列化診斷失敗:", err)
		os.Exit(1)
	}
	fmt.Println(string(b))
}
