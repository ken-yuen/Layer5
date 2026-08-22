// attest.go — 工具鏈握手（T-21c，見 YKC_22 §2.3）：版本即事實，事實入帳本。

package serve

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ykc/core"
	"ykc/internal/domain"
	"ykc/internal/panel"
)

// ToolchainPolicy 的合法值。
const (
	PolicyStrict  = "strict"  // 工具鏈缺席/失配 → 拒絕啟動（生產 fail-fast）
	PolicyDegrade = "degrade" // 缺席/失配 → 照常啟動，結論標記降級（開發預設）
)

// attestPayload 是 toolchain.attest 事實的內容。
type attestPayload struct {
	core.ToolchainInfo
	Available       bool   `json:"available"`
	Reason          string `json:"reason,omitempty"`
	ExpectedChannel string `json:"expected_channel,omitempty"` // rust-toolchain.toml 的 channel
	ExpectedSource  string `json:"expected_source,omitempty"`  // 該檔案路徑
	Match           bool   `json:"match"`                      // rustc 版本是否命中 expected
	Policy          string `json:"policy"`
}

// expectedChannel 就近尋找 rust-toolchain.toml 並抽出 channel（找不到回空）。
// 順序：$YKC_RUST_TOOLCHAIN_TOML → <root>/rust-toolchain.toml → <root>/deploy/rust-toolchain.toml。
func expectedChannel(root string) (channel, source string) {
	cands := []string{}
	if p := os.Getenv("YKC_RUST_TOOLCHAIN_TOML"); p != "" {
		cands = append(cands, p)
	}
	cands = append(cands,
		filepath.Join(root, "rust-toolchain.toml"),
		filepath.Join(root, "deploy", "rust-toolchain.toml"),
	)
	re := regexp.MustCompile(`(?m)^\s*channel\s*=\s*"([^"]+)"`)
	for _, p := range cands {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if m := re.FindSubmatch(b); m != nil {
			return string(m[1]), p
		}
	}
	return "", ""
}

// buildAttest 執行一次工具鏈握手，產出 attest payload。
func buildAttest(ctx context.Context, tc core.RustToolchain, root, policy string) attestPayload {
	pay := attestPayload{Policy: policy}
	pay.ExpectedChannel, pay.ExpectedSource = expectedChannel(root)
	ok, why := tc.Available()
	pay.Available, pay.Reason = ok, why
	if !ok {
		return pay
	}
	info, err := tc.Version(ctx)
	if err != nil {
		pay.Available, pay.Reason = false, err.Error()
		return pay
	}
	pay.ToolchainInfo = info
	// match 判準：無 expected 視為 match（未鎖版環境不誤紅）；
	// 有 expected 則 rustc 版本行必須含該 channel 字串。
	pay.Match = pay.ExpectedChannel == "" || strings.Contains(info.Rustc, pay.ExpectedChannel)
	return pay
}

// attestAll 在 serve 啟動時對每個專案帳本寫入 toolchain.attest 事實，
// 並按 policy 決定是否放行。回傳錯誤 = strict 拒絕啟動。
func (s *Server) attestAll(ctx context.Context, projects []string) error {
	pay := buildAttest(ctx, s.tc, s.cfg.Root, s.cfg.ToolchainPolicy)
	for _, p := range projects {
		if err := s.appendProjectEvent(p, domain.EventToolchainAttest, pay); err != nil {
			log.Printf("serve: toolchain.attest 寫入失敗（%s）: %v", p, err)
		}
	}
	switch {
	case !pay.Available && s.cfg.ToolchainPolicy == PolicyStrict:
		return fmt.Errorf("serve: toolchain policy=strict 但工具鏈不可用: %s（用 ykc-doctor 檢查）", pay.Reason)
	case !pay.Match && s.cfg.ToolchainPolicy == PolicyStrict:
		return fmt.Errorf("serve: toolchain policy=strict 但版本失配: 實際 %q ≠ 鎖定 %q（%s）",
			pay.Rustc, pay.ExpectedChannel, pay.ExpectedSource)
	case !pay.Available:
		log.Printf("serve: ⚠️ 工具鏈不可用（policy=degrade 放行）: %s", pay.Reason)
	case !pay.Match:
		log.Printf("serve: ⚠️ 工具鏈版本失配（policy=degrade 放行）: 實際 %q ≠ 鎖定 %q", pay.Rustc, pay.ExpectedChannel)
	default:
		log.Printf("serve: toolchain.attest ✅ %s / %s", pay.Rustc, pay.Cargo)
	}
	return nil
}

// handleToolchain — GET /api/toolchain：即時握手結果（唯讀；Trust Console 顯示用）。
func (s *Server) handleToolchain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	panel.WriteJSON(w, buildAttest(r.Context(), s.tc, s.cfg.Root, s.cfg.ToolchainPolicy))
}
