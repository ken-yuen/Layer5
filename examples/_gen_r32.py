#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Generate 32 rule-demo .cl files (r01–r32)."""
import os
HERE = os.path.dirname(os.path.abspath(__file__))

CASES = {
    "r01_eclash": (
        "E01 紅弧交越: b(&mut x) 產生於 a(&x) 的活躍區間內",
        """fn r01() {
  let x
  let a = &x
  use a
  let b = &mut x
  use a
  use b
}
""",
    ),
    "r02_ewrite": (
        "E02 借用活躍期間寫入被借者",
        """fn r02() {
  let x
  let a = &x
  set x
  use a
}
""",
    ),
    "r03_eread": (
        "E03 mut 活躍期間直接讀取被借者",
        """fn r03() {
  let x
  let a = &mut x
  use a
  use x
  use a
}
""",
    ),
    "r04_edangle": (
        "E04 經槽使用已結束作用域的被借者 (弧端點掉出圓)",
        """fn r04() {
  slot s
  {
    let y
    let a = &y
    store s = a
  }
  use s
}
""",
    ),
    "r05_eadrop": (
        "E05 drop 後使用",
        """fn r05() {
  let x
  dp x
  use x
}
""",
    ),
    "r06_emove": (
        "E06 move 後使用",
        """fn r06() {
  let x
  mv x
  use x
}
""",
    ),
    "r07_eloanmove": (
        "E07 借用活躍期間 move 被借者",
        """fn r07() {
  let x
  let a = &x
  mv x
  use a
}
""",
    ),
    "r08_eloandrop": (
        "E08 借用活躍期間 drop 被借者",
        """fn r08() {
  let x
  let a = &x
  dp x
  use a
}
""",
    ),
    "r09_erefuse": (
        "E09 別名層衝突: 參考 a 使用期間自身被借用活躍",
        """fn r09() {
  let x
  let a = &mut x
  let b = &a
  use a
  use b
}
""",
    ),
    "r10_ereturn": (
        "E10 回傳指向局部值的參考 (弧逸出 ROOT 圓)",
        """fn r10() {
  let x
  let a = &x
  ret a
}
""",
    ),
    "r11_eimmut": (
        "E11 寫入不可變地方",
        """fn r11() {
  let imm x
  set x
}
""",
    ),
    "r12_enotmut": (
        "E12 對不可變地方作 mut 出借",
        """fn r12() {
  let imm x
  let a = &mut x
  use a
}
""",
    ),
    "r13_eassignsh": (
        "E13 經共享參考寫入",
        """fn r13() {
  let x
  let a = &x
  set *a
}
""",
    ),
    "r14_emoveout": (
        "E14 經參考移出",
        """fn r14() {
  let x
  let a = &x
  mv *a
}
""",
    ),
    "r15_etemp": (
        "E15 暫存借用被非緊鄰陳述使用 (保守 E0716)",
        """fn r15() {
  tmp t
  let a = &t
  let x
  use a
}
""",
    ),
    "r16_eescape": (
        "E16 把指向局部的參考存入槽",
        """fn r16() {
  slot s
  let y
  let a = &y
  store s = a
}
""",
    ),
    "r17_estoretemp": (
        "E17 把指向暫存的參考存入槽",
        """fn r17() {
  slot s
  tmp t
  let a = &t
  store s = a
}
""",
    ),
    "r18_ebranchmove": (
        "E18 if 分支內 move 後於分支外使用",
        """fn r18() {
  let x
  if {
    mv x
  }
  use x
}
""",
    ),
    "r19_edoubledrop": (
        "E19 重複 drop",
        """fn r19() {
  let x
  dp x
  dp x
}
""",
    ),
    "r20_edropmoved": (
        "E20 move 後再 drop",
        """fn r20() {
  let x
  mv x
  dp x
}
""",
    ),
    "r21_ealiascyc": (
        "E21 把指向槽自身的參考存回槽 (別名環)",
        """fn r21() {
  slot s
  let a = &s
  store s = a
}
""",
    ),
    "r22_eimmfield": (
        "E22 寫入不可變地方之子字段",
        """fn r22() {
  let imm x
  set x.f
}
""",
    ),
    "r23_enotmutfield": (
        "E23 對不可變字段作 mut 出借",
        """fn r23() {
  let imm x
  let a = &mut x.f
  use a
}
""",
    ),
    "r24_eelsejoin": (
        "E24 else 分支內 move 後於分支外使用",
        """fn r24() {
  let x
  if {
    set x
  } else {
    mv x
  }
  use x
}
""",
    ),
    "r25_ecallmut": (
        "E25 呼叫讀取 mut 借用活躍之路徑",
        """fn r25() {
  let x
  let a = &mut x
  call f(x)
  use a
}
""",
    ),
    "r26_ecallmove": (
        "E26 呼叫消耗後再使用",
        """fn r26() {
  let x
  callmv take(x)
  use x
}
""",
    ),
    "r27_ecalloan": (
        "E27 借用活躍期間呼叫消耗被借者",
        """fn r27() {
  let x
  let a = &x
  callmv take(x)
  use a
}
""",
    ),
    "r28_erettemp": (
        "E28 回傳指向暫存的參考",
        """fn r28() {
  tmp t
  let a = &t
  ret a
}
""",
    ),
    "r29_euninit": (
        "E29 使用尚未初始化的洞",
        """fn r29() {
  let hole x
  use x
}
""",
    ),
    "r30_euninitret": (
        "E30 移出尚未初始化的洞",
        """fn r30() {
  let hole x
  mv x
}
""",
    ),
    "r31_eparamimmut": (
        "E31 寫入不可變參數",
        """fn r31(imm p) {
  set p
}
""",
    ),
    "r32_eparamnotmut": (
        "E32 對不可變參數作 mut 出借",
        """fn r32(imm p) {
  let a = &mut p
  use a
}
""",
    ),
}

def main():
    for name, (comment, body) in CASES.items():
        path = os.path.join(HERE, name + ".cl")
        with open(path, "w", encoding="utf-8") as f:
            f.write("// %s\n" % comment)
            f.write(body)
        print("wrote", name)

if __name__ == "__main__":
    main()
