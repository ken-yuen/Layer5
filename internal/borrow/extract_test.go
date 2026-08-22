package borrow

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

const rustE0502 = `fn main() {
    let mut x = 0u32;
    let a = &x;
    let b = &mut x;
    *b += 1;
    println!("{}", *a);
}
`

const rustE0506 = `fn f() {
    let mut s = 1u32;
    let r = &s;
    s = 2;
    let _v = *r;
}
`

const rustE0382 = `fn f() {
    let x = String::new();
    let y = x;
    let _n = x.len();
}
`

const rustE0505 = `fn f() {
    let x = String::new();
    let a = &x;
    let m = x;
    let _n = a.len();
}
`

const rustTooComplex = `fn f() {
    let x = match g() {
        Some(v) => v,
        None => return,
    };
}
`

func TestReduceRustE0502(t *testing.T) {
	red, err := ReduceRust(rustE0502, 4, "E0502", "main.rs")
	if err != nil {
		t.Fatalf("歸約失敗: %v", err)
	}
	if red.FnName != "main" {
		t.Errorf("fn 名 = %s", red.FnName)
	}
	for _, want := range []string{"let a = &x", "let b = &mut x", "use b", "use a"} {
		if !strings.Contains(red.CL, want) {
			t.Errorf(".cl 缺 %q:\n%s", want, red.CL)
		}
	}
	if len(red.StmtLines) == 0 {
		t.Error("缺行號映射")
	}
	if !strings.Contains(red.MappingText(), "main.rs:") {
		t.Error("MappingText 缺源檔名")
	}
}

func TestReduceRustMoveFamily(t *testing.T) {
	red, err := ReduceRust(rustE0382, 3, "E0382", "lib.rs")
	if err != nil {
		t.Fatalf("歸約失敗: %v", err)
	}
	if !strings.Contains(red.CL, "mv x") {
		t.Errorf("E0382 之 let y = x 應譯 mv x:\n%s", red.CL)
	}
}

func TestReduceRustBailsOnComplex(t *testing.T) {
	if _, err := ReduceRust(rustTooComplex, 3, "E0502", "x.rs"); err == nil {
		t.Fatal("match 應觸發放棄")
	}
	// fn 過長
	long := "fn f() {\n" + strings.Repeat("    let q = 1;\n", MaxReduceFnLines+2) + "}\n"
	if _, err := ReduceRust(long, 2, "E0502", "x.rs"); err == nil {
		t.Fatal("超長 fn 應觸發放棄")
	}
	// 錯誤行不在任何 fn 內
	if _, err := ReduceRust("const A: u32 = 1;\n", 1, "E0502", "x.rs"); err == nil {
		t.Fatal("非 fn 區域應觸發放棄")
	}
}

func TestChordFamilyCoversBorrowCodes(t *testing.T) {
	for code := range BorrowRustcCodes {
		if len(ChordFamily(code)) == 0 {
			t.Errorf("borrow 碼 %s 無幾何族映射", code)
		}
	}
}

func TestStripStrings(t *testing.T) {
	if got := stripStrings(`let s = "a { b } c";`); strings.ContainsAny(got, "{}") {
		t.Errorf("字串內括號未剝除: %q", got)
	}
	if got := stripStrings(`let c = 'x'; foo(y)`); !strings.Contains(got, "foo(y)") {
		t.Errorf("字元後內容丟失: %q", got)
	}
	// 生命週期 'a 不應觸發 char 模式吞掉後文
	if got := stripStrings(`fn f<'a>(x: &'a u32) { y }`); !strings.Contains(got, "y") {
		t.Errorf("生命週期標注誤判: %q", got)
	}
}

// ── 真實專案驗測回歸（2026-08-22, 7 專案 ~20 萬行掃描發現的邊角）──

// fd walk.rs: 塊註釋延續行以 * 開頭, 不可誤殺 *b += 1 解引用
func TestReduceBlockCommentVsDeref(t *testing.T) {
	src := `fn f() {
    /* block comment
     * continuation line
     */
    let mut x = 0u32;
    let b = &mut x;
    *b += 1;
}
`
	red, err := ReduceRust(src, 6, "E0502", "x.rs")
	if err != nil {
		t.Fatalf("歸約失敗: %v", err)
	}
	if !strings.Contains(red.CL, "use b") {
		t.Fatalf("*b += 1 應譯 use b:\n%s", red.CL)
	}
}

// ripgrep version.rs: 靜默塊(結構體字面量)內宣告 → 提升+set, 塊外可引用
func TestReduceBlockScopedDeclHoisting(t *testing.T) {
	src := `fn f() {
    {
        let features = 1u32;
        let x = features;
    }
    {
        let features = 2u32;
        let _y = features;
    }
}
`
	red, err := ReduceRust(src, 3, "E0502", "x.rs")
	if err != nil {
		t.Fatalf("歸約失敗: %v", err)
	}
	// 塊內重復宣告不可產生未定義引用: 頂部 let + 原位 set
	if !strings.Contains(red.CL, "let features") {
		t.Fatalf("塊內宣告應提升頂部 let:\n%s", red.CL)
	}
	if strings.Count(red.CL, "let features") != 1 {
		t.Fatalf("同名僅一個 let (其餘 set):\n%s", red.CL)
	}
}

// alacritty atlas.rs: for atlas in &mut atlas 自引用 shadowing → 退化不 panic
func TestReduceSelfShadowingFor(t *testing.T) {
	src := `fn f() {
    let mut atlas = vec![1u32];
    for atlas in &mut atlas {
        let _x = atlas;
    }
}
`
	red, err := ReduceRust(src, 3, "E0502", "x.rs")
	if err != nil {
		return // 放棄也合法
	}
	if strings.Contains(red.CL, "let atlas = &mut atlas") {
		t.Fatalf("自引用 shadowing 不可直譯:\n%s", red.CL)
	}
}

// rustlings hashmaps3: loop 內大量陳述 → 引擎過載防線觸發
func TestReduceLoopOverloadGuard(t *testing.T) {
	var b strings.Builder
	b.WriteString("fn f() {\n    let mut acc = 0u32;\n    let mut k = 0u32;\n    for i in 0..10 {\n")
	for j := 0; j < 8; j++ {
		fmt.Fprintf(&b, "        acc = k + %d;\n        k = acc + %d;\n", j, j)
	}
	b.WriteString("    }\n}\n")
	if _, err := ReduceRust(b.String(), 3, "E0506", "x.rs"); err == nil {
		t.Fatal("loop 內超過 MaxLoopStmts 應放棄（防 naive 引擎過載）")
	}
}

// zoxide db/mod.rs: 大寫構造子/型別是噪音（Ok/Err/Some/Database…）
func TestReduceUppercaseNoise(t *testing.T) {
	src := `fn f() {
    let x = String::new();
    let y = Some(3u32);
    if y.is_some() {
        return;
    }
    let _z = x;
}
`
	red, err := ReduceRust(src, 3, "E0382", "x.rs")
	if err != nil {
		t.Fatalf("歸約失敗: %v", err)
	}
	for _, bad := range []string{"use Some", "use Ok", "use String", "ret Ok", "use _"} {
		if strings.Contains(red.CL, bad) {
			t.Fatalf(".cl 不應含噪音 %q:\n%s", bad, red.CL)
		}
	}
}

// 端到端: 四類真實 Rust → 歸約 → 引擎驗證幾何族命中。
func TestReduceValidateEndToEnd(t *testing.T) {
	a := requireL5(t)
	cases := []struct {
		name, src, code string
		errLine         int
	}{
		{"E0502", rustE0502, "E0502", 4},
		{"E0506", rustE0506, "E0506", 4},
		{"E0382", rustE0382, "E0382", 3},
		{"E0505", rustE0505, "E0505", 4},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			red, err := ReduceRust(c.src, c.errLine, c.code, c.name+".rs")
			if err != nil {
				t.Fatalf("歸約失敗: %v", err)
			}
			r, valid := ValidateReduction(context.Background(), a, t.TempDir(), red, c.code)
			if !valid {
				t.Fatalf("驗證未通過 (幾何族 %v):\n%s", ChordFamily(c.code), red.CL)
			}
			topo := BuildTopology(r)
			if txt := topo.RenderText(); !strings.Contains(txt, "判定以 rustc 為準") {
				t.Error("拓撲渲染缺判定權聲明")
			}
		})
	}
}
