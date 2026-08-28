#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
YKC — 弦律內核（Yet Kernel of Chords）
=====================================
不是 rustc，也不靠 rustc 才有效。純 Python。

對偶（同一事實、三種載體）：

    Datalog reach     ⟷  布林矩陣 Warshall 閉包
    onregion(L,Q)     ⟷  區間向量 v_L[Q] = R[L,Q] ∧ R[Q,E]
    紅弧交越          ⟷  v_a ∧ v_b ≠ ∅ 且至少一 mut
    SIT LCRS          ⟷  樹自動機（父指針一致）

Gavril：借貸活躍集若為 CFG 上的「路徑區間」，重疊圖是 interval graph；
弦律圓示把區間畫成弧，交越＝interval overlap。
"""
from __future__ import print_function

import chordlaw as C
import sit


def stmt_index(pr):
    ids = [s for s, _t, _sc in pr.stmts]
    return ids, {s: i for i, s in enumerate(ids)}


def adjacency(pr):
    """CFG 鄰接矩陣 A（含後向邊；自反不含）。跨 fn 邊丟掉（設計定理是函數內）。"""
    ids, idx = stmt_index(pr)
    n = len(ids)
    A = [[0] * n for _ in range(n)]
    fn_of = getattr(pr, "fn_of_stmt", {}) or {}
    for pred, tup in pr.facts:
        if pred == "edge" and len(tup) >= 2 and tup[0] in idx and tup[1] in idx:
            if fn_of and fn_of.get(tup[0]) != fn_of.get(tup[1]):
                continue
            A[idx[tup[0]]][idx[tup[1]]] = 1
    return ids, A


def warshall(A):
    """布林傳遞閉包 + 自反（R[i,i]=1）。O(n³)，函數內 n 很小。"""
    n = len(A)
    R = [row[:] for row in A]
    for i in range(n):
        R[i][i] = 1
    for k in range(n):
        rk = R[k]
        for i in range(n):
            if R[i][k]:
                ri = R[i]
                for j in range(n):
                    if rk[j]:
                        ri[j] = 1
    return R


def _iter_reach(engine):
    facts = getattr(engine, "facts", None)
    if facts is None:
        return
    if isinstance(facts, dict):
        for tup in facts.get("reach") or ():
            yield tup
    else:
        for pred, tup in facts:
            if pred == "reach":
                yield tup


def datalog_reach_matrix(pr, dl, ids, idx):
    n = len(ids)
    M = [[0] * n for _ in range(n)]
    engines = []
    dls = getattr(pr, "dls", None) or {}
    if dls:
        engines.extend(dls.values())
    elif dl is not None:
        engines.append(dl)
    for eng in engines:
        for tup in _iter_reach(eng):
            if len(tup) >= 2 and tup[0] in idx and tup[1] in idx:
                M[idx[tup[0]]][idx[tup[1]]] = 1
    return M


def matrices_equal(A, B):
    if len(A) != len(B):
        return False
    for i in range(len(A)):
        if A[i] != B[i]:
            return False
    return True


def loan_vector(R, idx, lend, end):
    """v[Q] = R[L,Q] ∧ R[Q,E]  — 設計定理的向量形。"""
    n = len(R)
    if lend not in idx or end not in idx:
        return [0] * n
    iL, iE = idx[lend], idx[end]
    return [1 if R[iL][q] and R[q][iE] else 0 for q in range(n)]


def vec_and(a, b):
    return [x & y for x, y in zip(a, b)]


def vec_any(v):
    return any(v)


def interval_clash(va, ka, vb, kb):
    """幾何法則① 的矩陣形：區間交且至少一 mut。"""
    return vec_any(vec_and(va, vb)) and (ka == "mut" or kb == "mut")


class YKC(object):
    """一次分析的三種載體。"""

    def __init__(self, text, liveness="nll"):
        self.text = text
        self.pr, self.dl, self.errors = C.check(text, liveness)
        self.forest = sit.build(text)
        self.ids, self.A = adjacency(self.pr) if self.pr else ([], [])
        self.idx = {s: i for i, s in enumerate(self.ids)}
        self.R = warshall(self.A) if self.A else []
        self.R_dl = datalog_reach_matrix(self.pr, self.dl, self.ids, self.idx)
        self.reach_match = matrices_equal(self.R, self.R_dl)
        self.loans = []
        if self.dl is not None:
            for a in C.collect_arcs(self.pr, self.dl):
                if a["E"] is None:
                    continue
                v = loan_vector(self.R, self.idx, a["L"], a["E"])
                self.loans.append({
                    "ref": a["T"], "kind": a["K"], "lend": a["L"],
                    "end": a["E"], "vec": v, "points": a["I"],
                })
        self.tree_ok, self.tree_err = tree_automaton(self.forest.semantic)

    def as_dict(self):
        return {
            "engine": "YKC",
            "needs_rustc": False,
            "verdict": "PASS" if not self.errors else "FAIL",
            "n_stmt": len(self.ids),
            "reach_warshall_eq_datalog": self.reach_match,
            "n_loans": len(self.loans),
            "tree_automaton": self.tree_ok,
            "tree_err": self.tree_err,
            "errors": [e[0] for e in self.errors],
        }


def tree_automaton(root):
    """
    樹自動機：每個孩的 up 必須是父；兄弟鏈的 up 必須相同。
    接受 = 整棵 SIT 形狀合法（不檢查借用規則）。
    """
    if root is None:
        return True, None
    for n in sit.walk_preorder(root):
        kids = list(sit.iter_children(n))
        for c in kids:
            if c.up is not n:
                return False, "parent_mismatch:%s" % c.nid
        for c in kids:
            if c.right is not None and c.right.up is not n:
                return False, "sibling_parent:%s" % c.right.nid
    return True, None


def analyze(text, liveness="nll"):
    return YKC(text, liveness)


def main(argv=None):
    import argparse, json
    ap = argparse.ArgumentParser(description="YKC 內核（矩陣×自動機×弦律）")
    ap.add_argument("file")
    args = ap.parse_args(argv)
    text = open(args.file, encoding="utf-8").read()
    y = analyze(text)
    print(json.dumps(y.as_dict(), ensure_ascii=False, indent=2))
    return 0 if y.reach_match and y.tree_ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
