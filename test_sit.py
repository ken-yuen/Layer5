#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Layer6：左樹右樹 / parent / 呼叫圖。"""
import unittest

import sit


SRC = """
fn f(p) {
  let x
  let a = &x
  if {
    use a
  }
  call g(x)
}
fn g(q) {
  use q
}
"""


class TestLCRS(unittest.TestCase):
    def test_parent_pointer_and_siblings(self):
        f = sit.build(SRC)
        fns = [c for c in sit.iter_children(f.semantic) if c.kind == "fn"]
        self.assertEqual([n.attrs.get("name") or n.text for n in fns], ["f", "g"])
        # 兄弟：f.right == g，g.up == crate
        self.assertIs(fns[0].right, fns[1])
        self.assertIs(fns[1].up, f.semantic)
        self.assertIs(fns[0].up, f.semantic)

    def test_left_child_is_first(self):
        f = sit.build(SRC)
        fnf = f.fn_by_name["f"]
        kids = list(sit.iter_children(fnf))
        kinds = [k.kind for k in kids]
        self.assertIn("param", kinds)
        self.assertIn("if", kinds)
        self.assertIn("call", kinds)
        # 第一孩 = left
        self.assertIs(fnf.left, kids[0])
        # 兄弟鏈連到最後
        s = fnf.left
        while s.right:
            s = s.right
        self.assertIs(s, kids[-1])

    def test_has_ancestor_if_to_fn(self):
        f = sit.build(SRC)
        uses = [n for n in sit.walk_preorder(f.semantic) if n.kind == "use"]
        self.assertTrue(uses)
        fn = sit.has_ancestor(uses[0], kind="fn")
        self.assertIsNotNone(fn)
        self.assertEqual(fn.attrs.get("name") or fn.text, "f")

    def test_call_graph_traversal(self):
        f = sit.build(SRC)
        self.assertTrue(any(c == "g" for _a, c, _s in f.call_edges))
        reach = sit.call_graph_reachable(f, "f")
        self.assertEqual(reach[0], "f")
        self.assertIn("g", reach)

    def test_surface_keeps_keywords(self):
        f = sit.build(SRC)
        texts = [n.text for n in sit.walk_preorder(f.surface)]
        self.assertTrue(any(t.startswith("fn f") for t in texts))
        self.assertTrue(any("if" in t for t in texts))

    def test_inherit_lookup_param(self):
        f = sit.build(SRC)
        use_q = [n for n in sit.walk_preorder(f.semantic)
                 if n.kind == "use" and n.text.strip() == "use q"][0]
        decl = sit.inherit_lookup(use_q, "q")
        self.assertIsNotNone(decl)
        self.assertEqual(decl.kind, "param")


if __name__ == "__main__":
    unittest.main(verbosity=2)
