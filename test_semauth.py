#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""認証語義機：只報語義 X，不論表面輸入。"""
import unittest

import semauth

BASE = """
fn f() {
  let x
  let a = &x
  use a
  let b = &mut x
  use b
}
"""

SPACY = """

fn   f ( ) {
  let    x
  let a=&x   // extra comment
  use a
  let b = &mut x
  use b
}

"""

WITH_DIV = """
fn f() {
  let x
  88÷ww
  let a = &x
  use a
  let b = &mut x
  use b
}
"""

IFEQ_TRAP = """
fn f() {
  if=x
  let x
}
"""

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


class TestSemAuth(unittest.TestCase):
    def test_whitespace_comment_same_X(self):
        a = semauth.authenticate(BASE)
        b = semauth.authenticate(SPACY)
        self.assertEqual(a["X"], "PASS")
        self.assertEqual(a["X"], b["X"])
        self.assertEqual(a["digest"], b["digest"])

    def test_div_token_not_a_borrow(self):
        r = semauth.authenticate(WITH_DIV)
        self.assertEqual(r["X"], "PASS")
        self.assertEqual(r["digest"], semauth.authenticate(BASE)["digest"])

    def test_ifeq_is_error_not_invented_if(self):
        r = semauth.authenticate(IFEQ_TRAP)
        self.assertEqual(r["X"], "ERROR")
        self.assertNotEqual(r["X"], "PASS")

    def test_clash_still_fail(self):
        r = semauth.authenticate(CLASH)
        self.assertEqual(r["X"], "FAIL")
        self.assertIn("E01", r.get("codes") or [])

    def test_different_semantics_different_digest(self):
        a = semauth.authenticate(BASE)
        b = semauth.authenticate(CLASH)
        self.assertNotEqual(a["digest"], b["digest"])
        self.assertNotEqual(a["X"], b["X"])

    def test_no_rustc(self):
        self.assertFalse(semauth.authenticate(BASE)["needs_rustc"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
