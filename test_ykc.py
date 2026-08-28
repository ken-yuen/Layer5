#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""YKC：Warshall ≡ Datalog reach；樹自動機；不依賴 rustc。"""
import os
import unittest

import ykc

HERE = os.path.dirname(os.path.abspath(__file__))


class TestYKC(unittest.TestCase):
    def test_warshall_matches_datalog_ex2(self):
        src = open(os.path.join(HERE, "examples", "ex2_sequential.cl"),
                   encoding="utf-8").read()
        y = ykc.analyze(src)
        self.assertTrue(y.reach_match)
        self.assertTrue(y.tree_ok)
        self.assertEqual(y.errors, [])
        self.assertFalse(y.as_dict()["needs_rustc"])

    def test_warshall_matches_on_clash_and_loop(self):
        for name in ("ex1_clash.cl", "ex5_loop_write.cl", "ex12_if_branch.cl",
                     "ex16_two_fn.cl"):
            src = open(os.path.join(HERE, "examples", name), encoding="utf-8").read()
            y = ykc.analyze(src)
            self.assertTrue(y.reach_match, name)
            self.assertTrue(y.tree_ok, name)

    def test_interval_clash_detects_e01(self):
        src = open(os.path.join(HERE, "examples", "ex1_clash.cl"),
                   encoding="utf-8").read()
        y = ykc.analyze(src)
        self.assertGreaterEqual(len(y.loans), 2)
        hit = False
        for i, a in enumerate(y.loans):
            for b in y.loans[i + 1:]:
                if ykc.interval_clash(a["vec"], a["kind"], b["vec"], b["kind"]):
                    hit = True
        self.assertTrue(hit)

    def test_sequential_intervals_disjoint(self):
        src = open(os.path.join(HERE, "examples", "ex2_sequential.cl"),
                   encoding="utf-8").read()
        y = ykc.analyze(src)
        self.assertGreaterEqual(len(y.loans), 2)
        a, b = y.loans[0], y.loans[1]
        self.assertFalse(ykc.vec_any(ykc.vec_and(a["vec"], b["vec"])))


if __name__ == "__main__":
    unittest.main(verbosity=2)
