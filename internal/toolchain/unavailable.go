package toolchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"ykc/core"
)

// unmarshalStrict 是 replay fixture 的嚴格解碼（未知欄位即失敗，鎖定契約）。
func unmarshalStrict(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(v)
}

// Unavailable 是 T0 core-only 分發 adapter：如實申報缺席，一律降級不報錯。
// 所有查詢型操作回「空 + 錯誤」，證據型操作回「不可用 + 原因」——
// 呼叫方按 borrow.Analyzer 範式顯示「能力未安裝 + 安裝指引」。
type Unavailable struct {
	Reason string
}

// NewUnavailable 建立缺席工具鏈；reason 空則用預設指引。
func NewUnavailable(reason string) *Unavailable {
	if reason == "" {
		reason = "Rust 工具鏈未安裝（此為 ykc-core 精簡分發；安裝 rustup 或載入 ykc-pack-rust-toolchain）"
	}
	return &Unavailable{Reason: reason}
}

// Available 恆為不可用，附原因。
func (u *Unavailable) Available() (bool, string) { return false, u.Reason }

// Version 回錯誤。
func (u *Unavailable) Version(ctx context.Context) (core.ToolchainInfo, error) {
	return core.ToolchainInfo{}, fmt.Errorf("%s", u.Reason)
}

// Check 回錯誤（呼叫方降級）。
func (u *Unavailable) Check(ctx context.Context, dir string) (core.RustCheckResult, error) {
	return core.RustCheckResult{}, fmt.Errorf("%s", u.Reason)
}

// QuickCheck 回不可用證據。
func (u *Unavailable) QuickCheck(ctx context.Context, dir string) (bool, string) {
	return false, u.Reason
}

// Test 回不可用證據。
func (u *Unavailable) Test(ctx context.Context, dir string) (bool, string) {
	return false, u.Reason
}

// Fix 回錯誤。
func (u *Unavailable) Fix(ctx context.Context, dir string) error {
	return fmt.Errorf("%s", u.Reason)
}

// Explain 回空（無資料可解釋）。
func (u *Unavailable) Explain(ctx context.Context, dir, code string) string { return "" }

var _ core.RustToolchain = (*Unavailable)(nil)
