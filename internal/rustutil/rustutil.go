// Package rustutil 收斂 YKC 對「Rust 工具鏈 / 專案」的通用操作，消除各命令重複實作。
package rustutil

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Run 在 dir 內執行命令，回傳 stdout/stderr/exitCode；啟動失敗回 -1 並把錯誤寫入 stderr。
// 舊呼叫端沒有 context 時使用此便利包裝；有上限要求的路徑應使用 RunContext。
func Run(dir, name string, args ...string) (stdout, stderr string, exitCode int) {
	return RunContext(context.Background(), dir, name, args...)
}

// RunContext is the context-aware command primitive shared by toolchain
// adapters. The previous Run implementation ignored cancellation, so a cargo
// process could outlive a request or test timeout indefinitely.
func RunContext(ctx context.Context, dir, name string, args ...string) (stdout, stderr string, exitCode int) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	if err != nil {
		if ctx.Err() != nil {
			se.WriteString("exec context: " + ctx.Err().Error())
			return so.String(), se.String(), -1
		}
		if cmd.ProcessState == nil {
			se.WriteString("exec error: " + err.Error())
			return so.String(), se.String(), -1
		}
	}
	if cmd.ProcessState == nil {
		return so.String(), se.String(), -1
	}
	return so.String(), se.String(), cmd.ProcessState.ExitCode()
}

// PackageName 從 Cargo.toml 讀取 crate 名稱。
func PackageName(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "Cargo.toml"))
	if err != nil {
		return ""
	}
	m := regexp.MustCompile(`(?m)^\s*name\s*=\s*"([^"]+)"`).FindSubmatch(b)
	if m != nil {
		return string(m[1])
	}
	return ""
}

// Subcommands 從 clap 的 --help 輸出「Commands:」段落枚舉子命令（跳過自動生成的 help）。
func Subcommands(helpText string) []string {
	idx := strings.Index(helpText, "Commands:")
	if idx < 0 {
		return nil
	}
	var cmds []string
	for _, ln := range strings.Split(helpText[idx+len("Commands:"):], "\n") {
		if strings.HasPrefix(ln, "  ") && strings.TrimSpace(ln) != "" {
			f := strings.Fields(strings.TrimSpace(ln))
			if len(f) > 0 && f[0] != "help" {
				cmds = append(cmds, f[0])
			}
		} else if strings.TrimSpace(ln) == "" {
			continue
		} else {
			break // 區塊結束
		}
	}
	return cmds
}

// PanicDetected 偵測 stderr 中的 Rust panic 訊號。
func PanicDetected(stderr string) bool {
	return strings.Contains(stderr, "panicked at") || strings.Contains(stderr, "thread 'main' panicked")
}

// FirstLine 截斷訊息至第一行並限長（供證據/摘要顯示）。
func FirstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	return s
}

// Sign 對訊息做 HMAC-SHA256（收據簽名）。
func Sign(msg, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

// SHA256Hex 回傳輸入的十六進位 SHA-256。
func SHA256Hex(b string) string {
	s := sha256.Sum256([]byte(b))
	return hex.EncodeToString(s[:])
}
