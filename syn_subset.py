#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
P2 — syn 子集：Rust 表面語法 → .cl
================================
不是 rustc/syn crate（本環境無 cargo）。這是與 oracle 翻譯對偶的
**保守子集解析器**。看不懂的構造 → warning，不發明借用事實（合約 S）。

支援（對齊 oracle_check.py）:
  fn / struct / let mut / & / &mut / &* / *r += 1 / let _u = *r
  let _m = x  (move) / x = … / drop / if true / loop / 尾運算式回傳
"""
from __future__ import print_function

import re


class SynError(ValueError):
    pass


def _strip_comments(src):
    src = re.sub(r"/\*.*?\*/", " ", src, flags=re.S)
    out = []
    for ln in src.splitlines():
        if "//" in ln:
            ln = ln[:ln.index("//")]
        out.append(ln)
    return "\n".join(out)


def _norm_line(s):
    s = s.strip().rstrip(";").strip()
    s = re.sub(r"\s+", " ", s)
    return s


def _skip_fn_main(text):
    """去掉 fn main() { … } 整段。"""
    m = re.search(r"\bfn\s+main\s*\(\s*\)\s*\{", text)
    if not m:
        return text
    i = m.end() - 1
    depth = 0
    for j in range(i, len(text)):
        if text[j] == "{":
            depth += 1
        elif text[j] == "}":
            depth -= 1
            if depth == 0:
                return text[:m.start()] + text[j + 1:]
    return text[:m.start()]


def _path_of(expr):
    expr = expr.strip()
    expr = re.sub(r"^&\s*mut\s+", "", expr)
    expr = re.sub(r"^&\s*\*\s*", "", expr)
    expr = re.sub(r"^&\s*", "", expr)
    expr = re.sub(r"^\*\s*", "", expr)
    expr = expr.replace(" ", "")
    if re.match(r"^[\w]+(?:\.[\w]+)*$", expr):
        return expr
    return None


def rust_to_cl(src):
    """
    回傳 (cl_text, warnings, meta)
    meta: {structs, fns:[{name, ret, params}]}
    """
    warnings = []
    meta = {"structs": [], "fns": []}
    text = _strip_comments(src)
    text = re.sub(r"#\[[^\]]*\]", " ", text)
    text = _skip_fn_main(text)

    # structs
    for m in re.finditer(r"struct\s+(\w+)\s*\{([^}]*)\}", text):
        fields = []
        for part in m.group(2).split(","):
            part = part.strip()
            if not part:
                continue
            mm = re.match(r"(\w+)\s*:", part)
            if mm:
                fields.append(mm.group(1))
        meta["structs"].append({"name": m.group(1), "fields": fields})
    text = re.sub(r"struct\s+\w+\s*\{[^}]*\}", "\n", text)

    # 把大括號與分號拆成獨立 token 行（單行 fn { a; b; } 也能拆）
    buf = []
    i = 0
    n = len(text)
    while i < n:
        ch = text[i]
        if ch in "{}":
            buf.append("\n" + ch + "\n")
            i += 1
        elif ch == ";":
            buf.append("\n")
            i += 1
        else:
            buf.append(ch)
            i += 1
    raw_lines = "".join(buf).splitlines()

    cl = []
    stack = []  # fn/if/loop/block/else
    saw_fn = False

    def emit(s):
        cl.append(s)

    i = 0
    while i < len(raw_lines):
        line = _norm_line(raw_lines[i])
        i += 1
        if not line:
            continue

        m = re.match(r"^fn\s+(\w+)\s*\((.*)\)\s*(?:->\s*(.+))?$", line)
        if m:
            name, args, ret = m.group(1), m.group(2).strip(), (m.group(3) or "").strip()
            if name == "main":
                warnings.append("skip fn main")
                continue
            params = []
            cl_args = []
            for rawp in [x.strip() for x in args.split(",") if x.strip()]:
                # mut p: T  |  p: &T  |  p: T  |  p
                mm = re.match(r"^(?:(mut)\s+)?(\w+)(?:\s*:\s*(.+))?$", rawp)
                if not mm:
                    warnings.append("無法解析參數: %s" % rawp)
                    continue
                pname = mm.group(2)
                pty = (mm.group(3) or "").strip()
                params.append({"name": pname, "ty": pty, "mut": bool(mm.group(1))})
                cl_args.append(pname)
            meta["fns"].append({"name": name, "params": params, "ret": ret})
            emit("fn %s(%s) {" % (name, ", ".join(cl_args)))
            stack.append("fn")
            saw_fn = True
            continue

        if line == "{":
            # 可能是 if/loop 已處理; 裸區塊
            if stack and stack[-1] in ("if-pending",):
                stack[-1] = "if"
            elif stack and stack[-1] == "else-pending":
                stack[-1] = "else"
            elif stack and stack[-1] == "loop-pending":
                stack[-1] = "loop"
            else:
                emit("{")
                stack.append("block")
            continue

        if line == "} else {" or line == "}else{":
            emit("} else {")
            if stack:
                stack.pop()
            stack.append("else")
            continue

        if line == "}":
            emit("}")
            if stack:
                stack.pop()
            continue

        if line in ("if true", "if true {", "if"):
            emit("if {")
            if line.endswith("{"):
                stack.append("if")
            else:
                stack.append("if-pending")
            continue
        if re.match(r"^if\b", line):
            # if cond {  — 保守: 當 if true（控制流形狀保留）
            warnings.append("if 條件簡化為 true: %s" % line)
            emit("if {")
            if line.endswith("{"):
                stack.append("if")
            else:
                stack.append("if-pending")
            continue

        if line in ("else", "else {"):
            emit("} else {")
            if line.endswith("{"):
                stack.append("else")
            else:
                stack.append("else-pending")
            continue

        if line in ("loop", "loop {"):
            emit("loop {")
            if line.endswith("{"):
                stack.append("loop")
            else:
                stack.append("loop-pending")
            continue

        # drop
        m = re.match(r"^(?:std::mem::)?drop\s*\(\s*([\w.]+)\s*\)$", line)
        if m:
            emit("dp %s" % m.group(1))
            continue

        # *ref += 1  /  *ref = …  → use ref (mut 使用)
        m = re.match(r"^\*(\w+)\s*(\+=|=).*$", line)
        if m:
            emit("use %s" % m.group(1))
            continue

        # ref.field += 1 → 經 mut 參考使用;  x.f = … → 寫入地方
        m = re.match(r"^(\w+)(\.[\w.]+)\s*(\+=|=)\s*.*$", line)
        if m:
            root, rest, op = m.group(1), m.group(2), m.group(3)
            if op == "+=":
                emit("use %s" % root)
            else:
                emit("set %s%s" % (root, rest))
            continue

        # let _m = PATH  → move
        m = re.match(r"^let\s+_m\d*\s*=\s*([\w.]+)$", line)
        if m:
            emit("mv %s" % m.group(1))
            continue

        # let _uN = *REF  → use REF
        m = re.match(r"^let\s+_u\d*\s*=\s*\*\s*(\w+)$", line)
        if m:
            emit("use %s" % m.group(1))
            continue

        # let _uN = PATH.clone() / PATH
        m = re.match(r"^let\s+_u\d*\s*=\s*([\w.]+)(?:\.clone\(\))?$", line)
        if m:
            emit("use %s" % m.group(1))
            continue

        # let NAME = &mut PATH | &PATH | &*PATH
        m = re.match(r"^let\s+(?:mut\s+)?(\w+)\s*=\s*&mut\s+(.+)$", line)
        if m:
            pth = _path_of("&mut " + m.group(2))
            if pth:
                emit("let %s = &mut %s" % (m.group(1), pth))
                continue
        m = re.match(r"^let\s+(?:mut\s+)?(\w+)\s*=\s*&\s*\*?\s*(.+)$", line)
        if m:
            pth = _path_of("&" + m.group(2))
            if pth:
                emit("let %s = &%s" % (m.group(1), pth))
                continue

        # let mut NAME = EXPR  /  let NAME = EXPR  → 宣告
        m = re.match(r"^let\s+(?:mut\s+)?(\w+)\s*=\s*.+$", line)
        if m:
            emit("let %s" % m.group(1))
            continue

        # let NAME  (bare, rare)
        m = re.match(r"^let\s+(?:mut\s+)?(\w+)$", line)
        if m:
            emit("let %s" % m.group(1))
            continue

        # assignment PATH = EXPR
        m = re.match(r"^([\w.]+)\s*=\s*.+$", line)
        if m:
            emit("set %s" % m.group(1))
            continue

        # return IDENT
        m = re.match(r"^return\s+(\w+)$", line)
        if m:
            emit("ret %s" % m.group(1))
            continue

        # trailing ident (return expr)
        m = re.match(r"^(\w+)$", line)
        if m and m.group(1) not in ("true", "false"):
            emit("ret %s" % m.group(1))
            continue

        warnings.append("略過不支援語句: %s" % line)

    if not saw_fn:
        warnings.append("沒有可分析的 fn（非子集或只有 main）")

    # 補齊未閉合
    while stack:
        emit("}")
        stack.pop()

    cl_text = "\n".join(cl) + ("\n" if cl else "")
    return cl_text, warnings, meta
