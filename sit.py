#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Layer6 SIT — 左樹右樹語意分析器
================================
節點形狀（Bali / LCRS 經典）：

    left  = 第一個孩子     （左樹）
    right = 下一個兄弟     （右樹 / Sibling_Reference）
    up    = 父親           （Parent_Pointer）

兩層：
    surface  — 表面語法（Concrete / 行級，括號與關鍵字仍在）
    semantic — 語意意圖（抽象：fn / lend / use / call …）

遍歷：
    Class_Inheritance+   — 沿 up 找宣告／作用域祖先（has_ancestor）
    Call_Graph_Traversal — 沿 call 邊走（同樣用 LCRS 掛在 fn 下）
"""
from __future__ import print_function

import re

import chordlaw as C


class SNode(object):
    __slots__ = ("nid", "layer", "kind", "text", "left", "right", "up", "attrs")

    def __init__(self, nid, kind, text="", layer="semantic"):
        self.nid = nid
        self.layer = layer          # surface | semantic
        self.kind = kind
        self.text = text
        self.left = None            # first child
        self.right = None           # next sibling
        self.up = None              # parent
        self.attrs = {}

    def as_dict(self, children=True):
        d = {"id": self.nid, "layer": self.layer, "kind": self.kind,
             "text": self.text, "attrs": dict(self.attrs)}
        if children:
            d["children"] = [c.as_dict() for c in iter_children(self)]
        return d


def add_child(parent, child):
    """左樹掛第一孩，右樹串兄弟；同時寫 Parent_Pointer。"""
    if child is None or parent is None:
        return child
    child.up = parent
    child.right = None
    if parent.left is None:
        parent.left = child
        return child
    s = parent.left
    while s.right is not None:
        s = s.right
    s.right = child
    return child


def iter_children(node):
    c = node.left if node else None
    while c is not None:
        yield c
        c = c.right


def iter_siblings(node):
    """含自己，沿 right 走完整兄弟鏈。"""
    s = node
    while s is not None:
        yield s
        s = s.right


def iter_ancestors(node):
    p = node.up if node else None
    while p is not None:
        yield p
        p = p.up


def first_sibling(node):
    if node is None:
        return None
    p = node.up
    return p.left if p is not None else node


def has_ancestor(node, kind=None, pred=None):
    """Class_Inheritance+：沿父指針找符合的祖先（Bali hasAncestor）。"""
    for p in iter_ancestors(node):
        if kind is not None and p.kind == kind:
            return p
        if pred is not None and pred(p):
            return p
    return None


def walk_preorder(node):
    if node is None:
        return
    yield node
    for c in iter_children(node):
        for x in walk_preorder(c):
            yield x


class Forest(object):
    """一檔兩層樹 + 呼叫圖。"""

    def __init__(self):
        self.surface = SNode("surf0", "crate", layer="surface")
        self.semantic = SNode("sem0", "crate", layer="semantic")
        self.by_id = {}
        self.fn_by_name = {}
        self.call_edges = []        # (caller_fn, callee_name, site_nid)
        self._n = 0
        self.warnings = []

    def _id(self, prefix):
        self._n += 1
        return "%s%d" % (prefix, self._n)

    def alloc(self, kind, text="", layer="semantic"):
        n = SNode(self._id("n"), kind, text, layer)
        self.by_id[n.nid] = n
        return n


def build_surface(text):
    """
    表面語法樹：依 { } 做 LCRS（保留關鍵字與縮排資訊於 text）。
    不是 rustc CST；是行級混凝土樹。
    """
    forest = Forest()
    stack = [forest.surface]
    for raw in text.splitlines():
        line = raw.strip()
        if not line or line.startswith("//"):
            continue
        if line == "} else {":
            if len(stack) > 1:
                stack.pop()
            n = forest.alloc("else", "} else {", "surface")
            add_child(stack[-1], n)
            stack.append(n)
            continue
        if line == "}":
            if len(stack) > 1:
                stack.pop()
            continue
        kind = "line"
        if line.startswith("fn "):
            kind = "fn"
        elif line.startswith("struct "):
            kind = "struct"
        elif line.startswith("if"):
            kind = "if"
        elif line.startswith("loop"):
            kind = "loop"
        elif line == "{":
            kind = "block"
        n = forest.alloc(kind, line, "surface")
        add_child(stack[-1], n)
        if line.endswith("{") or kind == "block":
            stack.append(n)
    return forest


def _stmt_kind(text):
    t = text.strip()
    compact = t.replace(" ", "")
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
    return "stmt"


def _callee_of(text):
    m = re.match(r"^call(?:mv)?\s+(\w+)\s*\(", text.strip())
    return m.group(1) if m else None


def prepend_child(parent, child):
    """插到左樹最前（param 要在 body 之前）。"""
    if child is None or parent is None:
        return child
    child.up = parent
    child.right = parent.left
    parent.left = child
    return child


def _fn_params_from_src(text):
    out = {}
    if not text:
        return out
    for m in re.finditer(r"fn\s+(\w+)\s*\(([^)]*)\)", text):
        args = []
        for raw in m.group(2).split(","):
            raw = raw.strip()
            if raw.startswith("imm "):
                raw = raw[4:].strip()
            tok = raw.split(":")[0].strip()
            tok = tok.replace("mut ", "").strip()
            if tok:
                args.append(tok.split()[0])
        out[m.group(1)] = args
    return out


def build_semantic(pr, src_text=None):
    """Program 作用域樹 → 語意 LCRS（items 順序 = 源碼順序）。"""
    forest = Forest()
    if pr is None:
        return forest
    sc_node = {}
    for sc in pr.scopes:
        if sc["kind"] == "world":
            n = forest.semantic
            n.attrs["scope"] = sc["id"]
        else:
            n = forest.alloc(sc["kind"], sc.get("name") or sc["kind"])
            n.attrs["scope"] = sc["id"]
            if sc["kind"] == "fn" and sc.get("name"):
                n.attrs["name"] = sc["name"]
                forest.fn_by_name[sc["name"]] = n
        sc_node[sc["id"]] = n

    stmt_of = {sid: (text, sc) for sid, text, sc in pr.stmts}
    params_by_fn = _fn_params_from_src(src_text)

    # 只按 items 掛孩：scope 引用已建好的節點（父指針順便寫）
    for sc in pr.scopes:
        host = sc_node[sc["id"]]
        fn_name = sc.get("name") if sc["kind"] == "fn" else None
        if sc["kind"] == "fn":
            host.attrs["params"] = list(params_by_fn.get(fn_name) or [])
        for kind, ident in sc.get("items") or []:
            if kind == "stmt":
                text = stmt_of[ident][0]
                sn = forest.alloc(_stmt_kind(text), text)
                sn.attrs["sid"] = ident
                add_child(host, sn)
                cal = _callee_of(text)
                if cal:
                    sn.attrs["callee"] = cal
                    caller = fn_name
                    if not caller:
                        anc = has_ancestor(sn, kind="fn") or has_ancestor(host, kind="fn")
                        caller = anc.attrs.get("name") if anc else None
                    forest.call_edges.append((caller, cal, sn.nid))
            elif kind == "scope":
                child = sc_node.get(ident)
                if child is not None and child.up is None:
                    add_child(host, child)

        if sc["kind"] == "fn":
            for pname in reversed(host.attrs.get("params") or []):
                pn = forest.alloc("param", pname)
                pn.attrs["name"] = pname
                prepend_child(host, pn)

    # Call_Graph_Traversal：每個 fn 末掛 calls 鏈
    for caller, callee, site in forest.call_edges:
        fn = forest.fn_by_name.get(caller)
        if fn is None:
            continue
        existing = [c for c in iter_children(fn) if c.kind == "calls"]
        bag = existing[0] if existing else add_child(fn, forest.alloc("calls", "calls"))
        edge = forest.alloc("call_edge", "%s -> %s" % (caller, callee))
        edge.attrs["caller"] = caller
        edge.attrs["callee"] = callee
        edge.attrs["site"] = site
        add_child(bag, edge)
    return forest


def build(text):
    """表面 + 語意。語意失敗（E00）仍保留表面樹。"""
    surf = build_surface(text)
    pr, dl, errors = C.check(text, "nll")
    sem = build_semantic(pr, text)
    sem.surface = surf.surface
    sem.by_id.update(surf.by_id)
    sem.warnings = list(surf.warnings)
    sem.pr, sem.dl, sem.errors = pr, dl, errors
    return sem


def dump_tree(node, indent=0):
    lines = []
    pad = "  " * indent
    extra = ""
    if node.attrs:
        keep = {k: v for k, v in node.attrs.items() if k in ("sid", "name", "callee", "scope")}
        if keep:
            extra = " " + str(keep)
    lines.append("%s%s %s%s" % (pad, node.kind, node.text or "", extra))
    for c in iter_children(node):
        lines.extend(dump_tree(c, indent + 1))
    return lines


def call_graph_reachable(forest, start_fn, limit=64):
    """Call_Graph_Traversal：名稱圖 DFS。"""
    seen, out = set(), []
    stack = [start_fn]
    while stack and len(out) < limit:
        cur = stack.pop()
        if cur in seen:
            continue
        seen.add(cur)
        out.append(cur)
        for caller, callee, _site in forest.call_edges:
            if caller == cur and callee not in seen:
                stack.append(callee)
    return out


def inherit_lookup(node, name):
    """沿 Parent_Pointer 找 param/let 宣告（詞法繼承，非 OOP class）。"""
    cur = node
    while cur is not None:
        for c in iter_children(cur):
            if c.kind in ("param", "let", "imm", "hole") and (
                    c.text == name or c.attrs.get("name") == name or c.text.endswith(" " + name)):
                return c
            if c.kind == "let" and c.text == "let " + name:
                return c
        cur = cur.up
    return None


def main(argv=None):
    import argparse
    ap = argparse.ArgumentParser(description="Layer6 左樹右樹語意分析器")
    ap.add_argument("file")
    ap.add_argument("--layer", choices=("surface", "semantic", "both"), default="semantic")
    args = ap.parse_args(argv)
    text = open(args.file, encoding="utf-8").read()
    forest = build(text)
    if args.layer in ("surface", "both"):
        print("# surface")
        print("\n".join(dump_tree(forest.surface)))
    if args.layer in ("semantic", "both"):
        print("# semantic")
        print("\n".join(dump_tree(forest.semantic)))
        if forest.call_edges:
            print("# calls", forest.call_edges)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
