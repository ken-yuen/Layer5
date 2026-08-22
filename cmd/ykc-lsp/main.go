// YKC LSP 客戶端（T-09 命脈驗證）— 驅動 rust-analyzer 於編輯器外取診斷。
// 驗證「以 LSP 作為首階段接駁」：Go 進程 spawn rust-analyzer、stdio JSON-RPC，
// 並正確實作 LSP 的 Content-Length 框定（initialize → didOpen → publishDiagnostics）。
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type rpcMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

// send 以 LSP 標準 Content-Length 框定送出訊息。
func send(w io.Writer, m rpcMsg) error {
	m.JSONRPC = "2.0"
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(b)); err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// readMessage 依 Content-Length 讀出完整一則 LSP 訊息。
func readMessage(r *bufio.Reader) ([]byte, error) {
	contentLen := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "content-length:") {
			v := strings.TrimSpace(line[len("content-length:"):])
			contentLen, _ = strconv.Atoi(v)
		}
	}
	if contentLen <= 0 {
		return nil, fmt.Errorf("no content-length")
	}
	buf := make([]byte, contentLen)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

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
	initParams := map[string]any{
		"processId": nil,
		"rootUri":   "file://" + rootDir,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"publishDiagnostics": map[string]any{},
			},
		},
	}
	ip, _ := json.Marshal(initParams)
	_ = send(stdin, rpcMsg{ID: json.RawMessage("1"), Method: "initialize", Params: ip})

	// 讀到 initialize 回應
	var initResult json.RawMessage
	for {
		raw, err := readMessage(out)
		if err != nil {
			break
		}
		var m rpcMsg
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
	_ = send(stdin, rpcMsg{Method: "initialized", Params: json.RawMessage(`{}`)})

	// 3) didOpen
	content, _ := os.ReadFile(file)
	doc := map[string]any{
		"textDocument": map[string]any{
			"uri":        "file://" + absFile,
			"languageId": "rust",
			"version":    1,
			"text":       string(content),
		},
	}
	dp, _ := json.Marshal(doc)
	_ = send(stdin, rpcMsg{Method: "textDocument/didOpen", Params: dp})

	// 4) 等 publishDiagnostics（最多 20 秒，rust-analyzer 需載入專案後才推非空診斷）
	deadline := time.After(20 * time.Second)
	for {
		select {
		case <-deadline:
			fmt.Println("(超時未收到非空診斷)")
			return
		default:
		}
		raw, err := readMessage(out)
		if err != nil {
			fmt.Println("(連線結束)")
			return
		}
		var m rpcMsg
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
