#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
P1 — Rust 語意意圖樹 (SIT) + 生命週期演算法
================================================
生命週期不是標籤，是 CFG 點集：

    I(loan) = { Q | reach(L, Q) ∧ reach(Q, E) }

與 NLL / 弦律 onregion 同構。本模組把 Program+Datalog 投影成可 quote 的樹。
"""
from __future__ import print_function

import chordlaw as C


class Place(object):
    __slots__ = ("name", "kind", "ty", "mutable")

    def __init__(self, name, kind="owned", ty="u32", mutable=True):
        self.name = name
        self.kind = kind          # owned | param | ref | field
        self.ty = ty
        self.mutable = mutable

    def as_dict(self):
        return {"name": self.name, "kind": self.kind, "ty": self.ty,
                "mutable": self.mutable}


class Region(object):
    """一條借貸的生命週期 = 點集 I + 具名 'a。"""
    __slots__ = ("lt", "lend", "end", "kind", "referent", "ref", "points")

    def __init__(self, lt, lend, end, kind, referent, ref, points):
        self.lt = lt              # 'a
        self.lend = lend
        self.end = end
        self.kind = kind          # sh | mut
        self.referent = referent
        self.ref = ref
        self.points = frozenset(points)

    def as_dict(self):
        return {
            "lifetime": self.lt, "lend": self.lend, "end": self.end,
            "kind": self.kind, "referent": self.referent, "ref": self.ref,
            "points": sorted(self.points, key=_sid_key),
        }


class IntentFn(object):
    __slots__ = ("name", "params", "stmts", "regions", "lifetimes",
                 "returns_ref", "needs_string", "fields")

    def __init__(self, name):
        self.name = name
        self.params = []          # [Place]
        self.stmts = []           # [(sid, kind, cl_text, extra)]
        self.regions = []
        self.lifetimes = []       # ['a', 'b']
        self.returns_ref = False
        self.needs_string = False
        self.fields = set()

    def as_dict(self):
        return {
            "name": self.name,
            "params": [p.as_dict() for p in self.params],
            "stmts": [{"sid": s, "kind": k, "cl": t, "extra": e}
                      for s, k, t, e in self.stmts],
            "regions": [r.as_dict() for r in self.regions],
            "lifetimes": list(self.lifetimes),
            "returns_ref": self.returns_ref,
            "needs_string": self.needs_string,
            "fields": sorted(self.fields),
        }


class IntentTree(object):
    """語意意圖樹：一 crate 的 fn 集合 + 推斷出的區域。"""

    def __init__(self):
        self.fns = []
        self.structs = []
        self.warnings = []
        self.source_kind = "cl"   # cl | rs
        self.verdict = None
        self.errors = []

    def as_dict(self):
        return {
            "source_kind": self.source_kind,
            "verdict": self.verdict,
            "errors": self.errors,
            "warnings": self.warnings,
            "structs": self.structs,
            "fns": [f.as_dict() for f in self.fns],
        }


def _sid_key(s):
    try:
        return int(s[1:])
    except Exception:
        return 0


def _stmt_kind(text, facts_by_sid):
    fs = facts_by_sid.get(text and "", [])  # unused
    t = text.strip()
    if t.startswith("let ") and "=&mut" in t.replace(" ", ""):
        return "borrow_mut"
    if t.startswith("let ") and "=&" in t.replace(" ", ""):
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
    if t.startswith("let hole "):
        return "hole"
    if t.startswith("let imm "):
        return "imm"
    if t.startswith("tmp "):
        return "tmp"
    if t.startswith("slot "):
        return "slot"
    if t.startswith("store "):
        return "store"
    if t.startswith("callmv "):
        return "callmv"
    if t.startswith("call "):
        return "call"
    if t.startswith("let "):
        return "let"
    return "other"


def compute_lifetimes(pr, dl, fn_scope=None):
    """
    P1 演算法：從 onregion / span_end 抽出具名生命週期。
    回傳 [Region]，lt 按出借點序編 'a 'b 'c …
    """
    if dl is None:
        return []
    arcs = C.collect_arcs(pr, dl)
    if fn_scope is not None:
        fn_stmts = set(pr.all_stmts.get(fn_scope, []))
        arcs = [a for a in arcs if a["L"] in fn_stmts]
    out = []
    for i, a in enumerate(arcs):
        lt = "'" + chr(ord("a") + (i % 26)) + (str(i // 26) if i >= 26 else "")
        out.append(Region(lt, a["L"], a["E"], a["K"], a["Q"], a["T"], a["I"]))
    return out


def tree_from_program(pr, dl, errors, source_kind="cl"):
    """Program + 引擎事實 → 意圖樹（含生命週期）。"""
    tree = IntentTree()
    tree.source_kind = source_kind
    tree.verdict = "PASS" if not errors else "FAIL"
    tree.errors = [{"code": c, "stmt": s} for c, s, _t in errors]
    if pr is None:
        return tree

    facts_at = {}
    for pred, tup in pr.facts:
        if tup:
            facts_at.setdefault(tup[0], []).append((pred, tup))

    moved = {tup[1] for pred, tup in pr.facts if pred == "move"}
    field_names = set()
    for pred, tup in pr.facts:
        if pred in ("read", "write", "move", "drop", "lend") and len(tup) >= 2:
            p = tup[1]
            if "." in p:
                field_names.add(p.split(".", 1)[1].split(".", 1)[0])

    by_fn = {}
    for sid, text, sc in pr.stmts:
        fid = getattr(pr, "fn_of_stmt", {}).get(sid)
        by_fn.setdefault(fid, []).append((sid, text, sc))

    fn_scopes = [s for s in pr.scopes if s["kind"] == "fn"]
    for fsc in fn_scopes:
        ifn = IntentFn(fsc.get("name") or "f")
        # params
        for pred, tup in pr.facts:
            if pred == "param":
                name = tup[0]
                if pr.scope_of.get(name) == pr.root:
                    # 僅掛到「使用到此 param 的 fn」
                    used = False
                    for sid, text, _sc in by_fn.get(fsc["id"], []):
                        if name in text.split():
                            used = True
                            break
                    # 多 fn 時 param 屬 ROOT, 各 fn 各自宣告
                    # 從 fn 簽名無法還原; 從 scope: parse 時 param 在 fn 的 names
                    pass
        # 從 fn 體第一層可見: 看 facts param 且該 fn 的語句提到
        param_names = []
        for pred, tup in pr.facts:
            if pred != "param":
                continue
            nm = tup[0]
            # 該 fn 內有 lend/use/set 提到 nm
            mentioned = any(nm == tok or tok.startswith(nm + ".")
                            for sid, text, _ in by_fn.get(fsc["id"], [])
                            for tok in text.replace("=", " ").replace("*", " ").split())
            if mentioned or len(fn_scopes) == 1:
                param_names.append(nm)
        # 去重保序
        seen = set()
        for nm in param_names:
            if nm in seen:
                continue
            seen.add(nm)
            imm = any(pred == "imm" and tup[0] == nm for pred, tup in pr.facts)
            ifn.params.append(Place(nm, "param", "u32", mutable=not imm))

        for sid, text, _sc in by_fn.get(fsc["id"], []):
            kind = _stmt_kind(text, facts_at)
            extra = {}
            if kind.startswith("borrow"):
                extra["kind"] = "mut" if kind == "borrow_mut" else "sh"
            ifn.stmts.append((sid, kind, text, extra))
            if kind == "ret":
                ifn.returns_ref = True
            if kind == "move" or (kind == "let" and False):
                pass
            for tok in text.replace("=", " ").split():
                if "." in tok:
                    ifn.fields.add(tok.split(".", 1)[1].split(".")[0])
                root = tok.split(".", 1)[0]
                if root in moved:
                    ifn.needs_string = True

        if any(k == "move" for _s, k, _t, _e in ifn.stmts):
            ifn.needs_string = True
        if ifn.fields:
            tree.structs = [{"name": "Pair",
                             "fields": sorted(ifn.fields) or ["f", "g"]}]

        dlf = getattr(pr, "dls", {}).get(fsc["id"], dl)
        ifn.regions = compute_lifetimes(pr, dlf, fsc["id"])
        ifn.lifetimes = [r.lt for r in ifn.regions]
        # 回傳參考: 生命週期取被借者若為 param 則用該弧之 lt
        if ifn.returns_ref and ifn.regions:
            ifn.lifetimes = [ifn.regions[0].lt]
        tree.fns.append(ifn)
    return tree


def lifetimes_overlap(a, b):
    return bool(a.points & b.points)
