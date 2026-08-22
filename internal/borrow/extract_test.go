package borrow

import (
	"context"
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
