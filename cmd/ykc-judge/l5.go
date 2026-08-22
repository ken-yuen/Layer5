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
	"os"
	"path/filepath"
	"strings"
	"time"

	"ykc/internal/borrow"
	"ykc/internal/ledger"
	"ykc/internal/rustutil"
)

const maxL5Explain = 3

// l5Explain 對剩餘錯誤產出幾何解釋。回傳解釋文本（空 = 無 borrow 類錯誤）。
//
// 每個錯誤碼的解釋 = 規則卡摘錄 + 拓撲。拓撲優先級：
//  1. 真實歸約——把用家錯誤現場的 fn 行級歸約為 .cl 並經引擎驗證
//     （幾何族命中才用; 附 sN←file:line 對照, 拓撲直接錨回用家源碼）；
//  2. 驗證不過/結構太複雜 → 回退 canonical 模板（寧缺勿錯）。
func l5Explain(dir string, remaining []Error, led *ledger.Ledger) string {
	// 收集 borrow 類錯誤（每碼取首個實例, 保序、有界）
	var picked []Error
	seen := map[string]bool{}
	for _, e := range remaining {
		if borrow.IsBorrowCode(e.Code) && !seen[e.Code] {
			seen[e.Code] = true
			picked = append(picked, e)
			if len(picked) == maxL5Explain {
				break
			}
		}
	}
	if len(picked) == 0 {
		// 無 borrow 類錯誤: 清除舊 L5 報告（避免 panel 顯示陳舊紅邊）
		borrow.RemoveReport(dir)
		return ""
	}

	analyzer := &borrow.Analyzer{}
	l5OK, l5Why := analyzer.Available()

	var codes []string
	var graphs []borrow.ConflictGraph
	reducedCount, redEdges := 0, 0
	var b strings.Builder
	b.WriteString("L5 借用幾何解釋（解釋非判定; 判定以 rustc 為準）\n")
	for _, e := range picked {
		codes = append(codes, e.Code)
		// ① 規則卡摘錄（靜態, 永遠可用）
		if entries := borrow.ByRustcCode(e.Code); len(entries) > 0 {
			b.WriteString(borrow.RenderCard(entries))
		}
		if !l5OK {
			fmt.Fprintf(&b, "（%s 的拓撲不可用: %s）\n", e.Code, l5Why)
			continue
		}
		// ② 真實歸約（優先）
		if r, red := tryReduce(dir, e, analyzer); r != nil {
			reducedCount++
			topo := borrow.BuildTopology(r)
			redEdges += topo.RedEdges()
			graphs = append(graphs, topo.ConflictGraphs()...)
			fmt.Fprintf(&b, "\n%s 的實際幾何（由 %s:%d 所在 fn 歸約, 已經引擎驗證）:\n",
				e.Code, filepath.Base(e.File), e.Line)
			b.WriteString(topo.RenderText())
			b.WriteString(red.MappingText())
			continue
		}
		// ③ 回退 canonical 模板
		tp := borrow.TemplateFor(e.Code)
		if tp == nil {
			continue
		}
		r, err := analyzer.AnalyzeSource(context.Background(), dir, tp.CL)
		if err != nil {
			fmt.Fprintf(&b, "（%s 樣例分析失敗: %v）\n", e.Code, err)
			continue
		}
		fmt.Fprintf(&b, "\n%s 的典型幾何形狀（canonical 樣例「%s」; 現場歸約未通過驗證, 已回退）:\n",
			e.Code, tp.Title)
		b.WriteString(borrow.BuildTopology(r).RenderText())
	}
	text := b.String()

	// ④ 帳本事實（解釋可審計; reduced 記錄真實歸約命中數——量測的基礎數據）
	appendFact(led, "borrow.analysis", "ykc-judge", map[string]any{
		"codes":              codes,
		"l5_available":       l5OK,
		"reduced":            reducedCount,
		"red_edges":          redEdges,
		"explanation_sha256": rustutil.SHA256Hex(text),
		"engine":             "chordlaw-v0.3",
	})

	// ⑤ L5 報告落盤（panel 讀取展示; sha256 已上帳本, 檔案可對賬防竄改）
	if err := borrow.WriteReport(dir, borrow.AnalysisReport{
		Codes: codes, Reduced: reducedCount, RedEdges: redEdges,
		L5Available: l5OK, Graphs: graphs,
		ExplanationSHA256: rustutil.SHA256Hex(text),
		Explanation:       text,
		UpdatedAt:         time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ L5 報告落盤失敗: %v\n", err)
	}
	return text
}

// tryReduce 嘗試真實歸約＋引擎驗證。任何失敗回 (nil,nil)——呼叫方回退模板。
func tryReduce(dir string, e Error, analyzer *borrow.Analyzer) (*borrow.Report, *borrow.Reduction) {
	if e.File == "" || e.Line <= 0 {
		return nil, nil
	}
	src := e.File
	if !filepath.IsAbs(src) {
		src = filepath.Join(dir, src)
	}
	red, err := borrow.ReduceFromFile(src, e.Line, e.Code)
	if err != nil {
		return nil, nil
	}
	r, valid := borrow.ValidateReduction(context.Background(), analyzer, dir, red, e.Code)
	if !valid {
		return nil, nil
	}
	return r, red
}
