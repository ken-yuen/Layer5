package borrow

import (
	"encoding/json"
	"strings"
	"testing"
)

// 內嵌 fixture（ex1_clash 的引擎輸出快照, 不依賴 python——確定性單測）。
const fixtureEx1 = `{
  "schema": "chordlaw.report/v1",
  "file": "ex1_clash.cl",
  "liveness": "nll",
  "verdict": "FAIL",
  "errors": [
    {"code": "E01", "stmt": "s4", "fn": "ex1", "stmt_text": "let b = &mut x",
     "message": "紅弧交越: 兩借用重疊且至少一者為 mut (~ E0499/E0502)"}
  ],
  "regions": [
    {"fn": "ex1", "ref": "a", "referent": "x", "kind": "sh", "start": "s2", "end": "s5"},
    {"fn": "ex1", "ref": "b", "referent": "x", "kind": "mut", "start": "s4", "end": "s6"}
  ]
}`

// ex2_sequential: PASS, 順序借用。
const fixtureEx2 = `{
  "schema": "chordlaw.report/v1",
  "file": "ex2_sequential.cl",
  "liveness": "nll",
  "verdict": "PASS",
  "errors": [],
  "regions": [
    {"fn": "ex2", "ref": "a", "referent": "x", "kind": "sh", "start": "s2", "end": "s3"},
    {"fn": "ex2", "ref": "b", "referent": "x", "kind": "mut", "start": "s4", "end": "s5"}
  ]
}`

// split borrow: 不同字段, 無 path_conflict, 不應產生代數事實。
const fixtureSplit = `{
  "schema": "chordlaw.report/v1",
  "file": "ex14_field_split.cl",
  "liveness": "nll",
  "verdict": "PASS",
  "errors": [],
  "regions": [
    {"fn": "f", "ref": "a", "referent": "x.f", "kind": "mut", "start": "s2", "end": "s4"},
    {"fn": "f", "ref": "b", "referent": "x.g", "kind": "mut", "start": "s3", "end": "s5"}
  ]
}`

// 字段 vs 整體: path_conflict 成立。
const fixtureFieldWhole = `{
  "schema": "chordlaw.report/v1",
  "file": "ex15_field_clash.cl",
  "liveness": "nll",
  "verdict": "FAIL",
  "errors": [
    {"code": "E01", "stmt": "s3", "fn": "f", "stmt_text": "let b = &mut x",
     "message": "紅弧交越"}
  ],
  "regions": [
    {"fn": "f", "ref": "a", "referent": "x.f", "kind": "sh", "start": "s2", "end": "s4"},
    {"fn": "f", "ref": "b", "referent": "x", "kind": "mut", "start": "s3", "end": "s5"}
  ]
}`

func load(t *testing.T, s string) *Report {
	t.Helper()
	var r Report
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		t.Fatalf("fixture 解析失敗: %v", err)
	}
	return &r
}

func TestTopologyOverlapIllegal(t *testing.T) {
	topo := BuildTopology(load(t, fixtureEx1))
	if len(topo.Fns) != 1 {
		t.Fatalf("fns = %d, want 1", len(topo.Fns))
	}
	ft := topo.Fns[0]
	if ft.Fn != "ex1" || ft.Lo != 2 || ft.Hi != 6 {
		t.Fatalf("軸錯誤: fn=%s lo=%d hi=%d", ft.Fn, ft.Lo, ft.Hi)
	}
	if len(ft.Facts) != 1 {
		t.Fatalf("facts = %d, want 1", len(ft.Facts))
	}
	f := ft.Facts[0]
	if f.Rel != "OVERLAP[s4..s5]" || !f.Illegal || f.Suspect {
		t.Fatalf("代數事實錯誤: %+v", f)
	}
	if topo.RedEdges() != 1 {
		t.Fatalf("紅邊 = %d, want 1", topo.RedEdges())
	}
	if len(ft.Fixes) == 0 || !strings.Contains(ft.Fixes[0].Hint, "縮短 a") {
		t.Fatalf("修法缺失或錯誤: %+v", ft.Fixes)
	}
}

func TestTopologyDisjointLegal(t *testing.T) {
	topo := BuildTopology(load(t, fixtureEx2))
	ft := topo.Fns[0]
	if len(ft.Facts) != 1 || ft.Facts[0].Rel != "DISJOINT" || ft.Facts[0].Illegal {
		t.Fatalf("順序借用應 DISJOINT 合法: %+v", ft.Facts)
	}
	if topo.RedEdges() != 0 {
		t.Fatalf("紅邊 = %d, want 0", topo.RedEdges())
	}
}

func TestTopologySplitBorrowNoConflict(t *testing.T) {
	topo := BuildTopology(load(t, fixtureSplit))
	if n := len(topo.Fns[0].Facts); n != 0 {
		t.Fatalf("split borrow (x.f vs x.g) 不應有代數事實, got %d", n)
	}
}

func TestTopologyFieldVsWholeConflict(t *testing.T) {
	topo := BuildTopology(load(t, fixtureFieldWhole))
	ft := topo.Fns[0]
	if len(ft.Facts) != 1 || !ft.Facts[0].Illegal {
		t.Fatalf("x.f vs x 應為違法重疊: %+v", ft.Facts)
	}
}

func TestRenderTextShape(t *testing.T) {
	out := BuildTopology(load(t, fixtureEx1)).RenderText()
	for _, want := range []string{
		"判定以 rustc 為準", // 判定權聲明必須在
		"X",            // 錯誤點標記
		"OVERLAP[s4..s5]",
		"縮短 a 的區間",
		"#", "=", // mut/sh 字符
	} {
		if !strings.Contains(out, want) {
			t.Errorf("渲染缺少 %q\n%s", want, out)
		}
	}
	// mut 行必須以 # 繪製且 sh 行以 = 繪製
	var mutLine, shLine string
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "b: &mut x") {
			mutLine = ln
		}
		if strings.HasPrefix(ln, "a: &x") {
			shLine = ln
		}
	}
	if !strings.Contains(mutLine, "###") || strings.Contains(mutLine, "===") {
		t.Errorf("mut 行繪製錯誤: %q", mutLine)
	}
	if !strings.Contains(shLine, "===") || strings.Contains(shLine, "###") {
		t.Errorf("sh 行繪製錯誤: %q", shLine)
	}
}

func TestConflictGraphAndDOT(t *testing.T) {
	graphs := BuildTopology(load(t, fixtureEx1)).ConflictGraphs()
	if len(graphs) != 1 {
		t.Fatalf("graphs = %d", len(graphs))
	}
	g := graphs[0]
	if len(g.Nodes) != 2 || len(g.Edges) != 1 || !g.Edges[0].Illegal {
		t.Fatalf("衝突圖結構錯誤: %+v", g)
	}
	dot := g.RenderDOT()
	for _, want := range []string{"graph conflict_ex1", "color=red", `"a" -- "b"`} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT 缺少 %q:\n%s", want, dot)
		}
	}
	// 衝突圖必須可 JSON 序列化（供 --json 出口）
	if _, err := json.Marshal(g); err != nil {
		t.Fatalf("衝突圖 JSON 序列化失敗: %v", err)
	}
}

func TestSchemaGuard(t *testing.T) {
	// 未知 schema 的報告仍可構建拓撲（analyzer 層才做 schema 拒絕），
	// 但這裡驗證 sid 對異常輸入的韌性。
	if sid("") != 0 || sid("x9") != 0 || sid("s") != 0 || sid("s12") != 12 {
		t.Fatal("sid 解析韌性不足")
	}
	if !pathConflict("x", "x.f") || !pathConflict("x.f", "x") || pathConflict("x.f", "x.g") || !pathConflict("x", "x") {
		t.Fatal("pathConflict 語義錯誤")
	}
	if pathConflict("x", "xy") { // 前綴但非字段邊界
		t.Fatal("pathConflict 誤把 xy 當 x 的字段")
	}
}

func TestSuspectWithoutEngineEvidence(t *testing.T) {
	// 線性重疊含 mut 但引擎無錯誤 → Suspect（近似告警）非 Illegal。
	// 構造：迴圈類場景的線性化會出現這種形態。
	fixture := `{
	  "schema": "chordlaw.report/v1", "file": "loop.cl", "liveness": "nll",
	  "verdict": "PASS", "errors": [],
	  "regions": [
	    {"fn": "f", "ref": "a", "referent": "x", "kind": "mut", "start": "s2", "end": "s4"},
	    {"fn": "f", "ref": "b", "referent": "x", "kind": "sh", "start": "s3", "end": "s5"}
	  ]
	}`
	topo := BuildTopology(load(t, fixture))
	f := topo.Fns[0].Facts[0]
	if f.Illegal || !f.Suspect {
		t.Fatalf("無引擎佐證的重疊應為 Suspect 而非 Illegal: %+v", f)
	}
	if topo.RedEdges() != 0 {
		t.Fatal("Suspect 不應計入紅邊")
	}
}
