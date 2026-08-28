#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
P2 — quote 模板：意圖樹 / .cl → 正確-形狀的 Rust
================================================
模板對齊 oracle_check.py 的忠實翻譯（MIR 讀取用 let _uN = *r）。
生命週期參數只在「回傳參考」時寫進簽名（'a 來自 P1 compute_lifetimes）。
"""
from __future__ import print_function

import re

import chordlaw as C
from intent import IntentTree, tree_from_program


def _is_ref_name(pr, name):
    for pred, tup in pr.facts:
        if pred == "borrow_of" and tup[0] == name:
            return True
        if pred == "lend" and tup[2] == name:
            return True
    return False


def _lend_kind(pr, name):
    for pred, tup in pr.facts:
        if pred == "lend" and tup[2] == name:
            return tup[3]
    return "sh"


def _place_moved(pr, name):
    root = name.split(".", 1)[0]
    for pred, tup in pr.facts:
        if pred == "move" and tup[1].split(".", 1)[0] == root:
            return True
    return False


def quote_fn_rust(pr, ifn, indent="    "):
    """單一 IntentFn → Rust 字串（不含 struct/main）。"""
    uses = [0]
    lines = []
    moved = ifn.needs_string
    fields = sorted(ifn.fields)

    def u():
        uses[0] += 1
        return uses[0]

    # signature
    lt = None
    if ifn.returns_ref:
        lt = (ifn.lifetimes[0] if ifn.lifetimes else "'a")
    params_rs = []
    for p in ifn.params:
        if ifn.returns_ref:
            params_rs.append("%s: &%s %s" % (p.name, lt, "u32"))
        elif p.mutable:
            params_rs.append("mut %s: %s" % (p.name, "String" if moved else "u32"))
        else:
            params_rs.append("%s: %s" % (p.name, "u32"))
    ret = " -> &%s u32" % lt if ifn.returns_ref else ""
    lt_decl = "<%s>" % lt if ifn.returns_ref else ""
    lines.append("fn %s%s(%s)%s {" % (ifn.name, lt_decl, ", ".join(params_rs), ret))

    # body: walk stmts; insert scope braces from original? we only have flat stmts.
    # Control flow is in .cl via if/loop — but IntentFn.stmts are only Program.stmts
    # (no explicit if/loop tokens). Reconstruct from scope ids? Simpler: emit from
    # .cl text of the fn by a second path. Here we emit linear ops; quote_cl()
    # handles control flow.
    for sid, kind, text, extra in ifn.stmts:
        rs = quote_stmt(pr, text, kind, u, moved, use_pair=bool(fields))
        if rs:
            lines.append(indent + rs)
    lines.append("}")
    return "\n".join(lines)


def quote_stmt(pr, text, kind, u, moved, use_pair=False):
    t = text.strip()
    if kind == "let":
        m = re.match(r"^let\s+(\w+)$", t)
        if not m:
            return None
        name = m.group(1)
        if use_pair:
            ty_init = "Pair::default()"
        elif moved:
            ty_init = "String::new()"
        else:
            ty_init = "0u32"
        return "let mut %s = %s;" % (name, ty_init)
    if kind == "imm":
        m = re.match(r"^let\s+imm\s+(\w+)$", t)
        return "let %s = 0u32;" % (m.group(1) if m else "x")
    if kind == "hole":
        m = re.match(r"^let\s+hole\s+(\w+)$", t)
        return "let mut %s;" % (m.group(1) if m else "x")
    if kind == "borrow_sh":
        m = re.match(r"^let\s+(\w+)\s*=\s*&\s*(\S+)$", t)
        if not m:
            return None
        return "let %s = &%s;" % (m.group(1), m.group(2))
    if kind == "borrow_mut":
        m = re.match(r"^let\s+(\w+)\s*=\s*&mut\s+(\S+)$", t)
        if not m:
            return None
        return "let %s = &mut %s;" % (m.group(1), m.group(2))
    if kind == "use":
        m = re.match(r"^use\s+(\S+)$", t)
        if not m:
            return None
        nm = m.group(1)
        if _is_ref_name(pr, nm):
            if _lend_kind(pr, nm) == "mut":
                return "*%s += 1;" % nm
            return "let _u%d = *%s;" % (u(), nm)
        if moved and "." not in nm:
            return "let _u%d = %s.clone();" % (u(), nm)
        return "let _u%d = %s;" % (u(), nm)
    if kind == "write":
        m = re.match(r"^set\s+(\S+)$", t)
        if not m:
            return None
        p = m.group(1)
        if moved or "." in p:
            return "%s = %s;" % (p, "String::new()" if moved else "1")
        return "%s = 1;" % p
    if kind == "move":
        m = re.match(r"^mv\s+(\S+)$", t)
        return "let _m = %s;" % (m.group(1) if m else "x")
    if kind == "drop":
        m = re.match(r"^dp\s+(\S+)$", t)
        return "std::mem::drop(%s);" % (m.group(1) if m else "x")
    if kind == "ret":
        m = re.match(r"^ret\s+(\w+)$", t)
        return "%s" % (m.group(1) if m else "a")
    if kind == "store":
        m = re.match(r"^store\s+(\w+)\s*=\s*(\w+)$", t)
        if m:
            return "%s = %s;" % (m.group(1), m.group(2))
    if t.startswith("set *"):
        m = re.match(r"^set\s+\*\s*(\w+)$", t)
        return "*%s = 1;" % (m.group(1) if m else "a")
    if t.startswith("mv *"):
        m = re.match(r"^mv\s+\*\s*(\w+)$", t)
        return "let _m = *%s;" % (m.group(1) if m else "a")
    return None


def quote_cl_to_rust(cl_text, name="demo"):
    """
    .cl → 完整 .rs 檔（struct + fns + fn main）。
    控制流（if/loop/block）按 .cl 原樣展開。
    """
    pr, dl, errors = C.check(cl_text, "nll")
    tree = tree_from_program(pr, dl, errors, "cl")
    return quote_tree(tree, pr, cl_text), tree, errors


def quote_tree(tree, pr, cl_text=None):
    """意圖樹 + 原始 .cl（保留 if/loop）→ Rust crate 文本。"""
    parts = []
    fields = set()
    needs_string = any(f.needs_string for f in tree.fns)
    for f in tree.fns:
        fields |= f.fields
    if fields:
        flds = ", ".join("%s: %s" % (x, "String" if needs_string else "u32")
                         for x in sorted(fields) or ["f", "g"])
        parts.append("#[derive(Default, Clone)]")
        parts.append("struct Pair { %s }" % flds)
        parts.append("")

    if cl_text:
        parts.append(_quote_from_cl_lines(pr, tree, cl_text, needs_string, fields))
    else:
        for ifn in tree.fns:
            parts.append(quote_fn_rust(pr, ifn))
            parts.append("")
    parts.append("fn main() {}")
    return "\n".join(parts).rstrip() + "\n"


def _quote_from_cl_lines(pr, tree, cl_text, needs_string, fields):
    """逐行 .cl → Rust，保留縮排與 if/loop。"""
    fn_by_name = {f.name: f for f in tree.fns}
    cur_fn = None
    indent = 0
    out = []
    u_count = [0]

    def u():
        u_count[0] += 1
        return u_count[0]

    for raw in cl_text.splitlines():
        line = raw.strip()
        if not line or line.startswith("//"):
            continue
        m = re.match(r"^fn\s+(\w+)\s*\(([^)]*)\)\s*\{$", line)
        if m:
            cur_fn = fn_by_name.get(m.group(1))
            u_count[0] = 0
            lt = None
            ret = ""
            lt_decl = ""
            params_rs = []
            argnames = [x.strip() for x in m.group(2).split(",") if x.strip()]
            # imm prefix
            cleaned = []
            for a in argnames:
                if a.startswith("imm "):
                    cleaned.append(a[4:].strip())
                else:
                    cleaned.append(a)
            argnames = cleaned
            if cur_fn and cur_fn.returns_ref:
                lt = (cur_fn.lifetimes[0] if cur_fn.lifetimes else "'a")
                lt_decl = "<%s>" % lt
                ret = " -> &%s u32" % lt
                for a in argnames:
                    params_rs.append("%s: &%s u32" % (a, lt))
            else:
                for a in argnames:
                    ty = "String" if (cur_fn and cur_fn.needs_string) else "u32"
                    params_rs.append("mut %s: %s" % (a, ty))
            out.append("fn %s%s(%s)%s {" % (m.group(1), lt_decl, ", ".join(params_rs), ret))
            indent = 1
            continue
        if line == "if {":
            out.append("    " * indent + "if true {")
            indent += 1
            continue
        if line == "} else {":
            indent = max(1, indent - 1)
            out.append("    " * indent + "} else {")
            indent += 1
            continue
        if line == "loop {":
            out.append("    " * indent + "loop {")
            indent += 1
            continue
        if line == "{":
            out.append("    " * indent + "{")
            indent += 1
            continue
        if line == "}":
            indent = max(0, indent - 1)
            out.append("    " * indent + "}")
            if indent == 0:
                cur_fn = None
            continue
        # 普通陳述
        kind = "other"
        if line.startswith("let ") and "=&mut" in line.replace(" ", ""):
            kind = "borrow_mut"
        elif line.startswith("let ") and "=&" in line.replace(" ", ""):
            kind = "borrow_sh"
        elif line.startswith("use "):
            kind = "use"
        elif line.startswith("set "):
            kind = "write"
        elif line.startswith("mv "):
            kind = "move"
        elif line.startswith("dp "):
            kind = "drop"
        elif line.startswith("ret "):
            kind = "ret"
        elif line.startswith("let imm "):
            kind = "imm"
        elif line.startswith("let hole "):
            kind = "hole"
        elif line.startswith("let "):
            kind = "let"
        elif line.startswith("tmp "):
            m2 = re.match(r"^tmp\s+(\w+)$", line)
            rs = "let %s = 0u32;" % (m2.group(1) if m2 else "t")
            out.append("    " * indent + rs)
            continue
        elif line.startswith("slot "):
            m2 = re.match(r"^slot\s+(\w+)$", line)
            rs = "let mut %s = None::<&u32>;" % (m2.group(1) if m2 else "s")
            out.append("    " * indent + rs)
            continue
        use_pair = bool(cur_fn and cur_fn.fields)
        rs = quote_stmt(pr, line, kind, u, bool(cur_fn and cur_fn.needs_string),
                        use_pair=use_pair)
        if rs:
            out.append("    " * indent + rs)
        else:
            out.append("    " * indent + "// " + line)
    return "\n".join(out)
