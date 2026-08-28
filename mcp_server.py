#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
弦律 MCP 伺服器 — stdio JSON-RPC（協議 2024-11-05 相容）
stdout 只准 MCP 訊息；日誌走 stderr。
"""
from __future__ import print_function

import json
import os
import sys
import traceback

import chordlaw as C
import helper

PROTOCOL = "2024-11-05"

TOOLS = [
    {
        "name": "chordlaw_check",
        "description": "用弦律檢查 .cl，或 syn 子集 .rs（自動降到 .cl）。回傳 verdict、errors+證明、regions、幾何建議。非子集 .rs 仍直通 rustc。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "source": {"type": "string", "description": ".cl 源碼本文"},
                "path": {"type": "string", "description": ".cl 檔路徑"},
                "liveness": {"type": "string", "enum": ["nll", "referent", "lexical"],
                             "default": "nll"},
            },
        },
    },
    {
        "name": "chordlaw_explain",
        "description": "對錯誤給規則原文、證明樹、幾何違反（I(a)∩I(b)）與縮弧/移點/升圓修法。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "source": {"type": "string"},
                "path": {"type": "string"},
                "liveness": {"type": "string", "enum": ["nll", "referent", "lexical"],
                             "default": "nll"},
            },
        },
    },
    {
        "name": "chordlaw_diagram",
        "description": "渲染縱點節圖 SVG。可選寫到 out 路徑。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "source": {"type": "string"},
                "path": {"type": "string"},
                "out": {"type": "string"},
                "liveness": {"type": "string", "enum": ["nll", "referent", "lexical"],
                             "default": "nll"},
            },
        },
    },
    {
        "name": "chordlaw_rules",
        "description": "規則規格。可選 code=E01…E32 取單則。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "code": {"type": "string", "description": "如 E01"},
            },
        },
    },
    {
        "name": "chordlaw_report",
        "description": "檢查目錄下全部 .cl，評分、建議、寫入 .chordlaw/state.json，回傳報告 Markdown。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "root": {"type": "string", "description": "專案根目錄"},
                "liveness": {"type": "string", "enum": ["nll", "referent", "lexical"],
                             "default": "nll"},
                "out": {"type": "string", "description": "可選報告 .md 路徑"},
            },
            "required": ["root"],
        },
    },
    {
        "name": "chordlaw_history",
        "description": "讀 .chordlaw/state.json 最近會話與 diff。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "root": {"type": "string"},
                "n": {"type": "integer", "default": 5},
            },
            "required": ["root"],
        },
    },
]


class Ctx(object):
    def __init__(self):
        self.last_report = None
        self.last_session = None


def _log(msg):
    sys.stderr.write("[chordlaw-mcp] %s\n" % msg)
    sys.stderr.flush()


def _ok(id_, result):
    return {"jsonrpc": "2.0", "id": id_, "result": result}


def _err(id_, code, message):
    return {"jsonrpc": "2.0", "id": id_, "error": {"code": code, "message": message}}


def _text(obj):
    if isinstance(obj, str):
        body = obj
    else:
        body = json.dumps(obj, ensure_ascii=False, indent=2)
    return {"content": [{"type": "text", "text": body}]}


def _maybe_rs(text, label):
    if not (label or "").endswith(".rs"):
        return text, label
    import syn_subset
    cl, warns, _m = syn_subset.rust_to_cl(text)
    if not cl.strip().startswith("fn "):
        raise ValueError("非 syn 子集（抽不出 fn）；直通 rustc。warnings=%s" % warns)
    return cl, label


def _load_src(args):
    if args.get("source"):
        label = args.get("name") or "(source)"
        return _maybe_rs(args["source"], label)
    path = args.get("path")
    if not path:
        raise ValueError("需要 source 或 path")
    with open(path, encoding="utf-8") as f:
        text = f.read()
    return _maybe_rs(text, os.path.basename(path))


def call_tool(name, args, ctx):
    args = args or {}
    live = args.get("liveness") or "nll"
    if name == "chordlaw_check":
        text, label = _load_src(args)
        _pr, _dl, _err, data = helper.explain_source(text, label, live)
        slim = {k: data[k] for k in ("file", "liveness", "verdict", "errors",
                                     "regions", "advice") if k in data}
        return _text(slim)
    if name == "chordlaw_explain":
        text, label = _load_src(args)
        _pr, _dl, _err, data = helper.explain_source(text, label, live)
        return _text({"file": label, "verdict": data["verdict"],
                      "explain": data.get("explain"), "advice": data.get("advice")})
    if name == "chordlaw_diagram":
        text, label = _load_src(args)
        pr, dl, errors = C.check(text, live)
        if dl is None:
            return _text({"error": "前端錯誤，無圓示", "errors": [
                {"code": e[0], "message": e[2][1] if e[0] == "E00" else str(e)}
                for e in errors]})
        svg = C.render_svg(pr, dl, errors, label, live)
        out = args.get("out")
        if out:
            os.makedirs(os.path.dirname(os.path.abspath(out)) or ".", exist_ok=True)
            with open(out, "w", encoding="utf-8") as f:
                f.write(svg)
            return _text({"out": os.path.abspath(out), "bytes": len(svg)})
        return _text(svg)
    if name == "chordlaw_rules":
        code = (args.get("code") or "").upper()
        if code:
            pred = C.CODE2PRED.get(code)
            return _text({
                "code": code,
                "pred": pred,
                "message": C.CODE2MSG.get(code),
                "fix": C.FIX_HINTS.get(code),
                "law": C.LAW_OF.get(code),
                "law_name": C.LAW_NAME.get(C.LAW_OF.get(code)),
            })
        return _text(C.RULE_LEGEND)
    if name == "chordlaw_report":
        root = args["root"]
        sess = helper.run_project(root, live, persist=True)
        # 報告不塞完整 file_results（太大）
        slim = dict(sess)
        slim.pop("file_results", None)
        md = helper.render_report(sess)
        ctx.last_report = md
        ctx.last_session = slim
        out = args.get("out")
        if out:
            helper.write_report(sess, out)
            slim["report_path"] = os.path.abspath(out)
        return _text({"session": slim, "report": md})
    if name == "chordlaw_history":
        return _text(helper.history_summary(args["root"], int(args.get("n") or 5)))
    raise ValueError("未知工具: %s" % name)


def handle(msg, ctx):
    if not isinstance(msg, dict):
        return None
    method = msg.get("method")
    mid = msg.get("id", None)
    params = msg.get("params") or {}
    if method is None:
        return None
    # notifications have no id
    if method == "notifications/initialized" or method == "initialized":
        return None
    if method == "ping":
        return _ok(mid, {})
    if method == "initialize":
        return _ok(mid, {
            "protocolVersion": PROTOCOL,
            "capabilities": {
                "tools": {"listChanged": False},
                "resources": {"listChanged": False},
            },
            "serverInfo": {
                "name": "chordlaw",
                "version": C.VERSION,
                "description": "弦律：Rust 借用內環檢查器（Datalog × 圓示）",
            },
            "instructions": (
                "檢查 .cl 或 syn 子集 .rs。修法只用縮弧/移點/升圓。"
                "不要把 PASS 當成 rustc 放行。非子集 .rs 請交 rustc。"
            ),
        })
    if method == "tools/list":
        return _ok(mid, {"tools": TOOLS})
    if method == "tools/call":
        name = params.get("name")
        arguments = params.get("arguments") or {}
        try:
            return _ok(mid, call_tool(name, arguments, ctx))
        except Exception as ex:
            _log(traceback.format_exc())
            return _ok(mid, {
                "content": [{"type": "text", "text": "工具錯誤: %s" % ex}],
                "isError": True,
            })
    if method == "resources/list":
        return _ok(mid, {"resources": [
            {"uri": "chordlaw://rules", "name": "規則規格", "mimeType": "text/plain"},
            {"uri": "chordlaw://session/latest", "name": "最近會話", "mimeType": "application/json"},
            {"uri": "chordlaw://report/latest", "name": "最近報告", "mimeType": "text/markdown"},
        ]})
    if method == "resources/read":
        uri = params.get("uri") or ""
        if uri == "chordlaw://rules":
            return _ok(mid, {"contents": [{"uri": uri, "mimeType": "text/plain",
                                           "text": C.RULE_LEGEND}]})
        if uri == "chordlaw://session/latest":
            body = json.dumps(ctx.last_session or {}, ensure_ascii=False, indent=2)
            return _ok(mid, {"contents": [{"uri": uri, "mimeType": "application/json",
                                           "text": body}]})
        if uri == "chordlaw://report/latest":
            return _ok(mid, {"contents": [{"uri": uri, "mimeType": "text/markdown",
                                           "text": ctx.last_report or "(尚無報告)"}]})
        return _err(mid, -32602, "未知 resource: %s" % uri)
    if mid is None:
        return None
    return _err(mid, -32601, "未知方法: %s" % method)


def dispatch_line(line, ctx):
    line = line.strip()
    if not line:
        return None
    try:
        msg = json.loads(line)
    except ValueError as ex:
        return _err(None, -32700, "parse error: %s" % ex)
    if isinstance(msg, list):
        # 批次已從較新規格移除；保守逐一處理並只回有 id 的
        out = [handle(m, ctx) for m in msg]
        out = [x for x in out if x is not None]
        return out or None
    return handle(msg, ctx)


def serve_stdio(stdin=None, stdout=None):
    stdin = stdin or sys.stdin
    stdout = stdout or sys.stdout
    ctx = Ctx()
    _log("ready %s protocol=%s" % (C.VERSION, PROTOCOL))
    for raw in stdin:
        resp = dispatch_line(raw, ctx)
        if resp is None:
            continue
        if isinstance(resp, list):
            payload = json.dumps(resp, ensure_ascii=False, separators=(",", ":"))
        else:
            payload = json.dumps(resp, ensure_ascii=False, separators=(",", ":"))
        stdout.write(payload + "\n")
        stdout.flush()


def main():
    serve_stdio()


if __name__ == "__main__":
    main()
