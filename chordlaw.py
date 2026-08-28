#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
弦律 ChordLaw — 簡化 Rust 借用/生命週期檢查器 (工作原型 v0.7)
=============================================================
核心:
  1. mini Datalog 引擎: 分層 (stratified) 單調定點 + 證明樹 (provenance)
     + 有限域內建 (neq / path_conflict / covers / subpath)
  2. mini 前端: 玩具語言 .cl → 事實集
     (控制流 DAG 含 if/else 菱形、迴圈後向邊、作用域樹、多 fn、字段路徑/split borrow、
      imm/tmp/hole/slot、經參考寫/移、store、call/callmv)
  3. 圓示 (縱點節圖) SVG 渲染器: 點=陳述、弧=借用、圓=作用域
  4. 代理接口: --json / --explain / --rules / --check / --report / --mcp
  5. 32 則錯誤規則 E01–E32 (rules.dl) + 32 則圓示範例 (examples/rXX_*.cl)

規則 = 規則檔 (rules.dl + liveness_{nll,referent,lexical}.dl),
引擎是通用 Datalog 解譯器。純標準庫, 無外部依賴。

用法:
  python3 chordlaw.py                    # 全部範例
  python3 chordlaw.py examples/r01_eclash.cl
  python3 chordlaw.py --liveness nll|referent|lexical
  python3 chordlaw.py --json FILE        # 機器接口
  python3 chordlaw.py --explain FILE     # 規則原文+證明+幾何+修法
  python3 chordlaw.py --rules            # 規則規格速覽
  python3 chordlaw.py --check [DIR]      # 專案檢查
  python3 chordlaw.py --report [DIR]     # 評分+建議+寫入 .chordlaw/
  python3 chordlaw.py --history [DIR]
  python3 chordlaw.py --mcp              # stdio MCP 伺服器
  python3 chordlaw.py --factory          # AHPBB 批量產出正確 Rust
  python3 chordlaw.py --from-rs FILE.rs  # syn 子集 → 意圖樹 → 檢查

驗證:
  python3 test_chordlaw.py               # 回歸測試
  python3 oracle_check.py                # vs 真 rustc 差異測試 (需 rustc)

完整說明見 DOCS.md / RULES32.md; 計畫與論證見 PLAN.md。
"""
import os
import re
import sys
import glob

HERE = os.path.dirname(os.path.abspath(__file__))
VERSION = "v0.7"

def esc(s):
    return str(s).replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")

# ================================================================
# 1. mini Datalog 引擎 (分層 + 證明樹)
# ================================================================
def is_var(x):
    return isinstance(x, str) and (x == "_" or x[0] == "_" or x[0].isupper())

# 內建謂詞 (字串/路徑操作; 非 Datalog 事實)
BUILTINS = {"neq", "path_conflict", "covers", "subpath"}

def _field_prefix(a, b):
    """a 是 b 之嚴格字段前綴 (a='x', b='x.f')"""
    return len(b) > len(a) and b.startswith(a) and b[len(a)] == "."

def unify(subst, pat, val):
    s2 = dict(subst)
    for v, g in zip(pat, val):
        if is_var(v):
            if v in s2:
                if s2[v] != g:
                    return None
            else:
                s2[v] = g
        elif v != g:
            return None
    return s2

def bound(subst, v):
    if is_var(v):
        if v not in subst:
            raise RuntimeError("unbound variable in head: %s" % v)
        return subst[v]
    return v

class Datalog:
    def __init__(self):
        self.rules = []
        self.facts = {}   # pred -> set(tuple)
        self.why = {}     # (pred, tup) -> (rule_name, [(pred|!pred, tup), ...])

    def add_fact(self, pred, tup):
        tup = tuple(tup)
        self.facts.setdefault(pred, set()).add(tup)

    def add_rule(self, head_pred, head_args, pos, neg):
        self.rules.append(dict(head=(head_pred, tuple(head_args)),
                               pos=pos, neg=neg, name=None))

    def _body_preds(self, r):
        out = set()
        for P, _ in r["pos"]:
            if P not in BUILTINS:
                out.add((P, False))
        for P, _ in r["neg"]:
            out.add((P, True))
        return out

    def strata(self):
        preds = set()
        for r in self.rules:
            preds.add(r["head"][0])
            preds |= {q for q, _ in self._body_preds(r)}
        for P in self.facts:
            preds.add(P)
        st = {p: 0 for p in preds}
        changed = True
        while changed:
            changed = False
            for r in self.rules:
                P = r["head"][0]
                for Q, neg in self._body_preds(r):
                    if Q == P:
                        continue
                    need = st[Q] + (1 if neg else 0)
                    if need > st[P]:
                        st[P] = need
                        changed = True
        return st

    def run(self):
        dom = set()
        for _p, tups in self.facts.items():
            for t in tups:
                for x in t:
                    if isinstance(x, str) and not is_var(x):
                        dom.add(x)
        self._domain = tuple(sorted(dom))
        st = self.strata()
        top = max(st.values()) if st else 0
        for s in range(top + 1):
            preds = {p for p, d in st.items() if d == s}
            changed = True
            while changed:
                changed = False
                for r in self.rules:
                    if r["head"][0] not in preds:
                        continue
                    for h, body in self._fire(r):
                        if h[1] in self.facts.get(h[0], ()):
                            continue
                        self.add_fact(h[0], h[1])
                        self.why[h] = (r["name"] or h[0], body)
                        changed = True

    def _builtin_ok(self, P, a, b):
        if P == "neq":
            return a != b
        if P == "path_conflict":
            # 路徑衝突: 相等, 或一者為另一者之字段前綴 (整體 vs 子字段)
            return a == b or _field_prefix(a, b) or _field_prefix(b, a)
        if P == "covers":
            # 寫入 a 重新初始化 b (a == b 或 a 整體覆蓋 b)
            return a == b or _field_prefix(a, b)
        if P == "subpath":
            # a 嚴格深於 b (b 之子字段)
            return _field_prefix(b, a)
        return False

    def _fire(self, r):
        out = {}
        self._match(r, 0, {}, out, [])
        return out.items()

    def _match(self, r, i, subst, out, acc):
        pos = r["pos"]
        if i == len(pos):
            base_len = len(acc)
            for P, args in r["neg"]:
                t = tuple(bound(subst, v) for v in args)
                if P in BUILTINS:
                    if self._builtin_ok(P, *t):
                        del acc[base_len:]
                        return
                elif t in self.facts.get(P, ()):
                    del acc[base_len:]
                    return
                acc.append(("!" + P, t))
            h = (r["head"][0], tuple(bound(subst, v) for v in r["head"][1]))
            if h not in out:
                out[h] = list(acc)
            del acc[base_len:]
            return
        P, args = pos[i]
        if P in BUILTINS:
            unbound = [v for v in args if is_var(v) and v not in subst]
            if not unbound:
                vals = tuple(subst.get(v, v) if is_var(v) else v for v in args)
                if self._builtin_ok(P, *vals):
                    self._match(r, i + 1, subst, out, acc)
                return
            # 未綁定參數: 於有限全域 (所有常數) 上枚舉
            import itertools
            for combo in itertools.product(self._domain, repeat=len(unbound)):
                s2 = dict(subst)
                for v, g in zip(unbound, combo):
                    if v not in s2:
                        s2[v] = g
                vals = tuple(s2.get(v, v) if is_var(v) else v for v in args)
                if self._builtin_ok(P, *vals):
                    self._match(r, i + 1, s2, out, acc)
            return
        pat = tuple(subst.get(v, v) if is_var(v) else v for v in args)
        for tup in sorted(self.facts.get(P, ()), key=repr):
            s2 = unify(subst, pat, tup)
            if s2 is not None:
                acc.append((P, tup))
                self._match(r, i + 1, s2, out, acc)
                acc.pop()

def parse_rules(text):
    dl = Datalog()
    rn = 0
    for raw in text.splitlines():
        line = raw.strip()
        if not line or line.startswith("%"):
            continue
        line = line.rstrip().rstrip(".")
        if ":-" in line:
            head, body = line.split(":-", 1)
            atoms = parse_atoms(body)
        else:
            head, atoms = line, []
        hp, ha = parse_atom(head.strip())
        pos, neg = [], []
        for name, args in atoms:
            if name.startswith("!"):
                neg.append((name[1:], args))
            else:
                pos.append((name, args))
        rn += 1
        dl.add_rule(hp, ha, pos, neg)
        dl.rules[-1]["name"] = "R%02d %s" % (rn, hp)
        dl.rules[-1]["src"] = line
    return dl

def parse_atoms(s):
    out = []
    parts, depth, cur = [], 0, []
    for ch in s:
        if ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
        if ch == "," and depth == 0:
            parts.append("".join(cur))
            cur = []
        else:
            cur.append(ch)
    parts.append("".join(cur))
    for part in parts:
        p = part.strip()
        if not p:
            continue
        neg = p.startswith("!")
        if neg:
            p = p[1:]
        name, args = parse_atom(p)
        out.append(("!" + name if neg else name, args))
    return out

_FRESH = [0]

def parse_atom(p):
    p = p.strip()
    m = re.match(r"^([A-Za-z_][A-Za-z0-9_]*)(?:\(([^)]*)\))?$", p)
    if not m:
        raise RuntimeError("cannot parse atom: %r" % p)
    name = m.group(1)
    args = []
    if m.group(2) is not None:
        for a in m.group(2).split(","):
            a = a.strip()
            if a == "_":
                _FRESH[0] += 1
                a = "_g%d" % _FRESH[0]
            args.append(a)
    return name, args

# ================================================================
# 2. mini 前端: 玩具語言 .cl → 事實集
# ================================================================
# 語法 (每行一句):
#   fn NAME(args) { ... }     函數體 (參數之範圍 = ROOT; 每檔可多 fn; 參數可標 imm)
#   { ... }                   區塊
#   loop { ... }              迴圈 (後向邊)
#   if { ... } [else { ... }] 分支 (else 為兄弟; 菱形 CFG)
#   let P / let imm P / let hole P / tmp P / slot P
#   let P = &Q / &mut Q       出借 (sh / mut; Q 可為字段路徑 x.f)
#   use P / set P / mv P / dp P   讀取 / 寫入 / move / drop
#   set *T / mv *T / store P = T / call F(P) / callmv F(P)
#   ret T                     回傳參考 T / 回傳簡單名稱 = move
# 字段路徑: x.f.g — 根 x 須已宣告; 借 x.f 與 x.g 可共存 (split borrow), 與 x 整體不可;
#           經參考取字段 (deref coercion) 不受支持。

DNAME = r"(\w+(?:\.\w+)*)"   # 名稱或字段路徑

class Program:
    def __init__(self):
        self.root = "n0"
        self.scopes = [dict(id="n0", parent=None, kind="world", name=None, items=[], stmts=[])]
        self.names = [{}]          # 每個作用域一層: name -> (kind, scope_id)
        self.stack = []            # scope 堆疊
        self.stmts = []            # [(sid, text, scope_id)]
        self.facts = []            # (pred, tup)
        self.loop_scopes = []
        self.errors = []
        self.scope_of = {}         # place/參數/參考/路徑 -> scope_id (dict 僅供快速查; 事實以 facts 為準)
        self._path_scopes = set()  # (path, scope) 去重
        self.idx = {}
        self.all_stmts = {}        # scope_id -> [sid...] (子樹, 依序)
        self.descendants = {}      # scope_id -> set(scope_id...) (含自身)

    def cur(self):
        return self.stack[-1] if self.stack else self.scopes[0]

    def new_scope(self, kind, parent_id, name=None):
        sid = "n%d" % len(self.scopes)
        sc = dict(id=sid, parent=parent_id, kind=kind, name=name, items=[], stmts=[])
        self.scopes.append(sc)
        return sc

    def push(self, kind, name=None):
        sc = self.new_scope(kind, self.cur()["id"], name)
        if kind == "loop":
            self.loop_scopes.append(sc)
        self.cur()["items"].append(("scope", sc["id"]))   # 子作用域佔源碼順序之位
        self.stack.append(sc)
        self.names.append({})
        return sc

    def pop(self):
        sc = self.stack.pop()
        self.names.pop()
        return sc

    def resolve(self, name):
        for layer in reversed(self.names):
            if name in layer:
                return layer[name]
        return None

    def stmt(self, text):
        if not self.stack:
            self.errors.append((text, "E00 陳述必須在 fn 內"))
            return None
        sid = "s%d" % (len(self.stmts) + 1)
        self.stmts.append((sid, text, self.cur()["id"]))
        self.facts.append(("stmt", (sid,)))
        self.facts.append(("stmt_of", (sid, self.cur()["id"])))
        self.cur()["stmts"].append(sid)
        self.cur()["items"].append(("stmt", sid))
        return sid

    def _root_info(self, path, line):
        """路徑驗證: 回傳根之 (kind, scope) 或 None (已記錄 E00)"""
        if "." in path:
            root = path.split(".", 1)[0]
            info = self.resolve(root)
            if info is None:
                self.errors.append((line, "E00 未定義名稱: %s" % root))
                return None
            if info[0] == "ref":
                self.errors.append((line, "E00 經參考取字段 (deref coercion) 不受支持 (v0.3)"))
                return None
            return info
        info = self.resolve(path)
        if info is None:
            self.errors.append((line, "E00 未定義名稱: %s" % path))
            return None
        return info

    def _path_scope_fact(self, path):
        """路徑之 scope_of = 根之 scope (事實去重)"""
        if "." not in path:
            return
        root = path.split(".", 1)[0]
        sc = self.scope_of.get(root)
        if sc is None:
            return
        if (path, sc) not in self._path_scopes:
            self._path_scopes.add((path, sc))
            self.scope_of[path] = sc
            self.facts.append(("scope_of", (path, sc)))

    def _lend(self, new, q, kind, line):
        if new == q:
            self.errors.append((line, "E00 自引用/同名 shadowing 不受支持"))
            return
        if "." in new:
            self.errors.append((line, "E00 參考名稱須為簡單名稱 (v0.3)"))
            return
        info = self._root_info(q, line)
        if info is None:
            return
        sid = self.stmt(line)
        if sid is None:
            return
        self._path_scope_fact(q)
        self.names[-1][new] = ("ref", self.cur()["id"])
        self.scope_of[new] = self.cur()["id"]
        self.facts.append(("scope_of", (new, self.cur()["id"])))
        self.facts.append(("lend", (sid, q, new, kind)))
        self.facts.append(("borrow_of", (new, q)))

    def _place_op(self, op, nm, line):
        info = self._root_info(nm, line)
        if info is None:
            return
        if "." not in nm and info[0] == "ref":
            self.errors.append((line, "E00 對參考執行 %s" % op))
            return
        self._path_scope_fact(nm)
        sid = self.stmt(line)
        if sid is None:
            return
        self.facts.append((op, (sid, nm)))

def parse_cl(text):
    pr = Program()
    for raw in text.splitlines():
        line = raw.strip()
        if not line or line.startswith("//"):
            continue
        m = re.match(r"^fn\s+(\w+)\s*\(([^)]*)\)\s*\{$", line)
        if m:
            if pr.stack:
                pr.errors.append((line, "E00 fn 必須在頂層 (v0.3)"))
                continue
            pr.push("fn", name=m.group(1))
            for rawp in [x.strip() for x in m.group(2).split(",") if x.strip()]:
                mm = re.match(r"^(imm\s+)?(\w+)$", rawp)
                if not mm:
                    pr.errors.append((line, "E00 無法解析參數: %s" % rawp))
                    continue
                p = mm.group(2)
                pr.scope_of[p] = pr.root
                pr.names[-1][p] = ("param", pr.root)
                pr.facts.append(("scope_of", (p, pr.root)))
                pr.facts.append(("param", (p,)))
                if mm.group(1):
                    pr.facts.append(("imm", (p,)))
            continue
        if line == "loop {":
            pr.push("loop")
            continue
        if line == "if {":
            pr.push("if")
            continue
        if line.replace(" ", "") == "}else{":
            if not pr.stack:
                pr.errors.append((line, "E00 多餘的 } (未對應的閉合)"))
                continue
            popped = pr.pop()
            if popped["kind"] != "if":
                pr.errors.append((line, "E00 else 必須緊跟 if"))
                continue
            pr.push("else")
            continue
        if line == "else {":
            items = pr.cur()["items"]
            ok = False
            if items and items[-1][0] == "scope":
                sc = next((x for x in pr.scopes if x["id"] == items[-1][1]), None)
                if sc and sc["kind"] == "if":
                    ok = True
            if not ok:
                pr.errors.append((line, "E00 else 必須緊跟 if"))
                continue
            pr.push("else")
            continue
        if line == "{":
            pr.push("block")
            continue
        if line == "}":
            if not pr.stack:
                pr.errors.append((line, "E00 多餘的 } (未對應的閉合)"))
                continue
            pr.pop()
            continue
        m = re.match(r"^let\s+(\w+)\s*=\s*&mut\s+" + DNAME + "$", line)
        if m:
            pr._lend(m.group(1), m.group(2), "mut", line)
            continue
        m = re.match(r"^let\s+(\w+)\s*=\s*&\s*" + DNAME + "$", line)
        if m:
            pr._lend(m.group(1), m.group(2), "sh", line)
            continue
        m = re.match(r"^let\s+imm\s+(\w+)$", line)
        if m:
            p = m.group(1)
            sid = pr.stmt(line)
            if sid is None:
                continue
            pr.scope_of[p] = pr.cur()["id"]
            pr.names[-1][p] = ("place", pr.cur()["id"])
            pr.facts.append(("scope_of", (p, pr.cur()["id"])))
            pr.facts.append(("decl", (sid, p, pr.cur()["id"])))
            pr.facts.append(("imm", (p,)))
            continue
        m = re.match(r"^let\s+hole\s+(\w+)$", line)
        if m:
            p = m.group(1)
            sid = pr.stmt(line)
            if sid is None:
                continue
            pr.scope_of[p] = pr.cur()["id"]
            pr.names[-1][p] = ("place", pr.cur()["id"])
            pr.facts.append(("scope_of", (p, pr.cur()["id"])))
            pr.facts.append(("decl", (sid, p, pr.cur()["id"])))
            pr.facts.append(("hole", (p,)))
            continue
        m = re.match(r"^tmp\s+(\w+)$", line)
        if m:
            p = m.group(1)
            sid = pr.stmt(line)
            if sid is None:
                continue
            pr.scope_of[p] = pr.cur()["id"]
            pr.names[-1][p] = ("place", pr.cur()["id"])
            pr.facts.append(("scope_of", (p, pr.cur()["id"])))
            pr.facts.append(("decl", (sid, p, pr.cur()["id"])))
            pr.facts.append(("temp", (p,)))
            continue
        m = re.match(r"^slot\s+(\w+)$", line)
        if m:
            p = m.group(1)
            sid = pr.stmt(line)
            if sid is None:
                continue
            pr.scope_of[p] = pr.cur()["id"]
            pr.names[-1][p] = ("place", pr.cur()["id"])
            pr.facts.append(("scope_of", (p, pr.cur()["id"])))
            pr.facts.append(("decl", (sid, p, pr.cur()["id"])))
            continue
        m = re.match(r"^let\s+(\w+)$", line)
        if m:
            p = m.group(1)
            sid = pr.stmt(line)
            if sid is None:
                continue
            pr.scope_of[p] = pr.cur()["id"]
            pr.names[-1][p] = ("place", pr.cur()["id"])
            pr.facts.append(("scope_of", (p, pr.cur()["id"])))
            pr.facts.append(("decl", (sid, p, pr.cur()["id"])))
            continue
        m = re.match(r"^use\s+" + DNAME + "$", line)
        if m:
            nm = m.group(1)
            if "." in nm:
                if pr._root_info(nm, line) is None:
                    continue
                pr._path_scope_fact(nm)
                sid = pr.stmt(line)
                if sid is None:
                    continue
                pr.facts.append(("read", (sid, nm)))
                continue
            info = pr.resolve(nm)
            if info is None:
                pr.errors.append((line, "E00 未定義名稱: %s" % nm))
                continue
            sid = pr.stmt(line)
            if sid is None:
                continue
            if info[0] == "ref":
                pr.facts.append(("use_ref", (sid, nm)))
            else:
                pr.facts.append(("read", (sid, nm)))
            continue
        m = re.match(r"^set\s+\*\s*(\w+)$", line)
        if m:
            nm = m.group(1)
            info = pr.resolve(nm)
            if info is None:
                pr.errors.append((line, "E00 未定義名稱: %s" % nm))
                continue
            if info[0] != "ref":
                pr.errors.append((line, "E00 set * 須作用於參考"))
                continue
            sid = pr.stmt(line)
            if sid is None:
                continue
            pr.facts.append(("deref_write", (sid, nm)))
            pr.facts.append(("use_ref", (sid, nm)))
            continue
        m = re.match(r"^mv\s+\*\s*(\w+)$", line)
        if m:
            nm = m.group(1)
            info = pr.resolve(nm)
            if info is None:
                pr.errors.append((line, "E00 未定義名稱: %s" % nm))
                continue
            if info[0] != "ref":
                pr.errors.append((line, "E00 mv * 須作用於參考"))
                continue
            sid = pr.stmt(line)
            if sid is None:
                continue
            pr.facts.append(("deref_move", (sid, nm)))
            pr.facts.append(("use_ref", (sid, nm)))
            continue
        m = re.match(r"^store\s+(\w+)\s*=\s*(\w+)$", line)
        if m:
            slot, refn = m.group(1), m.group(2)
            si = pr.resolve(slot)
            ri = pr.resolve(refn)
            if si is None:
                pr.errors.append((line, "E00 未定義名稱: %s" % slot))
                continue
            if ri is None:
                pr.errors.append((line, "E00 未定義名稱: %s" % refn))
                continue
            if si[0] == "ref":
                pr.errors.append((line, "E00 store 目標須為槽/地方"))
                continue
            if ri[0] != "ref":
                pr.errors.append((line, "E00 store 來源須為參考"))
                continue
            sid = pr.stmt(line)
            if sid is None:
                continue
            pr.facts.append(("store", (sid, slot, refn)))
            pr.facts.append(("use_ref", (sid, refn)))
            continue
        m = re.match(r"^callmv\s+(\w+)\s*\(([^)]*)\)$", line)
        if m:
            args = [x.strip() for x in m.group(2).split(",") if x.strip()]
            if not args:
                pr.errors.append((line, "E00 callmv 需要參數"))
                continue
            bad = False
            for a in args:
                if pr._root_info(a, line) is None:
                    bad = True
                    break
            if bad:
                continue
            sid = pr.stmt(line)
            if sid is None:
                continue
            for a in args:
                pr._path_scope_fact(a)
                info = pr.resolve(a.split(".", 1)[0])
                if info and info[0] == "ref" and "." not in a:
                    pr.facts.append(("use_ref", (sid, a)))
                else:
                    pr.facts.append(("callmv", (sid, a)))
            continue
        m = re.match(r"^call\s+(\w+)\s*\(([^)]*)\)$", line)
        if m:
            args = [x.strip() for x in m.group(2).split(",") if x.strip()]
            if not args:
                pr.errors.append((line, "E00 call 需要參數"))
                continue
            bad = False
            for a in args:
                if pr._root_info(a, line) is None:
                    bad = True
                    break
            if bad:
                continue
            sid = pr.stmt(line)
            if sid is None:
                continue
            for a in args:
                pr._path_scope_fact(a)
                info = pr.resolve(a.split(".", 1)[0])
                if info and info[0] == "ref" and "." not in a:
                    pr.facts.append(("use_ref", (sid, a)))
                else:
                    pr.facts.append(("call", (sid, a)))
            continue
        m = re.match(r"^set\s+" + DNAME + "$", line)
        if m:
            pr._place_op("write", m.group(1), line)
            continue
        m = re.match(r"^mv\s+" + DNAME + "$", line)
        if m:
            pr._place_op("move", m.group(1), line)
            continue
        m = re.match(r"^dp\s+" + DNAME + "$", line)
        if m:
            pr._place_op("drop", m.group(1), line)
            continue
        m = re.match(r"^ret\s+(\w+)$", line)
        if m:
            nm = m.group(1)
            info = pr.resolve(nm)
            if info is None:
                pr.errors.append((line, "E00 未定義名稱: %s" % nm))
                continue
            sid = pr.stmt(line)
            if sid is None:
                continue
            if info[0] == "ref":
                pr.facts.append(("use_ref", (sid, nm)))
                pr.facts.append(("ret", (sid, nm)))
            else:
                pr.facts.append(("move", (sid, nm)))
            continue
        pr.errors.append((line, "E00 無法解析"))
    if pr.stack:
        pr.errors.append(("", "E00 未關閉的作用域"))
    pr._build_graph()
    return pr

def _build_graph(self):
    self.idx = {s[0]: i for i, s in enumerate(self.stmts)}
    by_id = {s["id"]: s for s in self.scopes}
    children = {}
    for s in self.scopes:
        if s["parent"] is not None:
            children.setdefault(s["parent"], []).append(s)

    # 子樹統計 (含自身)
    def subtree(sid):
        sc = by_id[sid]
        ids = {sid}
        out = list(sc["stmts"])
        for ch in children.get(sid, []):
            cids, cout = subtree(ch["id"])
            ids |= cids
            out += cout
        out.sort(key=lambda s: self.idx[s])
        return ids, out
    for s in self.scopes:
        self.descendants[s["id"]], self.all_stmts[s["id"]] = subtree(s["id"])

    # 把子樹陳述掛回 scope, 供圓示使用
    for sc in self.scopes:
        sc["all_stmts"] = self.all_stmts[sc["id"]]

    # 控制流邊: 依作用域樹遍歷; if/else 為菱形 (兄弟, 無 sequential 邊)
    def walk(scope):
        firsts = []
        prevs = []
        items = scope["items"]
        i = 0
        n = len(items)
        while i < n:
            kind, ref = items[i]
            if kind == "stmt":
                if prevs:
                    for p in prevs:
                        self.facts.append(("edge", (p, ref)))
                else:
                    firsts.append(ref)
                prevs = [ref]
                i += 1
                continue
            if not self.all_stmts.get(ref):
                i += 1
                continue
            child = by_id[ref]
            else_id = None
            if child["kind"] == "if" and i + 1 < n:
                k2, r2 = items[i + 1]
                if k2 == "scope" and by_id[r2]["kind"] == "else":
                    else_id = r2
            cf, cl = walk(child)
            i += 1
            ef, el = [], []
            if else_id:
                i += 1
                if self.all_stmts.get(else_id):
                    ef, el = walk(by_id[else_id])
            entries = list(cf) + list(ef)
            exits = (list(cl) + list(el)) if else_id else list(cl)
            if prevs:
                for p in prevs:
                    for e in entries:
                        if e:
                            self.facts.append(("edge", (p, e)))
            else:
                firsts.extend([e for e in entries if e])
            if exits:
                prevs = [e for e in exits if e]
        return firsts, prevs
    walk(self.scopes[0])   # world → 各 fn 子樹

    # 迴圈後向邊 (子樹最後 → 最前)
    for sc in self.loop_scopes:
        all_s = self.all_stmts[sc["id"]]
        if all_s:
            self.facts.append(("edge", (all_s[-1], all_s[0])))

    # 作用域結構
    for sc in self.scopes:
        self.facts.append(("same", (sc["id"], sc["id"])))
        if sc["parent"] is not None:
            self.facts.append(("parent", (sc["id"], sc["parent"])))
        if sc["kind"] == "if":
            self.facts.append(("if_scope", (sc["id"],)))
        elif sc["kind"] == "else":
            self.facts.append(("else_scope", (sc["id"],)))
    # 作用域終端 (lexical 活度等級用)
    for sc in self.scopes:
        all_s = self.all_stmts[sc["id"]]
        if all_s:
            self.facts.append(("scope_last", (sc["id"], all_s[-1])))

Program._build_graph = _build_graph

# ================================================================
# 3. 檢查器
# ================================================================
ERROR_CODES = {
    "eclash":       ("E01", "紅弧交越: 兩借用重疊且至少一者為 mut (~ E0499/E0502)"),
    "ewrite":       ("E02", "借用活躍期間寫入被借者 (~ E0506)"),
    "eread":        ("E03", "mut 借用活躍期間直接讀取被借者 (~ E0503)"),
    "edangle":      ("E04", "使用點落在被借者作用域之外 (~ E0597)"),
    "eadrop":       ("E05", "drop 後使用"),
    "emove":        ("E06", "move 後使用 (~ E0382)"),
    "eloanmove":    ("E07", "借用活躍期間 move 被借者 (~ E0505)"),
    "eloandrop":    ("E08", "借用活躍期間 drop 被借者"),
    "erefuse":      ("E09", "別名層衝突: 參考 t 使用期間, t 自身被借用活躍 (~ E0502/E0499 別名層)"),
    "ereturn":      ("E10", "回傳參考之被借者不活得比呼叫者久 (~ E0106)"),
    "eimmut":       ("E11", "寫入不可變地方 (~ E0384)"),
    "enotmut":      ("E12", "對不可變地方作 mut 出借 (~ E0596)"),
    "eassignsh":    ("E13", "經共享參考寫入 (~ E0594)"),
    "emoveout":     ("E14", "經參考移出 (~ E0507)"),
    "etemp":        ("E15", "暫存借用被非緊鄰陳述使用 (~ E0716 保守)"),
    "eescape":      ("E16", "把指向局部的參考存入槽 (~ E0521)"),
    "estoretemp":   ("E17", "把指向暫存的參考存入槽 (~ E0716)"),
    "ebranchmove":  ("E18", "if 分支內 move 後於分支外使用 (~ E0382)"),
    "edoubledrop":  ("E19", "重複 drop (無中間重初始化)"),
    "edropmoved":   ("E20", "move 後再 drop (~ E0382)"),
    "ealiascyc":    ("E21", "存入造成別名環 (槽指向自身)"),
    "eimmfield":    ("E22", "寫入不可變地方之子字段 (~ E0594)"),
    "enotmutfield": ("E23", "對不可變字段作 mut 出借 (~ E0596)"),
    "eelsejoin":    ("E24", "else 分支內 move 後於分支外使用 (~ E0382)"),
    "ecallmut":     ("E25", "呼叫讀取 mut 借用活躍之路徑 (~ E0503)"),
    "ecallmove":    ("E26", "呼叫消耗後再使用 (~ E0382)"),
    "ecalloan":     ("E27", "借用活躍期間呼叫消耗被借者 (~ E0505)"),
    "erettemp":     ("E28", "回傳指向暫存的參考 (~ E0515)"),
    "euninit":      ("E29", "使用尚未初始化的洞 (~ E0381)"),
    "euninitret":   ("E30", "移出尚未初始化的洞 (~ E0381)"),
    "eparamimmut":  ("E31", "寫入不可變參數 (~ E0594)"),
    "eparamnotmut": ("E32", "對不可變參數作 mut 出借 (~ E0596)"),
}
CODE2PRED = {v[0]: k for k, v in ERROR_CODES.items()}
CODE2MSG = {v[0]: v[1] for k, v in ERROR_CODES.items()}

LIVENESS_MODES = {
    "nll": "liveness_nll.dl",
    "referent": "liveness_referent.dl",
    "lexical": "liveness_lexical.dl",
}

def load_rules(liveness, root="n0"):
    with open(os.path.join(HERE, "rules.dl"), encoding="utf-8") as f:
        shared = f.read().replace("@ROOT@", root)
    with open(os.path.join(HERE, LIVENESS_MODES[liveness]), encoding="utf-8") as f:
        live = f.read()
    return parse_rules(shared + "\n" + live)

def _facts_for_fn(self, f):
    """該 fn 子樹所需之事實 (多 fn 各自獨立檢查; 無跨 fn 干擾)"""
    sub = self.descendants[f["id"]]
    st = set(self.all_stmts[f["id"]])
    out = []
    for pred, tup in self.facts:
        if pred in ("stmt", "stmt_of", "lend", "use_ref", "read", "write",
                    "move", "drop", "ret", "decl", "deref_write", "deref_move",
                    "call", "callmv", "store"):
            if tup[0] in st:
                out.append((pred, tup))
        elif pred == "edge":
            if tup[0] in st and tup[1] in st:
                out.append((pred, tup))
        elif pred == "scope_of":
            if tup[1] in sub or tup[1] == self.root:
                out.append((pred, tup))
        elif pred in ("parent", "same"):
            if all(x in sub or x == self.root for x in tup):
                out.append((pred, tup))
        elif pred in ("scope_last", "if_scope", "else_scope"):
            if tup[0] in sub:
                out.append((pred, tup))
        elif pred == "borrow_of":
            if self.scope_of.get(tup[0]) in sub:
                out.append((pred, tup))
        elif pred in ("imm", "temp", "hole", "param"):
            sc = self.scope_of.get(tup[0])
            if sc in sub or sc == self.root:
                out.append((pred, tup))
    return out

Program.facts_for_fn = _facts_for_fn

def check(text, liveness="nll"):
    """回傳 (pr, dl, errors); 多 fn 檔案各 fn 獨立分析 (設計: 函數內局部化)。
    errors: [(code, sid, tup)] — sid 全域編號; dl = 首個 fn 之引擎 (pr.dls 有全部)。"""
    if liveness not in LIVENESS_MODES:
        raise ValueError("unknown liveness: %s (可用: %s)" % (liveness, ", ".join(LIVENESS_MODES)))
    pr = parse_cl(text)
    if pr.errors:
        return pr, None, [("E00", "?", (e[0] or "(結構)", e[1])) for e in pr.errors]
    fns = [s for s in pr.scopes if s["kind"] == "fn"]
    if not fns:
        return pr, None, [("E00", "?", ("(結構)", "至少需要一個 fn"))]
    pr.dls = {}
    pr.fn_of_stmt = {}
    all_errors = []
    dl_main = None
    for f in fns:
        dl = load_rules(liveness)
        for pred, tup in pr.facts_for_fn(f):
            dl.add_fact(pred, tup)
        dl.run()
        pr.dls[f["id"]] = dl
        for sid in pr.all_stmts[f["id"]]:
            pr.fn_of_stmt[sid] = f["id"]
        if dl_main is None:
            dl_main = dl
        for pred, (code, _msg) in ERROR_CODES.items():
            for tup in sorted(dl.facts.get(pred, ()), key=repr):
                all_errors.append((code, tup[0], tup))
    all_errors.sort(key=lambda e: (int(e[1][1:]), e[0]))
    return pr, dl_main, all_errors

# ================================================================
# 4. 證明樹
# ================================================================
def fmt_fact(pred, tup):
    if pred.startswith("!"):
        return "¬%s(%s)" % (pred[1:], ", ".join(str(x) for x in tup))
    return "%s(%s)" % (pred, ", ".join(str(x) for x in tup))

def print_proof(dl, pred, tup, out, depth=0, maxd=6, seen=None):
    seen = seen or set()
    key = (pred, tup)
    indent = "    " * depth
    if key in seen or depth > maxd:
        out.append(indent + fmt_fact(pred, tup))
        return
    seen.add(key)
    if key in dl.why:
        out.append(indent + fmt_fact(pred, tup))
        rule, body = dl.why[key]
        out.append(indent + "└─ 規則 %s:" % rule)
        for bp, bt in body:
            if (bp, bt) in dl.why:
                print_proof(dl, bp, bt, out, depth + 1, maxd, seen)
            else:
                out.append(indent + "  " + fmt_fact(bp, bt) + "   (事實)")
    else:
        out.append(indent + fmt_fact(pred, tup) + "   (事實)")

# ================================================================
# 4b. 幾何 API (P-G1–P-G4: 法則可測、圖=事實、代理可操作)
# ================================================================
# 錯誤碼 → 法則編號 (GEOMETRY.md §5)
LAW_OF = {
    "E01": 1, "E09": 1,
    "E04": 2, "E10": 2, "E16": 2, "E17": 2, "E28": 2,
    "E05": 3, "E06": 3, "E15": 3, "E18": 3, "E19": 3, "E20": 3,
    "E24": 3, "E26": 3, "E29": 3, "E30": 3,
    "E02": 4, "E03": 4, "E25": 4,
    "E11": 10, "E12": 10, "E13": 10, "E14": 10, "E07": 10, "E08": 10,
    "E22": 10, "E23": 10, "E27": 10, "E31": 10, "E32": 10,
    "E21": 9,
}
LAW_NAME = {
    0: "法則⓪ 同構 (圖不立法)",
    1: "法則① 紅弧孤立",
    2: "法則② 弧在圓內",
    3: "法則③ 點序守紀",
    4: "法則④ 區間著色",
    5: "法則⑤ 同心圓包含",
    6: "法則⑥ 後向邊閉包",
    7: "法則⑦ 菱形不串線",
    8: "法則⑧ 路徑前綴衝突",
    9: "法則⑨ 別名鏈投影",
    10: "法則⑩ 權限點",
}


def scope_depth(pr, sc):
    d, c = 0, sc
    by_id = {s["id"]: s for s in pr.scopes}
    while c["parent"] is not None:
        c = by_id[c["parent"]]
        d += 1
    return d


def scope_rx(pr, sc):
    return 268 - 34 * scope_depth(pr, sc)


def collect_arcs(pr, dl):
    """每條活借貸 → {L,E,K,T,Q,I}；I = onregion 點集 (法則 0 的區間)。"""
    idx = pr.idx
    out = []
    for (L, Q, T, K) in sorted(dl.facts.get("lend", ()), key=lambda t: idx.get(t[0], 0)):
        ends = [t[2] for t in dl.facts.get("span_end", ()) if t[0] == L and t[1] == T]
        if not ends:
            continue
        E = max(ends, key=lambda s: idx[s])
        I = {q for (l, q) in dl.facts.get("onregion", ()) if l == L}
        out.append(dict(L=L, E=E, K=K, T=T, Q=Q, I=I))
    return out


def textual_span(pr, L, E):
    """同一 fn 內、τ(L)…τ(E) 的陳述 (含端點)。"""
    i0, i1 = pr.idx[L], pr.idx[E]
    fn = getattr(pr, "fn_of_stmt", {}).get(L)
    out = []
    for sid, _t, _sc in pr.stmts:
        i = pr.idx[sid]
        if i0 <= i <= i1 and (fn is None or getattr(pr, "fn_of_stmt", {}).get(sid) == fn):
            out.append(sid)
    return out


def arc_has_hole(pr, arc):
    """NLL 空洞: 文本跨度內有點不在 I(a) (法則 7 / P-G2)。"""
    span = textual_span(pr, arc["L"], arc["E"])
    return any(s not in arc["I"] for s in span)


def conflicting(qa, qb):
    return qa == qb or _field_prefix(qa, qb) or _field_prefix(qb, qa)


def law1_overlap_pairs(pr, dl):
    """法則 1: 路徑衝突且 ≥1 mut 且 I 相交的弧對。"""
    arcs = collect_arcs(pr, dl)
    pairs = []
    for i in range(len(arcs)):
        for j in range(i + 1, len(arcs)):
            a, b = arcs[i], arcs[j]
            if a["K"] != "mut" and b["K"] != "mut":
                continue
            if not conflicting(a["Q"], b["Q"]):
                continue
            inter = a["I"] & b["I"]
            if inter:
                pairs.append((a, b, inter))
    return pairs


def eclash_pairs(pr, dl, errors):
    """僅 eclash 證明所對應的弧對 (法則 0: ✕ 由證明驅動)。"""
    clash_sids = {sid for code, sid, _t in errors if code == "E01"}
    if not clash_sids:
        return []
    out = []
    seen = set()
    for a, b, inter in law1_overlap_pairs(pr, dl):
        if a["L"] in clash_sids or b["L"] in clash_sids:
            key = tuple(sorted((a["L"], b["L"])))
            if key not in seen:
                seen.add(key)
                out.append((a, b, inter))
    return out


def geometry_report(pr, dl, code, sid):
    """P-G4: 法則違反的可操作幾何報告 (區間、交集、縮弧/移點/升圓)。"""
    law = LAW_OF.get(code)
    lines = []
    if law is None:
        return None, lines
    lines.append(LAW_NAME[law])
    arcs = collect_arcs(pr, dl)
    idx = pr.idx
    if law == 1:
        for a, b, inter in law1_overlap_pairs(pr, dl):
            ia = "{" + ",".join(sorted(a["I"], key=lambda s: idx[s])) + "}"
            ib = "{" + ",".join(sorted(b["I"], key=lambda s: idx[s])) + "}"
            ii = "{" + ",".join(sorted(inter, key=lambda s: idx[s])) + "}"
            lines.append("  弧 %s : %s%s  I = %s" % (
                a["T"], "&mut " if a["K"] == "mut" else "&", a["Q"], ia))
            lines.append("  弧 %s : %s%s  I = %s" % (
                b["T"], "&mut " if b["K"] == "mut" else "&", b["Q"], ib))
            lines.append("  I(a) ∩ I(b) = %s ≠ ∅" % ii)
            first, second = (a, b) if idx[a["L"]] <= idx[b["L"]] else (b, a)
            # 縮短先出借者: 把終端使用移到後出借點之前
            pred = None
            for s, _t, _sc in pr.stmts:
                if idx[s] < idx[second["L"]] and s in first["I"]:
                    pred = s
            if pred and pred != first["E"]:
                lines.append("  修: 縮弧 — 把 %s 的終端使用從 %s 移到 %s (後弧出借點 %s 之前)" % (
                    first["T"], first["E"], pred, second["L"]))
            else:
                lines.append("  修: 縮弧 / clone — 令兩弧不再重疊")
    elif law == 2:
        lines.append("  點 %s 的使用／回傳／存槽穿過被借者作用域圓" % sid)
        lines.append("  修: 升圓 — 令被借者來自參數 (ROOT), 或把使用移進圓內")
    elif law == 3:
        lines.append("  點 %s 落在消耗點之後、重初始化之前 (有效區間外)" % sid)
        lines.append("  修: 移點 — 先使用再消耗, 或消耗後先 set 再讀")
    elif law == 4:
        on = [a for a in arcs if sid in a["I"]]
        if on:
            lines.append("  點 %s 上的活弧: %s" % (
                sid, "; ".join("%s(:%s)" % (a["T"], a["K"]) for a in on)))
        lines.append("  修: 縮弧 — 令該點不再落在衝突弧跨內")
    elif law == 9:
        lines.append("  別名鏈投影成環 (槽指向自身)")
        lines.append("  修: 不要把指向槽自身的參考存回槽")
    elif law == 10:
        lines.append("  點 %s 上的操作超出該弧允許的權限" % sid)
        lines.append("  修: 改經引用存取, 或把操作移出弧跨")
    return law, lines


# ================================================================
# 5. 圓示 (縱點節圖) SVG 渲染器
# ================================================================
AX = 300          # 縱軸 x
Y0 = 96           # 第一點 y
DY = 54           # 點距

def _bez(p0, p1, p2, p3, t):
    u = 1 - t
    x = u**3 * p0[0] + 3 * u * u * t * p1[0] + 3 * u * t * t * p2[0] + t**3 * p3[0]
    y = u**3 * p0[1] + 3 * u * u * t * p1[1] + 3 * u * t * t * p2[1] + t**3 * p3[1]
    return (x, y)

def _seg_hit(a, b, c, d):
    def cross(o, p, q):
        return (p[0] - o[0]) * (q[1] - o[1]) - (p[1] - o[1]) * (q[0] - o[0])
    d1 = cross(c, d, a)
    d2 = cross(c, d, b)
    d3 = cross(a, b, c)
    d4 = cross(a, b, d)
    if ((d1 > 0 and d2 < 0) or (d1 < 0 and d2 > 0)) and \
       ((d3 > 0 and d4 < 0) or (d3 < 0 and d4 > 0)):
        t = d1 / (d1 - d2)
        return (a[0] + t * (b[0] - a[0]), a[1] + t * (b[1] - a[1]))
    return None

def _sample(pts, n=28):
    return [(_bez(*pts, i / n), _bez(*pts, (i + 1) / n)) for i in range(n)]

def _stmt_at_y(pr, yy):
    n = len(pr.stmts)
    if n == 0:
        return None
    i = int(round((yy - Y0) / float(DY)))
    return pr.stmts[max(0, min(n - 1, i))][0]

def _draw_arc_with_holes(svg, pr, p0, p1, p2, p3, I, col, w):
    """實心只畫 I(a) 上的點; 文本跨度內的空洞改虛線 (P-G2)。"""
    n_samp = 36
    samples = [_bez(p0, p1, p2, p3, i / float(n_samp)) for i in range(n_samp + 1)]

    def live_at(pt):
        sid = _stmt_at_y(pr, pt[1])
        return sid is None or sid in I

    if all(live_at(pt) for pt in samples):
        svg.append('<path d="M %.0f %.0f C %.0f %.0f, %.0f %.0f, %.0f %.0f" fill="none" stroke="%s" stroke-width="%s"/>' % (
            p0[0], p0[1], p1[0], p1[1], p2[0], p2[1], p3[0], p3[1], col, w))
        return
    runs = []
    for pt in samples:
        lv = live_at(pt)
        if not runs or runs[-1][0] != lv:
            runs.append((lv, [pt]))
        else:
            runs[-1][1].append(pt)
    for k in range(1, len(runs)):
        prev = runs[k - 1][1][-1]
        if runs[k][1][0] != prev:
            runs[k] = (runs[k][0], [prev] + runs[k][1])
    for live, pts in runs:
        if len(pts) < 2:
            continue
        d = "M %.1f %.1f" % pts[0]
        for p in pts[1:]:
            d += " L %.1f %.1f" % p
        dash = "" if live else ' stroke-dasharray="6 5"'
        svg.append('<path d="%s" fill="none" stroke="%s" stroke-width="%s"%s/>' % (d, col, w, dash))

def render_svg(pr, dl, errors, title, liveness):
    idx = pr.idx
    n = len(pr.stmts)
    y = lambda i: Y0 + DY * i
    W = 860
    H = Y0 + DY * (n - 1) + 190
    svg = []
    svg.append('<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="monospace">' % (W, H, W, H))
    svg.append('<rect width="100%" height="100%" fill="#0f172a"/>')

    fail = [e for e in errors if e[0] != "E00"]
    if fail:
        svg.append('<rect x="12" y="10" width="%d" height="30" rx="6" fill="#7f1d1d"/>' % (W - 24))
        svg.append('<text x="24" y="30" fill="#fecaca" font-size="15" font-weight="bold">✗ FAIL — %d 個錯誤  ·  圓示 Chord Diagram · 弦律 %s · liveness=%s</text>' % (len(fail), VERSION, liveness))
    else:
        svg.append('<rect x="12" y="10" width="%d" height="30" rx="6" fill="#14532d"/>' % (W - 24))
        svg.append('<text x="24" y="30" fill="#bbf7d0" font-size="15" font-weight="bold">✓ PASS — 無違規  ·  圓示 Chord Diagram · 弦律 %s · liveness=%s</text>' % (VERSION, liveness))
    svg.append('<text x="%d" y="30" fill="#64748b" font-size="12" text-anchor="end">%s</text>' % (W - 20, esc(title)))

    # 縱軸 (時間向下)
    if n >= 1:
        svg.append('<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#334155" stroke-width="2"/>' % (AX, Y0 - 26, AX, Y0 + DY * (n - 1) + 26))

    # 作用域圓 (同心橢圓; 巢狀 = 同心)
    def depth_of(sc):
        d = 0
        c = sc
        by_id = {s["id"]: s for s in pr.scopes}
        while c["parent"] is not None:
            c = by_id[c["parent"]]
            d += 1
        return d
    for sc in pr.scopes:
        sc_stmts = sc.get("all_stmts") or pr.all_stmts.get(sc["id"]) or []
        if sc["kind"] == "world" or not sc_stmts:
            continue
        ys = [y(idx[s]) for s in sc_stmts]
        cy = (min(ys) + max(ys)) / 2
        ry = (max(ys) - min(ys)) / 2 + 34
        rx = 268 - 34 * depth_of(sc)
        col = {"fn": "#64748b", "block": "#64748b", "loop": "#8b5cf6", "if": "#0ea5e9", "else": "#f59e0b"}.get(sc["kind"], "#64748b")
        svg.append('<ellipse cx="%d" cy="%.0f" rx="%d" ry="%.0f" fill="none" stroke="%s" stroke-width="1.5" stroke-dasharray="7 5"/>' % (AX, cy, rx, ry, col))
        lab = sc["kind"] + (" 迴圈 (含後向邊)" if sc["kind"] == "loop" else "")
        if sc["kind"] == "fn" and sc.get("name"):
            lab = "fn %s" % sc["name"]
        svg.append('<text x="%d" y="%.0f" fill="%s" font-size="11" text-anchor="middle">%s</text>' % (AX, cy - ry - 7, col, lab))

    # 迴圈後向邊 (左側虛線迴路 + 箭頭)
    for sc in pr.loop_scopes:
        sc_stmts = sc.get("all_stmts") or pr.all_stmts.get(sc["id"]) or []
        if not sc_stmts:
            continue
        yf, yl = y(idx[sc_stmts[0]]), y(idx[sc_stmts[-1]])
        xl = AX - 30
        svg.append('<path d="M %d %d L %d %d L %d %d L %d %d" fill="none" stroke="#8b5cf6" stroke-width="1.5" stroke-dasharray="4 4"/>' % (AX - 8, yl, xl, yl, xl, yf, AX - 16, yf))
        svg.append('<polygon points="%d,%d %d,%d %d,%d" fill="#8b5cf6"/>' % (AX - 8, yf, AX - 18, yf - 4, AX - 18, yf + 4))
        svg.append('<text x="%d" y="%d" fill="#8b5cf6" font-size="10" text-anchor="end">下輪</text>' % (xl - 4, (yf + yl) / 2 + 3))

    # 陳述點 (縱點節圖之「點」)
    err_at = {}
    for code, sid, _t in fail:
        err_at.setdefault(sid, []).append(code)
    for i, (sid, text, _sc) in enumerate(pr.stmts):
        yy = y(i)
        lends = [t for t in dl.facts.get("lend", ()) if t[0] == sid]
        if lends:
            col = "#f87171" if lends[0][3] == "mut" else "#60a5fa"
        elif sid in err_at:
            col = "#ef4444"
        else:
            col = "#94a3b8"
        svg.append('<circle cx="%d" cy="%d" r="5" fill="%s"/>' % (AX, yy, col))
        if sid in err_at:
            svg.append('<circle cx="%d" cy="%d" r="10" fill="none" stroke="#ef4444" stroke-width="2"/>' % (AX, yy))
        lab = text if len(text) <= 24 else text[:23] + "…"
        svg.append('<text x="%d" y="%d" fill="#cbd5e1" font-size="12" text-anchor="end">%s  %s</text>' % (AX - 14, yy + 4, esc(sid), esc(lab)))

    # 借用弧
    def referent_place(t):
        bo = dl.facts.get("borrow_of", ())
        cur, guard = t, 0
        while cur in {x[0] for x in bo} and guard < 8:
            nxt = [x[1] for x in bo if x[0] == cur]
            if not nxt:
                break
            cur, guard = nxt[0], guard + 1
        return cur
    collected = collect_arcs(pr, dl)
    I_of = {(a["L"], a["T"]): a["I"] for a in collected}
    lend_facts = sorted(dl.facts.get("lend", ()), key=lambda t: idx[t[0]])
    lanes, arcs = {}, []
    for (L, Q, T, K) in lend_facts:
        rp = referent_place(T)
        if rp not in lanes:
            lanes[rp] = len(lanes)
        ends = [t[2] for t in dl.facts.get("span_end", ()) if t[0] == L and t[1] == T]
        if not ends:
            yy = y(idx[L])
            svg.append('<circle cx="%d" cy="%d" r="3.5" fill="none" stroke="#64748b" stroke-width="1.5"/>' % (AX + 10, yy))
            svg.append('<text x="%d" y="%d" fill="#64748b" font-size="11">(死借貸: 無使用)</text>' % (AX + 18, yy + 4))
            continue
        E = max(ends, key=lambda s: idx[s])
        bulge = 84 + 58 * lanes[rp]
        p0 = (AX, y(idx[L]))
        p3 = (AX, y(idx[E]))
        p1 = (AX + bulge, y(idx[L]))
        p2 = (AX + bulge, y(idx[E]))
        col = "#ef4444" if K == "mut" else "#3b82f6"
        w = 3.5 if K == "mut" else 2
        I = I_of.get((L, T), set())
        _draw_arc_with_holes(svg, pr, p0, p1, p2, p3, I, col, w)
        mid = _bez(p0, p1, p2, p3, 0.5)
        svg.append('<text x="%.0f" y="%.0f" fill="%s" font-size="11">%s : %s %s</text>' % (mid[0] + 8, mid[1] + 4, col, esc(T), "&amp;mut" if K == "mut" else "&amp;", esc(Q)))
        arcs.append(dict(L=L, E=E, K=K, pts=(p0, p1, p2, p3), T=T, Q=Q, I=I))

    # ✕ 僅由 eclash 證明驅動 (法則 0 / P-G1); 無幾何交點則標在首個重疊點
    marks = []
    vis = {(a["L"], a["T"]): a for a in arcs}
    for a, b, inter in eclash_pairs(pr, dl, errors):
        va, vb = vis.get((a["L"], a["T"])), vis.get((b["L"], b["T"]))
        hit = None
        if va and vb:
            for s1 in _sample(va["pts"]):
                for s2 in _sample(vb["pts"]):
                    cand = _seg_hit(s1[0], s1[1], s2[0], s2[1])
                    if cand:
                        hit = cand
                        break
                if hit:
                    break
        if hit is None:
            first = min(inter, key=lambda s: idx[s]) if inter else a["L"]
            hit = (AX + 72, y(idx[first]))
        if all(abs(hit[0] - m[0]) + abs(hit[1] - m[1]) > 8 for m in marks):
            marks.append(hit)
    for (mx, my) in marks:
        svg.append('<circle cx="%.0f" cy="%.0f" r="7" fill="none" stroke="#fbbf24" stroke-width="2"/>' % (mx, my))
        svg.append('<text x="%.0f" y="%.0f" fill="#fbbf24" font-size="12" text-anchor="middle">✕</text>' % (mx, my + 4))

    # E18/E24 maybe-live (RustOwl 波浪): 消耗點 → join 後使用, 虛線琥珀
    for code, sid, _t in fail:
        if code not in ("E18", "E24") or sid not in idx:
            continue
        moves = [t[0] for pred, t in pr.facts
                 if pred == "move" and t[0] in idx and idx[t[0]] < idx[sid]]
        if not moves:
            continue
        mv = max(moves, key=lambda s: idx[s])
        y0, y1 = y(idx[mv]), y(idx[sid])
        xl = AX - 52
        svg.append('<path d="M %d %d C %d %d, %d %d, %d %d" fill="none" stroke="#f59e0b" stroke-width="1.8" stroke-dasharray="3 4"/>' % (
            AX - 10, y0, xl, y0, xl, y1, AX - 10, y1))
        svg.append('<circle cx="%d" cy="%d" r="8" fill="none" stroke="#f59e0b" stroke-width="1.5" stroke-dasharray="3 3"/>' % (AX, y1))
        svg.append('<text x="%d" y="%d" fill="#f59e0b" font-size="10" text-anchor="end">可能活 %s</text>' % (xl - 4, (y0 + y1) / 2 + 3, code))

    # 逸出箭頭 (弧逸出作用域圓): E10/E16/E28/E04
    for code, sid, _t in fail:
        if code not in ("E10", "E16", "E28", "E04"):
            continue
        yy = y(idx[sid])
        top = yy - 40
        for fn in [s for s in pr.scopes if s["kind"] == "fn" and sid in s.get("all_stmts", [])]:
            ys = [y(idx[s]) for s in fn["all_stmts"]]
            cy = (min(ys) + max(ys)) / 2
            ry = (max(ys) - min(ys)) / 2 + 34
            top = cy - ry - 22
            break
        svg.append('<line x1="%d" y1="%d" x2="%d" y2="%.0f" stroke="#ef4444" stroke-width="2" stroke-dasharray="5 4"/>' % (AX, yy - 12, AX, top + 12))
        svg.append('<polygon points="%d,%.0f %d,%.0f %d,%.0f" fill="#ef4444"/>' % (AX, top, AX - 5, top + 12, AX + 5, top + 12))
        svg.append('<text x="%d" y="%.0f" fill="#fca5a5" font-size="11">→ 呼叫者 (弧逸出作用域圓)</text>' % (AX + 10, top + 34))

    # 錯誤註記 (右側欄)
    ey = 62
    for code, sid, _t in fail:
        msg = CODE2MSG[code]
        svg.append('<text x="486" y="%d" fill="#fca5a5" font-size="11">%s @ %s  %s</text>' % (ey, esc(code), esc(sid), esc(msg[:38])))
        ey += 17

    # 圖例 / 法則
    ly = Y0 + DY * (n - 1) + 56
    svg.append('<line x1="24" y1="%d" x2="64" y2="%d" stroke="#3b82f6" stroke-width="2"/>' % (ly, ly))
    svg.append('<text x="70" y="%d" fill="#94a3b8" font-size="11">sh 共享借用弧 (可共存)</text>' % (ly + 4))
    svg.append('<line x1="240" y1="%d" x2="280" y2="%d" stroke="#ef4444" stroke-width="3.5"/>' % (ly, ly))
    svg.append('<text x="286" y="%d" fill="#94a3b8" font-size="11">mut 排他借用弧 (紅弧)</text>' % (ly + 4))
    svg.append('<ellipse cx="470" cy="%d" rx="24" ry="10" fill="none" stroke="#64748b" stroke-dasharray="5 4"/>' % (ly))
    svg.append('<text x="500" y="%d" fill="#94a3b8" font-size="11">作用域圓 (區域; 巢狀 = 同心圓)</text>' % (ly + 4))
    svg.append('<text x="24" y="%d" fill="#e2e8f0" font-size="12">圓示法則 ① 紅弧孤立: mut 弧之弧跨內不得含他弧端點, 且不得與他弧弦交越 (交越處標 ✕)</text>' % (ly + 26))
    svg.append('<text x="24" y="%d" fill="#e2e8f0" font-size="12">圓示法則 ② 弧在圓內: 弧端點須落在被借者之作用域圓內; 回傳者之被借者須活得比呼叫者 (ROOT) 久</text>' % (ly + 46))
    svg.append('<text x="24" y="%d" fill="#e2e8f0" font-size="12">圓示法則 ③ 點序守紀: move/drop 之後的點不得再讀同路徑; 紅弧跨內不得有寫/移/呼叫消耗</text>' % (ly + 66))
    svg.append('<text x="24" y="%d" fill="#94a3b8" font-size="11">虛線弧 = NLL 空洞 (點 ∉ I(a)); 琥珀虛線 = 可能活 (E18/E24); ✕ 僅標 eclash 證明</text>' % (ly + 86))
    svg.append('</svg>')
    return "\n".join(svg)

# ================================================================
# 6. CLI
# ================================================================
def build_result(name, liveness, pr, errors):
    data = {
        "file": name,
        "liveness": liveness,
        "verdict": "PASS" if not errors else "FAIL",
        "errors": [],
        "regions": [],
    }
    dls = getattr(pr, "dls", None)
    fns = {f["id"]: f for f in pr.scopes if f["kind"] == "fn"}
    for code, sid, tup in errors:
        if code == "E00":  # 前端錯誤: tup = (offending_line, message)
            e = {"code": code, "stmt": sid, "stmt_text": tup[0], "message": tup[1]}
        else:
            fn = fns.get(getattr(pr, "fn_of_stmt", {}).get(sid))
            e = {"code": code, "stmt": sid,
                 "fn": fn.get("name") if fn else None,
                 "stmt_text": pr.stmts[pr.idx[sid]][1] if sid in pr.idx else None,
                 "message": CODE2MSG.get(code, "")}
        dl = dls.get(getattr(pr, "fn_of_stmt", {}).get(sid)) if dls else None
        if dl is not None and code in CODE2PRED:
            out = []
            print_proof(dl, CODE2PRED[code], tup, out)
            e["proof"] = out
        data["errors"].append(e)
    if dls:
        for fid, dl in dls.items():
            fn = fns.get(fid)
            for (L, Q, T, K) in sorted(dl.facts.get("lend", ()), key=lambda t: pr.idx[t[0]]):
                ends = [t[2] for t in dl.facts.get("span_end", ()) if t[0] == L and t[1] == T]
                data["regions"].append({
                    "fn": fn.get("name") if fn else None,
                    "ref": T, "referent": Q, "kind": K, "start": L,
                    "end": max(ends, key=lambda s: pr.idx[s]) if ends else None,
                })
    return data

def run_one(path, liveness, outdir, quiet=False):
    with open(path, encoding="utf-8") as f:
        text = f.read()
    pr, dl, errors = check(text, liveness)
    name = os.path.basename(path)
    data = build_result(name, liveness, pr, errors)
    if not quiet:
        print("=" * 74)
        print("檔案: %s   (liveness=%s)" % (name, liveness))
        if pr.errors:
            print("verdict: FAIL (前端)")
            for code, _sid, tup in errors:
                print("  [%s] %s: %s" % (code, tup[0], tup[1]))
        else:
            print("verdict: %s" % ("PASS" if not errors else "FAIL — %d 個錯誤" % len(errors)))
            for code, sid, tup in errors:
                stext = pr.stmts[pr.idx[sid]][1] if sid in pr.idx else "?"
                fn = [f for f in pr.scopes if f["kind"] == "fn" and sid in f.get("all_stmts", [])]
                fnlab = ("  fn %s" % fn[0].get("name")) if fn else ""
                print("  [%s] %s%s  「%s」" % (code, sid, fnlab, stext))
                print("      %s" % CODE2MSG[code])
                dlf = getattr(pr, "dls", {}).get(getattr(pr, "fn_of_stmt", {}).get(sid))
                if dlf is not None:
                    out = []
                    print_proof(dlf, CODE2PRED[code], tup, out)
                    for ln in out[:16]:
                        print("      " + ln)
                    if len(out) > 16:
                        print("      … (證明樹略)")
            if getattr(pr, "dls", None):
                for fid, dlf in pr.dls.items():
                    fn = [f for f in pr.scopes if f["id"] == fid][0]
                    print("  區域%s:" % (("  fn " + str(fn.get("name"))) if fn.get("name") else ""))
                    for (L, Q, T, K) in sorted(dlf.facts.get("lend", ()), key=lambda t: pr.idx[t[0]]):
                        ends = [t[2] for t in dlf.facts.get("span_end", ()) if t[0] == L and t[1] == T]
                        if ends:
                            E = max(ends, key=lambda s: pr.idx[s])
                            print("    %s : %s %s   區間 %s → %s" % (T, "&mut" if K == "mut" else "&", Q, L, E))
                        else:
                            print("    %s : %s %s   (死借貸)" % (T, "&mut" if K == "mut" else "&", Q))
    outpath = os.path.join(outdir, os.path.splitext(name)[0] + ".svg")
    if dl is not None:
        with open(outpath, "w", encoding="utf-8") as f:
            f.write(render_svg(pr, dl, errors, name, liveness))
        if not quiet:
            print("  圓示: %s" % os.path.relpath(outpath, os.getcwd()))
    return name, "PASS" if not errors else "FAIL", errors, data

# ================================================================
# 代理工具: --rules (規格) / --explain (錯誤 → 規則原文+證明+幾何+修法)
# ================================================================
RULE_LEGEND = """\
弦律規則規格 v0.4 (rules.dl + liveness_*.dl 為完整可執行規格; 此為速覽)
錯誤代碼            語義                              ~ rustc
E01 eclash          路徑衝突之兩借用重疊, ≥1 mut      E0499/E0502
E02 ewrite          借用活躍期寫入被借者(含子字段)    E0506
E03 eread           mut 活躍期直接讀被借者(含子字段)  E0503
E04 edangle         使用點落在被借者作用域外          E0597
E05 eadrop          drop 後使用
E06 emove           move 後使用 (整體/字段互涉)       E0382
E07 eloanmove       借用活躍期 move 被借者            E0505
E08 eloandrop       借用活躍期 drop 被借者
E09 erefuse         別名層衝突 (使用點攔截 2-phase)   E0502/E0499
E10 ereturn         回參考之被借者不活得比呼叫者久    E0106/E0515
E11 eimmut          寫入不可變地方                    E0384
E12 enotmut         對不可變地方作 mut 出借           E0596
E13 eassignsh       經共享參考寫入                    E0594
E14 emoveout        經參考移出                        E0507
E15 etemp           暫存借用被非緊鄰陳述使用          E0716
E16 eescape         把指向局部的參考存入槽            E0521
E17 estoretemp      把指向暫存的參考存入槽            E0716
E18 ebranchmove     if 內 move 後於分支外使用         E0382
E19 edoubledrop     重複 drop
E20 edropmoved      move 後再 drop                    E0382
E21 ealiascyc       存入造成別名環
E22 eimmfield       寫入不可變字段                    E0594
E23 enotmutfield    對不可變字段作 mut 出借           E0596
E24 eelsejoin       else 內 move 後於分支外使用       E0382
E25 ecallmut        呼叫讀取 mut 借用活躍之路徑       E0503
E26 ecallmove       呼叫消耗後再使用                  E0382
E27 ecallloan       借用活躍期呼叫消耗被借者          E0505
E28 erettemp        回傳指向暫存的參考                E0515
E29 euninit         使用尚未初始化的洞                E0381
E30 euninitret      移出尚未初始化的洞                E0381
E31 eparamimmut     寫入不可變參數                    E0594
E32 eparamnotmut    對不可變參數作 mut 出借           E0596
活度等級  --liveness: nll (預設, 參考終端使用+別名閉包) | referent (被借者終端使用) | lexical (作用域終端)
"""

FIX_HINTS = {
    "E01": "縮短其中一條弧: 將先出借者的「最終使用」移到後出借者出借點之前 (依序化), 或 clone 所需值",
    "E02": "將寫入移到該借用最終使用之前 (縮短弧), 或改用該引用寫入",
    "E03": "改經引用存取 (用 use <ref> 取代直接讀), 或將讀取移到 mut 借用最終使用之後",
    "E04": "將使用移進被借者作用域內 (弧端點须在圓內)",
    "E05": "先使用再 drop, 或使用前重新初始化該值",
    "E06": "先使用再 move, 或 clone/copy 該值 (寫子字段需先重新初始化整體)",
    "E07": "將 move 移到該借用最終使用之後 (縮短弧)",
    "E08": "將 drop 移到該借用最終使用之後",
    "E09": "拆成兩句: 先終結對該引用的借用 (其最終使用), 再排他使用該引用 (2-phase 於陳述粒度不可表達)",
    "E10": "讓被借者來自參數 (呼叫者所有), 或回傳擁有值 (clone)",
    "E11": "把地方改為可變 (let x 而非 let imm x), 或不要寫入",
    "E12": "改用共享借用 (&), 或把地方改為可變",
    "E13": "把參考改為 &mut, 或不要經共享參考寫入",
    "E14": "改為 clone/copy, 或先 mem::replace 再移出",
    "E15": "把暫存綁進具名地方 (let t), 或把使用緊貼出借句",
    "E16": "只存入指向參數/呼叫者的參考, 或改存擁有值",
    "E17": "先把暫存綁進具名地方再出借",
    "E18": "兩個分支都 move 並在分支內用完, 或分支外先重初始化",
    "E19": "每個值只 drop 一次; 第二次前先 set 重初始化",
    "E20": "move 後不要再 drop; 需要時先 set 重初始化",
    "E21": "不要把指向槽自身的參考存回槽 (自引用結構)",
    "E22": "把整體改為可變, 或不要寫子字段",
    "E23": "對字段改用共享借用, 或把整體改為可變",
    "E24": "與 E18 相同: 分支外使用前重初始化, 或把使用留在分支內",
    "E25": "先結束 mut 借用再呼叫, 或改經該 mut 參考傳遞",
    "E26": "呼叫前 clone, 或呼叫後 set 重初始化再使用",
    "E27": "先結束借用再把值移入呼叫",
    "E28": "回傳擁有值, 或讓被借者來自參數",
    "E29": "使用前先 set 初始化該洞",
    "E30": "移出前先 set 初始化該洞",
    "E31": "把參數改為可變 (去掉 imm), 或不要寫入參數",
    "E32": "對參數改用共享借用, 或把參數改為可變",
}

def _rule_src(dl, pred):
    for r in dl.rules:
        if r["head"][0] == pred and r.get("src"):
            return r["src"]
    return None

def _geometry_hint(dl, sid, idx):
    """該點落在哪些弧跨內 (幾何事實)"""
    out = []
    lends = dl.facts.get("lend", ())
    for (L, Q, T, K) in sorted(lends, key=lambda t: idx[t[0]]):
        if (L, sid) in dl.facts.get("onregion", ()):
            ends = [t[2] for t in dl.facts.get("span_end", ()) if t[0] == L and t[1] == T]
            E = max(ends, key=lambda s: idx[s]) if ends else "?"
            out.append("%s (:%s) %s→%s" % (T, "mut" if K == "mut" else "sh", L, E))
    return out

def explain_file(path, liveness):
    with open(path, encoding="utf-8") as f:
        text = f.read()
    pr, dl, errors = check(text, liveness)
    name = os.path.basename(path)
    print("=" * 74)
    print("explain: %s  (liveness=%s)  verdict: %s" %
          (name, liveness, "PASS" if not errors else "FAIL — %d 錯誤" % len(errors)))
    if pr.errors:
        for code, _sid, tup in errors:
            print("  [%s] %s: %s" % (code, tup[0], tup[1]))
        return
    dls = getattr(pr, "dls", {})
    fns = {f["id"]: f for f in pr.scopes if f["kind"] == "fn"}
    for code, sid, tup in errors:
        fn = fns.get(getattr(pr, "fn_of_stmt", {}).get(sid))
        stext = pr.stmts[pr.idx[sid]][1]
        dlf = dls.get(getattr(pr, "fn_of_stmt", {}).get(sid))
        print()
        print("[%s] %s  fn %s  「%s」" % (code, sid, fn.get("name") if fn else "?", stext))
        print("  語義: %s" % CODE2MSG.get(code, ""))
        src = _rule_src(dlf, CODE2PRED[code]) if dlf is not None else None
        if src:
            print("  規則: %s" % src)
        if dlf is not None:
            out = []
            print_proof(dlf, CODE2PRED[code], tup, out)
            print("  證明:")
            for ln in out:
                print("    " + ln)
            geo = _geometry_hint(dlf, sid, pr.idx)
            if geo:
                print("  幾何: 點 %s 落在弧跨內: %s" % (sid, "; ".join(geo)))
            law, glines = geometry_report(pr, dlf, code, sid)
            if glines:
                print("  幾何違反: %s" % glines[0])
                for ln in glines[1:]:
                    print(ln)
        print("  修法: %s" % FIX_HINTS.get(code, ""))

def main():
    import json
    args = sys.argv[1:]
    if "--mcp" in args:
        import mcp_server
        mcp_server.serve_stdio()
        return
    if "--factory" in args or "--from-rs" in args:
        import ahpbb
        # 把本行程式參數交給工廠 (去掉 chordlaw 自身旗標重複)
        argv = [a for a in sys.argv[1:] if a != "--factory"]
        if "--factory" in sys.argv[1:] and "--batch" not in argv and "--from-rs" not in argv and "--from-cl" not in argv:
            argv = ["--batch"] + argv
        raise SystemExit(ahpbb.main(argv))
    liveness = "nll"
    if "--liveness" in args:
        liveness = args[args.index("--liveness") + 1]
        args = args[:args.index("--liveness")] + args[args.index("--liveness") + 2:]
    out_md = None
    if "-o" in args:
        i = args.index("-o")
        out_md = args[i + 1]
        args = args[:i] + args[i + 2:]
    as_json = "--json" in args
    if as_json:
        args = [a for a in args if a != "--json"]
    if "--rules" in args:
        print(RULE_LEGEND)
        for fn in ["rules.dl", "liveness_nll.dl", "liveness_referent.dl", "liveness_lexical.dl"]:
            p = os.path.join(HERE, fn)
            if os.path.exists(p):
                print()
                print("── %s ──" % fn)
                print(open(p, encoding="utf-8").read().rstrip())
        return
    as_explain = "--explain" in args
    if as_explain:
        args = [a for a in args if a != "--explain"]
    import helper as H
    if "--history" in args:
        rest = [a for a in args if a != "--history"]
        root = rest[0] if rest else HERE
        print(json.dumps(H.history_summary(root), ensure_ascii=False, indent=2))
        return
    if "--report" in args:
        rest = [a for a in args if a != "--report"]
        root = rest[0] if rest else HERE
        sess = H.run_project(root, liveness, persist=True)
        md = H.render_report(sess)
        dest = out_md or os.path.join(root, "reports", "latest.md")
        H.write_report(sess, dest)
        if as_json:
            slim = dict(sess)
            slim.pop("file_results", None)
            slim["report_path"] = os.path.abspath(dest)
            print(json.dumps(slim, ensure_ascii=False, indent=2))
        else:
            print(md)
            print("報告: %s" % dest)
            if sess.get("state"):
                print("狀態: %s" % sess["state"])
        return
    if "--check" in args:
        rest = [a for a in args if a != "--check"]
        root = rest[0] if rest else HERE
        sess = H.run_project(root, liveness, persist=True)
        if as_json:
            slim = dict(sess)
            slim.pop("file_results", None)
            print(json.dumps(slim, ensure_ascii=False, indent=2))
            return
        sc = sess["score"]
        print("check %s  overall=%d (%s)  PASS %d/%d  錯誤 %d" % (
            root, sc["overall"], sc["band"],
            sc["counts"]["pass"], sc["counts"]["checked"], sc["counts"]["errors"]))
        for a in (sess.get("advice") or [])[:20]:
            print("  [%s] %s %s@%s  %s — %s" % (
                a.get("priority"), a.get("op"), a.get("code"),
                a.get("stmt"), a.get("file"), (a.get("detail") or "")[:80]))
        if sess.get("state"):
            print("狀態: %s" % sess["state"])
        return
    if liveness not in LIVENESS_MODES:
        print("錯誤: 未知 liveness %r (可用: %s)" % (liveness, ", ".join(LIVENESS_MODES)))
        sys.exit(2)
    if not args or args == ["--all"]:
        files = sorted(glob.glob(os.path.join(HERE, "examples", "*.cl")))
    else:
        files = []
        for a in args:
            files += sorted(glob.glob(a)) if any(c in a for c in "*?[") else [a]
    outdir = os.path.join(os.path.dirname(files[0]), "out") if files else os.path.join(HERE, "out")
    os.makedirs(outdir, exist_ok=True)
    rows, datas = [], []
    for f in files:
        if as_explain:
            explain_file(f, liveness)
            continue
        name, verdict, errs, data = run_one(f, liveness, outdir, quiet=as_json)
        rows.append((name, verdict, "; ".join("%s@%s" % (e[0], e[1]) for e in errs) or "—"))
        datas.append(data)
    if as_json:
        print(json.dumps(datas if len(datas) > 1 else datas[0], ensure_ascii=False, indent=2))
        return
    if len(rows) > 1:
        print()
        print("%-24s %-7s %s" % ("範例", "判決", "錯誤"))
        for r in rows:
            print("%-24s %-7s %s" % r)

if __name__ == "__main__":
    main()
