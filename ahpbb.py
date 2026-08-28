#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
AutoHPBorrowBase (AHPBB) — 自動生命借貸工廠
==========================================
工廠能：
  1. syn 子集解析 Rust → 語意意圖樹
  2. 演算法計算生命週期（onregion = reach(L,Q)∧reach(Q,E)）
  3. quote 模板產出**正確**的 Rust borrow/lifetime 程式
  4. 多線批量生產
  5. 寫入一個**新開**資料夾（絕不覆蓋舊批次）

不發明 rustc 沒說過的事實。PASS 才出貨。
"""
from __future__ import print_function

import json
import os
import re
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timezone, timedelta

import chordlaw as C
import syn_subset
import quote_tpl
from intent import tree_from_program, compute_lifetimes

HKT = timezone(timedelta(hours=8))
HERE = os.path.dirname(os.path.abspath(__file__))


# ------------------------------------------------------------------
# 正確產品配方（意圖規格 → .cl；工廠保證 quote 後弦律 PASS）
# ------------------------------------------------------------------
RECIPES = [
    ("seq_sh_then_mut", """
fn f() {
  let x
  let a = &x
  use a
  let b = &mut x
  use b
}
"""),
    ("two_sh_overlap", """
fn f() {
  let x
  let a = &x
  let b = &x
  use a
  use b
}
"""),
    ("mut_seq", """
fn f() {
  let x
  let a = &mut x
  use a
  let b = &mut x
  use b
}
"""),
    ("reinit_after_move", """
fn f() {
  let x
  mv x
  set x
  use x
}
"""),
    ("return_param_ref", """
fn f(p) {
  let a = &p
  ret a
}
"""),
    ("write_after_loan_dead", """
fn f() {
  let x
  let a = &x
  use a
  set x
}
"""),
    ("if_use_no_extend", """
fn f() {
  let x
  let a = &x
  if {
    use a
  }
  set x
}
"""),
    ("field_split", """
fn f() {
  let x
  let a = &x.f
  let b = &mut x.g
  use a
  use b
}
"""),
    ("reborrow_sh_dead", """
fn f() {
  let x
  let a = &x
  let b = &a
  use b
  use a
}
"""),
    ("param_write_after_use", """
fn f(p) {
  let a = &p
  use a
  set p
}
"""),
    ("empty_fn", "fn f() {\n}\n"),
    ("block_then_write", """
fn f() {
  let x
  {
    let a = &x
    use a
  }
  set x
}
"""),
    ("whole_reinit_after_field_move", """
fn f() {
  let x
  mv x.f
  set x
  use x
}
"""),
    ("imm_shared", """
fn f() {
  let imm x
  let a = &x
  use a
}
"""),
]


# 已知反例 → 幾何修復後應 PASS
FIX_RECIPES = [
    ("fix_clash_shrink", """
fn f() {
  let x
  let a = &x
  use a
  let b = &mut x
  use a
  use b
}
""", "shrink_arc"),
    ("fix_return_raise", """
fn f() {
  let x
  let a = &x
  ret a
}
""", "raise_circle"),
    ("fix_use_after_move", """
fn f() {
  let x
  mv x
  use x
}
""", "move_point"),
]


def _now_tag():
    return datetime.now(HKT).strftime("%Y%m%d_%H%M%S")


def write_batch_cargo_toml(outdir, shipped):
    """只寫在出貨資料夾，絕不碰 Python 倉根。"""
    lines = [
        "# 本檔只服務這一批判出的 .rs；不要複製到 Layer5 根目錄。",
        "[package]",
        'name = "ahpbb_batch"',
        'version = "0.1.0"',
        'edition = "2021"',
        "publish = false",
        "",
    ]
    for s in shipped:
        stem = "%s_%s" % (s["id"], s["name"])
        bin_name = re.sub(r"[^A-Za-z0-9_-]", "_", stem)[:64]
        lines.append("[[bin]]")
        lines.append('name = "%s"' % bin_name)
        lines.append('path = "products/%s.rs"' % stem)
        lines.append("")
    with open(os.path.join(outdir, "Cargo.toml"), "w", encoding="utf-8") as f:
        f.write("\n".join(lines))


def new_batch_dir(parent=None):
    """永遠開新資料夾，不覆蓋。"""
    parent = os.path.abspath(parent or os.path.join(HERE, "factory_out"))
    os.makedirs(parent, exist_ok=True)
    path = os.path.join(parent, "AHPBB_%s" % _now_tag())
    n = 0
    base = path
    while os.path.exists(path):
        n += 1
        path = base + "_%d" % n
    os.makedirs(path)
    os.makedirs(os.path.join(path, "products"))
    return path


def apply_geometry_fix(cl_text, op):
    """最小自動修：縮弧 / 移點 / 升圓。保守，修不了就原樣。"""
    if op == "shrink_arc":
        # 刪掉「第二條出借之後、對第一條參考的 use」（典型 E01）
        pr, _dl, errors = C.check(cl_text, "nll")
        if not any(c == "E01" for c, _s, _t in errors):
            return cl_text
        lines = [ln for ln in cl_text.splitlines()]
        # 找第一條 sh 名與第二條 lend 之後的 use
        refs = []
        for sid, text, _sc in pr.stmts:
            if "=&" in text.replace(" ", "") or " = &" in text:
                m = re.match(r"^let\s+(\w+)\s*=", text)
                if m:
                    refs.append((sid, m.group(1), "mut" in text))
        if len(refs) < 2:
            return cl_text
        first_name = refs[0][1]
        second_sid = refs[1][0]
        drop = set()
        for sid, text, _sc in pr.stmts:
            if pr.idx[sid] > pr.idx[second_sid] and text.strip() == "use " + first_name:
                drop.add(sid)
        if not drop:
            return cl_text
        # 按 stmt 文本刪第一個匹配（源碼順序）
        out, removed = [], False
        want = "use " + first_name
        # 只刪 second lend 之後出現的
        seen_second = False
        second_txt = pr.stmts[pr.idx[second_sid]][1]
        for ln in lines:
            s = ln.strip()
            if s == second_txt:
                seen_second = True
                out.append(ln)
                continue
            if seen_second and s == want and not removed:
                removed = True
                continue
            out.append(ln)
        return "\n".join(out) + "\n"
    if op == "raise_circle":
        # 局部 let x + ret &x → 參數 p
        t = cl_text
        t = re.sub(r"fn\s+(\w+)\s*\(\s*\)\s*\{", r"fn \1(p) {", t, count=1)
        t = t.replace("let x\n", "", 1).replace("let x\r\n", "", 1)
        t = t.replace("= &x", "= &p").replace("=&x", "= &p")
        return t
    if op == "move_point":
        # 把「mv x」之後的「use x」刪掉，或把 use 移到 mv 前
        lines = cl_text.splitlines()
        mv_i = next((i for i, l in enumerate(lines) if l.strip().startswith("mv ")), None)
        use_i = next((i for i, l in enumerate(lines) if l.strip() == "use x"), None)
        if mv_i is not None and use_i is not None and use_i > mv_i:
            use_ln = lines.pop(use_i)
            lines.insert(mv_i, use_ln)
            return "\n".join(lines) + "\n"
    return cl_text


class Product(object):
    def __init__(self, pid, name):
        self.id = pid
        self.name = name
        self.cl = ""
        self.rs = ""
        self.verdict = "FAIL"
        self.errors = []
        self.regions = []
        self.warnings = []
        self.elapsed_ms = 0
        self.ok = False

    def as_dict(self):
        return {
            "id": self.id, "name": self.name, "verdict": self.verdict,
            "ok": self.ok, "errors": self.errors, "regions": self.regions,
            "warnings": self.warnings, "elapsed_ms": self.elapsed_ms,
        }


def produce_one(name, cl_text, pid="p"):
    """單一配方 → Product（檢查 + 意圖樹 + quote）。"""
    t0 = time.time()
    pr, dl, errors = C.check(cl_text, "nll")
    tree = tree_from_program(pr, dl, errors, "cl")
    rs = quote_tpl.quote_tree(tree, pr, cl_text)
    p = Product(pid, name)
    p.cl = cl_text.strip() + "\n"
    p.rs = rs
    p.verdict = "PASS" if not errors else "FAIL"
    p.errors = [{"code": c, "stmt": s} for c, s, _ in errors]
    if dl is not None:
        p.regions = [r.as_dict() for r in compute_lifetimes(pr, dl)]
    p.ok = p.verdict == "PASS"
    p.elapsed_ms = int((time.time() - t0) * 1000)
    return p


def ingest_rust(src, name="from_rs"):
    cl, warns, meta = syn_subset.rust_to_cl(src)
    p = produce_one(name, cl, "in")
    p.warnings = warns
    p.meta = meta
    return p, cl


class AutoHPBorrowBase(object):
    """自動生命借貸工廠。"""

    def __init__(self, workers=4, liveness="nll"):
        self.workers = max(1, int(workers))
        self.liveness = liveness

    def make(self, name, cl_text, pid="p"):
        return produce_one(name, cl_text, pid)

    def make_from_rs(self, src, name="from_rs"):
        p, _cl = ingest_rust(src, name)
        return p

    def repair(self, name, cl_text, op, pid="p"):
        fixed = apply_geometry_fix(cl_text, op)
        prod = produce_one(name, fixed, pid)
        prod.warnings.append("applied:%s" % op)
        return prod

    def catalog(self):
        """全部配方（正確 + 修復）。"""
        specs = []
        for name, cl in RECIPES:
            specs.append(("ok_" + name, cl.strip() + "\n", None))
        for name, cl, op in FIX_RECIPES:
            specs.append((name, cl.strip() + "\n", op))
        return specs

    def batch(self, specs=None, out_parent=None, only_pass=True):
        """
        多線批量生產，寫入新開資料夾。
        回傳 (outdir, manifest)
        """
        specs = specs if specs is not None else self.catalog()
        outdir = new_batch_dir(out_parent)
        prod_dir = os.path.join(outdir, "products")
        jobs = []
        for i, spec in enumerate(specs):
            name, cl, op = spec if len(spec) == 3 else (spec[0], spec[1], None)
            pid = "p%04d" % (i + 1)
            jobs.append((pid, name, cl, op))

        results = []

        def run(job):
            pid, name, cl, op = job
            if op:
                return self.repair(name, cl, op, pid)
            return self.make(name, cl, pid)

        if self.workers <= 1 or len(jobs) <= 1:
            for j in jobs:
                results.append(run(j))
        else:
            with ThreadPoolExecutor(max_workers=self.workers) as pool:
                futs = {pool.submit(run, j): j[0] for j in jobs}
                by_id = {}
                for fut in as_completed(futs):
                    by_id[futs[fut]] = fut.result()
                results = [by_id[j[0]] for j in jobs]

        shipped = []
        rejected = []
        for p in results:
            if only_pass and not p.ok:
                rejected.append(p.as_dict())
                continue
            base = os.path.join(prod_dir, "%s_%s" % (p.id, p.name))
            with open(base + ".cl", "w", encoding="utf-8") as f:
                f.write(p.cl)
            with open(base + ".rs", "w", encoding="utf-8") as f:
                f.write(p.rs)
            with open(base + ".json", "w", encoding="utf-8") as f:
                json.dump(p.as_dict(), f, ensure_ascii=False, indent=2)
                f.write("\n")
            shipped.append(p.as_dict())

        manifest = {
            "factory": "AutoHPBorrowBase",
            "engine": C.VERSION,
            "ts": datetime.now(HKT).isoformat(timespec="seconds"),
            "outdir": outdir,
            "workers": self.workers,
            "requested": len(jobs),
            "shipped": len(shipped),
            "rejected": len(rejected),
            "only_pass": only_pass,
            "products": shipped,
            "rejected_items": rejected,
        }
        with open(os.path.join(outdir, "manifest.json"), "w", encoding="utf-8") as f:
            json.dump(manifest, f, ensure_ascii=False, indent=2)
            f.write("\n")
        write_batch_cargo_toml(outdir, shipped)
        readme = [
            "# AHPBB 批次 %s" % os.path.basename(outdir),
            "",
            "工廠：AutoHPBorrowBase · 引擎 %s · 工人 %d" % (C.VERSION, self.workers),
            "出貨 %d / 請求 %d（only_pass=%s）" % (len(shipped), len(jobs), only_pass),
            "",
            "每個產品三件套：`.cl`（意圖）· `.rs`（quote 的 Rust）· `.json`（生命週期）。",
            "內環已 PASS；**rustc 仍是終審**。",
            "",
            "本資料夾有 `Cargo.toml`（多 bin），**不是** Python 倉根目錄的 crate。",
            "有 Rust 工具鏈的環境：",
            "",
            "```bash",
            "rustc products/p0001_ok_seq_sh_then_mut.rs   # 單檔",
            "cargo check                                  # 整批",
            "```",
            "",
            "只有 Python 3 的環境：工廠本身可跑；不要在 Layer5 根目錄加 Cargo.toml。",
            "",
        ]
        for s in shipped:
            readme.append("- `%s_%s`  regions=%d" % (
                s["id"], s["name"], len(s.get("regions") or [])))
        with open(os.path.join(outdir, "README.md"), "w", encoding="utf-8") as f:
            f.write("\n".join(readme) + "\n")
        return outdir, manifest


def main(argv=None):
    import argparse
    ap = argparse.ArgumentParser(description="AutoHPBorrowBase 工廠")
    ap.add_argument("--batch", action="store_true", help="批量生產正確 Rust")
    ap.add_argument("-o", "--out", default=None, help="批次父目錄（其下再開新資料夾）")
    ap.add_argument("-j", "--jobs", type=int, default=4)
    ap.add_argument("--from-rs", dest="from_rs")
    ap.add_argument("--from-cl", dest="from_cl")
    ap.add_argument("--include-fail", action="store_true")
    args = ap.parse_args(argv)
    fac = AutoHPBorrowBase(workers=args.jobs)
    if args.from_rs:
        src = open(args.from_rs, encoding="utf-8").read()
        p = fac.make_from_rs(src, os.path.basename(args.from_rs))
        print(json.dumps(p.as_dict(), ensure_ascii=False, indent=2))
        print("----- .cl -----")
        print(p.cl)
        print("----- .rs -----")
        print(p.rs)
        return 0 if p.ok else 1
    if args.from_cl:
        src = open(args.from_cl, encoding="utf-8").read()
        p = fac.make(os.path.basename(args.from_cl), src)
        print(json.dumps(p.as_dict(), ensure_ascii=False, indent=2))
        print(p.rs)
        return 0 if p.ok else 1
    outdir, man = fac.batch(out_parent=args.out, only_pass=not args.include_fail)
    print("AHPBB 出貨 %d/%d → %s" % (man["shipped"], man["requested"], outdir))
    return 0 if man["shipped"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
