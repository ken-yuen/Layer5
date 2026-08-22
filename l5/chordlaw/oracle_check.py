#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
弦律 ChordLaw — rustc oracle 差異測試
======================================
對同一語料, 比較 弦律判定 與 rustc 判定 (accept/reject):
  - 合約 S (無虛假放行): 弦律 FAIL 的案例, rustc 必須也 reject
  - 保守性記錄: 弦律 PASS 的案例, rustc 必須 accept (本原型目標: 雙向一致)

翻譯約定 (玩具 .cl → 忠實 Rust; 備註見各 case):
  use P (place)      → let _uN = P;     (真實 MIR 讀取; `let _ = P` 會被 MIR 消除, 不忠實)
  use R (sh 參考)    → let _uN = *R;    (透過參考讀取, 延續 R 活度)
  use R (mut 參考)   → *R += 1;         (排他使用 = 讀寫, 對應玩具「使用」)
  set P              → P = …;           (寫入)
  mv P               → let _m = P;      (P 用 String 使 mv 為真實 move)
  dp P               → std::mem::drop(P)
  loop {…}           → loop {…}         (無 break: 忠實無限迴圈; 後向邊語義相同)

需要: rustc。運行: python3 oracle_check.py (26 例)
"""
import os
import re
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import chordlaw as C  # noqa: E402

RUSTC = os.path.expanduser("~/.cargo/bin/rustc") if os.path.exists(os.path.expanduser("~/.cargo/bin/rustc")) else "rustc"
TMP = os.path.join(HERE, ".oracle_tmp")

# name -> (cl_source, rust_source, 翻譯備註)
CASES = {
    # ---------------- 12 個 examples ----------------
    "ex1_clash": (
"""fn f() {
  let x
  let a = &x
  use a
  let b = &mut x
  use a
  use b
}""",
"""fn f() {
    let mut x = 0u32;
    let a = &x;
    let _u1 = *a;
    let b = &mut x;
    let _u2 = *a;
    *b += 1;
}
fn main() {}
""", "a=sh→讀取; b=mut→排他使用; 衝突於 b 產生點 (E0502)"),

    "ex2_sequential": (
"""fn f() {
  let x
  let a = &x
  use a
  let b = &mut x
  use b
}""",
"""fn f() {
    let mut x = 0u32;
    let a = &x;
    let _u1 = *a;
    let b = &mut x;
    *b += 1;
}
fn main() {}
""", "NLL 順序借用 (a 死於 b 之前)"),

    "ex3_return": (
"""fn f() {
  let x
  let a = &x
  ret a
}""",
"""fn f() -> &u32 {
    let x = 0u32;
    let a = &x;
    a
}
fn main() {}
""", "回傳局部引用 (E0106)"),

    "ex3b_return_param": (
"""fn f(p) {
  let a = &p
  ret a
}""",
"""fn f(p: &u32) -> &u32 {
    let a = &*p;
    a
}
fn main() {}
""", "參數=呼叫者之引用; a=再借用 (參數範圍=ROOT)"),

    "ex4_loop_ok": (
"""fn f() {
  let v
  let r = &v
  loop {
    use r
  }
}""",
"""fn f() {
    let mut v = 0u32;
    let r = &v;
    loop {
        let _u1 = *r;
    }
}
fn main() {}
""", "外部借用在迴圈內使用 (無限迴圈, 忠實後向邊)"),

    "ex5_loop_write": (
"""fn f() {
  let v
  let r = &v
  loop {
    use r
    set v
  }
}""",
"""fn f() {
    let mut v = 0u32;
    let r = &v;
    loop {
        let _u1 = *r;
        v = 1;
    }
}
fn main() {}
""", "迴圈內寫被借者 (r 用於下輪 → 活躍; E0506)"),

    "ex6_move": (
"""fn f() {
  let x
  let a = &x
  mv x
  use a
}""",
"""fn f() {
    let x = String::new();
    let a = &x;
    let _m = x;
    let _u1 = *a;
}
fn main() {}
""", "x 用 String 使 mv 為真實 move (E0505)"),

    "ex7_alias_use": (
"""fn f() {
  let x
  let a = &mut x
  let b = &a
  use a
  use b
}""",
"""fn f() {
    let mut x = 0u32;
    let a = &mut x;
    let b = &a;
    *a += 1;
    let _u1 = *b;
}
fn main() {}
""", "use a (mut 參考) → *a += 1 排他使用; b 活躍期間 (E0506)"),

    "ex8_reassign": (
"""fn f() {
  let x
  mv x
  set x
  use x
}""",
"""fn f() {
    let mut x = String::new();
    let _m = x;
    x = String::new();
    let _u1 = x.clone();
}
fn main() {}
""", "move 後重新初始化; 讀取=clone (String 不可 Copy)"),

    "ex9_two_sh": (
"""fn f() {
  let x
  let a = &x
  let b = &x
  use a
  use b
}""",
"""fn f() {
    let x = 0u32;
    let a = &x;
    let b = &x;
    let _u1 = *a;
    let _u2 = *b;
}
fn main() {}
""", "sh+sh 重疊 (合法)"),

    "ex10_mut_seq": (
"""fn f() {
  let x
  let a = &mut x
  use a
  let b = &mut x
  use b
}""",
"""fn f() {
    let mut x = 0u32;
    let a = &mut x;
    *a += 1;
    let b = &mut x;
    *b += 1;
}
fn main() {}
""", "mut 依序 (前貸死後再借)"),

    "ex11_alias_chain": (
"""fn f() {
  let x
  let a = &mut x
  let b = &a
  use a
  let c = &mut x
  use c
  use b
}""",
"""fn f() {
    let mut x = 0u32;
    let a = &mut x;
    let b = &a;
    *a += 1;
    let c = &mut x;
    *c += 1;
    let _u1 = *b;
}
fn main() {}
""", "別名鏈活度 (b 活 → a 活 → x 被 mut 借用)"),

    # ---------------- 額外語料 ----------------
    "pass_reborrow_sh_dead_then_use": (
"""fn f() {
  let x
  let a = &x
  let b = &a
  use b
  use a
}""",
"""fn f() {
    let x = 0u32;
    let a = &x;
    let b = &a;
    let _u1 = *b;
    let _u2 = *a;
}
fn main() {}
""", "再借用 b 死後使用 a (NLL 放行)"),

    "fail_write_during_loan": (
"""fn f() {
  let x
  let a = &x
  set x
  use a
}""",
"""fn f() {
    let mut x = 0u32;
    let a = &x;
    x = 1;
    let _u1 = *a;
}
fn main() {}
""", "借用活躍期間寫入 (E0506)"),

    "fail_read_during_mut": (
"""fn f() {
  let x
  let a = &mut x
  use a
  use x
  use a
}""",
"""fn f() {
    let mut x = 0u32;
    let a = &mut x;
    *a += 1;
    let _u1 = x;
    *a += 1;
}
fn main() {}
""", "mut 活躍期間直接讀取被借者 (E0503; 注意 `let _ = x` 會被 MIR 消除故用 _u1)"),

    "fail_drop_during_loan": (
"""fn f() {
  let x
  let a = &x
  use a
  dp x
  use a
}""",
"""fn f() {
    let x = String::new();
    let a = &x;
    let _u1 = *a;
    std::mem::drop(x);
    let _u2 = *a;
}
fn main() {}
""", "drop 期間借用活躍 (E0505/E0382)"),

    "pass_loop_use_then_write_after": (
"""fn f() {
  let v
  let r = &v
  loop {
    use r
  }
  set v
}""",
"""fn f() {
    let mut v = 0u32;
    let r = &v;
    loop {
        let _u1 = *r;
    }
    v = 1;
}
fn main() {}
""", "迴圈(無限)後寫入: r 於 v=1 點不活躍 (無路徑自 v=1 達迴圈內使用) → 放行"),

    # ---------------- v0.3: if 分支 / 字段路徑 / 多 fn ----------------
    "ex12_if_branch": (
"""fn f() {
  let x
  let a = &x
  if {
    use a
  }
  set x
}""",
"""fn f() {
    let mut x = 0u32;
    let a = &x;
    if true {
        let _u1 = *a;
    }
    x = 1;
}
fn main() {}
""", "NLL 分支: 分支內使用不延長活度過分支 (rustc 放行)"),

    "ex13_if_clash": (
"""fn f() {
  let x
  let a = &x
  if {
    let b = &mut x
    use a
    use b
  }
}""",
"""fn f() {
    let mut x = 0u32;
    let a = &x;
    if true {
        let b = &mut x;
        let _u1 = *a;
        *b += 1;
    }
}
fn main() {}
""", "分支內 a 仍被使用 → b 產生時衝突 (E0502)"),

    "ex14_field_split": (
"""fn f() {
  let x
  let a = &x.f
  let b = &mut x.g
  use a
  use b
}""",
"""#[derive(Default)]
struct Pair { f: u32, g: u32 }
fn f() {
    let mut x = Pair::default();
    let a = &x.f;
    let b = &mut x.g;
    let _u1 = *a;
    *b += 1;
}
fn main() {}
""", "split borrow: 不同字段 sh/mut 共存 (rustc 放行)"),

    "ex15_field_clash": (
"""fn f() {
  let x
  let a = &x.f
  let b = &mut x
  use a
  use b
}""",
"""#[derive(Default)]
struct Pair { f: u32, g: u32 }
fn f() {
    let mut x = Pair::default();
    let a = &x.f;
    let b = &mut x;
    let _u1 = *a;
    b.f += 1;
}
fn main() {}
""", "整體 vs 子字段路徑衝突 (E0502)"),

    "ex16_two_fn": (
"""fn ok() {
  let x
  let a = &x
  use a
}
fn bad() {
  let x
  let a = &x
  set x
  use a
}""",
"""fn ok() {
    let x = 0u32;
    let a = &x;
    let _u1 = *a;
}
fn bad() {
    let mut x = 0u32;
    let a = &x;
    x = 1;
    let _u1 = *a;
}
fn main() {}
""", "多 fn 檔案: 僅 bad 違規 (E0506)"),

    "fail_subfield_write_after_move": (
"""fn f() {
  let x
  mv x
  set x.f
}""",
"""#[derive(Default)]
struct S { f: String }
fn f() {
    let x = S::default();
    let _m = x;
    x.f = String::new();
}
fn main() {}
""", "整體 move 後寫子字段 = 使用被移值 (E0382)"),

    "pass_whole_reinit_after_field_move": (
"""fn f() {
  let x
  mv x.f
  set x
  use x
}""",
"""#[derive(Default, Clone)]
struct S { f: u32 }
fn f() {
    let mut x = S::default();
    let _m = x.f;
    x = S::default();
    let _u1 = x.clone();
}
fn main() {}
""", "字段 move 後重新初始化整體 → 合法"),

    "fail_field_move_while_loan": (
"""fn f() {
  let x
  let a = &x
  mv x.f
  use a
}""",
"""#[derive(Default)]
struct S { f: String }
fn f() {
    let mut x = S::default();
    let a = &x;
    let _m = x.f;
    let _u1 = *a;
}
fn main() {}
""", "借用整體 x 活躍期間 move 子字段 (E0505)"),

    "pass_param_write_after_use": (
"""fn f(p) {
  let a = &p
  use a
  set p
}""",
"""fn f(mut p: u32) {
    let a = &p;
    let _u1 = *a;
    p = 1;
}
fn main() {}
""", "參數=呼叫者值 (按值, 可寫, 不回傳)"),
}


def run_rustc(name, src):
    os.makedirs(TMP, exist_ok=True)
    path = os.path.join(TMP, name + ".rs")
    with open(path, "w", encoding="utf-8") as f:
        f.write(src)
    p = subprocess.run([RUSTC, "--edition", "2021", "-A", "warnings", path,
                        "-o", os.path.join(TMP, name + ".out")],
                       capture_output=True, text=True)
    codes = sorted(set(re.findall(r"error\[(E\d+)\]", p.stderr)))
    return (p.returncode == 0, codes)


def main():
    os.makedirs(TMP, exist_ok=True)
    print("%-34s %-8s %-8s %-20s %s" % ("case", "弦律", "rustc", "rustc 錯誤碼", "一致"))
    bad = 0
    for name in sorted(CASES):
        cl, _rs, _note = CASES[name]
        _pr, _dl, errors = C.check(cl, "nll")
        chord = "PASS" if not errors else "FAIL"
        ok, codes = run_rustc(name, CASES[name][1])
        rust = "accept" if ok else "reject"
        match = (chord == "PASS") == ok
        chord_codes = ";".join("%s@%s" % (e[0], e[1]) for e in errors) or "—"
        print("%-34s %-8s %-8s %-20s %s  %s" %
              (name, chord, rust, ",".join(codes) or "—", "✓" if match else "✗✗✗", chord_codes))
        if not match:
            bad += 1
    print()
    if bad:
        print("不一致: %d 例 — 需要修正!" % bad)
        sys.exit(1)
    print("全部 %d 例判定一致 (弦律 ⊆ rustc 合約 S 通過)" % len(CASES))
    for f in os.listdir(TMP):
        os.remove(os.path.join(TMP, f))
    os.rmdir(TMP)


if __name__ == "__main__":
    main()
