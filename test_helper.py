#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""P3a: helper 評分/持久化 + MCP 協議煙霧。"""
import io
import json
import os
import shutil
import tempfile
import unittest

import chordlaw as C
import helper
import mcp_server

HERE = os.path.dirname(os.path.abspath(__file__))

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

OK = """
fn f() {
  let x
  let a = &x
  use a
  let b = &mut x
  use b
}
"""


class TestHelper(unittest.TestCase):
    def test_advise_shrink_arc_on_clash(self):
        pr, dl, errors, data = helper.explain_source(CLASH, "clash.cl")
        self.assertEqual(data["verdict"], "FAIL")
        self.assertTrue(data["advice"])
        a = data["advice"][0]
        self.assertEqual(a["code"], "E01")
        self.assertEqual(a["op"], "shrink_arc")
        self.assertEqual(a["priority"], "P0")
        self.assertTrue(a["geometry"])

    def test_score_all_pass_high(self):
        _pr, _dl, _e, ok = helper.explain_source(OK, "ok.cl")
        sc = helper.score_project([ok], [])
        self.assertEqual(sc["counts"]["fail"], 0)
        self.assertGreaterEqual(sc["dimensions"]["correctness"], 99)
        self.assertGreaterEqual(sc["overall"], 85)
        self.assertEqual(sc["band"], "ready")

    def test_score_coverage_penalizes_skipped_rs(self):
        _pr, _dl, _e, ok = helper.explain_source(OK, "ok.cl")
        sc = helper.score_project([ok], [{"path": "src/lib.rs", "reason": "no_syn_frontend"}])
        self.assertLess(sc["dimensions"]["coverage"], 100)
        self.assertLess(sc["overall"], helper.score_project([ok], [])["overall"])

    def test_persist_and_history(self):
        tmp = tempfile.mkdtemp(prefix="clstate_")
        try:
            os.makedirs(os.path.join(tmp, "src"))
            with open(os.path.join(tmp, "src", "ok.cl"), "w", encoding="utf-8") as f:
                f.write(OK)
            s1 = helper.run_project(tmp, persist=True)
            self.assertTrue(os.path.isfile(helper.state_path(tmp)))
            self.assertEqual(s1["score"]["counts"]["checked"], 1)
            with open(os.path.join(tmp, "src", "bad.cl"), "w", encoding="utf-8") as f:
                f.write(CLASH)
            s2 = helper.run_project(tmp, persist=True)
            self.assertTrue(s2["diff"]["new"])
            hist = helper.history_summary(tmp, 5)
            self.assertEqual(hist["n"], 2)
            raw = open(helper.state_path(tmp), encoding="utf-8").read()
            self.assertNotIn("let a = &x", raw)  # 不存源碼
        finally:
            shutil.rmtree(tmp)

    def test_product_score_honest_rust_gap(self):
        p = helper.product_score()
        # P2 syn 子集已上，但仍非全量 rustc — 分數在中段
        self.assertGreaterEqual(p["dimensions"]["rust_coverage"]["score"], 40)
        self.assertLess(p["dimensions"]["rust_coverage"]["score"], 80)


class TestMCP(unittest.TestCase):
    def _rpc(self, msgs):
        ctx = mcp_server.Ctx()
        out = []
        for m in msgs:
            r = mcp_server.dispatch_line(json.dumps(m, ensure_ascii=False), ctx)
            if r is not None:
                out.append(r)
        return out, ctx

    def test_initialize_and_tools(self):
        res, _ = self._rpc([
            {"jsonrpc": "2.0", "id": 1, "method": "initialize",
             "params": {"protocolVersion": "2024-11-05",
                        "capabilities": {}, "clientInfo": {"name": "t", "version": "0"}}},
            {"jsonrpc": "2.0", "method": "notifications/initialized"},
            {"jsonrpc": "2.0", "id": 2, "method": "tools/list"},
        ])
        self.assertEqual(res[0]["result"]["serverInfo"]["name"], "chordlaw")
        names = {t["name"] for t in res[1]["result"]["tools"]}
        self.assertGreaterEqual(names, {
            "chordlaw_check", "chordlaw_explain", "chordlaw_diagram",
            "chordlaw_rules", "chordlaw_report", "chordlaw_history",
        })

    def test_check_tool_returns_advice(self):
        res, _ = self._rpc([
            {"jsonrpc": "2.0", "id": 3, "method": "tools/call",
             "params": {"name": "chordlaw_check",
                        "arguments": {"source": CLASH}}},
        ])
        body = json.loads(res[0]["result"]["content"][0]["text"])
        self.assertEqual(body["verdict"], "FAIL")
        self.assertEqual(body["advice"][0]["op"], "shrink_arc")

    def test_stdio_lines_are_json(self):
        req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize",
                          "params": {"protocolVersion": "2024-11-05",
                                     "capabilities": {},
                                     "clientInfo": {"name": "t", "version": "0"}}})
        stdin = io.StringIO(req + "\n")
        stdout = io.StringIO()
        mcp_server.serve_stdio(stdin, stdout)
        line = stdout.getvalue().strip()
        json.loads(line)
        self.assertNotIn("\n", line)

    def test_rs_syn_subset_accepted(self):
        tmp = tempfile.NamedTemporaryFile("w", suffix=".rs", delete=False, encoding="utf-8")
        try:
            tmp.write(
                "fn f() {\n"
                "    let mut x = 0u32;\n"
                "    let a = &x;\n"
                "    let _u1 = *a;\n"
                "}\nfn main() {}\n"
            )
            tmp.close()
            res, _ = self._rpc([
                {"jsonrpc": "2.0", "id": 4, "method": "tools/call",
                 "params": {"name": "chordlaw_check",
                            "arguments": {"path": tmp.name}}},
            ])
            self.assertFalse(res[0]["result"].get("isError"))
            body = json.loads(res[0]["result"]["content"][0]["text"])
            self.assertEqual(body["verdict"], "PASS")
        finally:
            os.unlink(tmp.name)

    def test_rs_outside_subset_rejected(self):
        res, _ = self._rpc([
            {"jsonrpc": "2.0", "id": 5, "method": "tools/call",
             "params": {"name": "chordlaw_check",
                        "arguments": {"source": "mod x { impl Foo {} }",
                                      "name": "lib.rs"}}},
        ])
        self.assertTrue(res[0]["result"].get("isError"))


if __name__ == "__main__":
    unittest.main(verbosity=2)
