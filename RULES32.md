# 弦律 ChordLaw v0.4 — 32 則規則目錄

> **Datalog 規則 × DAG 拓撲 × 圓示（縱點節圖）**
> 每則：謂詞、Datalog 本體、DAG 落點、幾何化身、反例、修法。
> 可執行規格：[`rules.dl`](rules.dl)。圓示畫廊：[`gallery.html`](gallery.html)。

**健全性合約 S**：弦律接受集 ⊆ rustc 接受集（寧可多拒、不可錯放）。
活度預設 `nll`（`reach(lend,Q) ∧ reach(Q, span_end)` = NLL 區域）。

---

## 三條幾何法則（圓示的真值）

| 法則 | 幾何 | 對應規則 |
|---|---|---|
| ① **紅弧孤立** | mut 弧之弧跨內不得含他弧端點，不得與他弧弦交越（交越標 ✕） | E01 / E02 / E03 / E09 / E25 |
| ② **弧在圓內** | 弧端點須落在被借者作用域圓內；回傳／存槽之被借者須活得比 ROOT 久 | E04 / E10 / E16 / E17 / E28 |
| ③ **點序守紀** | move/drop 之後的點不得再讀同路徑；紅弧跨內不得有寫／移／呼叫消耗 | E05–E08 / E18–E20 / E26–E27 / E29–E30 |

縱點節圖 `(V, P, A, C)`：`P` 點＝陳述（時間向下）、`A` 弧＝借用、`C` 圓＝作用域（巢狀＝同心）。

---

## E01–E10　核心四不變式（v0.3，行為不變）

### E01 `eclash`　紅弧交越　~ rustc E0499 / E0502

```datalog
eclash(S) :- lend(A,P1,_,mut), lend(S,P2,_,_), after(A,S), overlap(A,S,P1).
eclash(S) :- lend(A,P1,_,_),   lend(S,P2,_,mut), after(A,S), overlap(A,S,P1).
```

- **DAG**：後出借點落在先出借之 `onregion`（`reach` 路徑上）。
- **幾何**：紅弧與他弧弦交越 ✕。
- **反例**：[`r01_eclash.cl`](examples/r01_eclash.cl) → E01@s4
- **修法**：依序化（先弧終端使用移到後弧出借之前），或 clone。

### E02 `ewrite`　弧跨內寫入　~ E0506

```datalog
ewrite(S) :- write(S,P), onregion(L,S), lend(L,R,_,_), path_conflict(R,P).
```

- **幾何**：寫入點落在弧跨內。
- **反例**：[`r02_ewrite.cl`](examples/r02_ewrite.cl) → E02@s3

### E03 `eread`　紅弧跨內直讀　~ E0503

```datalog
eread(S) :- read(S,P), onregion(L,S), lend(L,R,_,mut), path_conflict(R,P).
```

- **反例**：[`r03_eread.cl`](examples/r03_eread.cl) → E03@s4
- **修法**：改 `use <ref>`，不要直接讀被借者。

### E04 `edangle`　弧端點掉出圓　~ E0597

```datalog
edangle(S) :- usep(S,P), stmt_of(S,N), scope_of(P,M), !inside(N,M).
```

- **DAG**：使用點之作用域不是被借者作用域的子孫。
- **反例**：[`r04_edangle.cl`](examples/r04_edangle.cl) → E04@s5（經槽讀已結束區塊的 `y`；同句 E16）

### E05 `eadrop`　drop 後讀

```datalog
eadrop(S) :- readp(S,P1), drop(D,P2), after(D,S), path_conflict(P1,P2), !reinited(D,S,P2).
```

- **反例**：[`r05_eadrop.cl`](examples/r05_eadrop.cl) → E05@s3

### E06 `emove`　move 後用　~ E0382

```datalog
emove(S) :- readp(S,P1), move(D,P2), after(D,S), path_conflict(P1,P2), !reinited(D,S,P2).
emove(S) :- write(S,P1), move(D,P2), after(D,S), path_conflict(P1,P2), !reinited(D,S,P2).
```

- **反例**：[`r06_emove.cl`](examples/r06_emove.cl) → E06@s3
- 整體／字段互涉；`set` 覆蓋寫入可豁免（`reinited`）。

### E07 `eloanmove`　弧跨內 move　~ E0505

```datalog
eloanmove(S) :- move(S,P), onregion(L,S), lend(L,R,_,_), path_conflict(R,P).
```

- **反例**：[`r07_eloanmove.cl`](examples/r07_eloanmove.cl) → E07@s3（＋E06@s4）

### E08 `eloandrop`　弧跨內 drop

```datalog
eloandrop(S) :- drop(S,P), onregion(L,S), lend(L,R,_,_), path_conflict(R,P).
```

- **反例**：[`r08_eloandrop.cl`](examples/r08_eloandrop.cl) → E08@s3（＋E05@s4）

### E09 `erefuse`　別名層衝突　~ E0502 / E0499

```datalog
erefuse(S) :- use_ref(S,T), onregion(M,S), lend(M,T,_,mut).
erefuse(S) :- use_ref(S,T), lend(L,_,T,mut), onregion(M,S), lend(M,T,_,_), neq(M,L).
```

- **幾何**：紅弧跨內排他使用參考自身（2-phase 於陳述粒度不可表達 → 使用點攔截）。
- **反例**：[`r09_erefuse.cl`](examples/r09_erefuse.cl) → E09@s4

### E10 `ereturn`　弧逸出 ROOT 圓　~ E0106 / E0515

```datalog
ereturn(S) :- ret(S,T), ref_of(T,P), scope_of(P,M), !same(M,n0), !temp(P).
```

- **反例**：[`r10_ereturn.cl`](examples/r10_ereturn.cl) → E10@s3
- 暫存回傳改由 E28 專責。

---

## E11–E32　v0.4 擴充（新事實觸發，不擾 E01–E10 語料）

### E11 `eimmut`　寫不可變地方　~ E0384

```datalog
eimmut(S) :- write(S,P), imm(P), !param(P).
```

- **前端**：`let imm x`
- **反例**：[`r11_eimmut.cl`](examples/r11_eimmut.cl) → E11@s2
- **對照**：`let a = &x; use a` 合法（共享讀）。

### E12 `enotmut`　對不可變作 mut 出借　~ E0596

```datalog
enotmut(S) :- lend(S,P,_,mut), imm(P), !param(P).
```

- **反例**：[`r12_enotmut.cl`](examples/r12_enotmut.cl) → E12@s2

### E13 `eassignsh`　經 `&` 寫入　~ E0594

```datalog
eassignsh(S) :- deref_write(S,T), lend(_,_,T,sh).
```

- **前端**：`set *a`
- **反例**：[`r13_eassignsh.cl`](examples/r13_eassignsh.cl) → E13@s3

### E14 `emoveout`　經參考移出　~ E0507

```datalog
emoveout(S) :- deref_move(S,T), borrow_of(T,_).
```

- **前端**：`mv *a`
- **反例**：[`r14_emoveout.cl`](examples/r14_emoveout.cl) → E14@s3
- 保守：不論 sh/mut、不論 Copy，一律拒（合約 S）。

### E15 `etemp`　暫存不過句　~ E0716 保守

```datalog
etemp(S) :- temp(P), lend(L,P,T,_), use_ref(S,T), after(L,S), !edge(L,S).
```

- **DAG**：暫存出借只允許**緊鄰後繼邊**上的使用。
- **前端**：`tmp t`
- **反例**：[`r15_etemp.cl`](examples/r15_etemp.cl) → E15@s4
- **對照**：`tmp t; let a = &t; use a` 合法。

### E16 `eescape`　存局部參考入槽　~ E0521

```datalog
eescape(S) :- store(S,Slot,T), ref_of(T,P), scope_of(P,M), !same(M,n0), neq(P,Slot).
```

- **幾何**：把短命弧寫進活得更久的槽＝弧逸出圓。
- **反例**：[`r16_eescape.cl`](examples/r16_eescape.cl) → E16@s4
- 只允許存入指向 ROOT（參數／呼叫者）的參考。

### E17 `estoretemp`　存暫存參考入槽　~ E0716

```datalog
estoretemp(S) :- store(S,Slot,T), ref_of(T,P), temp(P).
```

- **反例**：[`r17_estoretemp.cl`](examples/r17_estoretemp.cl) → E17@s4（＋E16）

### E18 `ebranchmove`　if 內 move，join 後用　~ E0382

```datalog
ebranchmove(S) :- move(D,P), readp(S,P1), after(D,S), path_conflict(P,P1),
                  !reinited(D,S,P), stmt_of(D,N), if_scope(I), inside(N,I),
                  stmt_of(S,M), !inside(M,I).
```

- **DAG**：if 是純前向子圖；`after` 沿 `if.last → succ` 走出分支。
- **反例**：[`r18_ebranchmove.cl`](examples/r18_ebranchmove.cl) → E18@s3（＋E06）

### E19 `edoubledrop`　重複 drop

```datalog
edoubledrop(S) :- drop(D,P1), drop(S,P2), after(D,S), path_conflict(P1,P2), !reinited(D,S,P1).
```

- **反例**：[`r19_edoubledrop.cl`](examples/r19_edoubledrop.cl) → E19@s3

### E20 `edropmoved`　move 後再 drop　~ E0382

```datalog
edropmoved(S) :- move(D,P1), drop(S,P2), after(D,S), path_conflict(P1,P2), !reinited(D,S,P1).
```

- **反例**：[`r20_edropmoved.cl`](examples/r20_edropmoved.cl) → E20@s3

### E21 `ealiascyc`　存入造成別名環

```datalog
ealiascyc(S) :- store(S,Slot,T), ref_of(T,Q), path_conflict(Q,Slot).
```

- **DAG**：別名圖出現 `Slot → … → Slot`。
- **反例**：[`r21_ealiascyc.cl`](examples/r21_ealiascyc.cl) → E21@s3

### E22 `eimmfield`　寫不可變字段　~ E0594

```datalog
eimmfield(S) :- write(S,P), imm(R), !param(R), subpath(P,R).
```

- **反例**：[`r22_eimmfield.cl`](examples/r22_eimmfield.cl) → E22@s2

### E23 `enotmutfield`　對不可變字段 mut 出借　~ E0596

```datalog
enotmutfield(S) :- lend(S,P,_,mut), imm(R), !param(R), subpath(P,R).
```

- **反例**：[`r23_enotmutfield.cl`](examples/r23_enotmutfield.cl) → E23@s2

### E24 `eelsejoin`　else 內 move，join 後用　~ E0382

```datalog
eelsejoin(S) :- move(D,P), readp(S,P1), after(D,S), path_conflict(P,P1),
                !reinited(D,S,P), stmt_of(D,N), else_scope(I), inside(N,I),
                stmt_of(S,M), !inside(M,I).
```

- **DAG**：if/else 菱形——`pred → if.first`、`pred → else.first`；`if.last → succ`、`else.last → succ`。無 sequential 邊跨越兄弟。
- **反例**：[`r24_eelsejoin.cl`](examples/r24_eelsejoin.cl) → E24@s4（＋E06）
- **對照**：兩支都 move 後 `set x; use x` 合法（`reinited`）。

### E25 `ecallmut`　呼叫讀 mut 活躍路徑　~ E0503

```datalog
ecallmut(S) :- call(S,P), onregion(L,S), lend(L,R,_,mut), path_conflict(R,P).
```

- **前端**：`call f(x)`
- **反例**：[`r25_ecallmut.cl`](examples/r25_ecallmut.cl) → E25@s3

### E26 `ecallmove`　呼叫消耗後再用　~ E0382

```datalog
ecallmove(S) :- callmv(D,P2), readp(S,P1), after(D,S), path_conflict(P1,P2), !reinited(D,S,P2).
```

- **前端**：`callmv take(x)`（不發 `move` 事實，以免與 E06 雙報）
- **反例**：[`r26_ecallmove.cl`](examples/r26_ecallmove.cl) → E26@s3

### E27 `ecalloan`　弧跨內呼叫消耗　~ E0505

```datalog
ecalloan(S) :- callmv(S,P), onregion(L,S), lend(L,R,_,_), path_conflict(R,P).
```

- **反例**：[`r27_ecalloan.cl`](examples/r27_ecalloan.cl) → E27@s3（＋E26@s4）

### E28 `erettemp`　回傳暫存參考　~ E0515

```datalog
erettemp(S) :- ret(S,T), ref_of(T,P), temp(P).
```

- **反例**：[`r28_erettemp.cl`](examples/r28_erettemp.cl) → E28@s3

### E29 `euninit`　讀未初始化洞　~ E0381

```datalog
euninit(S) :- hole(P), readp(S,P1), path_conflict(P,P1), !inited(S,P).
```

- **前端**：`let hole x`
- **反例**：[`r29_euninit.cl`](examples/r29_euninit.cl) → E29@s2
- **對照**：`let hole x; set x; use x` 合法。

### E30 `euninitret`　移出未初始化洞　~ E0381

```datalog
euninitret(S) :- hole(P), move(S,P1), path_conflict(P,P1), !inited(S,P).
```

- **反例**：[`r30_euninitret.cl`](examples/r30_euninitret.cl) → E30@s2

### E31 `eparamimmut`　寫不可變參數　~ E0594

```datalog
eparamimmut(S) :- write(S,P), imm(R), param(R), path_conflict(R,P).
```

- **前端**：`fn f(imm p)`
- **反例**：[`r31_eparamimmut.cl`](examples/r31_eparamimmut.cl) → E31@s1

### E32 `eparamnotmut`　對不可變參數 mut 出借　~ E0596

```datalog
eparamnotmut(S) :- lend(S,P,_,mut), imm(R), param(R), path_conflict(R,P).
```

- **反例**：[`r32_eparamnotmut.cl`](examples/r32_eparamnotmut.cl) → E32@s1
- **對照**：`fn f(imm p) { let a = &p; use a }` 合法。

---

## 結構謂詞（非錯誤，承載語義）

| 謂詞 | 定義 | 角色 |
|---|---|---|
| `reach` / `after` | CFG 達達性（含迴圈後向邊） | NLL fixpoint 替身 |
| `anc` / `inside` | 作用域樹祖孫 | 區域包含 |
| `ref_of` | `borrow_of` 遞推 | 別名鏈 |
| `onregion(L,Q)` | `reach(L,Q) ∧ reach(Q, span_end)` | 設計定理落點 |
| `overlap` | 路徑衝突＋出借點落他區 | E01 |
| `reinited` / `inited` | 覆蓋寫入 | E05/E06/E18/E29 豁免 |
| `if_scope` / `else_scope` | 分支作用域標記 | E18 / E24 菱形 |

---

## 前端新構造 → 事實

| 語法 | 事實 | 觸發 |
|---|---|---|
| `let imm x` | `imm(x)` | E11/E12/E22/E23 |
| `fn f(imm p)` | `imm(p), param(p)` | E31/E32 |
| `tmp t` | `temp(t)` | E15/E17/E28 |
| `let hole x` | `hole(x)` | E29/E30 |
| `slot s` | 地方（可 `store`） | E16/E17/E21 |
| `set *a` | `deref_write` | E13 |
| `mv *a` | `deref_move` | E14 |
| `store s = a` | `store` | E16/E17/E21 |
| `call f(x)` | `call` | E25 |
| `callmv f(x)` | `callmv` | E26/E27 |
| `else { }` | `else_scope` + 菱形邊 | E24 |

---

## 驗證

```bash
python3 test_chordlaw.py          # 46 項：v0.3 回歸 + v0.4 32 則
python3 chordlaw.py examples/r01_eclash.cl
python3 chordlaw.py --explain examples/r18_ebranchmove.cl
```

v0.3 的 19 項回歸與 17 個舊範例判定**不變**（新規則只靠新事實點火）。
