// YKC LSP 客戶端（T-09 命脈驗證）— 驅動 rust-analyzer 於編輯器外取診斷。
// 驗證「以 LSP 作為首階段接駁」：Go 進程 spawn rust-analyzer、stdio JSON-RPC，
// 並正確實作 LSP 的 Content-Length 框定（initialize → didOpen → publishDiagnostics）。
//
// T-21d 後：框定與訊息類型抽入 internal/lsp（與 ykc-rustd 常駐 daemon 共用）；
// 本命令保留一次性驗證行為（等非空診斷，最多 20 秒）。
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"ykc/internal/lsp"
)

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
	absFile, _ := filepath.Abs(file)
	rootDir := filepath.Dir(absFile)

	cmd := exec.Command(server)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "建立 stdin pipe 失敗:", err)
		os.Exit(1)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "建立 stdout pipe 失敗:", err)
		os.Exit(1)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "啟動", server, "失敗:", err)
		os.Exit(1)
	}
	defer func() { _ = cmd.Process.Kill() }()

	out := bufio.NewReader(stdout)

	// 1) initialize
	initParams, _ := json.Marshal(map[string]any{
		"processId": nil,
		"rootUri":   "file://" + rootDir,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"publishDiagnostics": map[string]any{},
			},
		},
	})
	_ = lsp.Send(stdin, lsp.Msg{ID: json.RawMessage("1"), Method: "initialize", Params: initParams})

	// 讀到 initialize 回應
	var initResult json.RawMessage
	for {
		raw, err := lsp.ReadMessage(out)
		if err != nil {
			break
		}
		var m lsp.Msg
		if json.Unmarshal(raw, &m) == nil && string(m.ID) == "1" {
			initResult = m.Result
			break
		}
	}
	if initResult == nil {
		fmt.Fprintln(os.Stderr, "未收到 initialize 回應")
		os.Exit(1)
	}
	fmt.Printf("✅ LSP initialize 成功（server capabilities %d bytes）\n", len(initResult))

	// 2) initialized 通知
	_ = lsp.Send(stdin, lsp.Msg{Method: "initialized", Params: json.RawMessage(`{}`)})

	// 3) didOpen
	content, _ := os.ReadFile(file)
	dp, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{
			"uri":        "file://" + absFile,
			"languageId": "rust",
			"version":    1,
			"text":       string(content),
		},
	})
	_ = lsp.Send(stdin, lsp.Msg{Method: "textDocument/didOpen", Params: dp})

	// 4) 等 publishDiagnostics（最多 20 秒，rust-analyzer 需載入專案後才推非空診斷）
	deadline := time.After(20 * time.Second)
	for {
		select {
		case <-deadline:
			fmt.Println("(超時未收到非空診斷)")
			return
		default:
		}
		raw, err := lsp.ReadMessage(out)
		if err != nil {
			fmt.Println("(連線結束)")
			return
		}
		var m lsp.Msg
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		if m.Method == "textDocument/publishDiagnostics" {
			var pretty map[string]any
			_ = json.Unmarshal(m.Params, &pretty)
			diags, _ := pretty["diagnostics"].([]any)
			if len(diags) == 0 {
				continue // 初始空診斷，繼續等
			}
			fmt.Printf("== LSP publishDiagnostics：%d 條診斷 ==\n", len(diags))
			b, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Println(string(b))
			return
		}
	}
}
