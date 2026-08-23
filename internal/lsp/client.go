// Package lsp — LSP stdio JSON-RPC 客戶端基建（T-21d，見 YKC_22 §2.4）。
//
// 自 cmd/ykc-lsp 抽取框定（Content-Length）與握手邏輯，供兩處復用：
//   - cmd/ykc-lsp   一次性驗證（T-09 命脈驗證，行為不變）
//   - cmd/ykc-rustd 常駐 daemon（initialize 一次，之後毫秒級取診斷）
//
// 角色定位不變：LSP 診斷只是低延遲「前哨」，裁判定論仍以 cargo check 為準。
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Msg 是一則 JSON-RPC 2.0 訊息（請求/回應/通知共用）。
type Msg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

// Send 以 LSP 標準 Content-Length 框定送出訊息。
func Send(w io.Writer, m Msg) error {
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

// MaxContentLength bounds one LSP frame. A rust-analyzer process is local, but
// treating its stdout as untrusted input prevents a malformed header from
// turning into a negative-length panic or an unbounded allocation.
const MaxContentLength = 16 * 1024 * 1024

// maxHeaderLine is deliberately much smaller than MaxContentLength: headers
// contain metadata only and must not be allowed to grow without a bound.
const maxHeaderLine = 8 * 1024

func readHeaderLine(r *bufio.Reader) (string, error) {
	var line []byte
	for {
		part, prefix, err := r.ReadLine()
		if err != nil {
			return "", err
		}
		if len(line)+len(part) > maxHeaderLine {
			return "", fmt.Errorf("lsp header line exceeds %d bytes", maxHeaderLine)
		}
		line = append(line, part...)
		if !prefix {
			return strings.TrimRight(string(line), "\r"), nil
		}
	}
}

// ReadMessage 依 Content-Length 讀出完整一則 LSP 訊息。
func ReadMessage(r *bufio.Reader) ([]byte, error) {
	contentLen := 0
	seenLength := false
	for {
		line, err := readHeaderLine(r)
		if err != nil {
			return nil, err
		}
		if line == "" {
			break
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "content-length:") {
			if seenLength {
				return nil, fmt.Errorf("duplicate content-length")
			}
			seenLength = true
			v := strings.TrimSpace(line[len("content-length:"):])
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("invalid content-length %q", v)
			}
			if n > MaxContentLength {
				return nil, fmt.Errorf("content-length %d exceeds %d", n, MaxContentLength)
			}
			contentLen = int(n)
		}
	}
	if !seenLength || contentLen <= 0 {
		return nil, fmt.Errorf("no content-length")
	}
	buf := make([]byte, contentLen)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}
