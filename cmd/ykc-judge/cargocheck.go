// cargo check / fix / explain 的接駁層（T-21a 後）：
// 實際 exec 與 JSON 解析已遷入 internal/toolchain（port = core.RustToolchain）。
// 本檔只保留 ykc-judge 的類型別名與注入點——「rustc 是唯一真相」公理不變：
// 所有錯誤與其位置一律取自 rustc 結構化輸出（或其忠實錄音帶），不由任何敘述產生。
package main

import (
	"context"

	"ykc/core"
	"ykc/internal/toolchain"
)

// tc 是本命令的工具鏈 port：生產期 = native；測試可注入 replay。
var tc core.RustToolchain = toolchain.NewNative()

// Error / CheckResult 沿用舊名（收據 JSON 欄位、l5.go、kb.go 均不受影響）。
type Error = core.RustError
type CheckResult = core.RustCheckResult

// cargoCheck 執行 cargo check 並解析結構化診斷。
func cargoCheck(dir string) (CheckResult, error) {
	return tc.Check(context.Background(), dir)
}

// cargoFix 套用 rustc 的 machine-applicable 建議（機械修復）。
func cargoFix(dir string) error {
	return tc.Fix(context.Background(), dir)
}

// explain 取官方錯誤說明（rustc --explain + Compiler Error Index 的線下版）。
func explain(dir, code string) string {
	return tc.Explain(context.Background(), dir, code)
}

// fingerprints 把錯誤集縮成指紋集（寫進事實帳本用）。
func fingerprints(errs []Error) []string {
	return toolchain.Fingerprints(errs)
}
