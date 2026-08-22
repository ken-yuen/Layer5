// Rust 工具鏈窄介面（T-21a，見 YKC_22）。
//
// 本檔補充 package core 的工具鏈 port 定義。
//
// 設計意圖：
//   - cargo / rustc / rust-analyzer 是「外部不可信依賴」，五層與 cmd 只依賴此 port，
//     不直接 exec；生產期注入 native adapter，測試期注入 replay adapter，
//     T0 core-only 分發注入 unavailable adapter（如實申報缺席，不報錯）。
//   - Available() 沿用 internal/borrow.Analyzer 範式：如實申報 + 降級，判定權不轉移。
//   - 「rustc 是唯一真相」公理不變：RustCheckResult 一律源自 cargo/rustc 的
//     結構化輸出（或其忠實錄音帶），不由任何敘述產生。

package core

import (
	"context"
	"strconv"
)

// ToolchainInfo 是工具鏈握手（ykc doctor / toolchain.attest 事實）的版本指紋。
type ToolchainInfo struct {
	Rustc        string `json:"rustc"`         // rustc -vV 首行
	RustcHash    string `json:"rustc_hash"`    // rustc -vV 的 commit-hash 行
	Cargo        string `json:"cargo"`         // cargo --version
	RustAnalyzer string `json:"rust_analyzer"` // rust-analyzer --version（可空 = 未安裝）
	CargoPath    string `json:"cargo_path"`    // 實際解析到的 cargo 路徑
	CargoSHA256  string `json:"cargo_sha256"`  // cargo 二進制指紋（供 attest 比對）
}

// RustError 是「可指紋化」的編譯錯誤（自 cmd/ykc-judge 遷移，語義不變）。
type RustError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Label   string `json:"label,omitempty"`
}

// Fingerprint 錯誤指紋：同碼同位置視為同一條（去重、追蹤修復前後）。
func (e RustError) Fingerprint() string {
	return e.Code + "@" + e.File + ":" + strconv.Itoa(e.Line)
}

// RustCheckResult 是一次 cargo check 的結構化結論。
type RustCheckResult struct {
	Errors   []RustError
	Warnings int
}

// RustToolchain 是對 cargo / rustc 的唯一出口（port）。
// 全部呼叫都可能失敗於「工具鏈缺席」——呼叫方必須先問 Available() 或容忍錯誤降級。
type RustToolchain interface {
	// Available 如實申報工具鏈是否可用；不可用時回傳人類可讀原因。
	Available() (bool, string)
	// Version 回傳工具鏈版本指紋（握手 / attest 用）。
	Version(ctx context.Context) (ToolchainInfo, error)
	// Check 執行 cargo check --message-format=json 並解析結構化診斷。
	Check(ctx context.Context, dir string) (RustCheckResult, error)
	// QuickCheck 執行 cargo check --quiet，只回傳可否編譯與 stderr 證據。
	QuickCheck(ctx context.Context, dir string) (ok bool, evidence string)
	// Test 執行 cargo test --quiet，只回傳是否通過與 stderr 證據。
	Test(ctx context.Context, dir string) (ok bool, evidence string)
	// Fix 套用 rustc 的 machine-applicable 建議（cargo fix）。
	Fix(ctx context.Context, dir string) error
	// Explain 取官方錯誤說明（rustc --explain E0502），限長截斷；失敗回空字串。
	Explain(ctx context.Context, dir, code string) string
}

// Diagnostic 是 LSP publishDiagnostics 的最小投影（前哨訊號，非判定）。
type Diagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"` // 1-based
	Col      int    `json:"col"`
	Severity int    `json:"severity"` // 1=error 2=warning（LSP 語義）
	Code     string `json:"code,omitempty"`
	Message  string `json:"message"`
}

// DiagnosticsProvider 是對 rust-analyzer 的唯一出口。
// 角色定位（YKC_22 §2.4）：低延遲前哨；裁判定論仍以 RustToolchain.Check 為準。
type DiagnosticsProvider interface {
	Available() (bool, string)
	Diagnostics(ctx context.Context, file string) ([]Diagnostic, error)
}
