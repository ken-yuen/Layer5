package toolchain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"ykc/core"
	"ykc/internal/rustutil"
)

// Native 是生產期 adapter：包裹真實 cargo / rustc。
// 這是全倉唯一允許直接 exec cargo/rustc 的地方（沙盒路徑除外）。
type Native struct {
	once   sync.Once
	avail  bool
	reason string
}

// NewNative 建立生產期工具鏈。
func NewNative() *Native { return &Native{} }

// Available 如實申報：cargo 與 rustc 都在 PATH 才算可用（惰性探測一次）。
func (n *Native) Available() (bool, string) {
	n.once.Do(func() {
		for _, tool := range []string{"cargo", "rustc"} {
			if _, err := exec.LookPath(tool); err != nil {
				n.avail, n.reason = false, tool+" 不在 PATH（安裝 rustup 或載入 ykc-pack-rust-toolchain）"
				return
			}
		}
		n.avail = true
	})
	return n.avail, n.reason
}

// Version 取版本指紋（握手 / toolchain.attest 事實用）。
func (n *Native) Version(ctx context.Context) (core.ToolchainInfo, error) {
	if ok, why := n.Available(); !ok {
		return core.ToolchainInfo{}, fmt.Errorf("工具鏈不可用: %s", why)
	}
	info := core.ToolchainInfo{}
	if so, _, c := rustutil.Run("", "rustc", "-vV"); c == 0 {
		for _, ln := range strings.Split(so, "\n") {
			ln = strings.TrimSpace(ln)
			switch {
			case strings.HasPrefix(ln, "rustc "):
				info.Rustc = ln
			case strings.HasPrefix(ln, "commit-hash:"):
				info.RustcHash = strings.TrimSpace(strings.TrimPrefix(ln, "commit-hash:"))
			}
		}
	}
	if so, _, c := rustutil.Run("", "cargo", "--version"); c == 0 {
		info.Cargo = strings.TrimSpace(so)
	}
	if p, err := exec.LookPath("cargo"); err == nil {
		info.CargoPath = p
		if b, err := os.ReadFile(p); err == nil {
			h := sha256.Sum256(b)
			info.CargoSHA256 = hex.EncodeToString(h[:])
		}
	}
	if _, err := exec.LookPath("rust-analyzer"); err == nil {
		if so, _, c := rustutil.Run("", "rust-analyzer", "--version"); c == 0 {
			info.RustAnalyzer = strings.TrimSpace(so)
		}
	}
	if info.Rustc == "" && info.Cargo == "" {
		return info, fmt.Errorf("rustc/cargo 均無法回報版本")
	}
	return info, nil
}

// Check 執行 cargo check --message-format=json 並解析結構化診斷。
func (n *Native) Check(ctx context.Context, dir string) (core.RustCheckResult, error) {
	if ok, why := n.Available(); !ok {
		return core.RustCheckResult{}, fmt.Errorf("工具鏈不可用: %s", why)
	}
	so, se, exit := rustutil.Run(dir, "cargo", "check", "--message-format=json")
	res := ParseCargoCheckJSON(so)
	// 若非 0 退出但沒解析到任何錯誤，視為工具鏈級失敗（語義沿襲 cargocheck.go）
	if exit != 0 && len(res.Errors) == 0 {
		return res, fmt.Errorf("cargo check 失敗 (exit %d): %s", exit, strings.TrimSpace(se))
	}
	return res, nil
}

// QuickCheck 執行 cargo check --quiet（僅退出碼 + stderr 證據）。
func (n *Native) QuickCheck(ctx context.Context, dir string) (bool, string) {
	if ok, why := n.Available(); !ok {
		return false, "工具鏈不可用: " + why
	}
	_, se, exit := rustutil.Run(dir, "cargo", "check", "--quiet")
	if exit == 0 {
		return true, ""
	}
	return false, strings.TrimSpace(se)
}

// Test 執行 cargo test --quiet。
func (n *Native) Test(ctx context.Context, dir string) (bool, string) {
	if ok, why := n.Available(); !ok {
		return false, "工具鏈不可用: " + why
	}
	_, se, exit := rustutil.Run(dir, "cargo", "test", "--quiet")
	if exit == 0 {
		return true, ""
	}
	return false, strings.TrimSpace(se)
}

// Fix 套用 rustc 的 machine-applicable 建議（機械修復）。
func (n *Native) Fix(ctx context.Context, dir string) error {
	if ok, why := n.Available(); !ok {
		return fmt.Errorf("工具鏈不可用: %s", why)
	}
	_, se, exit := rustutil.Run(dir, "cargo", "fix", "--allow-no-vcs", "--allow-dirty", "--broken-code")
	if exit != 0 {
		return fmt.Errorf("cargo fix exit %d: %s", exit, strings.TrimSpace(se))
	}
	return nil
}

// Explain 取官方錯誤說明（rustc --explain），限長 500；失敗回空字串。
func (n *Native) Explain(ctx context.Context, dir, code string) string {
	if code == "" {
		return ""
	}
	if ok, _ := n.Available(); !ok {
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

var _ core.RustToolchain = (*Native)(nil)
