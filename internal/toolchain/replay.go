package toolchain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"ykc/core"
)

// Replay 是使用前（測試/無 Rust CI）adapter：回放 golden fixture。
// fixture 佈局（Dir 之下）：
//
//	check.json      cargo check --message-format=json 的 stdout 原文（錄音帶）
//	version.json    core.ToolchainInfo 的 JSON（可缺；缺則回合成版本）
//	explain-<CODE>.txt  rustc --explain 的 stdout（如 explain-E0502.txt）
//	quickcheck.ok / test.ok  存在即代表通過；否則讀 *.err 作證據
//
// 錄音帶由 toolchain-lane CI 以鎖版 cargo 錄製；格式漂移先在契約測試爆。
type Replay struct {
	Dir string
}

// NewReplay 建立回放工具鏈。
func NewReplay(dir string) *Replay { return &Replay{Dir: dir} }

// Available 恆為可用（它就是錄音帶）。
func (r *Replay) Available() (bool, string) { return true, "" }

// Version 讀 version.json；缺檔回合成指紋（測試場景足夠）。
func (r *Replay) Version(ctx context.Context) (core.ToolchainInfo, error) {
	b, err := os.ReadFile(filepath.Join(r.Dir, "version.json"))
	if err != nil {
		return core.ToolchainInfo{Rustc: "rustc (replay)", Cargo: "cargo (replay)"}, nil
	}
	var info core.ToolchainInfo
	if err := unmarshalStrict(b, &info); err != nil {
		return core.ToolchainInfo{}, fmt.Errorf("version.json 解析失敗: %w", err)
	}
	return info, nil
}

// Check 回放 check.json 錄音帶。
func (r *Replay) Check(ctx context.Context, dir string) (core.RustCheckResult, error) {
	b, err := os.ReadFile(filepath.Join(r.Dir, "check.json"))
	if err != nil {
		return core.RustCheckResult{}, fmt.Errorf("replay fixture 缺失: %w", err)
	}
	return ParseCargoCheckJSON(string(b)), nil
}

// QuickCheck 以標記檔決定結果。
func (r *Replay) QuickCheck(ctx context.Context, dir string) (bool, string) {
	if _, err := os.Stat(filepath.Join(r.Dir, "quickcheck.ok")); err == nil {
		return true, ""
	}
	return false, r.readErr("quickcheck.err")
}

// Test 以標記檔決定結果。
func (r *Replay) Test(ctx context.Context, dir string) (bool, string) {
	if _, err := os.Stat(filepath.Join(r.Dir, "test.ok")); err == nil {
		return true, ""
	}
	return false, r.readErr("test.err")
}

// Fix 回放場景視為 no-op 成功（機械修復的效果由 after fixture 表達）。
func (r *Replay) Fix(ctx context.Context, dir string) error { return nil }

// Explain 讀 explain-<CODE>.txt。
func (r *Replay) Explain(ctx context.Context, dir, code string) string {
	if code == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(r.Dir, "explain-"+code+".txt"))
	if err != nil {
		return ""
	}
	return string(b)
}

func (r *Replay) readErr(name string) string {
	b, err := os.ReadFile(filepath.Join(r.Dir, name))
	if err != nil {
		return "replay: 標記為失敗（無證據檔）"
	}
	return string(b)
}

var _ core.RustToolchain = (*Replay)(nil)
