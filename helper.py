#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
弦律小幫手 — 專案檢查 / 建議 / 評分 / 持久化 (P3a)
CLI 與 MCP 共用此模組。判定一律來自 chordlaw.check（法則 0）。
"""
from __future__ import print_function

import hashlib
import json
import os
import re
import time
from datetime import datetime, timezone, timedelta

import chordlaw as C

STATE_DIR = ".chordlaw"
STATE_FILE = "state.json"
MAX_SESSIONS = 20
HKT = timezone(timedelta(hours=8))

# 高頻修復成本 (PLAN §1.1)
SEVERITY = {
    "E00": 10, "E01": 12, "E02": 8, "E03": 8, "E04": 10, "E05": 8,
    "E06": 12, "E07": 12, "E08": 8, "E09": 10, "E10": 12,
    "E11": 6, "E12": 6, "E13": 8, "E14": 8, "E15": 7, "E16": 10,
    "E17": 7, "E18": 10, "E19": 6, "E20": 6, "E21": 8, "E22": 6,
    "E23": 6, "E24": 10, "E25": 8, "E26": 8, "E27": 10, "E28": 10,
    "E29": 8, "E30": 8, "E31": 6, "E32": 6,
}
P0 = {"E01", "E06", "E07", "E10"}
P1 = {"E02", "E03", "E09", "E18", "E24"}

SKIP_DIRS = {
    ".git", ".chordlaw", "__pycache__", "node_modules", "target",
    ".venv", "dist", "build", "out", "factory_out",
}

RS_BORROW_RE = re.compile(
    r"&mut\b|&\s*\w+|mem::drop|\.clone\(|let\s+mut\b|std::mem::",
    re.M,
)

# 本倉庫當「Rust 小幫手」的產品量表 (不進 Datalog)
PRODUCT_RUBRIC = {
    "engine": {"score": 88, "note": "32 則 + 幾何可測 + 92 回歸"},
    "soundness": {"score": 90, "note": "合約 S：oracle 26/26，0 虛假放行"},
    "agent_api": {"score": 80, "note": "MCP 六工具 + --json/--explain/--report"},
    "persistence": {"score": 75, "note": ".chordlaw/state.json 會話 + diff"},
    "rust_coverage": {"score": 55, "note": "P2 syn 子集可檢 oracle 形 .rs；全量 syn crate 仍待 cargo"},
    "repair_loop": {"score": 78, "note": "AHPBB 可縮弧/升圓/移點後批量產出正確 Rust"},
}


def now_iso():
    return datetime.now(HKT).isoformat(timespec="seconds")


def session_id():
    ts = datetime.now(HKT).strftime("%Y%m%d_%H%M%S")
    return "s_%s_%s" % (ts, hashlib.sha1(os.urandom(8)).hexdigest()[:4])


def state_path(root):
    return os.path.join(os.path.abspath(root), STATE_DIR, STATE_FILE)


def load_state(root):
    p = state_path(root)
    if not os.path.isfile(p):
        return {"version": 1, "project": os.path.abspath(root),
                "updated": None, "sessions": []}
    with open(p, encoding="utf-8") as f:
        return json.load(f)


def save_state(root, state):
    d = os.path.join(os.path.abspath(root), STATE_DIR)
    os.makedirs(d, exist_ok=True)
    state["updated"] = now_iso()
    state["sessions"] = state.get("sessions", [])[-MAX_SESSIONS:]
    tmp = state_path(root) + ".tmp"
    with open(tmp, "w", encoding="utf-8") as f:
        json.dump(state, f, ensure_ascii=False, indent=2)
        f.write("\n")
    os.replace(tmp, state_path(root))
    return state_path(root)


def collect_targets(root):
    """回傳 (cl_files, rs_skipped) 絕對路徑。"""
    root = os.path.abspath(root)
    cl, rs = [], []
    for dirpath, dirs, files in os.walk(root):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS and not d.startswith(".")]
        for fn in files:
            path = os.path.join(dirpath, fn)
            if fn.endswith(".cl"):
                cl.append(path)
            elif fn.endswith(".rs"):
                try:
                    text = open(path, encoding="utf-8", errors="ignore").read()
                except OSError:
                    continue
                if not RS_BORROW_RE.search(text):
                    continue
                try:
                    import syn_subset
                    cl, warns, _m = syn_subset.rust_to_cl(text)
                    # 子集能抽出至少一個 fn 才納入檢查
                    if cl.strip().startswith("fn "):
                        cl.append(path)  # 當可檢目標；check_path 會轉
                        continue
                except Exception:
                    pass
                rs.append({"path": path, "reason": "outside_syn_subset",
                           "hint": "非 syn 子集；直通 rustc"})
    cl.sort()
    return cl, rs


def _priority(code):
    if code in P0:
        return "P0"
    if code in P1:
        return "P1"
    return "P2"


def _op_of(code):
    law = C.LAW_OF.get(code)
    if law in (1, 4):
        return "shrink_arc"
    if law == 2:
        return "raise_circle"
    if law == 3:
        return "move_point"
    if law == 10:
        return "change_perm"
    return "clone_or_reinit"


def advise_errors(pr, errors, file_label):
    """每條錯誤 → 可執行建議 (縮弧/移點/升圓)。"""
    out = []
    dls = getattr(pr, "dls", {}) or {}
    for code, sid, tup in errors:
        dlf = dls.get(getattr(pr, "fn_of_stmt", {}).get(sid)) if dls else None
        law, glines = (None, [])
        if dlf is not None and sid in getattr(pr, "idx", {}):
            law, glines = C.geometry_report(pr, dlf, code, sid)
        detail = C.FIX_HINTS.get(code, "")
        geo_fix = next((ln.strip() for ln in glines if "修:" in ln or "修：" in ln), "")
        out.append({
            "priority": _priority(code),
            "op": _op_of(code),
            "file": file_label,
            "code": code,
            "stmt": sid,
            "stmt_text": (pr.stmts[pr.idx[sid]][1]
                          if getattr(pr, "idx", None) and sid in pr.idx else None),
            "law": law,
            "law_name": C.LAW_NAME.get(law) if law else None,
            "detail": geo_fix or detail,
            "hint": detail,
            "geometry": glines,
        })
    order = {"P0": 0, "P1": 1, "P2": 2}
    out.sort(key=lambda a: (order.get(a["priority"], 9), a["code"], str(a["stmt"])))
    return out


def explain_source(text, name="(source)", liveness="nll"):
    """結構化 explain（MCP / 報告用，不 print）。"""
    pr, dl, errors = C.check(text, liveness)
    data = C.build_result(name, liveness, pr, errors)
    data["advice"] = advise_errors(pr, errors, name)
    items = []
    dls = getattr(pr, "dls", {}) or {}
    fns = {f["id"]: f for f in pr.scopes if f["kind"] == "fn"}
    for code, sid, tup in errors:
        rec = {"code": code, "stmt": sid}
        if code == "E00":
            rec["message"] = tup[1] if len(tup) > 1 else str(tup)
            items.append(rec)
            continue
        rec["fn"] = (fns.get(getattr(pr, "fn_of_stmt", {}).get(sid)) or {}).get("name")
        rec["stmt_text"] = pr.stmts[pr.idx[sid]][1] if sid in pr.idx else None
        rec["message"] = C.CODE2MSG.get(code, "")
        rec["fix"] = C.FIX_HINTS.get(code, "")
        dlf = dls.get(getattr(pr, "fn_of_stmt", {}).get(sid))
        if dlf is not None and code in C.CODE2PRED:
            rec["rule"] = None
            for r in dlf.rules:
                if r["head"][0] == C.CODE2PRED[code] and r.get("src"):
                    rec["rule"] = r["src"]
                    break
            proof = []
            C.print_proof(dlf, C.CODE2PRED[code], tup, proof)
            rec["proof"] = proof
            law, glines = C.geometry_report(pr, dlf, code, sid)
            rec["law"] = law
            rec["geometry"] = glines
        items.append(rec)
    data["explain"] = items
    return pr, dl, errors, data


def check_path(path, liveness="nll"):
    with open(path, encoding="utf-8") as f:
        text = f.read()
    label = os.path.basename(path)
    pr, dl, errors, data = explain_source(text, label, liveness)
    data["path"] = os.path.abspath(path)
    return pr, dl, errors, data


def score_project(file_results, skipped_rs):
    """專案檢查分。file_results: list of build_result+advice dicts。"""
    n = len(file_results)
    n_pass = sum(1 for r in file_results if r.get("verdict") == "PASS")
    n_fail = n - n_pass
    correctness = 100.0 if n == 0 else 100.0 * n_pass / n

    sev_sum = 0
    n_err = 0
    n_explained = 0
    by_code, by_law = {}, {}
    for r in file_results:
        for e in r.get("errors") or []:
            n_err += 1
            code = e.get("code") or "E00"
            sev_sum += SEVERITY.get(code, 8)
            by_code[code] = by_code.get(code, 0) + 1
        for a in r.get("advice") or []:
            if a.get("detail") or a.get("geometry"):
                n_explained += 1
            if a.get("law") is not None:
                by_law[str(a["law"])] = by_law.get(str(a["law"]), 0) + 1
    severity = max(0.0, 100.0 - min(100, sev_sum))
    explainability = 100.0 if n_err == 0 else 100.0 * n_explained / n_err

    n_rs = len(skipped_rs or [])
    coverage = 100.0 * n / max(1, n + n_rs)

    overall = (0.40 * correctness + 0.30 * severity +
               0.15 * explainability + 0.15 * coverage)
    overall = int(round(overall))
    if overall >= 85:
        band = "ready"
    elif overall >= 50:
        band = "needs_work"
    else:
        band = "blocked"
    return {
        "overall": overall,
        "band": band,
        "dimensions": {
            "correctness": int(round(correctness)),
            "severity": int(round(severity)),
            "explainability": int(round(explainability)),
            "coverage": int(round(coverage)),
        },
        "counts": {
            "checked": n, "pass": n_pass, "fail": n_fail,
            "errors": n_err, "skipped_rs": n_rs,
        },
        "by_code": dict(sorted(by_code.items())),
        "by_law": dict(sorted(by_law.items(), key=lambda kv: int(kv[0]))),
    }


def product_score():
    dims = PRODUCT_RUBRIC
    overall = int(round(sum(v["score"] for v in dims.values()) / len(dims)))
    return {"overall": overall, "band": "needs_work", "dimensions": dims,
            "note": "產品就緒分 ≠ 專案檢查分。內環預檢，rustc 終審。"}


def session_diff(prev, cur):
    """比較兩次 session 的 (file, code, stmt) 集合。"""
    def keys(sess):
        s = set()
        for f in sess.get("files") or []:
            for c in f.get("codes") or []:
                s.add((os.path.basename(f.get("path") or ""), c))
        return s
    if not prev:
        return {"fixed": [], "new": [], "still": []}
    a, b = keys(prev), keys(cur)
    return {
        "fixed": sorted("%s %s" % (p, c) for p, c in a - b),
        "new": sorted("%s %s" % (p, c) for p, c in b - a),
        "still": sorted("%s %s" % (p, c) for p, c in a & b),
    }


def run_project(root, liveness="nll", persist=True):
    """掃目錄、檢查、評分、寫 state。回傳 session dict。"""
    root = os.path.abspath(root)
    cl_files, skipped_rs = collect_targets(root)
    file_results = []
    files_meta = []
    all_advice = []
    t0 = time.time()
    for path in cl_files:
        _pr, _dl, errors, data = check_path(path, liveness)
        rel = os.path.relpath(path, root)
        data["path"] = rel
        file_results.append(data)
        files_meta.append({
            "path": rel,
            "verdict": data["verdict"],
            "codes": [e["code"] for e in data.get("errors") or []],
        })
        for a in data.get("advice") or []:
            a = dict(a)
            a["file"] = rel
            all_advice.append(a)
    score = score_project(file_results, skipped_rs)
    sess = {
        "id": session_id(),
        "ts": now_iso(),
        "liveness": liveness,
        "root": root,
        "engine": C.VERSION,
        "elapsed_ms": int((time.time() - t0) * 1000),
        "score": score,
        "product": product_score(),
        "files": files_meta,
        "advice": all_advice,
        "skipped_rs": [{"path": os.path.relpath(x["path"], root),
                        "reason": x["reason"], "hint": x["hint"]}
                       for x in skipped_rs],
        "file_results": file_results,
    }
    if persist:
        st = load_state(root)
        prev = st["sessions"][-1] if st.get("sessions") else None
        slim = {k: sess[k] for k in
                ("id", "ts", "liveness", "root", "engine", "elapsed_ms",
                 "score", "product", "files", "advice", "skipped_rs")}
        # 建議不存完整 geometry 行以免膨脹
        slim["advice"] = [{k: a[k] for k in
                           ("priority", "op", "file", "code", "stmt",
                            "law", "detail") if k in a}
                          for a in all_advice]
        slim["diff"] = session_diff(prev, slim)
        st.setdefault("sessions", []).append(slim)
        save_state(root, st)
        sess["diff"] = slim["diff"]
        sess["state"] = state_path(root)
    return sess


def render_report(sess):
    """Markdown 報告。"""
    sc = sess["score"]
    pd = sess.get("product") or product_score()
    dim = sc["dimensions"]
    cnt = sc["counts"]
    lines = []
    lines.append("# 弦律小幫手報告 %s" % sess.get("engine", C.VERSION))
    lines.append("")
    lines.append("> 內環預檢，**rustc 終審**。分數不是 rustc 品質，是「代理能不能用幾何動詞修完」。")
    lines.append(">")
    lines.append("> 專案：`%s` · 活度 `%s` · 會話 `%s` · %s · %d ms" % (
        sess.get("root", ""), sess.get("liveness", "nll"),
        sess.get("id", ""), sess.get("ts", ""), sess.get("elapsed_ms", 0)))
    lines.append("")
    lines.append("## 1. 產品就緒（本倉庫當 Rust 小幫手）")
    lines.append("")
    lines.append("| 維度 | 分 | 說明 |")
    lines.append("|---|---:|---|")
    for k, v in pd["dimensions"].items():
        lines.append("| %s | %d | %s |" % (k, v["score"], v["note"]))
    lines.append("| **overall** | **%d** | %s |" % (pd["overall"], pd.get("note", "")))
    lines.append("")
    lines.append("## 2. 專案檢查分")
    lines.append("")
    lines.append("- **overall %d / 100** · 帶 **`%s`**" % (sc["overall"], sc["band"]))
    lines.append("- 已檢 %d（PASS %d / FAIL %d）· 錯誤 %d · 略過 .rs %d" % (
        cnt["checked"], cnt["pass"], cnt["fail"], cnt["errors"], cnt["skipped_rs"]))
    lines.append("")
    lines.append("| 維度 | 分 |")
    lines.append("|---|---:|")
    for k in ("correctness", "severity", "explainability", "coverage"):
        lines.append("| %s | %d |" % (k, dim[k]))
    lines.append("")
    if sc.get("by_code"):
        lines.append("### 錯誤碼分布")
        lines.append("")
        lines.append("| 碼 | 次 | 法則 | 優先 |")
        lines.append("|---|---:|---|---|")
        for code, n in sc["by_code"].items():
            law = C.LAW_OF.get(code)
            lname = C.LAW_NAME.get(law, "—") if law else "—"
            lines.append("| %s | %d | %s | %s |" % (code, n, lname, _priority(code)))
        lines.append("")
    diff = sess.get("diff")
    if diff and (diff.get("fixed") or diff.get("new") or diff.get("still")):
        lines.append("### 相對上次會話")
        lines.append("")
        lines.append("- 已消：%s" % (", ".join(diff["fixed"]) or "—"))
        lines.append("- 新冒：%s" % (", ".join(diff["new"]) or "—"))
        lines.append("- 仍在：%d 項" % len(diff.get("still") or []))
        lines.append("")
    lines.append("## 3. 建議（按 P0 → P2，幾何動詞）")
    lines.append("")
    adv = sess.get("advice") or []
    if not adv:
        lines.append("無錯誤。內環接受 — 仍須 rustc 終審。")
        lines.append("")
    else:
        lines.append("| 優先 | 操作 | 檔 | 碼 | 點 | 修 |")
        lines.append("|---|---|---|---|---|---|")
        for a in adv[:40]:
            det = (a.get("detail") or a.get("hint") or "").replace("|", "/")
            if len(det) > 72:
                det = det[:70] + "…"
            lines.append("| %s | `%s` | %s | %s | %s | %s |" % (
                a.get("priority"), a.get("op"), a.get("file"),
                a.get("code"), a.get("stmt"), det))
        if len(adv) > 40:
            lines.append("")
            lines.append("… 其餘 %d 條略（見 state.json）" % (len(adv) - 40))
        lines.append("")
    skip = sess.get("skipped_rs") or []
    if skip:
        lines.append("## 4. 未檢 .rs（誠實缺口）")
        lines.append("")
        for s in skip:
            lines.append("- `%s` — %s" % (s["path"], s.get("hint") or s["reason"]))
        lines.append("")
    lines.append("## 5. 檔案判決")
    lines.append("")
    lines.append("| 檔 | 判決 | 錯誤 |")
    lines.append("|---|---|---|")
    for f in sess.get("files") or []:
        lines.append("| %s | %s | %s |" % (
            f["path"], f["verdict"], ", ".join(f.get("codes") or []) or "—"))
    lines.append("")
    lines.append("---")
    lines.append("產生器：弦律 %s · helper + AHPBB · 狀態 `%s`" % (
        sess.get("engine", C.VERSION), sess.get("state", STATE_FILE)))
    lines.append("")
    return "\n".join(lines)


def write_report(sess, out_path):
    md = render_report(sess)
    os.makedirs(os.path.dirname(os.path.abspath(out_path)) or ".", exist_ok=True)
    with open(out_path, "w", encoding="utf-8") as f:
        f.write(md)
    return out_path


def history_summary(root, n=5):
    st = load_state(root)
    sess = st.get("sessions") or []
    tail = sess[-n:]
    return {
        "project": st.get("project"),
        "updated": st.get("updated"),
        "n": len(sess),
        "sessions": [{
            "id": s.get("id"), "ts": s.get("ts"),
            "overall": (s.get("score") or {}).get("overall"),
            "band": (s.get("score") or {}).get("band"),
            "errors": (s.get("score") or {}).get("counts", {}).get("errors"),
            "diff": s.get("diff"),
        } for s in tail],
    }
