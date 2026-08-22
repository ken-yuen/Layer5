// L5 掛鉤：剩餘錯誤中的 borrow 類 → 幾何解釋（規則卡 + canonical 樣例拓撲）。
//
// 紀律（決策 D22）：
//   - L5 輸出是 explanation, 判定以 rustc 為準（本檔不觸碰 overall/receipt）；
//   - 解釋寫入帳本 type="borrow.analysis"（含 sha256——解釋本身可審計）；
//   - 有界輸出: 最多解釋前 maxL5Explain 個不同錯誤碼；
//   - L5 引擎（python3+chordlaw）缺席時降級為純規則卡（靜態, 永遠可用）。
package main

import (
	"context"
	"fmt"
	"strings"

	"ykc/internal/borrow"
	"ykc/internal/ledger"
	"ykc/internal/rustutil"
)

const maxL5Explain = 3

// l5Explain 對剩餘錯誤產出幾何解釋。回傳解釋文本（空 = 無 borrow 類錯誤）。
func l5Explain(dir string, remaining []Error, led *ledger.Ledger) string {
	// 收集 borrow 類錯誤碼（去重、保序、有界）
	var codes []string
	seen := map[string]bool{}
	for _, e := range remaining {
		if borrow.IsBorrowCode(e.Code) && !seen[e.Code] {
			seen[e.Code] = true
			codes = append(codes, e.Code)
			if len(codes) == maxL5Explain {
				break
			}
		}
	}
	if len(codes) == 0 {
		return ""
	}

	analyzer := &borrow.Analyzer{}
	l5OK, l5Why := analyzer.Available()

	var b strings.Builder
	b.WriteString("L5 借用幾何解釋（解釋非判定; 判定以 rustc 為準）\n")
	for _, code := range codes {
		// ① 規則卡摘錄（靜態, 永遠可用）
		if entries := borrow.ByRustcCode(code); len(entries) > 0 {
			b.WriteString(borrow.RenderCard(entries))
		}
		// ② canonical 樣例拓撲（需引擎）
		tp := borrow.TemplateFor(code)
		if tp == nil {
			continue
		}
		if !l5OK {
			fmt.Fprintf(&b, "（%s 的樣例拓撲不可用: %s）\n", code, l5Why)
			continue
		}
		r, err := analyzer.AnalyzeSource(context.Background(), dir, tp.CL)
		if err != nil {
			fmt.Fprintf(&b, "（%s 樣例分析失敗: %v）\n", code, err)
			continue
		}
		fmt.Fprintf(&b, "\n%s 的典型幾何形狀（canonical 樣例「%s」）:\n", code, tp.Title)
		b.WriteString(borrow.BuildTopology(r).RenderText())
	}
	text := b.String()

	// ③ 帳本事實（解釋可審計）
	appendFact(led, "borrow.analysis", "ykc-judge", map[string]any{
		"codes":              codes,
		"l5_available":       l5OK,
		"explanation_sha256": rustutil.SHA256Hex(text),
		"engine":             "chordlaw-v0.3",
	})
	return text
}
