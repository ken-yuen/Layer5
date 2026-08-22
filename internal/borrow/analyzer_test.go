package borrow

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot 由本檔位置推導（internal/borrow → 倉庫根）。
func repoRoot() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..")
}

// requireL5 檢查 python3 + vendored 引擎；缺席時 Skip（capabilities 降級精神）。
func requireL5(t *testing.T) *Analyzer {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 不可用, 跳過 L5 E2E")
	}
	a := &Analyzer{ScriptDir: filepath.Join(repoRoot(), "l5", "chordlaw")}
	if avail, why := a.Available(); !avail {
		t.Skipf("L5 不可用: %s", why)
	}
	return a
}

// golden: vendored 17 範例 → 預期 verdict 與首個錯誤碼。
// 這是未來 Go 引擎重寫（P3）的 differential testing 基準。
var goldenExamples = []struct {
	file     string
	verdict  string
	firstErr string // 空 = 無錯誤
}{
	{"ex1_clash.cl", "FAIL", "E01"},
	{"ex2_sequential.cl", "PASS", ""},
	{"ex3_return.cl", "FAIL", "E10"},
	{"ex3b_return_param.cl", "PASS", ""},
	{"ex4_loop_ok.cl", "PASS", ""},
	{"ex5_loop_write.cl", "FAIL", "E02"},
	{"ex6_move.cl", "FAIL", "E07"},
	{"ex7_alias_use.cl", "FAIL", "E09"},
	{"ex8_reassign.cl", "PASS", ""},
	{"ex9_two_sh.cl", "PASS", ""},
	{"ex10_mut_seq.cl", "PASS", ""},
	{"ex11_alias_chain.cl", "FAIL", "E09"},
	{"ex12_if_branch.cl", "PASS", ""},
	{"ex13_if_clash.cl", "FAIL", "E01"},
	{"ex14_field_split.cl", "PASS", ""},
	{"ex15_field_clash.cl", "FAIL", "E01"},
	{"ex16_two_fn.cl", "FAIL", "E02"},
}

func TestGoldenExamples(t *testing.T) {
	a := requireL5(t)
	dir := filepath.Join(repoRoot(), "l5", "chordlaw", "examples")
	for _, g := range goldenExamples {
		g := g
		t.Run(g.file, func(t *testing.T) {
			r, err := a.AnalyzeFile(context.Background(), filepath.Join(dir, g.file))
			if err != nil {
				t.Fatalf("分析失敗: %v", err)
			}
			if r.Verdict != g.verdict {
				t.Fatalf("verdict = %s, want %s", r.Verdict, g.verdict)
			}
			if g.firstErr == "" && len(r.Errors) != 0 {
				t.Fatalf("不應有錯誤, got %+v", r.Errors)
			}
			if g.firstErr != "" {
				if len(r.Errors) == 0 || r.Errors[0].Code != g.firstErr {
					t.Fatalf("首錯 = %+v, want %s", r.Errors, g.firstErr)
				}
			}
		})
	}
}

// 每個內嵌模板都必須被引擎判出預期的 ChordLaw 錯誤碼（決策 B3 的驗收）。
func TestTemplatesProduceExpectedCodes(t *testing.T) {
	a := requireL5(t)
	for _, tp := range Templates {
		tp := tp
		t.Run(tp.RustcCode, func(t *testing.T) {
			r, err := a.AnalyzeSource(context.Background(), t.TempDir(), tp.CL)
			if err != nil {
				t.Fatalf("模板分析失敗: %v", err)
			}
			if r.Verdict == "PASS" {
				t.Fatalf("模板 %s 應 FAIL", tp.Title)
			}
			found := false
			for _, e := range r.Errors {
				if e.Code == tp.ChordCode {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("模板 %s 未產生預期 %s, got %+v", tp.Title, tp.ChordCode, r.Errors)
			}
		})
	}
}

func TestAnalyzeSourceBounds(t *testing.T) {
	a := &Analyzer{} // 不需引擎——限長檢查在調用前
	if _, err := a.AnalyzeSource(context.Background(), t.TempDir(), strings.Repeat("x", MaxSourceBytes+1)); err == nil {
		t.Fatal("超長源碼應被拒")
	}
	if _, err := a.AnalyzeSource(context.Background(), t.TempDir(), "   \n  "); err == nil {
		t.Fatal("空源碼應被拒")
	}
}

func TestAvailableReporting(t *testing.T) {
	bad := &Analyzer{ScriptDir: "/nonexistent"}
	if avail, why := bad.Available(); avail || why == "" {
		t.Fatal("無效 ScriptDir 應報不可用並給原因")
	}
	badPy := &Analyzer{Python: "python-does-not-exist-xyz",
		ScriptDir: filepath.Join(repoRoot(), "l5", "chordlaw")}
	if avail, _ := badPy.Available(); avail {
		t.Fatal("無效 python 應報不可用")
	}
}

// 端到端: 拓撲管線對真引擎輸出的整合驗證。
func TestEndToEndTopology(t *testing.T) {
	a := requireL5(t)
	r, err := a.AnalyzeFile(context.Background(),
		filepath.Join(repoRoot(), "l5", "chordlaw", "examples", "ex1_clash.cl"))
	if err != nil {
		t.Fatal(err)
	}
	topo := BuildTopology(r)
	if topo.RedEdges() != 1 {
		t.Fatalf("ex1 紅邊 = %d, want 1", topo.RedEdges())
	}
	txt := topo.RenderText()
	if !strings.Contains(txt, "OVERLAP") || !strings.Contains(txt, "判定以 rustc 為準") {
		t.Fatalf("端到端渲染缺關鍵內容:\n%s", txt)
	}
}
