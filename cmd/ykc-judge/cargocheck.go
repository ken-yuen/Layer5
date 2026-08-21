// cargo check --message-format=json 解析器 + 錯誤指紋（L4 除錯閉環的地基）。
// 「rustc 是唯一真相」：所有錯誤與其位置一律取自 rustc 輸出，不由任何敘述產生。
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"ykc/internal/rustutil"
)

type Span struct {
	FileName  string `json:"file_name"`
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
	ColStart  int    `json:"column_start"`
	ColEnd    int    `json:"column_end"`
	IsPrimary bool   `json:"is_primary"`
	Label     string `json:"label"`
}

type Code struct {
	Code string `json:"code"`
}

type Diagnostic struct {
	Message  string       `json:"message"`
	Code     *Code        `json:"code"`
	Level    string       `json:"level"`
	Spans    []Span       `json:"spans"`
	Children []Diagnostic `json:"children"`
}

type CompilerMsg struct {
	Reason  string     `json:"reason"`
	Message Diagnostic `json:"message"`
}

// Error 是「可指紋化」的編譯錯誤。
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Label   string `json:"label,omitempty"`
}

// Fingerprint 錯誤指紋：同碼同位置的錯誤視為同一條（去重、追蹤修復前後）。
func (e Error) Fingerprint() string {
	return fmt.Sprintf("%s@%s:%d", e.Code, e.File, e.Line)
}

type CheckResult struct {
	Errors   []Error
	Warnings int
}

// cargoCheck 執行 cargo check 並解析結構化診斷。
func cargoCheck(dir string) (CheckResult, error) {
	so, se, code := rustutil.Run(dir, "cargo", "check", "--message-format=json")
	res := CheckResult{}
	for _, line := range strings.Split(so, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var cm CompilerMsg
		if err := json.Unmarshal([]byte(line), &cm); err != nil {
			continue
		}
		if cm.Reason != "compiler-message" {
			continue
		}
		m := cm.Message
		if m.Level == "warning" {
			res.Warnings++
			continue
		}
		if m.Level != "error" {
			continue
		}
		e := Error{Message: m.Message}
		if m.Code != nil {
			e.Code = m.Code.Code
		}
		var primary *Span
		for i := range m.Spans {
			if m.Spans[i].IsPrimary {
				primary = &m.Spans[i]
				break
			}
		}
		if primary == nil && len(m.Spans) > 0 {
			primary = &m.Spans[0]
		}
		if primary != nil {
			e.File = primary.FileName
			e.Line = primary.LineStart
			e.Col = primary.ColStart
			e.Label = primary.Label
		}
		res.Errors = append(res.Errors, e)
	}
	// 若非 0 退出但沒解析到任何錯誤，視為工具鏈級失敗
	if code != 0 && len(res.Errors) == 0 {
		return res, fmt.Errorf("cargo check 失敗 (exit %d): %s", code, strings.TrimSpace(se))
	}
	return res, nil
}

// cargoFix 套用 rustc 的 machine-applicable 建議（機械修復）。
func cargoFix(dir string) error {
	_, se, code := rustutil.Run(dir, "cargo", "fix", "--allow-no-vcs", "--allow-dirty", "--broken-code")
	if code != 0 {
		return fmt.Errorf("cargo fix exit %d: %s", code, strings.TrimSpace(se))
	}
	return nil
}

// explain 取官方錯誤說明（rustc --explain + Compiler Error Index 的線下版）。
func explain(dir, code string) string {
	if code == "" {
		return ""
	}
	so, _, c := rustutil.Run(dir, "rustc", "--explain", code)
	if c != 0 || so == "" {
		return ""
	}
	if len(so) > 500 {
		so = so[:500] + "…"
	}
	return so
}

// fingerprints 把錯誤集縮成指紋集（寫進事實帳本用）。
func fingerprints(errs []Error) []string {
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Fingerprint())
	}
	return out
}
