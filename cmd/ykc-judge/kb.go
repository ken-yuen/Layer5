package main

import (
	"fmt"
	"strings"

	"ykc/internal/kb"
	"ykc/internal/rustutil"
)

const (
	kbContextDepth  = 2
	kbContextBudget = 12_000
	maxKBUsages     = 16
)

// kbUsage 是一個 rustc 錯誤碼在本輪 judge 中實際取用的知識閉包。AtomIDs 與
// ContextSHA256 讓審計者能以資料集版本重放「當時交給代理的上下文」。
type kbUsage struct {
	Code          string   `json:"code"`
	RootAtomID    string   `json:"root_atom_id"`
	AtomIDs       []string `json:"atom_ids"`
	ContextSHA256 string   `json:"context_sha256"`
}

// kbAnalysis 是寫入 fact ledger 的知識面 provenance。它與 borrow.analysis 一樣
// 是解釋/輔助證據，不參與 rustc 的判定權。
type kbAnalysis struct {
	KBVersion        string    `json:"kb_version"`
	RustcVersion     string    `json:"rustc_version,omitempty"`
	Source           string    `json:"source"`
	ErrorIndexURL    string    `json:"error_index_url,omitempty"`
	ErrorIndexSHA256 string    `json:"error_index_sha256,omitempty"`
	Usages           []kbUsage `json:"usages"`
	Truncated        bool      `json:"truncated,omitempty"`
}

// judgeKnowledge 保留顯示層所需的 per-code lookup，同時提供可直接上帳本的 payload。
type judgeKnowledge struct {
	analysis kbAnalysis
	byCode   map[string]kbUsage
}

func openJudgeKnowledge(path string) (*kb.Store, error) {
	if strings.TrimSpace(path) == "" {
		return kb.Open()
	}
	return kb.OpenFile(path)
}

// buildJudgeKnowledge 從實際剩餘 rustc 診斷中選取唯一錯誤碼，對每個碼做精確 KB
// Retrieve（答案 + 規則 + 出處閉包）。輸出上限避免一次壞掉的專案把帳本塞滿。
func buildJudgeKnowledge(st *kb.Store, remaining []Error) judgeKnowledge {
	meta := st.Metadata()
	out := judgeKnowledge{
		analysis: kbAnalysis{
			KBVersion:        st.Version(),
			RustcVersion:     meta.RustcVersion,
			Source:           st.Source(),
			ErrorIndexURL:    meta.ErrorIndexURL,
			ErrorIndexSHA256: meta.ErrorIndexSHA256,
		},
		byCode: map[string]kbUsage{},
	}
	seen := map[string]bool{}
	for _, diagnostic := range remaining {
		code := strings.ToUpper(strings.TrimSpace(diagnostic.Code))
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		if len(out.analysis.Usages) == maxKBUsages {
			out.analysis.Truncated = true
			break
		}
		root, ok := st.ByCode(code)
		if !ok || root.Kind != kb.KindError {
			continue
		}
		bundle := st.Retrieve(code, kb.SearchOpts{
			K:           1,
			ExpandDepth: kbContextDepth,
			BudgetBytes: kbContextBudget,
		})
		ids := make([]string, 0, len(bundle.Atoms))
		for _, atom := range bundle.Atoms {
			ids = append(ids, atom.ID)
		}
		if len(ids) == 0 {
			// root 已存在卻被預算完全截斷只會是異常配置；不把「未提供內容」
			// 偽裝成已使用的 KB。
			continue
		}
		usage := kbUsage{
			Code:          code,
			RootAtomID:    root.ID,
			AtomIDs:       ids,
			ContextSHA256: rustutil.SHA256Hex(kb.RenderMarkdown(bundle)),
		}
		out.analysis.Usages = append(out.analysis.Usages, usage)
		out.byCode[code] = usage
	}
	return out
}

func (k judgeKnowledge) usageFor(code string) (kbUsage, bool) {
	u, ok := k.byCode[strings.ToUpper(strings.TrimSpace(code))]
	return u, ok
}

func (k judgeKnowledge) summaryFor(code string) string {
	u, ok := k.usageFor(code)
	if !ok {
		return ""
	}
	rustc := k.analysis.RustcVersion
	if rustc == "" {
		rustc = "legacy/unknown"
	}
	return fmt.Sprintf("KB rustc=%s、dataset=%s、%d atoms、context=%s", rustc, k.analysis.KBVersion, len(u.AtomIDs), u.ContextSHA256)
}
