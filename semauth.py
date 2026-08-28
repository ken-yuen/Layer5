#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
認証語義機 SemAuth
==================
報 X 當且僅當 ⟦輸入⟧ = X。表面寫法（空白、註解、unnamed、÷）不進判決。

層（tree-sitter 命名）：
  CST  Concrete Syntax Tree     全標記，含 extra / unnamed / ERROR
  SST  Surface Syntax Tree      去掉 extra（註解、空白行）
  NNT  Named Node Tree          只留 named
  EIT  Extra-Inclusive Tree     CST 但 extra 打標、不混進語義
  ENT  Error Node Tree          ERROR/MISSING 單獨成樹；禁止降成 lend
  AST  Abstract Syntax Tree     脫糖 named
  DAG  語義 DAG                 hash-cons 正規形（重寫合流後）

重寫（地面項、意圖合流）：
  extra → ε
  unnamed 標點 → ε（語義不靠分號）
  "÷" → unnamed(div)  （不是借用）
  "if=" 無 "{" → ERROR（不發明 if {）
  ERROR → ⊥，認証結果 = ERROR，永不 PASS
"""
from __future__ import print_function

import hashlib
import re

import chordlaw as C


EXTRA_KINDS = frozenset(("comment", "blank"))
UNNAMED_KINDS = frozenset(("punct", "divop"))
ERROR_KINDS = frozenset(("ERROR", "MISSING"))
NAMED_KINDS = frozenset((
    "fn", "struct", "let", "imm", "hole", "borrow_sh", "borrow_mut",
    "use", "write", "move", "drop", "ret", "call", "callmv",
    "if", "else", "loop", "block", "tmp", "slot", "store", "param",
    "crate",
))


class TNode(object):
    __slots__ = ("kind", "named", "extra", "error", "text", "kids")

    def __init__(self, kind, text="", named=True, extra=False, error=False, kids=None):
        self.kind = kind
        self.text = text
        self.named = named
        self.extra = extra
        self.error = error
        self.kids = kids or []

    def as_dict(self):
        return {
            "kind": self.kind, "text": self.text,
            "named": self.named, "extra": self.extra, "error": self.error,
            "kids": [k.as_dict() for k in self.kids],
        }


def _tok_line(raw):
    s = raw.rstrip("\n")
    stripped = s.strip()
    if not stripped:
        return TNode("blank", s, named=False, extra=True)
    if stripped.startswith("//"):
        return TNode("comment", stripped, named=False, extra=True)
    # 行內註解：表面留下碼，註解當 extra 孩
    code, extra = stripped, None
    if "//" in stripped:
        i = stripped.index("//")
        code, extra = stripped[:i].rstrip(), stripped[i:]
    if "÷" in code:
        # 除號不是借用；整行若只有 88÷ww 之類 → unnamed，不進語義
        rest = re.sub(r"\s+", "", code)
        if re.match(r"^[\w.]+÷[\w.]+$", rest) or not re.search(
                r"\b(fn|let|use|set|mv|dp|ret|if|loop|call)\b", code):
            n = TNode("divop", code, named=False, extra=False)
            if extra:
                n.kids.append(TNode("comment", extra, named=False, extra=True))
            return n
    # if= 無塊 → ERROR，禁止發明 if {
    if re.match(r"^if\s*=", code) and "{" not in code:
        return TNode("ERROR", code, named=False, extra=False, error=True)
    if code in ("{", "}"):
        return TNode("punct", code, named=False)
    if code == "} else {":
        n = TNode("else", code, named=True)
        return n
    kind = _named_kind(code)
    if kind is None:
        return TNode("ERROR", code, named=False, extra=False, error=True)
    n = TNode(kind, code, named=True)
    if extra:
        n.kids.append(TNode("comment", extra, named=False, extra=True))
    return n


def _named_kind(code):
    t = code.strip()
    compact = t.replace(" ", "")
    if t.startswith("fn "):
        return "fn"
    if t.startswith("struct "):
        return "struct"
    if t.startswith("let ") and "=&mut" in compact:
        return "borrow_mut"
    if t.startswith("let ") and "=&" in compact:
        return "borrow_sh"
    if t.startswith("use "):
        return "use"
    if t.startswith("set "):
        return "write"
    if t.startswith("mv "):
        return "move"
    if t.startswith("dp "):
        return "drop"
    if t.startswith("ret "):
        return "ret"
    if t.startswith("callmv "):
        return "callmv"
    if t.startswith("call "):
        return "call"
    if t.startswith("let imm "):
        return "imm"
    if t.startswith("let hole "):
        return "hole"
    if t.startswith("let "):
        return "let"
    if t.startswith("tmp "):
        return "tmp"
    if t.startswith("slot "):
        return "slot"
    if t.startswith("store "):
        return "store"
    if t.startswith("if"):
        return "if"
    if t.startswith("loop"):
        return "loop"
    if t.startswith("else"):
        return "else"
    return None


def parse_cst(src):
    """Concrete：每行一節點，保留 extra / unnamed / ERROR。"""
    root = TNode("crate", named=True)
    stack = [root]
    for raw in src.splitlines(True):
        n = _tok_line(raw)
        if n.kind == "punct" and n.text.strip() == "}":
            if len(stack) > 1:
                stack.pop()
            root.kids.append(n) if False else None
            # } 是 unnamed，掛在當前父的兄弟層：作為父的孩再 pop？ 
            # 混凝土：閉括號掛在即將離開的塊上
            if stack:
                stack[-1].kids.append(n)
            if len(stack) > 1 and n.text.strip() == "}":
                pass
            continue
        stack[-1].kids.append(n)
        if n.named and n.kind in ("fn", "if", "loop", "else", "block") and (
                n.text.rstrip().endswith("{") or n.kind == "block"):
            stack.append(n)
        elif n.kind == "punct" and n.text.strip() == "{":
            blk = TNode("block", "{", named=True)
            stack[-1].kids[-1] = blk
            stack.append(blk)
    return root


def rewrite_strip_extra(n):
    """R1 extra → ε"""
    kids = [rewrite_strip_extra(c) for c in n.kids]
    kids = [c for c in kids if c is not None and not c.extra]
    if n.extra:
        return None
    out = TNode(n.kind, n.text, n.named, False, n.error, kids)
    return out


def rewrite_named_only(n):
    """NNT：只留 named；ERROR 仍留（認証要看見 ⊥）"""
    kids = []
    for c in n.kids:
        cn = rewrite_named_only(c)
        if cn is None:
            continue
        if cn.named or cn.error:
            kids.append(cn)
    if not n.named and not n.error:
        # 把孩提升
        return TNode("lift", "", named=False, kids=kids) if False else None
    return TNode(n.kind, n.text, n.named, n.extra, n.error, kids)


def _canon_text(kind, t):
    t = re.sub(r"\s+", " ", t).strip()
    t = t.replace("if true {", "if {")
    if kind == "fn":
        t = re.sub(r"\s+", " ", t)
        t = re.sub(r"\s*\(\s*", "(", t)
        t = re.sub(r"\s*\)\s*", ")", t)
        t = re.sub(r"\s*\{\s*$", " {", t)
        t = re.sub(r"fn\s+", "fn ", t)
    if kind in ("borrow_sh", "borrow_mut"):
        t = re.sub(r"\s*=\s*&mut\s*", " = &mut ", t)
        t = re.sub(r"\s*=\s*&(?!mut)", " = &", t)
        t = re.sub(r"\s+", " ", t).strip()
    return t


def rewrite_normalize_text(n):
    """空白正規化；if true { → if { ；unnamed ÷ 已在 CST 排除。"""
    t = _canon_text(n.kind, n.text)
    kids = [rewrite_normalize_text(c) for c in n.kids]
    return TNode(n.kind, t, n.named, n.extra, n.error, kids)


def has_error(n):
    if n.error or n.kind in ERROR_KINDS:
        return True
    return any(has_error(c) for c in n.kids)


def linearize_cl(n):
    """AST named → 正規 .cl（語義載體）。ERROR 不輸出假陳述。"""
    lines = []

    def walk(x, in_fn):
        if x.error or x.kind in ERROR_KINDS:
            return
        if x.extra or not x.named:
            for c in x.kids:
                walk(c, in_fn)
            return
        t = x.text
        if x.kind == "crate":
            for c in x.kids:
                walk(c, False)
            return
        if x.kind == "fn":
            header = t if t.endswith("{") else (t + " {")
            lines.append(header)
            for c in x.kids:
                walk(c, True)
            lines.append("}")
            return
        if x.kind == "if":
            lines.append("if {")
            for c in x.kids:
                walk(c, in_fn)
            lines.append("}")
            return
        if x.kind == "else":
            if lines and lines[-1] == "}":
                lines[-1] = "} else {"
            else:
                lines.append("} else {")
            for c in x.kids:
                walk(c, in_fn)
            lines.append("}")
            return
        if x.kind == "loop":
            lines.append("loop {")
            for c in x.kids:
                walk(c, in_fn)
            lines.append("}")
            return
        if x.kind in ("punct", "blank", "comment", "divop", "block"):
            for c in x.kids:
                walk(c, in_fn)
            return
        if t:
            # 去掉尾 {
            if t.endswith("{") and x.kind not in ("fn", "if", "loop", "else"):
                t = t[:-1].rstrip()
            lines.append(t)
        for c in x.kids:
            walk(c, in_fn)

    walk(n, False)
    return "\n".join(lines) + ("\n" if lines else "")


def hashcons(cl_text):
    """語義 DAG 摘要：正規 .cl 的 sha1。相同語義 → 相同 digest。"""
    return hashlib.sha1(cl_text.encode("utf-8")).hexdigest()[:16]


def authenticate(src, liveness="nll"):
    """
    認証：只報語義 X。
    回傳 dict: X, digest, error_nodes, layers
    """
    cst = parse_cst(src)
    eit = cst  # extra 已打標
    sst = rewrite_strip_extra(cst)
    nnt = rewrite_named_only(sst) if sst else TNode("crate", named=True)
    ast = rewrite_normalize_text(nnt)
    ent_err = has_error(cst)
    cl = linearize_cl(ast)
    digest = hashcons(cl)
    if ent_err or not cl.strip().startswith("fn "):
        return {
            "engine": "SemAuth",
            "X": "ERROR",
            "digest": digest,
            "needs_rustc": False,
            "cl": cl,
            "reason": "ERROR/MISSING 或抽不出 fn — 不發明語義",
            "syntax_independent": True,
        }
    pr, dl, errors = C.check(cl, liveness)
    X = "PASS" if not errors else "FAIL"
    codes = [e[0] for e in errors]
    return {
        "engine": "SemAuth",
        "X": X,
        "codes": codes,
        "digest": digest,
        "needs_rustc": False,
        "cl": cl,
        "syntax_independent": True,
    }


def same_semantics(a, b):
    ra, rb = authenticate(a), authenticate(b)
    return ra["X"] == rb["X"] and ra.get("digest") == rb.get("digest")


def main(argv=None):
    import argparse, json, sys
    ap = argparse.ArgumentParser(description="認証語義機")
    ap.add_argument("file")
    args = ap.parse_args(argv)
    src = open(args.file, encoding="utf-8").read()
    r = authenticate(src)
    print(json.dumps({k: r[k] for k in r if k != "cl"}, ensure_ascii=False, indent=2))
    return 0 if r["X"] != "ERROR" else 1


if __name__ == "__main__":
    raise SystemExit(main())
