#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""P1/P2 + AHPBB 工廠測試。"""
import json
import os
import shutil
import tempfile
import unittest

import chordlaw as C
import syn_subset
import quote_tpl
import ahpbb
from intent import tree_from_program, compute_lifetimes, lifetimes_overlap

HERE = os.path.dirname(os.path.abspath(__file__))

EX2_CL = """
fn f() {
  let x
  let a = &x
  use a
  let b = &mut x
  use b
}
"""

EX2_RS = """
fn f() {
    let mut x = 0u32;
    let a = &x;
    let _u1 = *a;
    let b = &mut x;
    *b += 1;
}
fn main() {}
"""

EX1_RS = """
fn f() {
    let mut x = 0u32;
    let a = &x;
    let _u1 = *a;
    let b = &mut x;
    let _u2 = *a;
    *b += 1;
}
fn main() {}
"""


class TestSynSubset(unittest.TestCase):
    def test_ex2_rs_to_cl_pass(self):
        cl, warns, meta = syn_subset.rust_to_cl(EX2_RS)
        pr, dl, errors = C.check(cl, "nll")
        self.assertEqual(errors, [], "ex2 rust→cl 應 PASS: %s\n%s" % (errors, cl))

    def test_ex1_rs_to_cl_fail_e01(self):
        cl, warns, meta = syn_subset.rust_to_cl(EX1_RS)
        pr, dl, errors = C.check(cl, "nll")
        self.assertTrue(any(c == "E01" for c, _s, _t in errors), errors)

    def test_oracle_ex2_file_roundtrip_verdict(self):
        import oracle_check
        cl0, rs, _n = oracle_check.CASES["ex2_sequential"]
        cl1, warns, _m = syn_subset.rust_to_cl(rs)
        v0 = "PASS" if not C.check(cl0, "nll")[2] else "FAIL"
        v1 = "PASS" if not C.check(cl1, "nll")[2] else "FAIL"
        self.assertEqual(v0, v1)

    def test_oracle_verdicts_preserved(self):
        import oracle_check
        bad = []
        for name, (cl0, rs, _n) in oracle_check.CASES.items():
            cl1, warns, _m = syn_subset.rust_to_cl(rs)
            e0 = C.check(cl0, "nll")[2]
            e1 = C.check(cl1, "nll")[2]
            v0 = "PASS" if not e0 else "FAIL"
            v1 = "PASS" if not e1 else "FAIL"
            if v0 != v1:
                bad.append((name, v0, v1, cl1, warns))
        self.assertEqual(bad, [], "verdict drift: %s" % [
            (n, a, b, w) for n, a, b, _c, w in bad])


class TestIntentLifetimes(unittest.TestCase):
    def test_sequential_regions_disjoint(self):
        pr, dl, errors = C.check(EX2_CL, "nll")
        self.assertEqual(errors, [])
        regs = compute_lifetimes(pr, dl)
        self.assertGreaterEqual(len(regs), 2)
        self.assertFalse(lifetimes_overlap(regs[0], regs[1]))

    def test_clash_regions_overlap(self):
        cl = open(os.path.join(HERE, "examples", "ex1_clash.cl"), encoding="utf-8").read()
        pr, dl, errors = C.check(cl, "nll")
        self.assertTrue(any(c == "E01" for c, _s, _t in errors))
        regs = compute_lifetimes(pr, dl)
        self.assertTrue(any(lifetimes_overlap(regs[i], regs[j])
                            for i in range(len(regs)) for j in range(i + 1, len(regs))))

    def test_tree_names_lifetimes(self):
        pr, dl, errors = C.check(EX2_CL, "nll")
        tree = tree_from_program(pr, dl, errors)
        self.assertEqual(tree.verdict, "PASS")
        self.assertTrue(tree.fns[0].regions)
        self.assertTrue(all(r.lt.startswith("'") for r in tree.fns[0].regions))


class TestQuote(unittest.TestCase):
    def test_quote_then_syn_same_verdict(self):
        rs, tree, errors = quote_tpl.quote_cl_to_rust(EX2_CL, "ex2")
        self.assertIn("fn f", rs)
        self.assertIn("fn main", rs)
        cl2, warns, _m = syn_subset.rust_to_cl(rs)
        e2 = C.check(cl2, "nll")[2]
        self.assertEqual(e2, [], cl2)


class TestAHPBB(unittest.TestCase):
    def test_factory_batch_new_dir_only_pass(self):
        tmp = tempfile.mkdtemp(prefix="ahpbb_")
        try:
            fac = ahpbb.AutoHPBorrowBase(workers=3)
            out, man = fac.batch(out_parent=tmp, only_pass=True)
            self.assertTrue(os.path.isdir(out))
            self.assertTrue(out.startswith(tmp))
            self.assertGreaterEqual(man["shipped"], 10)
            self.assertEqual(man["rejected"], 0)
            # 每個出貨產品弦律 PASS
            for name in os.listdir(os.path.join(out, "products")):
                if name.endswith(".cl"):
                    src = open(os.path.join(out, "products", name), encoding="utf-8").read()
                    err = C.check(src, "nll")[2]
                    self.assertEqual(err, [], name)
            # 再開一批 = 另一個新資料夾
            out2, _m2 = fac.batch(specs=ahpbb.RECIPES[:2], out_parent=tmp)
            self.assertNotEqual(out, out2)
            self.assertTrue(os.path.isfile(os.path.join(out, "manifest.json")))
            cargo = os.path.join(out, "Cargo.toml")
            self.assertTrue(os.path.isfile(cargo))
            body = open(cargo, encoding="utf-8").read()
            self.assertIn("[package]", body)
            self.assertIn("[[bin]]", body)
            # 絕不污染 Python 倉根
            self.assertFalse(os.path.isfile(os.path.join(HERE, "Cargo.toml")))
        finally:
            shutil.rmtree(tmp)

    def test_repair_clash_becomes_pass(self):
        fac = ahpbb.AutoHPBorrowBase(workers=1)
        raw = ahpbb.FIX_RECIPES[0][1]
        before = C.check(raw, "nll")[2]
        self.assertTrue(any(c == "E01" for c, _s, _t in before))
        p = fac.repair("fix", raw, "shrink_arc")
        self.assertEqual(p.verdict, "PASS", p.cl)

    def test_repair_return_raise(self):
        fac = ahpbb.AutoHPBorrowBase(workers=1)
        raw = ahpbb.FIX_RECIPES[1][1]
        p = fac.repair("ret", raw, "raise_circle")
        self.assertEqual(p.verdict, "PASS", p.cl)
        self.assertIn("&'a", p.rs or "") or self.assertIn("->", p.rs)

    def test_ingest_rs(self):
        p = ahpbb.AutoHPBorrowBase().make_from_rs(EX2_RS, "ex2")
        self.assertEqual(p.verdict, "PASS")


if __name__ == "__main__":
    unittest.main(verbosity=2)
