#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
弦律 ChordLaw — 測試套件 (v0.5: 幾何法則 + 32 則規則)
=======================================
以 rustc 真值為基準的回歸測試 (語料逐一核對過 rustc 判定):
  - TestVerdicts:     34 語料 — PASS 防守虛假拒絕; FAIL 防守虛假放行 (錯誤碼+位置須全符)
  - TestFrontend:     前端 E00 (多餘 }、自引用、未定義、fn 外陳述; 多 fn 為功能)
  - TestParserScopes: 已閉作用域後之陳述歸屬 (cur() 回歸)
  - TestLivenessModes: nll / referent 兩級行為
  - TestV03Features:  if 分支活度、lexical 等級、字段 split borrow、多 fn、字段 move
  - TestV04Rules32:   E11–E32 + 32 則 rXX 範例
  - TestEngine:       變數慣例、reach、neq、分層否定、路徑內建、證明樹
  - TestExamples:     examples/ 17 檔基準
  - TestGeometry:     P-G1 法則守衛 + P-G2 圖對齊 + P-G3 可執行引理 + P-G4 --explain
運行: python3 test_chordlaw.py   (或 python3 -m unittest)
"""
import os
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import chordlaw as C  # noqa: E402


def verdict(code, liveness="nll"):
    """回傳 (verdict, {(code, stmt), ...})"""
    pr, dl, errors = C.check(code, liveness)
    v = "PASS" if not errors else "FAIL"
    return v, {(e[0], e[1]) for e in errors}


def expect_pass(name, code, liveness="nll"):
    def run(self):
        v, errs = verdict(code, liveness)
        self.assertEqual(v, "PASS", "%s 應為 PASS (rustc 合法), 實際錯誤: %s" % (name, errs))
    return run


def expect_fail(name, code, expected, liveness="nll"):
    """expected: set of (code, stmt) — 必須完全相符"""
    def run(self):
        v, errs = verdict(code, liveness)
        self.assertEqual(v, "FAIL", "%s 應為 FAIL (rustc 拒絕)" % name)
        self.assertEqual(errs, set(expected),
                         "%s 錯誤不符: 預期 %s, 實際 %s" % (name, set(expected), errs))
    return run


# ------------------------------------------------------------------
# PASS 語料 (rustc 合法 — 防守虛假拒絕)
# ------------------------------------------------------------------
PASS_CASES = [
    ("pass_seq_sh_then_mut", """
fn f() {
  let x
  let a = &x
  use a
  let b = &mut x
  use b
}"""),
    ("pass_reassign_after_move", """
fn f() {
  let x
  mv x
  set x
  use x
}"""),
    ("pass_two_sh_overlap", """
fn f() {
  let x
  let a = &x
  let b = &x
  use a
  use b
}"""),
    ("pass_seq_mut", """
fn f() {
  let x
  let a = &mut x
  use a
  let b = &mut x
  use b
}"""),
    ("pass_reborrow_sh_dead_then_use", """
fn f() {
  let x
  let a = &x
  let b = &a
  use b
  use a
}"""),
    ("pass_reborrow_mut_dead_then_use", """
fn f() {
  let x
  let a = &mut x
  let b = &mut a
  use b
  use a
}"""),
    ("pass_write_after_loan_dead", """
fn f() {
  let x
  let a = &x
  use a
  set x
}"""),
    ("pass_loop_use_then_write_after", """
fn f() {
  let v
  let r = &v
  loop {
    use r
  }
  set v
}"""),
    ("pass_param_write_after_use", """
fn f(p) {
  let a = &p
  use a
  set p
}"""),
    ("pass_return_param_ref", """
fn f(p) {
  let a = &p
  ret a
}"""),
    ("pass_loop_borrow_from_outside", """
fn f() {
  let v
  let r = &v
  loop {
    use r
  }
}"""),
    ("pass_empty_fn", "fn f() {\n}"),
    ("pass_move_after_loan_dead", """
fn f() {
  let x
  let a = &x
  use a
  mv x
}"""),
]

# ------------------------------------------------------------------
# FAIL 語料 (rustc 拒絕 — 防守虛假放行; 錯誤碼與位置須完全相符)
# ------------------------------------------------------------------
FAIL_CASES = [
    ("fail_clash", """
fn f() {
  let x
  let a = &x
  use a
  let b = &mut x
  use a
  use b
}""", {("E01", "s4")}),
    ("fail_write_during_loan", """
fn f() {
  let x
  let a = &x
  set x
  use a
}""", {("E02", "s3")}),
    ("fail_write_during_two_sh", """
fn f() {
  let x
  let a = &x
  let b = &x
  set x
  use a
}""", {("E02", "s4")}),
    ("fail_read_during_mut", """
fn f() {
  let x
  let a = &mut x
  use a
  use x
  use a
}""", {("E03", "s4")}),
    ("fail_return_local_ref", """
fn f() {
  let x
  let a = &x
  ret a
}""", {("E10", "s3")}),
    ("fail_return_chain_local_ref", """
fn f(p) {
  let x
  let a = &x
  let b = &a
  ret b
}""", {("E10", "s4")}),
    ("fail_loop_write", """
fn f() {
  let v
  let r = &v
  loop {
    use r
    set v
  }
}""", {("E02", "s4")}),
    ("fail_use_after_move", """
fn f() {
  let x
  mv x
  use x
}""", {("E06", "s3")}),
    ("fail_loan_move_and_use", """
fn f() {
  let x
  let a = &x
  mv x
  use a
}""", {("E07", "s3"), ("E06", "s4")}),
    ("fail_drop_during_loan", """
fn f() {
  let x
  let a = &x
  use a
  dp x
  use a
}""", {("E08", "s4"), ("E05", "s5")}),
    ("fail_alias_mut_ref_used", """
fn f() {
  let x
  let a = &mut x
  let b = &a
  use a
  use b
}""", {("E09", "s4")}),
    ("fail_alias_sh_ref_used", """
fn f() {
  let x
  let a = &x
  let b = &mut a
  use a
  use b
}""", {("E09", "s4")}),
    ("fail_alias_chain_liveness", """
fn f() {
  let x
  let a = &mut x
  let b = &a
  use a
  let c = &mut x
  use c
  use b
}""", {("E09", "s4"), ("E01", "s5")}),
    ("fail_reborrow_mut_live_use", """
fn f() {
  let x
  let a = &mut x
  let b = &mut a
  use a
  use b
}""", {("E09", "s4")}),
]

# ------------------------------------------------------------------
# 前端結構錯誤 (E00)
# ------------------------------------------------------------------
FRONTEND_CASES = [
    ("front_extra_brace", "fn f() {\n  let x\n}\n}"),
    ("front_two_fn", "fn f() {\n  let x\n}\nfn g() {\n  let y\n}"),
    ("front_self_ref", "fn f() {\n  let a = &a\n}"),
    ("front_undefined", "fn f() {\n  use q\n}"),
    ("front_stmt_outside_fn", "let x"),
]


class TestVerdicts(unittest.TestCase):
    pass


for _name, _code in PASS_CASES:
    setattr(TestVerdicts, "test_" + _name, expect_pass(_name, _code))
for _name, _code, _exp in FAIL_CASES:
    setattr(TestVerdicts, "test_" + _name, expect_fail(_name, _code, _exp))


class TestFrontend(unittest.TestCase):
    def _e00(self, code):
        pr, dl, errors = C.check(code, "nll")
        self.assertTrue(errors, "應有前端錯誤")
        self.assertTrue(all(e[0] == "E00" for e in errors),
                        "應僅為 E00 前端錯誤, 實際: %s" % errors)

    def test_extra_brace(self):
        self._e00("fn f() {\n  let x\n}\n}")

    def test_two_fn_independent(self):
        # v0.3: 每檔可多 fn, 各自獨立檢查 (無跨 fn 干擾; 同名 place 不衝突)
        code = ("fn f() {\n  let x\n}\n"
                "fn g() {\n  let x\n  let a = &x\n  set x\n  use a\n}")
        pr, dl, errors = C.check(code, "nll")
        self.assertEqual(len(errors), 1, "僅 g 之違規: %s" % errors)
        self.assertEqual(errors[0][0], "E02")
        self.assertEqual(errors[0][1], "s4")   # 違規於寫入點 (set x)
        self.assertEqual(len(pr.dls), 2)       # 每 fn 一個引擎

    def test_self_ref(self):
        self._e00("fn f() {\n  let a = &a\n}")

    def test_undefined(self):
        self._e00("fn f() {\n  use q\n}")

    def test_stmt_outside_fn(self):
        self._e00("let x")


class TestLivenessModes(unittest.TestCase):
    def test_referent_is_stricter(self):
        # NLL 接受 (參考用完即死), referent 級拒絕 (被借者未用完)
        code = """
fn f() {
  let x
  let a = &x
  use a
  let b = &mut x
  use b
}"""
        v_nll, _ = verdict(code, "nll")
        v_ref, _ = verdict(code, "referent")
        self.assertEqual(v_nll, "PASS")
        self.assertEqual(v_ref, "FAIL")

    def test_nll_and_referent_agree_on_clash(self):
        code = """
fn f() {
  let x
  let a = &x
  use a
  let b = &mut x
  use a
  use b
}"""
        v_nll, e_nll = verdict(code, "nll")
        v_ref, e_ref = verdict(code, "referent")
        self.assertEqual(v_nll, "FAIL")
        self.assertEqual(v_ref, "FAIL")
        self.assertIn(("E01", "s4"), e_nll)
        self.assertIn(("E01", "s4"), e_ref)


class TestEngine(unittest.TestCase):
    def test_variable_convention(self):
        # 常數 (陳述/作用域/名稱) 小寫開頭; 變數大寫或 _ 開頭
        self.assertTrue(C.is_var("S"))
        self.assertTrue(C.is_var("T1"))
        self.assertTrue(C.is_var("_g1"))
        self.assertTrue(C.is_var("_"))
        self.assertFalse(C.is_var("s5"))
        self.assertFalse(C.is_var("x"))
        self.assertFalse(C.is_var("n0"))
        self.assertFalse(C.is_var("mut"))

    def test_reach_transitive_and_reflexive(self):
        dl = C.Datalog()
        for p in ["s1", "s2", "s3"]:
            dl.add_fact("stmt", (p,))
        dl.add_fact("edge", ("s1", "s2"))
        dl.add_fact("edge", ("s2", "s3"))
        dl.add_rule("reach", ("A", "B"), [("edge", ["A", "B"])], [])
        dl.add_rule("reach", ("A", "A"), [("stmt", ["A"])], [])
        dl.add_rule("reach", ("A", "C"), [("reach", ["A", "B"]), ("edge", ["B", "C"])], [])
        dl.run()
        r = {(t[0], t[1]) for t in dl.facts["reach"]}
        self.assertIn(("s1", "s3"), r)
        self.assertIn(("s1", "s1"), r)
        self.assertNotIn(("s3", "s1"), r)

    def test_neq(self):
        dl = C.Datalog()
        dl.add_fact("p", ("a",))
        dl.add_fact("p", ("b",))
        dl.add_rule("q", ("X",), [("p", ["X"]), ("neq", ["X", "a"])], [])
        dl.run()
        self.assertEqual(dl.facts.get("q"), {("b",)})

    def test_stratified_negation(self):
        dl = C.Datalog()
        dl.add_fact("use", ("s1", "t"))
        dl.add_fact("use", ("s2", "t"))
        dl.add_fact("after", ("s1", "s2"))
        dl.add_rule("sh", ("S", "T"), [("use", ["A", "T"]), ("after", ["S", "A"])], [])
        dl.add_rule("last", ("S", "T"), [("use", ["S", "T"])], [("sh", ["S", "T"])])
        dl.run()
        self.assertEqual(dl.facts.get("last"), {("s2", "t")})

    def test_path_builtins(self):
        dl = C.Datalog()
        dl.add_fact("p", ("x",))
        dl.add_fact("p", ("x.f",))
        dl.add_fact("p", ("x.g",))
        dl.add_fact("p", ("y",))
        # path_conflict: x 與 x.f 衝突; x 與 x.g 衝突; x 與 y 不衝突
        dl.add_rule("c", ("A",), [("p", ["A"]), ("path_conflict", ["A", "x"])], [])
        dl.run()
        self.assertEqual(dl.facts.get("c"), {("x",), ("x.f",), ("x.g",)})
        # covers: 寫 x 覆蓋 x.f; 寫 x.f 不覆蓋 x
        dl2 = C.Datalog()
        dl2.add_fact("w", ("x",))
        dl2.add_fact("w", ("x.f",))
        dl2.add_rule("cv", ("A",), [("w", ["A"]), ("covers", ["A", "x.f"])], [])
        dl2.run()
        self.assertEqual(dl2.facts.get("cv"), {("x",), ("x.f",)})
        # subpath: x.f 深於 x; x.g 不深於 x.f
        dl3 = C.Datalog()
        dl3.add_fact("q", ("x.f",))
        dl3.add_fact("q", ("x.g",))
        dl3.add_rule("sp", ("A",), [("q", ["A"]), ("subpath", ["A", "x"])], [])
        dl3.run()
        self.assertEqual(dl3.facts.get("sp"), {("x.f",), ("x.g",)})
        dl3b = C.Datalog()
        dl3b.add_fact("q", ("x.f",))
        dl3b.add_fact("q", ("x.g",))
        dl3b.add_rule("sp", ("A",), [("q", ["A"]), ("subpath", ["A", "x.f"])], [])
        dl3b.run()
        self.assertEqual(dl3b.facts.get("sp") or set(), set())  # x.g 不深於 x.f

    def test_rule_provenance(self):
        dl = C.Datalog()
        dl.add_fact("edge", ("s1", "s2"))
        dl.add_rule("reach", ("A", "B"), [("edge", ["A", "B"])], [])
        dl.run()
        key = ("reach", ("s1", "s2"))
        self.assertIn(key, dl.why)
        rule, body = dl.why[key]
        self.assertIn(("edge", ("s1", "s2")), body)


class TestParserScopes(unittest.TestCase):
    """回歸: 已關閉區塊/迴圈之後的陳述屬於外層作用域
    (舊 bug: cur() 回傳最後「建立」的 scope 而非堆疊頂端 → 陳述誤入已閉作用域)"""

    def test_write_after_loop_not_in_loop(self):
        code = """
fn f() {
  let v
  let r = &v
  loop {
    use r
  }
  set v
}"""
        pr, dl, errors = C.check(code, "nll")
        self.assertEqual(errors, [])
        self.assertEqual(pr.stmts[3][2], "n1")          # set v 在 fn 作用域
        self.assertEqual(pr.loop_scopes[0]["stmts"], ["s3"])

    def test_stmt_after_block(self):
        code = """
fn f() {
  let x
  {
    use x
  }
  set x
}"""
        pr, dl, errors = C.check(code, "nll")
        self.assertEqual(errors, [])
        self.assertEqual(pr.stmts[2][2], "n1")          # set x 在 fn 作用域

    def test_block_decl_not_visible_after(self):
        code = """
fn f() {
  {
    let y
  }
  use y
}"""
        pr, dl, errors = C.check(code, "nll")
        self.assertTrue(errors)
        self.assertTrue(all(e[0] == "E00" for e in errors))


class TestV03Features(unittest.TestCase):
    """v0.3: if 分支 / lexical 等級 / 多 fn / 字段路徑 (split borrow)"""

    def test_if_use_does_not_extend_liveness(self):
        # NLL 益處: 分支內使用不延長活度過分支 → 分支後寫入合法
        code = """
fn f() {
  let x
  let a = &x
  if {
    use a
  }
  set x
}"""
        v, errs = verdict(code, "nll")
        self.assertEqual(v, "PASS", errs)

    def test_if_clash_inside_branch(self):
        code = """
fn f() {
  let x
  let a = &x
  if {
    let b = &mut x
    use a
    use b
  }
}"""
        v, errs = verdict(code, "nll")
        self.assertEqual(v, "FAIL")
        self.assertIn(("E01", "s3"), errs)   # b 之出借點 (if 內)

    def test_lexical_stricter_than_nll(self):
        # nll: a 用完即死 → 寫入合法; lexical: a 活到作用域尾 → 寫入違規
        code = """
fn f() {
  let x
  let a = &x
  use a
  set x
}"""
        v_nll, _ = verdict(code, "nll")
        v_lex, e_lex = verdict(code, "lexical")
        self.assertEqual(v_nll, "PASS")
        self.assertEqual(v_lex, "FAIL")
        self.assertIn(("E02", "s4"), e_lex)

    def test_field_split_borrow_ok(self):
        # x.f 與 x.g 無路徑衝突 → 可共存 (split borrow)
        code = """
fn f() {
  let x
  let a = &x.f
  let b = &mut x.g
  use a
  use b
}"""
        v, errs = verdict(code, "nll")
        self.assertEqual(v, "PASS", errs)

    def test_field_vs_whole_clash(self):
        # 整體 x 之 mut 借用與 x.f 之 sh 借用路徑衝突 → E01
        code = """
fn f() {
  let x
  let a = &x.f
  let b = &mut x
  use a
  use b
}"""
        v, errs = verdict(code, "nll")
        self.assertEqual(v, "FAIL")
        self.assertIn(("E01", "s3"), errs)

    def test_subfield_write_after_whole_move(self):
        # 整體 move 後寫子字段 = 使用被移值 → E06 (rustc E0382)
        code = """
fn f() {
  let x
  mv x
  set x.f
}"""
        v, errs = verdict(code, "nll")
        self.assertEqual(v, "FAIL")
        self.assertIn(("E06", "s3"), errs)

    def test_whole_reinit_after_field_move(self):
        # 字段 move 後重新初始化整體 → 合法
        code = """
fn f() {
  let x
  mv x.f
  set x
  use x
}"""
        v, errs = verdict(code, "nll")
        self.assertEqual(v, "PASS", errs)

    def test_unknown_liveness_rejected(self):
        with self.assertRaises(ValueError):
            C.check("fn f() {\n  let x\n}", "bogus")


class TestV04Rules32(unittest.TestCase):
    """v0.4: E11–E32 + 新前端 + 32 則規則各有對應錯誤碼"""

    def test_imm_write(self):
        v, e = verdict("fn f() {\n  let imm x\n  set x\n}")
        self.assertEqual(v, "FAIL")
        self.assertIn(("E11", "s2"), e)

    def test_imm_mut_borrow(self):
        v, e = verdict("fn f() {\n  let imm x\n  let a = &mut x\n  use a\n}")
        self.assertEqual(v, "FAIL")
        self.assertIn(("E12", "s2"), e)

    def test_assign_through_sh(self):
        v, e = verdict("fn f() {\n  let x\n  let a = &x\n  set *a\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E13", "s3")})

    def test_move_out_of_ref(self):
        v, e = verdict("fn f() {\n  let x\n  let a = &x\n  mv *a\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E14", "s3")})

    def test_temp_non_adjacent(self):
        v, e = verdict("fn f() {\n  tmp t\n  let a = &t\n  let x\n  use a\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E15", "s4")})

    def test_temp_adjacent_ok(self):
        v, e = verdict("fn f() {\n  tmp t\n  let a = &t\n  use a\n}")
        self.assertEqual(v, "PASS", e)

    def test_escape_store(self):
        v, e = verdict("fn f() {\n  slot s\n  let y\n  let a = &y\n  store s = a\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E16", "s4")})

    def test_store_temp(self):
        v, e = verdict("fn f() {\n  slot s\n  tmp t\n  let a = &t\n  store s = a\n}")
        self.assertEqual(v, "FAIL")
        self.assertIn(("E17", "s4"), e)

    def test_branch_move(self):
        v, e = verdict("fn f() {\n  let x\n  if {\n    mv x\n  }\n  use x\n}")
        self.assertEqual(v, "FAIL")
        self.assertIn(("E18", "s3"), e)

    def test_double_drop(self):
        v, e = verdict("fn f() {\n  let x\n  dp x\n  dp x\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E19", "s3")})

    def test_drop_after_move(self):
        v, e = verdict("fn f() {\n  let x\n  mv x\n  dp x\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E20", "s3")})

    def test_alias_cycle(self):
        v, e = verdict("fn f() {\n  slot s\n  let a = &s\n  store s = a\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E21", "s3")})

    def test_imm_field_write(self):
        v, e = verdict("fn f() {\n  let imm x\n  set x.f\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E22", "s2")})

    def test_imm_field_mut(self):
        v, e = verdict("fn f() {\n  let imm x\n  let a = &mut x.f\n  use a\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E23", "s2")})

    def test_else_join_move(self):
        v, e = verdict("fn f() {\n  let x\n  if {\n    set x\n  } else {\n    mv x\n  }\n  use x\n}")
        self.assertEqual(v, "FAIL")
        self.assertIn(("E24", "s4"), e)

    def test_call_during_mut(self):
        v, e = verdict("fn f() {\n  let x\n  let a = &mut x\n  call g(x)\n  use a\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E25", "s3")})

    def test_callmv_then_use(self):
        v, e = verdict("fn f() {\n  let x\n  callmv take(x)\n  use x\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E26", "s3")})

    def test_callmv_while_loan(self):
        v, e = verdict("fn f() {\n  let x\n  let a = &x\n  callmv take(x)\n  use a\n}")
        self.assertEqual(v, "FAIL")
        self.assertIn(("E27", "s3"), e)

    def test_ret_temp(self):
        v, e = verdict("fn f() {\n  tmp t\n  let a = &t\n  ret a\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E28", "s3")})

    def test_uninit_use(self):
        v, e = verdict("fn f() {\n  let hole x\n  use x\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E29", "s2")})

    def test_uninit_move(self):
        v, e = verdict("fn f() {\n  let hole x\n  mv x\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E30", "s2")})

    def test_imm_param_write(self):
        v, e = verdict("fn f(imm p) {\n  set p\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E31", "s1")})

    def test_imm_param_mut(self):
        v, e = verdict("fn f(imm p) {\n  let a = &mut p\n  use a\n}")
        self.assertEqual(v, "FAIL")
        self.assertEqual(e, {("E32", "s1")})

    def test_imm_shared_ok(self):
        v, e = verdict("fn f() {\n  let imm x\n  let a = &x\n  use a\n}")
        self.assertEqual(v, "PASS", e)

    def test_hole_init_ok(self):
        v, e = verdict("fn f() {\n  let hole x\n  set x\n  use x\n}")
        self.assertEqual(v, "PASS", e)

    def test_else_both_move_reinit_ok(self):
        v, e = verdict("fn f() {\n  let x\n  if {\n    mv x\n  } else {\n    mv x\n  }\n  set x\n  use x\n}")
        self.assertEqual(v, "PASS", e)

    def test_r32_examples_each_fires_own_code(self):
        expect = {
            "r01_eclash.cl": "E01", "r02_ewrite.cl": "E02", "r03_eread.cl": "E03",
            "r04_edangle.cl": "E04", "r05_eadrop.cl": "E05", "r06_emove.cl": "E06",
            "r07_eloanmove.cl": "E07", "r08_eloandrop.cl": "E08", "r09_erefuse.cl": "E09",
            "r10_ereturn.cl": "E10", "r11_eimmut.cl": "E11", "r12_enotmut.cl": "E12",
            "r13_eassignsh.cl": "E13", "r14_emoveout.cl": "E14", "r15_etemp.cl": "E15",
            "r16_eescape.cl": "E16", "r17_estoretemp.cl": "E17", "r18_ebranchmove.cl": "E18",
            "r19_edoubledrop.cl": "E19", "r20_edropmoved.cl": "E20", "r21_ealiascyc.cl": "E21",
            "r22_eimmfield.cl": "E22", "r23_enotmutfield.cl": "E23", "r24_eelsejoin.cl": "E24",
            "r25_ecallmut.cl": "E25", "r26_ecallmove.cl": "E26", "r27_ecalloan.cl": "E27",
            "r28_erettemp.cl": "E28", "r29_euninit.cl": "E29", "r30_euninitret.cl": "E30",
            "r31_eparamimmut.cl": "E31", "r32_eparamnotmut.cl": "E32",
        }
        for name, code in expect.items():
            path = os.path.join(HERE, "examples", name)
            with open(path, encoding="utf-8") as f:
                src = f.read()
            v, errs = verdict(src, "nll")
            self.assertEqual(v, "FAIL", "%s 應 FAIL" % name)
            self.assertTrue(any(c == code for c, _s in errs),
                            "%s 應含 %s, 實際 %s" % (name, code, errs))


class TestExamples(unittest.TestCase):

    def test_corpus(self):
        expected = {
            "ex1_clash.cl": "FAIL",
            "ex2_sequential.cl": "PASS",
            "ex3_return.cl": "FAIL",
            "ex3b_return_param.cl": "PASS",
            "ex4_loop_ok.cl": "PASS",
            "ex5_loop_write.cl": "FAIL",
            "ex6_move.cl": "FAIL",
            "ex7_alias_use.cl": "FAIL",
            "ex8_reassign.cl": "PASS",
            "ex9_two_sh.cl": "PASS",
            "ex10_mut_seq.cl": "PASS",
            "ex11_alias_chain.cl": "FAIL",
            "ex12_if_branch.cl": "PASS",
            "ex13_if_clash.cl": "FAIL",
            "ex14_field_split.cl": "PASS",
            "ex15_field_clash.cl": "FAIL",
            "ex16_two_fn.cl": "FAIL",
        }
        for name, want in expected.items():
            path = os.path.join(HERE, "examples", name)
            with open(path, encoding="utf-8") as f:
                code = f.read()
            v, _ = verdict(code, "nll")
            self.assertEqual(v, want, "%s 應為 %s" % (name, want))



# ------------------------------------------------------------------
# P-G1 / P-G2 / P-G3 / P-G4: 幾何法則可測、圖=事實、可執行引理
# ------------------------------------------------------------------
CLASH = """
fn f() {
  let x
  let a = &x
  use a
  let b = &mut x
  use a
  use b
}
"""

TWO_SH = """
fn f() {
  let x
  let a = &x
  let b = &x
  use a
  use b
}
"""

SEQ_MUT = """
fn f() {
  let x
  let a = &mut x
  use a
  let b = &mut x
  use b
}
"""

HOLE_ELSE = """
fn f() {
  let x
  let a = &x
  if {
    let z
    use z
  } else {
    use a
  }
}
"""


class TestGeometry(unittest.TestCase):
    """法則 1/2/5/6/7/8 守衛 + 圖對齊 + 健全/完備/Gavril 引理 + --explain。"""

    def test_law1_cross_iff_overlap(self):
        pr, dl, errors = C.check(CLASH, "nll")
        self.assertTrue(any(c == "E01" for c, _s, _t in errors), errors)
        pairs = C.law1_overlap_pairs(pr, dl)
        self.assertTrue(pairs, "eclash 存在 => 衝突弧區間真重疊")
        for a, b, inter in pairs:
            self.assertTrue(inter)
            self.assertTrue(a["I"] & b["I"])
        svg = C.render_svg(pr, dl, errors, "clash", "nll")
        self.assertIn('text-anchor="middle">✕</text>', svg)

    def test_law1_no_false_x(self):
        pr, dl, errors = C.check(TWO_SH, "nll")
        self.assertEqual(errors, [])
        self.assertEqual(C.eclash_pairs(pr, dl, errors), [])
        svg = C.render_svg(pr, dl, errors, "two_sh", "nll")
        self.assertNotIn('text-anchor="middle">✕</text>', svg)

    def test_law2_endpoints_in_circle(self):
        code = """
fn f() {
  let x
  let a = &x
  use a
}
"""
        pr, dl, errors = C.check(code, "nll")
        self.assertEqual(errors, [])
        arcs = C.collect_arcs(pr, dl)
        self.assertTrue(arcs)
        by_id = {s["id"]: s for s in pr.scopes}
        for a in arcs:
            root = a["Q"].split(".", 1)[0]
            qsc = pr.scope_of.get(root)
            if qsc is None or qsc not in by_id:
                continue
            sc_stmts = pr.all_stmts.get(qsc) or []
            if not sc_stmts:
                continue
            ys = [C.Y0 + C.DY * pr.idx[s] for s in sc_stmts]
            cy = (min(ys) + max(ys)) / 2.0
            ry = (max(ys) - min(ys)) / 2.0 + 34
            for sid in (a["L"], a["E"]):
                yy = C.Y0 + C.DY * pr.idx[sid]
                self.assertGreaterEqual(yy, cy - ry, "%s 端點掉出圓" % sid)
                self.assertLessEqual(yy, cy + ry, "%s 端點掉出圓" % sid)

    def test_law5_nested_radii(self):
        code = """
fn f() {
  let x
  {
    let y
    loop {
      use y
    }
  }
}
"""
        pr, dl, _errors = C.check(code, "nll")
        for sc in pr.scopes:
            if sc["parent"] is None:
                continue
            parent = next(s for s in pr.scopes if s["id"] == sc["parent"])
            self.assertLess(C.scope_rx(pr, sc), C.scope_rx(pr, parent),
                            "parent(%s,%s) => r_child < r_parent" % (sc["id"], parent["id"]))

    def test_law6_backedge_closes(self):
        path = os.path.join(HERE, "examples", "ex5_loop_write.cl")
        with open(path, encoding="utf-8") as f:
            code = f.read()
        pr, dl, errors = C.check(code, "nll")
        self.assertTrue(pr.loop_scopes)
        loop = pr.loop_scopes[0]
        all_s = pr.all_stmts[loop["id"]]
        first, last = all_s[0], all_s[-1]
        self.assertIn(("edge", (last, first)), pr.facts)
        arcs = C.collect_arcs(pr, dl)
        self.assertTrue(any(first in a["I"] for a in arcs),
                        "loop 內 use => onregion 含 loop 首")
        self.assertTrue(any(c == "E02" for c, _s, _t in errors))

    def test_law7_no_sibling_edge(self):
        code = """
fn f() {
  let x
  if {
    set x
  } else {
    use x
  }
}
"""
        pr, dl, _errors = C.check(code, "nll")
        ifs = [s for s in pr.scopes if s["kind"] == "if"]
        elses = [s for s in pr.scopes if s["kind"] == "else"]
        self.assertTrue(ifs and elses)
        if_s = set(pr.all_stmts[ifs[0]["id"]])
        else_s = set(pr.all_stmts[elses[0]["id"]])
        edges = {t for pred, t in pr.facts if pred == "edge"}
        for a in if_s:
            for b in else_s:
                self.assertNotIn((a, b), edges)
                self.assertNotIn((b, a), edges)

    def test_law8_split_no_conflict(self):
        path = os.path.join(HERE, "examples", "ex14_field_split.cl")
        with open(path, encoding="utf-8") as f:
            code = f.read()
        pr, dl, errors = C.check(code, "nll")
        self.assertFalse(C.conflicting("x.f", "x.g"))
        self.assertFalse(any(c == "E01" for c, _s, _t in errors), errors)
        arcs = C.collect_arcs(pr, dl)
        self.assertGreaterEqual(len(arcs), 2)
        inter = arcs[0]["I"] & arcs[1]["I"]
        self.assertTrue(inter, "異字段兩弧可 onregion 同點")

    def test_pg2_holes_are_dashed(self):
        pr, dl, errors = C.check(HOLE_ELSE, "nll")
        arcs = C.collect_arcs(pr, dl)
        self.assertTrue(arcs)
        self.assertTrue(C.arc_has_hole(pr, arcs[0]), "else 終端使用 => if 支為空洞")
        svg = C.render_svg(pr, dl, errors, "hole", "nll")
        self.assertIn('stroke="#3b82f6"', svg)
        self.assertIn('stroke-dasharray="6 5"', svg)

    def test_pg2_maybe_live_mark(self):
        path = os.path.join(HERE, "examples", "r18_ebranchmove.cl")
        with open(path, encoding="utf-8") as f:
            code = f.read()
        pr, dl, errors = C.check(code, "nll")
        self.assertTrue(any(c == "E18" for c, _s, _t in errors), errors)
        svg = C.render_svg(pr, dl, errors, "r18", "nll")
        self.assertIn("可能活", svg)
        self.assertIn("E18", svg)

    def test_lemma_soundness(self):
        """健全性: 滿足法則 1 (無衝突 mut 重疊) 的 PASS 語料不推出 E01-E32。"""
        for name, code in PASS_CASES:
            pr, dl, errors = C.check(code, "nll")
            if dl is None:
                continue
            self.assertEqual(errors, [], name)
            self.assertEqual(C.law1_overlap_pairs(pr, dl), [], name)

    def test_lemma_completeness(self):
        """完備性: 每條 E0k 對應恰好一條法則; rXX 的幾何報告命中該法則。"""
        for _pred, (code, _msg) in C.ERROR_CODES.items():
            self.assertIn(code, C.LAW_OF, "%s 未對應法則" % code)
        expect = {
            "r01_eclash.cl": 1, "r04_edangle.cl": 2, "r06_emove.cl": 3,
            "r02_ewrite.cl": 4, "r21_ealiascyc.cl": 9, "r11_eimmut.cl": 10,
        }
        for name, want_law in expect.items():
            path = os.path.join(HERE, "examples", name)
            with open(path, encoding="utf-8") as f:
                src = f.read()
            pr, dl, errors = C.check(src, "nll")
            self.assertTrue(errors, name)
            law, lines = C.geometry_report(pr, dl, errors[0][0], errors[0][1])
            self.assertEqual(law, want_law, "%s 應對法則 %s, 得 %s" % (name, want_law, law))
            self.assertTrue(lines)

    def test_lemma_gavril(self):
        """Gavril: 無空洞直線程序上, 衝突弧 I 相交 <=> 文本區間真重疊 <=> overlap。"""
        pr, dl, errors = C.check(CLASH, "nll")
        pairs = C.law1_overlap_pairs(pr, dl)
        self.assertTrue(pairs)
        self.assertTrue(dl.facts.get("overlap"))
        for a, b, inter in pairs:
            self.assertFalse(C.arc_has_hole(pr, a))
            self.assertFalse(C.arc_has_hole(pr, b))
            ia = set(C.textual_span(pr, a["L"], a["E"]))
            ib = set(C.textual_span(pr, b["L"], b["E"]))
            self.assertEqual(a["I"], ia)
            self.assertEqual(b["I"], ib)
            self.assertEqual(inter, ia & ib)
            self.assertTrue(ia & ib)
        pr2, dl2, err2 = C.check(SEQ_MUT, "nll")
        self.assertEqual(err2, [])
        self.assertEqual(C.law1_overlap_pairs(pr2, dl2), [])
        self.assertFalse(dl2.facts.get("overlap"))

    def test_explain_geometry_block(self):
        pr, dl, errors = C.check(CLASH, "nll")
        code, sid, _t = errors[0]
        law, lines = C.geometry_report(pr, dl, code, sid)
        self.assertEqual(law, 1)
        joined = "\n".join(lines)
        self.assertIn("I(a) ∩ I(b)", joined)
        self.assertIn("縮弧", joined)


if __name__ == "__main__":
    unittest.main(verbosity=2)
