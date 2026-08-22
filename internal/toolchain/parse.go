// Package toolchain 是 core.RustToolchain port 的實作集（T-21a，見 YKC_22）。
//
// 三個 adapter：
//   - Native      生產期：包裹真實 cargo/rustc（唯一允許 exec 工具鏈的地方）
//   - Replay      使用前：回放 golden fixture（真 cargo 輸出的錄音帶），零 Rust 依賴
//   - Unavailable T0 分發：如實申報缺席，一律降級不報錯
//
// 紀律（CI toolchain-guard 把關）：internal/toolchain 之外禁止直接 exec cargo/rustc
// （internal/precompile 走沙盒 runStage、internal/sandbox 為執行器，屬既有豁免）。
package toolchain

import (
	"encoding/json"
	"strings"

	"ykc/core"
)

// span / code / diagnostic / compilerMsg 對應 cargo --message-format=json 的
// 輸出結構（rustc 診斷 JSON）。契約測試（contract_test.go）鎖定此格式漂移。
type span struct {
	FileName  string `json:"file_name"`
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
	ColStart  int    `json:"column_start"`
	ColEnd    int    `json:"column_end"`
	IsPrimary bool   `json:"is_primary"`
	Label     string `json:"label"`
}

type code struct {
	Code string `json:"code"`
}

type diagnostic struct {
	Message  string       `json:"message"`
	Code     *code        `json:"code"`
	Level    string       `json:"level"`
	Spans    []span       `json:"spans"`
	Children []diagnostic `json:"children"`
}

type compilerMsg struct {
	Reason  string     `json:"reason"`
	Message diagnostic `json:"message"`
}

// ParseCargoCheckJSON 解析 cargo check --message-format=json 的 stdout 流。
// 自 cmd/ykc-judge/cargocheck.go 遷移，語義不變；獨立成純函數以便零工具鏈測試。
func ParseCargoCheckJSON(stdout string) core.RustCheckResult {
	res := core.RustCheckResult{}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var cm compilerMsg
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
		e := core.RustError{Message: m.Message}
		if m.Code != nil {
			e.Code = m.Code.Code
		}
		var primary *span
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
	return res
}

// Fingerprints 把錯誤集縮成指紋集（寫進事實帳本用）。
func Fingerprints(errs []core.RustError) []string {
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Fingerprint())
	}
	return out
}
