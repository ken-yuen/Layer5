package kb

import (
	"fmt"
	"strings"
)

// SearchOpts 是檢索 + 上下文組裝的參數。
type SearchOpts struct {
	K           int // 檢索命中數（<=0 → 8）
	ExpandDepth int // 依賴項圖展開深度（<0 → 2）
	BudgetBytes int // 上下文預算（<=0 → 12000 bytes ≈ 3000 tokens）
}

// ContextBundle 是「代理上下文」：檢索命中 + 依賴項展開 + 預算截斷後的自足上下文。
type ContextBundle struct {
	Query       string  `json:"query"`
	Version     string  `json:"version"` // 資料版本（同查詢同版本 → 可緩存命中）
	Hits        []Hit   `json:"hits"`
	Atoms       []*Atom `json:"atoms"` // 已去重、依相關性排序、預算內
	TotalBytes  int     `json:"total_bytes"`
	BudgetBytes int     `json:"budget_bytes"`
	Truncated   bool    `json:"truncated"`
	EstTokens   int     `json:"est_tokens"`
	CacheHit    bool    `json:"cache_hit"`
}

// Retrieve 是代理的主入口：精準檢索 → 依賴項圖展開 → 預算截斷組裝，
// 並經上下文緩存（查詢指紋 + 資料版本）去重。決定論：同輸入必同輸出。
func (s *Store) Retrieve(q string, o SearchOpts) *ContextBundle {
	if o.K <= 0 {
		o.K = 8
	}
	if o.ExpandDepth < 0 {
		o.ExpandDepth = 2
	}
	if o.BudgetBytes <= 0 {
		o.BudgetBytes = 12000
	}

	key := fmt.Sprintf("%s\x00%s\x00%d\x00%d\x00%d", s.version, q, o.K, o.ExpandDepth, o.BudgetBytes)
	if b, ok := s.cache.get(key); ok {
		b.CacheHit = true
		return b
	}
	if b, ok := s.persistent.get(key, s); ok {
		// Disk entry 已以資料版本、key digest、原子 ID 重建／校驗，再送進 LRU；
		// 之後相同程序的查詢不再碰磁碟。
		s.cache.put(key, b, int64(b.TotalBytes)+256)
		return b
	}

	hits := s.Search(q, o.K)
	roots := make([]string, 0, len(hits))
	for _, h := range hits {
		roots = append(roots, h.Atom.ID)
	}
	order := s.graph.ExpandContext(roots, o.ExpandDepth)

	atoms := make([]*Atom, 0, len(order))
	total := 0
	truncated := false
	for _, id := range order {
		a, ok := s.byID[id]
		if !ok {
			continue
		}
		if total+a.Size() > o.BudgetBytes && len(atoms) > 0 {
			truncated = true
			break
		}
		atoms = append(atoms, a)
		total += a.Size()
	}

	bundle := &ContextBundle{
		Query:       q,
		Version:     s.version,
		Hits:        hits,
		Atoms:       atoms,
		TotalBytes:  total,
		BudgetBytes: o.BudgetBytes,
		Truncated:   truncated,
		EstTokens:   total / 4,
	}
	s.cache.put(key, bundle, int64(total)+256)
	s.persistent.put(key, bundle)
	return bundle
}

// RenderMarkdown 把 ContextBundle 渲染為可貼入 LLM 提示的緊湊 Markdown。
func RenderMarkdown(b *ContextBundle) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# YKC 知識庫上下文（query: %s）\n", b.Query)
	fmt.Fprintf(&sb, "> version=%s · 命中 %d · 上下文 %d bytes ≈ %d tokens%s\n\n",
		b.Version, len(b.Hits), b.TotalBytes, b.EstTokens, truncNote(b.Truncated))

	for i, a := range b.Atoms {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		writeAtomMD(&sb, a)
	}
	return sb.String()
}

func truncNote(t bool) string {
	if t {
		return " · ⚠ 超出預算已截斷"
	}
	return ""
}

func writeAtomMD(sb *strings.Builder, a *Atom) {
	switch a.Kind {
	case KindError:
		fmt.Fprintf(sb, "## %s — %s\n", a.Code, a.Title)
		if a.ZH != "" {
			fmt.Fprintf(sb, "**繁中摘要**：%s\n", a.ZH)
		}
		if a.Body != "" {
			fmt.Fprintf(sb, "**說明**\n%s\n", trimCodeBlock(a.Body))
		}
		if a.Err != "" {
			fmt.Fprintf(sb, "**錯誤範例**\n```rust\n%s\n```\n", a.Err)
		}
		if a.Fix != "" {
			fmt.Fprintf(sb, "**正解**\n```rust\n%s\n```\n", a.Fix)
		}
		if a.Source != "" {
			fmt.Fprintf(sb, "出處：%s\n", a.Source)
		}
	case KindRule:
		fmt.Fprintf(sb, "## 規則 %s（%s）— %s\n", a.Code, a.Domain, a.Title)
		if a.ZH != "" {
			fmt.Fprintf(sb, "**規則**：%s\n", a.ZH)
		}
		if a.Why != "" {
			fmt.Fprintf(sb, "**為什麼**：%s\n", a.Why)
		}
		if len(a.Fixes) > 0 {
			sb.WriteString("**修法**\n")
			for i, f := range a.Fixes {
				fmt.Fprintf(sb, "%d. %s\n", i+1, f)
			}
		}
	case KindBook, KindPart, KindTOC:
		fmt.Fprintf(sb, "## %s\n", a.Title)
		if a.Body != "" {
			fmt.Fprintf(sb, "%s\n", trimCodeBlock(a.Body))
		}
		if len(a.Sections) > 0 {
			sb.WriteString("**小節**\n")
			for _, sec := range a.Sections {
				fmt.Fprintf(sb, "- %s\n", sec.Title)
			}
		}
		if a.Source != "" {
			fmt.Fprintf(sb, "出處：%s\n", a.Source)
		}
	}
}

func trimCodeBlock(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 1600 {
		s = s[:1600] + "…"
	}
	return s
}
