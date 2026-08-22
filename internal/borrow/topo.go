package borrow

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ============================================================
// 軌 A：代理可讀幾何 — 文字拓撲 + 區間代數 + 衝突圖 + 修法提示
//
// 設計決策 B2（線性化近似）：
//   區間以陳述序號線性化計算重疊。這對「直線最小樣例」精確；
//   對含迴圈/分支的控制流是近似。因此：
//   - AlgebraFact.Illegal 只在「線性重疊 ∧ 含 mut ∧ 路徑衝突 ∧
//     引擎報告確有錯誤佐證（同 fn 有 borrow 類錯誤）」時為 true；
//   - 無佐證的重疊只標 Suspect（語氣降級），錯誤判定一律以
//     Report.Errors（Datalog 引擎, 控制流精確）為準。
// ============================================================

// Topology 是整份報告的幾何視圖（按 fn 分組）。
type Topology struct {
	File     string
	Liveness string
	Verdict  string
	Fns      []FnTopology
}

// FnTopology 是單一 fn 的區間幾何。
type FnTopology struct {
	Fn    string
	Lo    int           // 軸起點（陳述序號）
	Hi    int           // 軸終點
	Rows  []IntervalRow // 每借用一行（按出借點排序）
	Facts []AlgebraFact // 區間代數事實
	Fixes []FixHint     // 幾何修法（由錯誤點反推）
}

// IntervalRow 是一條借用區間行。
type IntervalRow struct {
	Ref      string
	Referent string
	Kind     string // sh | mut
	Start    int
	End      int
	ErrAt    []int // 落在本區間內的錯誤點（渲染 X 標記）
}

// AlgebraFact 是兩借用的區間關係（機器可讀，供代理組合推理）。
type AlgebraFact struct {
	A, B    string // 參考名
	Rel     string // OVERLAP[sX..sY] | DISJOINT
	Illegal bool   // 線性重疊 ∧ 含mut ∧ 路徑衝突 ∧ 引擎錯誤佐證
	Suspect bool   // 同上但無引擎佐證（近似告警，非判定）
	Note    string
}

// FixHint 是從幾何直接推導的修法。
type FixHint struct {
	Code string // E01…
	Stmt string // 錯誤點
	Hint string // 拓撲操作描述
}

// BuildTopology 由引擎報告構建幾何視圖。
func BuildTopology(r *Report) *Topology {
	t := &Topology{File: r.File, Liveness: r.Liveness, Verdict: r.Verdict}

	// fn → regions / errors 分組
	fnRegions := map[string][]Region{}
	var fnOrder []string
	for _, reg := range r.Regions {
		if _, seen := fnRegions[reg.Fn]; !seen {
			fnOrder = append(fnOrder, reg.Fn)
		}
		fnRegions[reg.Fn] = append(fnRegions[reg.Fn], reg)
	}
	fnErrors := map[string][]Error{}
	for _, e := range r.Errors {
		fnErrors[e.Fn] = append(fnErrors[e.Fn], e)
		if _, seen := fnRegions[e.Fn]; !seen && e.Fn != "" {
			fnOrder = append(fnOrder, e.Fn) // 有錯但無區間的 fn 也要有節
		}
	}

	for _, fn := range fnOrder {
		regs, errs := fnRegions[fn], fnErrors[fn]
		ft := FnTopology{Fn: fn}

		// 軸範圍
		lo, hi := 1<<30, 0
		upd := func(n int) {
			if n < lo {
				lo = n
			}
			if n > hi {
				hi = n
			}
		}
		for _, g := range regs {
			upd(sid(g.Start))
			upd(sid(g.End))
		}
		for _, e := range errs {
			if n := sid(e.Stmt); n > 0 {
				upd(n)
			}
		}
		if hi == 0 { // 無任何座標
			continue
		}
		ft.Lo, ft.Hi = lo, hi

		// 區間行
		sorted := append([]Region(nil), regs...)
		sort.Slice(sorted, func(i, j int) bool { return sid(sorted[i].Start) < sid(sorted[j].Start) })
		for _, g := range sorted {
			row := IntervalRow{Ref: g.Ref, Referent: g.Referent, Kind: g.Kind,
				Start: sid(g.Start), End: sid(g.End)}
			for _, e := range errs {
				n := sid(e.Stmt)
				if n >= row.Start && n <= row.End {
					row.ErrAt = append(row.ErrAt, n)
				}
			}
			ft.Rows = append(ft.Rows, row)
		}

		// 區間代數（pair-wise）
		hasEngineErr := len(errs) > 0
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				g1, g2 := sorted[i], sorted[j]
				if !pathConflict(g1.Referent, g2.Referent) {
					continue // 不同路徑（split borrow）: 無關係可言
				}
				a1, b1 := sid(g1.Start), sid(g1.End)
				a2, b2 := sid(g2.Start), sid(g2.End)
				oLo, oHi := max(a1, a2), min(b1, b2)
				f := AlgebraFact{A: g1.Ref, B: g2.Ref}
				if oLo <= oHi {
					f.Rel = fmt.Sprintf("OVERLAP[s%d..s%d]", oLo, oHi)
					hasMut := g1.Kind == "mut" || g2.Kind == "mut"
					switch {
					case hasMut && hasEngineErr:
						f.Illegal = true
						f.Note = "違法: 路徑衝突之重疊且含 mut（引擎已證: 見 errors）"
					case hasMut:
						f.Suspect = true
						f.Note = "近似告警: 線性重疊含 mut, 但引擎未證違法（控制流上可能不相交）"
					default:
						f.Note = "合法: sh 與 sh 可共存"
					}
				} else {
					f.Rel = "DISJOINT"
					f.Note = "合法: 區間不相交（NLL 順序借用）"
				}
				ft.Facts = append(ft.Facts, f)
			}
		}

		// 幾何修法：錯誤點 → 覆蓋該點的「他者」區間 → 拓撲操作
		for _, e := range errs {
			ft.Fixes = append(ft.Fixes, deriveFixes(e, sorted)...)
		}

		t.Fns = append(t.Fns, ft)
	}
	return t
}

// deriveFixes 由錯誤碼類型與覆蓋錯誤點的區間推導修法（封閉菜單）。
func deriveFixes(e Error, regs []Region) []FixHint {
	n := sid(e.Stmt)
	var out []FixHint
	add := func(h string) { out = append(out, FixHint{Code: e.Code, Stmt: e.Stmt, Hint: h}) }

	// 覆蓋錯誤點、且不是在錯誤點才誕生的區間 = 「擋路的弧」
	var blockers []Region
	for _, g := range regs {
		if s, en := sid(g.Start), sid(g.End); s <= n && n <= en && s != n {
			blockers = append(blockers, g)
		}
	}
	switch e.Code {
	case "E01", "E02", "E03", "E07", "E08", "E09": // 法則① 紅弧孤立 違反族
		for _, b := range blockers {
			add(fmt.Sprintf("縮短 %s 的區間: 把 %s 的最後使用（現於 s%d）移到 %s 之前, 兩弧即不相交",
				b.Ref, b.Ref, sid(b.End), e.Stmt))
		}
		if len(blockers) > 0 {
			add(fmt.Sprintf("或延後本操作（%s）到擋路借用死亡之後", firstWord(e.StmtText)))
			add("或拆分路徑（split borrow）: 若實際觸及不同字段, 改借 x.f 與 x.g 而非整體 x")
		}
	case "E05", "E06": // use-after-move/drop
		add("在使用點之前重新初始化該 place（整體寫入 set x 即豁免）")
		add("或改為借用而非 move（&x / &mut x），或 clone")
	case "E04", "E10": // 法則② 弧在圓內 違反族
		add("提升被借者的作用域: 讓它活得比引用（或呼叫者）更久")
		add("或不返回引用: 改返回擁有值（move 出去）")
	}
	return out
}

// ============================================================
// 渲染（軌 A 文字輸出，直接可進 agent prompt）
// ============================================================

// RenderText 渲染完整幾何解釋（含頭部聲明——判定權歸 rustc）。
func (t *Topology) RenderText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "L5 借用幾何解釋 (引擎: ChordLaw, liveness=%s; 本節是解釋, 判定以 rustc 為準)\n", t.Liveness)
	for _, ft := range t.Fns {
		if len(t.Fns) > 1 || ft.Fn != "" {
			fmt.Fprintf(&b, "\n■ fn %s\n", ft.Fn)
		}
		b.WriteString(ft.renderRows())
		if len(ft.Facts) > 0 {
			b.WriteString("\n區間代數事實:\n")
			for _, f := range ft.Facts {
				mark := ""
				if f.Illegal {
					mark = "  ← 違法"
				} else if f.Suspect {
					mark = "  ← 存疑(近似)"
				}
				fmt.Fprintf(&b, "  %s vs %s: %s%s — %s\n", f.A, f.B, f.Rel, mark, f.Note)
			}
		}
		if len(ft.Fixes) > 0 {
			b.WriteString("\n幾何修法 (由拓撲推導):\n")
			for _, x := range ft.Fixes {
				fmt.Fprintf(&b, "  [%s @ %s] %s\n", x.Code, x.Stmt, x.Hint)
			}
		}
	}
	return b.String()
}

// renderRows 渲染 ASCII 區間圖。#=mut 排他, ==sh 共享, X=錯誤點。
func (ft *FnTopology) renderRows() string {
	if len(ft.Rows) == 0 {
		return ""
	}
	const cw = 6 // 每陳述列寬
	var b strings.Builder
	b.WriteString("借用區間拓撲 (軸=陳述序; #=mut 排他, ==sh 共享, X=錯誤點):\n")

	// 表頭軸
	label := func(r IntervalRow) string {
		amp := "&"
		if r.Kind == "mut" {
			amp = "&mut "
		}
		return fmt.Sprintf("%s: %s%s", r.Ref, amp, r.Referent)
	}
	lw := 14
	for _, r := range ft.Rows {
		if n := len(label(r)) + 2; n > lw {
			lw = n
		}
	}
	b.WriteString(strings.Repeat(" ", lw))
	for i := ft.Lo; i <= ft.Hi; i++ {
		b.WriteString(pad("s"+strconv.Itoa(i), cw))
	}
	b.WriteString("\n")

	for _, r := range ft.Rows {
		ch := "="
		if r.Kind == "mut" {
			ch = "#"
		}
		b.WriteString(pad(label(r), lw))
		for i := ft.Lo; i <= ft.Hi; i++ {
			var cell string
			switch {
			case i >= r.Start && i < r.End:
				cell = strings.Repeat(ch, cw)
			case i == r.End:
				cell = pad(ch+"(end)", cw)
			default:
				cell = strings.Repeat(" ", cw)
			}
			if hasInt(r.ErrAt, i) && i >= r.Start && i <= r.End {
				cell = "X" + cell[1:]
			}
			b.WriteString(cell)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// ============================================================
// 衝突圖（節點=借用, 紅邊=違法重疊）——修復收斂判據:「紅邊清零」
// ============================================================

// ConflictGraph 是衝突圖的機器可讀形式。
type ConflictGraph struct {
	Fn    string         `json:"fn"`
	Nodes []ConflictNode `json:"nodes"`
	Edges []ConflictEdge `json:"edges"`
}

type ConflictNode struct {
	ID       string `json:"id"` // 參考名
	Referent string `json:"referent"`
	Kind     string `json:"kind"`
	Span     string `json:"span"` // sX..sY
}

type ConflictEdge struct {
	A       string `json:"a"`
	B       string `json:"b"`
	Rel     string `json:"rel"`
	Illegal bool   `json:"illegal"` // 紅邊
	Suspect bool   `json:"suspect,omitempty"`
}

// ConflictGraphs 由拓撲導出各 fn 的衝突圖。
func (t *Topology) ConflictGraphs() []ConflictGraph {
	var out []ConflictGraph
	for _, ft := range t.Fns {
		g := ConflictGraph{Fn: ft.Fn}
		for _, r := range ft.Rows {
			g.Nodes = append(g.Nodes, ConflictNode{
				ID: r.Ref, Referent: r.Referent, Kind: r.Kind,
				Span: fmt.Sprintf("s%d..s%d", r.Start, r.End)})
		}
		for _, f := range ft.Facts {
			g.Edges = append(g.Edges, ConflictEdge{A: f.A, B: f.B, Rel: f.Rel,
				Illegal: f.Illegal, Suspect: f.Suspect})
		}
		out = append(out, g)
	}
	return out
}

// RedEdges 統計違法邊總數（=0 即幾何收斂; 供修復進度判據）。
func (t *Topology) RedEdges() int {
	n := 0
	for _, ft := range t.Fns {
		for _, f := range ft.Facts {
			if f.Illegal {
				n++
			}
		}
	}
	return n
}

// RenderDOT 輸出 Graphviz DOT（紅邊=違法; 供面板/文檔渲染）。
func (g *ConflictGraph) RenderDOT() string {
	var b strings.Builder
	fmt.Fprintf(&b, "graph conflict_%s {\n  label=\"fn %s 借用衝突圖 (紅=違法)\";\n", sanitize(g.Fn), g.Fn)
	for _, n := range g.Nodes {
		shape := "ellipse"
		if n.Kind == "mut" {
			shape = "box"
		}
		fmt.Fprintf(&b, "  %q [label=\"%s\\n&%s%s\\n%s\", shape=%s];\n",
			n.ID, n.ID, mutStr(n.Kind), n.Referent, n.Span, shape)
	}
	for _, e := range g.Edges {
		attr := "color=gray, style=dashed"
		if e.Illegal {
			attr = "color=red, penwidth=2"
		} else if e.Suspect {
			attr = "color=orange, style=dashed"
		}
		fmt.Fprintf(&b, "  %q -- %q [label=%q, %s];\n", e.A, e.B, e.Rel, attr)
	}
	b.WriteString("}\n")
	return b.String()
}

// ============================================================
// 小工具
// ============================================================

// sid 解析 "s12" → 12；非法回 0。
func sid(s string) int {
	if len(s) < 2 || s[0] != 's' {
		return 0
	}
	n, err := strconv.Atoi(s[1:])
	if err != nil {
		return 0
	}
	return n
}

// pathConflict 等價於引擎內建: 相等或一者為另一者之字段前綴。
func pathConflict(a, b string) bool {
	if a == b {
		return true
	}
	return fieldPrefix(a, b) || fieldPrefix(b, a)
}

func fieldPrefix(a, b string) bool { // a 是 b 的嚴格前綴 (a=x, b=x.f)
	return len(b) > len(a) && strings.HasPrefix(b, a) && b[len(a)] == '.'
}

func pad(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func hasInt(xs []int, n int) bool {
	for _, x := range xs {
		if x == n {
			return true
		}
	}
	return false
}

func firstWord(s string) string {
	if i := strings.IndexByte(s, ' '); i > 0 {
		return s[:i]
	}
	return s
}

func mutStr(kind string) string {
	if kind == "mut" {
		return "mut "
	}
	return ""
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}
