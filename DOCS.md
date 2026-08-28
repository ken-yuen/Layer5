# 弦律 ChordLaw — 專案說明文件（DOCS.md）

> 版本 v0.7 · 2026-08-27 · 狀態：**可運行原型，32 則規則 + MCP + AHPBB 工廠（syn 子集）**
> 92+9+12 項回歸測試全綠 · rustc oracle 26/26 · 見 [MCP.md](MCP.md) / [AHPBB.md](AHPBB.md)

**配套文件**：
- [README.md](README.md) — 快速上手（運行/測試/檔案總覽）
- [PLAN.md](PLAN.md) — 完整開發計畫、設計論證、成功論證、roadmap
- [DOCS.md](DOCS.md)（本檔）— **專案全面說明：規格、架構、規則參考、接口、測試、限制**

---

## 目錄

1. [專案概覽](#1-專案概覽)
2. [目錄結構](#2-目錄結構)
3. [快速開始](#3-快速開始)
4. [.cl 玩具語言完整規格](#4-cl-玩具語言完整規格)
5. [引擎架構](#5-引擎架構)
6. [規則完整參考](#6-規則完整參考)
7. [理論基礎](#7-理論基礎)
8. [圓示（縱點節圖）規格](#8-圓示縱點節圖規格)
9. [輸出與機器接口](#9-輸出與機器接口)
10. [測試與驗證體系](#10-測試與驗證體系)
11. [限制與邊界](#11-限制與邊界)
12. [變更記錄](#12-變更記錄)
13. [參考](#13-參考)

---

## 1. 專案概覽

**弦律 ChordLaw** 是一個仿照 Rust 借用/生命週期規則、但複雜度大幅簡化的檢查器。它**不取代 `rustc`**，而是作為 **AI 代理開發 Rust 時的高頻、局部、可視化內層反饋環**：

> 把「**迭代 fixpoint 推導**（NLL/Polonius 的區域推斷）」重構成「**有向無環圖上的達達性閉包 ＋ Datalog 單調規則 ＋ 圓示幾何視覺**」。
> 規則可讀（Datalog 檔）、反例可證（證明樹）、圖形可看（圓示）、`rustc` 為最終門。

### 設計四刀（複雜度簡化）

| # | 重構 | 從 | 到 |
|---|---|---|---|
| 1 | 區域推斷 → DAG 達達性閉包 | 迭代 fixpoint dataflow | 單遍 Datalog 遞推（`reach`） |
| 2 | 活度分級 | NLL 全域活度 | 三個可切換等級：`nll` / `referent` / `lexical`（已全數實作） |
| 3 | 裁除罕用/高危機制 | 2-phase、Deref、closure、unsafe | 禁止（E09/E00 明示）或延後 |
| 4 | 函數內局部化 | 全域/跨程序區域 | 每 fn 獨立檢查（多 fn 檔案支援） |

### 健全性合約（核心主張）

> **合約 S（無虛假放行 / no false pass）**：凡弦律**接受**的程式，`rustc` 也接受。
> 弦律接受集 ⊆ rustc 接受集（弦律是 rustc 的**保守精化**）。

- 對「提升代理成功率」而言，虛假放行致命、保守拒絕可接受（代理多重構一次，rustc 仍是最終門）。
- 合約 S 由 `oracle_check.py`（以真實 rustc 為 oracle 的差異測試）持續守衛。**現況：26/26 一致，0 虛假放行。**

### 四大可視化支柱

1. **Datalog**：規則即代碼（~100 行可讀規則檔）、終止性是語言性質、每個事實自帶證明樹。
2. **圖形拓撲**：控制流 DAG＋作用域樹＋借用弧三圖緊耦合；拓撲即語義。
3. **DAG**：達達性只在切掉後向邊的 DAG 上定義；迴圈後向邊是語義標記。
4. **圓示（縱點節圖）**：點=陳述（時間向下）、弧=借用、圓=作用域；違規 = 一眼可讀的幾何矛盾。

---

## 2. 目錄結構

```
chordlaw/
├── chordlaw.py              # 主程式（純標準庫）: Datalog 引擎 + .cl 前端 + 圓示 SVG + CLI
├── rules.dl                 # 核心規則 v0.3（E01–E10 + 結構謂詞，81 行，含註解即完整規格）
├── liveness_nll.dl          # 活度等級: nll（預設）— 參考終端使用 + 別名鏈閉包
├── liveness_referent.dl     # 活度等級: referent（保守）— 被借者終端使用
├── liveness_lexical.dl      # 活度等級: lexical（最保守）— 作用域終端
├── examples/                # 17 個範例（.cl）
│   ├── ex1_clash.cl … ex16_two_fn.cl
│   └── out/*.svg            # 每範例的圓示（每次運行自動再生）
├── test_chordlaw.py         # 回歸測試（19 項）
├── oracle_check.py          # rustc 差異測試（26 例，需 rustc）
├── DOCS.md                  # 本檔
├── README.md                # 快速上手
└── PLAN.md                  # 計畫 + 設計論證 + roadmap
```

**依賴**：Python 3（純標準庫，零外部依賴）。oracle 測試需要 `rustc`（自動偵測 `~/.cargo/bin/rustc` 或 PATH）。

---

## 3. 快速開始

```bash
cd chordlaw

# 運行
python3 chordlaw.py                                  # 全部 17 範例（liveness=nll）
python3 chordlaw.py examples/ex1_clash.cl            # 單檔
python3 chordlaw.py --liveness referent FILE         # 保守級
python3 chordlaw.py --liveness lexical FILE          # 最保守級（作用域級）

# 代理接口
python3 chordlaw.py --json FILE                      # 結構化 JSON（verdict/errors+證明樹/regions）
python3 chordlaw.py --explain FILE                   # 錯誤 → 規則原文 + 證明 + 幾何 + 修法
python3 chordlaw.py --rules                          # 規則規格速覽 + 完整規則檔

# 驗證
python3 test_chordlaw.py                             # 19 項回歸測試（期望: OK）
python3 oracle_check.py                              # 26 例 vs 真 rustc（期望: 全部一致）
```

---

## 4. `.cl` 玩具語言完整規格

### 4.1 語法（每行一句；`//` 註解）

| 構造 | 語法 | 說明 |
|---|---|---|
| 函數 | `fn name(args) { ... }` | 參數之範圍 = **ROOT**（`n0`，來自呼叫者）；**每檔可多 fn**（頂層；各自獨立檢查） |
| 區塊 | `{ ... }` | 普通巢狀作用域 |
| 迴圈 | `loop { ... }` | **後向邊**（子樹最後 → 最前）；活度自動閉包環過迴圈 |
| 分支 | `if { ... }` | 單分支（無 else）；**分支內使用不延長活度過分支**（NLL 益處） |
| 宣告 | `let x` | 宣告 place（僅簡單名稱） |
| 共享出借 | `let a = &x` | sh 借用；`x` 可為字段路徑（`x.f`） |
| 排他出借 | `let b = &mut x` | mut 借用 |
| 使用 | `use P` | place → 讀取（`read`）；參考 → 使用（`use_ref`）；`P` 可為字段路徑 |
| 寫入 | `set P` | 寫入（可為字段路徑）；語義含**重新初始化** |
| 移動 | `mv P` | move（可為字段路徑）；消耗該位置 |
| 釋放 | `dp P` | 顯式 drop（可為字段路徑） |
| 回傳 | `ret T` | 參考 → 逸出檢查（E10）；簡單 place 名稱 → move |

**名稱慣例**：
- 簡單名稱：`x`、`a`、`_tmp`（字母/數字/底線，不可數字開頭）。
- 字段路徑：`x.f`、`x.f.g`（根 `x` 必須已宣告為 place/參數）。
- 參考名稱必須是簡單名稱；字段路徑不可作為參考名。

### 4.2 語義規則

1. **作用域**：巢狀層級；內層可見外層名稱；區塊/迴圈/分支/函數各一層。
2. **參數 = ROOT 範圍**：`fn f(p)` 的 `p` 屬於 `n0`（呼叫者所有）→ 回傳 `&p` 合法（ex3b）。
3. **字段路徑（split borrow）**：
   - 借 `x.f` 與 `x.g` 可共存（無路徑衝突）。
   - 借 `x`（整體）與借 `x.f`（子字段）**路徑衝突**（前綴關係）。
   - 對 `x.f` 的讀/寫/使用，視為對 `x`（任何前綴）的**路徑相關存取**（活度與衝突皆按 `path_conflict` 判定）。
   - 整體 move 後：讀/寫任何子字段 = E06（使用被移值）；子字段 move 後：讀整體 = E06。
   - **重新初始化豁免**：move/drop 之後對**該位置或其整體**的 `set`，豁免其後的 E05/E06（`mv x; set x; use x` 合法）。
4. **多 fn**：每 fn 獨立檢查（設計定理為函數內性質）；同名 place 跨 fn 不衝突；錯誤帶 fn 標記。
5. **回傳**：`ret` 一個參考 = 把該參考交給呼叫者 → 其被借者須活得比 ROOT 久，否則 E10。

### 4.3 前端錯誤（E00，結構性，不進 Datalog）

| 情境 | 訊息 |
|---|---|
| 陳述不在任何 fn 內 | `E00 陳述必須在 fn 內` |
| 多餘的 `}`（堆疊空） | `E00 多餘的 } (未對應的閉合)` |
| 未關閉的作用域（檔尾堆疊非空） | `E00 未關閉的作用域` |
| `fn` 非頂層 | `E00 fn 必須在頂層` |
| 檔案無任何 fn | `E00 至少需要一個 fn` |
| `let a = &a`（自引用/同名 shadowing） | `E00 自引用/同名 shadowing 不受支持` |
| 未定義名稱 | `E00 未定義名稱: <name>` |
| 對參考執行 `set`/`mv`/`dp` | `E00 對參考執行 <op>` |
| 經參考取字段（deref coercion） | `E00 經參考取字段 (deref coercion) 不受支持` |
| 參考名含字段點 | `E00 參考名稱須為簡單名稱` |
| 無法解析的行 | `E00 無法解析` |

E00 時該檔判定 FAIL（前端），不進行 Datalog 分析。

---

## 5. 引擎架構

### 5.1 mini Datalog 引擎（`chordlaw.py` §1）

**語言**：事實 `p(a,b).` 與規則 `head(A,B) :- body1(...), body2(...), !neg(...).`（單行一則；`%` 註解）。

**變數慣例（正確性不變式）**：大寫開頭或 `_` 開頭 = **變數**；小寫開頭 = **常數**（故陳述 ID 為 `s1…`、作用域 `n0…`、place 為 `x`/`x.f`）。

**評估（stratified 單調定點）**：
1. **分層**：由規則依賴圖（否定邊權 +1）計算每謂詞的 stratum；同一 stratum 內正遞迴。
2. **逐層定點**：每 stratum 內反復火規則直至不再增長（單調 → 必收斂）。
3. **證明樹（provenance）**：每個推出的事實記錄 `(規則名, 身體事實列表)`（含否定事實的 `¬` 標記）→ 每個錯誤可回溯到具體事實與規則。

**內建謂詞（有限域）**：

| 內建 | 語義 | 用途 |
|---|---|---|
| `neq(A,B)` | `A ≠ B` | 區分不同借貸/陳述 |
| `path_conflict(A,B)` | `A == B`，或一者為另一者之**字段前綴**（`x` vs `x.f`） | 路徑衝突（整體/子字段） |
| `covers(A,B)` | `A == B`，或 `A` 嚴格前綴 `B`（寫 `x` 覆蓋 `x.f`） | 重新初始化判定 |
| `subpath(A,B)` | `A` 嚴格深於 `B` | 字段層級關係 |

**域枚舉**：內建謂詞的**未綁定參數**於有限全域（所有初始事實中的常數）上枚舉——使 `readp(S,P) :- read(S,P2), path_conflict(P,P2)` 這類「由已綁定路徑**產生**新路徑**」的頭部變數可被推導。

### 5.2 前端：`.cl` → 事實集（`chordlaw.py` §2）

| 事實 | 產生 | 說明 |
|---|---|---|
| `stmt(S)` | 每陳述 | 陳述存在（reach 自反性基礎） |
| `stmt_of(S,N)` | 每陳述 | 陳述所在（直接）作用域 |
| `edge(A,B)` | **作用域樹遍歷** | 控制流邊：同作用域相鄰陳述、陳述→子作用域首、子作用域末→下一陳述；**fn 之間、兄弟分支之間無邊**；迴圈追加後向邊（子樹末→首） |
| `parent(N,M)` / `same(N,M)` | 作用域樹 | 巢狀結構（區域包含的基礎） |
| `scope_last(N,S)` | 每有陳述之作用域 | 子樹最後陳述（lexical 等級用） |
| `scope_of(P,N)` | 宣告/參數/參考/路徑 | 名稱 → 作用域；**路徑取根之作用域** |
| `decl(S,P,N)` | `let` | 宣告事件 |
| `lend(S,Q,T,K)` | `let T = &Q` | 出借：S=出借陳述、Q=被借者(路徑)、T=參考名、K=`sh`/`mut` |
| `borrow_of(T,Q)` | 同上 | 別名鏈基礎 |
| `read(S,P)` / `write(S,P)` | `use`/`set`（place） | place 存取（P 可為路徑） |
| `use_ref(S,T)` | `use`/`ret`（參考） | 參考使用 |
| `move(S,P)` / `drop(S,P)` | `mv`/`dp`（`ret` place 亦為 move） | 消耗事件 |
| `ret(S,T)` | `ret`（參考） | 逸出檢查基礎 |

**控制流建構（v0.3 核心重構）**：邊不再假設全局直線，而是**依作用域樹 DFS**：

```
walk(scope):
  依 items 順序（陳述/子作用域交錯，源碼順序）:
    陳述 s: 前驅 → s
    子作用域 X: 前驅 → X.first;  X.last → 後續
迴圈: 追加 edge(X.last, X.first)     # 後向邊
if:   無後向邊                        # 分支 = 純 DAG 拓撲
```

`after` 不再是前端事實，而是規則：`after(A,B) :- reach(A,B), neq(A,B)`（**CFG 嚴格在前**）——使 if 分支活度語義精確（分支內使用不延長活度過分支），無需任何專門規則。

**多 fn 切分**（`check()`）：每 fn 取**其子樹所需之事實**（陳述在子樹內、作用域為子樹成員或 ROOT、borrow 之參考宣告於子樹）→ 獨立 Datalog 執行 → 錯誤合併（sid 全域編號，附 fn 標記）。無跨 fn 干擾。

### 5.3 檢查器（`chordlaw.py` §3）

`check(text, liveness)`:
1. 解析（E00 即止）→ 2. 每 fn 載入規則（`rules.dl` + `liveness_*.dl`）＋事實 → 3. Datalog 定點 → 4. 收集 E01–E10 事實為錯誤 `[(code, sid, tup)]`（按 sid、code 排序）→ 5. 回傳 `(pr, dl, errors)`（`pr.dls` 含每 fn 引擎、`pr.fn_of_stmt` 為 sid→fn 映射）。

---

## 6. 規則完整參考

### 6.1 結構謂詞（承載語義，非錯誤）

| 謂詞 | 定義 | 角色 |
|---|---|---|
| `reach(A,B)` | `edge` 遞推 ＋ `stmt(A)→reach(A,A)` 自反 | **NLL fixpoint 的替身**：控制流達達性 |
| `after(A,B)` | `reach(A,B), neq(A,B)` | 嚴格在前（E05/E06/E01/shadow 的時序判據） |
| `anc(N,M)` / `inside(N,M)` | 作用域樹祖孫/同域 | 區域包含（E04） |
| `ref_of(T,P)` | `borrow_of` 遞推 | 別名鏈（參考 → 最終被借者） |
| `usep(S,P)` | `read`/`write`/`use_ref`（經 `ref_of`）與 `P` **路徑衝突** | 「使用被借者 P」（referent 活度、E04） |
| `readp(S,P)` | 同 `usep` 但**排除寫入** | 讀取性使用（E05/E06 判據；寫入=重新初始化，非「使用」） |
| `onregion(L,Q)` | `lend(L,_,T,_), span_end(L,T,E), reach(L,Q), reach(Q,E)` | **設計定理落點**：借貸 L 於 Q 活躍 |
| `overlap(L1,L2,P1)` | 兩借貸**路徑衝突** ＋ 其一出借點落另一區域 | 重疊（E01） |
| `reinited(D,S,P)` | move/drop D 與使用 S 之間（含 S 自身）存在對 P 之**覆蓋寫入** | E05/E06 豁免 |
| `tuse(S,T)`（nll） | `use_ref(S,T)` ∪ `use_ref(S,U), ref_of(U,T)` | 對 T 之使用**含別名鏈使用**（NLL origin 活度） |
| `span_end(L,T,E)` | 由活度等級檔定義（見 6.3） | 借貸終端（活度掛鉤） |

### 6.2 十條錯誤規則

| 弦律 | 規則（簡寫） | 語義 | ~ rustc | 幾何化身 |
|---|---|---|---|---|
| **E01** `eclash` | `lend(A,P1,_,mut), lend(S,P2,T2,_), after(A,S), overlap(A,S,P1)`（＋對稱：新貸為 mut） | 路徑衝突之兩借用重疊、至少一者 mut | E0499/E0502 | 紅弧交越 ✕ |
| **E02** `ewrite` | `write(S,P), onregion(L,S), lend(L,R,T,_), path_conflict(R,P)` | 借用活躍期間寫入被借者（含子字段） | E0506 | 寫入點落弧跨內 |
| **E03** `eread` | `read(S,P), onregion(L,S), lend(L,R,T,mut), path_conflict(R,P)` | mut 活躍期間直接讀取被借者（含子字段） | E0503 | 讀點落紅弧跨內 |
| **E04** `edangle` | `usep(S,P), stmt_of(S,N), scope_of(P,M), !inside(N,M)` | 使用點落在被借者作用域外（深度防禦） | E0597 | 弧端點掉出圓 |
| **E05** `eadrop` | `readp(S,P1), drop(D,P2), after(D,S), path_conflict(P1,P2), !reinited(D,S,P2)` | drop 後之讀取性使用 | —（E0382 系） | 使用點在 drop 之後 |
| **E06** `emove` | `readp(S,P1), move(D,P2), after(D,S), path_conflict(P1,P2), !reinited(D,S,P2)`；**＋** `write(S,P1), …`（整體 move 後寫子字段） | move 後使用（整體/字段互涉） | E0382 | 使用點在 move 之後 |
| **E07** `eloanmove` | `move(S,P), onregion(L,S), lend(L,R,T,_), path_conflict(R,P)` | 借用活躍期間 move 被借者（含子字段） | E0505 | move 點落弧跨內 |
| **E08** `eloandrop` | `drop(S,P), onregion(L,S), lend(L,R,T,_), path_conflict(R,P)` | 借用活躍期間顯式 drop 被借者 | —（E0505 系） | drop 點落弧跨內 |
| **E09** `erefuse` | a) `use_ref(S,T), onregion(M,S), lend(M,T,_,mut)`；b) `use_ref(S,T), lend(L,_,T,mut), onregion(M,S), lend(M,T,_,_), neq(M,L)` | **別名層衝突**：參考 T 的使用點，T 自身被借用活躍（2-phase 於陳述粒度不可表達 → 使用點攔截） | E0502/E0499（別名層） | 紅弧跨內排他使用 |
| **E10** `ereturn` | `ret(S,T), ref_of(T,P), scope_of(P,M), !same(M,n0)` | 回傳參考之被借者不活得比呼叫者（ROOT）久 | E0106/E0515 | 弧逸出作用域圓 |

**rustc 錯誤碼對照（經 oracle 實證）**：E01→E0502/E0499、E02→E0506、E03→E0503、E06→E0382/E0594 系、E07→E0505、E09→E0502/E0506（別名層）、E10→E0106。

### 6.3 活度三個等級（`--liveness` 旋鈕）

| 等級 | `span_end(L,T,E)` 定義 | 接受集 | 用途 |
|---|---|---|---|
| `nll`（預設） | `last_use_ref(E,T), reach(L,E)`；其中 `last_use_ref` = `tuse` 的**終端使用**（`tuse` 含別名鏈閉包；`shadowed_ref(S,T) :- tuse(A,T), after(S,A)`） | ⊇ referent | 與 NLL RFC 2094 區域語義一致 |
| `referent`（保守） | `last_use_place(E,P), reach(L,E)`（P = 被借者路徑；`last_use_place` 由 `usep` 終端定義） | 較小 | 更直覺（「被借者用完才死」） |
| `lexical`（最保守） | `scope_of(T,S), scope_last(S,E), reach(L,E)`（參考宣告作用域的終端陳述） | 最小 | 快速預檢（Polonius LI 思路）/ 教學 |

> 對應 Polonius 的 Naive/Opt/LI 分級。NLL 相對於保守級的全部差別 = **換一個 `span_end` 定義**（各 ~3 行）——複雜度被隔離在最小面。

---

## 7. 理論基礎

### 7.1 設計定理（核心主張）

> 對**函數內** CFG，一條借貸在某點 Q 的「活躍性」（NLL 區域語義）=
> `{ Q : reach(lend, Q) ∧ reach(Q, E) }`，其中 `E` 是該參考的**終端使用**。
> **NLL 的 loan 活度 = 出借點到終端使用點路徑上的點 = 活度點之向前閉包。**

- NLL 用迭代 fixpoint 算這個向前閉包；弦律用 Datalog 的 `reach` 遞推算出**同一集合**——同結果、不同載體；終止性/證明樹免費。
- **迴圈**：後向邊進入 `reach`，區域自動閉包環過迴圈（ex5，與 NLL「迴圈內使用的借用活於整個迴圈」一致）。
- **分支**：`after`/`onregion` 皆基於 CFG 達達性 → 分支內使用不延長活度過分支（ex12，與 rustc 一致），零額外規則。

### 7.2 四條核心不變式（與 rustc 相同，不新增、不放松）

1. 單一所有權/移動消耗 → E05/E06/E07。
2. 排他 vs 共享 → E01/E02/E03（v0.3：按路徑前綴判定）。
3. 不懸垂 → E04/E10。
4. 區域包含 → E10。

### 7.3 兩條幾何法則（規則的幾何化身，印於每張圓示底部）

> **法則①（紅弧孤立）**：`mut`（紅）弧的弧跨內不得含任何其他弧的端點，且不得與任何其他弧的弦交越。 ⟺ E01/E02/E03。
> **法則②（弧在圓內）**：任何弧的端點須落在被借者的作用域圓內；回傳者之被借者須活得比呼叫者（ROOT）久。 ⟺ E04/E10。

幾何是直覺、**事實是真值**：✕ 標記與錯誤註記由證明樹產生，不是重新推導；圖是同一事實集的渲染，無轉譯損失。

---

## 8. 圓示（縱點節圖）規格

`(V, P, A, C)`：

| 元素 | 定義 | 視覺 |
|---|---|---|
| **P 點** | 陳述，沿縱軸由上而下（**時間向下**） | 圓點（灰；出借點=藍/紅；錯誤點=紅環） |
| **A 弧** | 借用：出借點 → 終端使用點，貝茲弦弧向右凸；凸度 = 被借者 lane（不疊弧） | sh 細藍 `#3b82f6` / mut 粗紅 `#ef4444`；死借貸（無使用）= 灰小圈 |
| **C 圓** | 作用域同心橢圓環抱其陳述（**巢狀 = 同心圓**） | fn/block 灰虛線、loop 紫 `#8b5cf6`、if 藍 `#0ea5e9`；標籤 = fn 名/種類 |
| 後向邊 | 迴圈子樹末 → 首 | 左側紫色虛線迴路＋箭頭（「下輪」） |
| 交越 | 兩含 mut 弧之弦相交 | ✕ 金圈 `#fbbf24`（幾何直覺；真值以錯誤事實為準） |
| 逸出 | E10 | 弧端點處紅色虛線箭頭穿出 fn 圓（「→ 呼叫者」） |
| 錯誤註記 | 右欄 | `E01 @ s4  紅弧交越: …` |
| 圖例+法則 | 底部 | 藍/紅弧、作用域圓、兩條法則 |

版面：縱軸 x=300、首點 y=96、點距 54px；寬 780。純 SVG（XML 轉義），無外部資源。

---

## 9. 輸出與機器接口

### 9.1 終端報告（預設）

```
檔案: ex15_field_clash.cl   (liveness=nll)
verdict: FAIL — 1 個錯誤
  [E01] s3  fn ex15  「let b = &mut x」
      紅弧交越: 兩借用重疊且至少一者為 mut (~ E0499/E0502)
      eclash(s3)
      └─ 規則 R22 eclash:
        lend(s2, x.f, a, sh)   (事實)
        lend(s3, x, b, mut)   (事實)
        …（證明樹，前 16 行，多則略）
  區域  fn ex15:
    a : & x.f   區間 s2 → s4
    b : &mut x   區間 s3 → s5
  圓示: examples/out/ex15_field_clash.svg
```

### 9.2 `--json`（供代理/CI）

```json
{
  "file": "ex16_two_fn.cl",
  "liveness": "nll",
  "verdict": "FAIL",
  "errors": [
    {"code": "E02", "stmt": "s6", "fn": "bad",
     "stmt_text": "set x",
     "message": "借用活躍期間寫入被借者 (~ E0506)",
     "proof": ["ewrite(s6)", "└─ 規則 R24 ewrite:", "  lend(s5, x, a, sh)   (事實)", "…"]}
  ],
  "regions": [
    {"fn": "ok",  "ref": "a", "referent": "x", "kind": "sh",  "start": "s2", "end": "s3"},
    {"fn": "bad", "ref": "a", "referent": "x", "kind": "sh",  "start": "s5", "end": "s7"}
  ]
}
```

- 多檔輸入 → JSON 陣列。
- E00（前端錯誤）：`{"code":"E00","stmt":"?","stmt_text":"< offending 行>","message":"…"}`，無 proof。
- `regions[].end = null` 表示死借貸（無使用）。

### 9.3 `--explain`（代理修復接口）

每個錯誤輸出一段：

```
[E01] s3  fn ex15  「let b = &mut x」
  語義: 紅弧交越: 兩借用重疊且至少一者為 mut (~ E0499/E0502)
  規則: eclash(S) :- lend(A,P1,_,mut), lend(S,P2,T2,_), after(A,S), overlap(A,S,P1)
  證明:
    eclash(s3)
    └─ 規則 R22 eclash: …（完整證明樹）
  幾何: 點 s3 落在弧跨內: a (:sh) s2→s4
  修法: 縮短其中一條弧: 將先出借者的「最終使用」移到後出借者出借點之前 (依序化), 或 clone 所需值
```

十條代碼各有一條**修法模板**（縮短弧/拆句/依序化/改經引用存取/移進作用域/重初始化/clone/參數化被借者…）。

### 9.4 `--rules`

十條規則速覽表（代碼/語義/~rustc/活度等級）＋ 四個規則檔全文（`rules.dl`、`liveness_{nll,referent,lexical}.dl`）——**代理可直接讀取的規格**。

---

## 10. 測試與驗證體系

### 10.1 `test_chordlaw.py`（19 項，`python3 test_chordlaw.py`）

| 測試類 | 項數 | 防守目標 |
|---|---|---|
| `TestVerdicts`（34 語料自動生成） | 1 | PASS 語料防守**虛假拒絕**；FAIL 語料防守**虛假放行**，且 **(code, stmt) 集合須完全相符** |
| `TestFrontend` | 5 | E00 結構錯誤（多餘 }、自引用、未定義、fn 外陳述；多 fn 獨立性） |
| `TestParserScopes` | 3 | 已閉作用域後之陳述歸屬（`cur()` 回歸：舊 bug 把陳述誤塞進已閉作用域） |
| `TestLivenessModes` | 2 | referent 嚴格於 nll；兩級對交越一致 |
| `TestV03Features` | 8 | if 分支活度、分支內交越、lexical 翻轉、split borrow、路徑衝突、字段 move 互涉、未知等級拒斥 |
| `TestEngine` | 5 | 變數慣例、reach（傳遞/自反）、neq、分層否定、**路徑內建（域枚舉）**、證明樹 |
| `TestExamples` | 1 | `examples/` 17 檔判定基準 |

**加測試**：`PASS_CASES`/`FAIL_CASES` 各加一行 `(name, code, expected_set)`；新測試類繼承 `unittest.TestCase` 並放於 `TestExamples` 之前。

### 10.2 `oracle_check.py`（26 例，`python3 oracle_check.py`，需 rustc）

對每例：弦律判定（nll）vs 真 rustc 編譯結果（accept/reject）核對；rustc 錯誤碼記錄在案。

**忠實翻譯約定**（.cl → Rust；每例含備註）：

| .cl | Rust | 備註 |
|---|---|---|
| `use P`（place） | `let _uN = P;` | 真實 MIR 讀取（`let _ = P` 會被 MIR **消除**，不忠實） |
| `use R`（sh 參考） | `let _uN = *R;` | 透過參考讀取，延續 R 活度 |
| `use R`（mut 參考） | `*R += 1;` | 排他使用 = 讀寫 |
| `set P` | `P = …;` | |
| `mv P` | `let _m = P;` | P 用 `String`/結構體使其為真實 move |
| `dp P` | `std::mem::drop(P);` | |
| `loop {…}` | `loop {…}` | **無 `break`**（忠實無限迴圈；break 會截斷活度語義） |
| `if {…}` | `if true {…}` | |
| `x.f` | 結構體 `struct S { f: … }` 之字段 | |

**加案例**：`CASES` dict 加 `(cl_source, rust_source, note)` 三元組即可。

### 10.3 驗證歷史（健檢中發現並修正的關鍵問題）

| # | 發現 | 影響 | 處置 |
|---|---|---|---|
| 1 | **MIR 讀取消除**：`let _ = x;` 被 MIR 消除，rustc 不檢查 | oracle 翻譯失真 | 翻譯改 `let _uN = x;`；E03 對照碼改為 **E0503** |
| 2 | **讀取 vs 寫入**：mut 活躍期，讀被借者→E0503、寫→E0506；讀不與 sh 衝突 | 規則精度 | E02/E03 分設，均經 oracle 驗證 |
| 3 | **`break` 截斷活度**：無限迴圈加 break 改變語義 | 3 例 oracle 誤判 | 翻譯去 break；弦律原判定正確 |
| 4 | **`cur()` 解析 bug**：回傳最後「建立」之 scope 而非堆疊頂端 → 陳述誤入已閉作用域 | 迴圈/區塊後寫入被虛假拒絕 | 修 `cur()`＋3 個回歸測試鎖定 |
| 5 | **2-phase borrow** 舊規則（再出借點攔截）產生誤報 | 虛假拒絕 | E09 改在**使用點**攔截（`erefuse`） |
| 6 | E05/E06 對「move 後重初始化」誤報 | 虛假拒絕（`mv x; set x; use x` 合法） | `readp`（排除寫入）＋`!reinited` |
| 7 | liveness 參數序混用 (ref,stmt) vs (stmt,ref) | `span_end` 空 → 大量虛假 PASS | 統一 (S,T)=(陳述,參考) 慣例 |
| 8 | 內建謂詞未綁定參數被當常數求值 | 路徑推導全滅 | **有限域枚舉**（v0.3） |
| 9 | 分支活度需 CFG 達達性而非文本先後 | if 語義錯誤 | `after := reach`（v0.3） |
| 10 | 字段 move 互涉（整體↔子字段） | 虛假放行風險 | E06 雙子句＋`path_conflict`＋`covers`（v0.3，經 rustc E0382 驗證） |

**現況**：19/19 測試、26/26 oracle、17/17 範例——三套驗證相互獨立（測試=預期語義、oracle=真 rustc、範例=基準回歸）。

---

## 11. 限制與邊界

| 不支援 | 行為 | 理由 / 出路 |
|---|---|---|
| 2-phase borrow | E09 於使用點攔截（要求拆句） | 陳述粒度不可表達；保守方向 |
| Deref coercion（經參考取字段） | E00 明示 | 需建模 `Deref` trait 鏈；保守裁除 |
| 暫存生命週期延展（`let x = &*f(&y) + …`） | 不建模 | 玩具語言無表達式；P2 syn 前端範圍 |
| closure 捕捉 | 無語法 | 需函體才可分析 capture；P2 |
| 跨程序區域推斷 | 每 fn 獨立；參數=ROOT 範圍 | E10 已覆蓋回傳場景；P2/P4 |
| 命名生命週期 / variance | 區域=詞法作用域+ROOT | 保守 |
| `unsafe` | 不在域內 | 獨立健全性域 |
| `while`/`match`/多分支 if-else | 無語法（`if` 僅單分支、`loop` 無 `break` 語義） | `loop` 可模擬 while；P2 前端擴充 |
| 字段 move 後的**逐字段**失效精化（部分 move 追蹤） | 按路徑衝突保守處理 | 現規則對整體/字段互涉已與 rustc 一致（26/26）；更深結構（巢狀結構體逐字段）待 P2 |
| 全量 Rust 語法 | 玩具語言 `.cl` | 設計為內環子集；P2 `syn` 前端 |

**健全性方向**：所有限制都是「接受得更少」（或 E00 明示），符合合約 S。

---

## 12. 變更記錄

### v0.6（2026-08-27）

- **P3a 代理工具鏈**：`helper.py` 專案檢查／建議／評分／`.chordlaw/state.json`；`mcp_server.py` stdio MCP 六工具。
- CLI：`--check` / `--report` / `--history` / `--mcp`。
- 規格：[MCP.md](MCP.md)。本倉庫報告：[reports/helper_v0.6.md](reports/helper_v0.6.md)。
- 測試：`test_helper.py` 9 項。`.rs` 不假裝已檢查。

### v0.4（2026-08-24）

- **32 則錯誤規則 E01–E32**（`rules.dl`）：E01–E10 行為不變；E11–E32 僅由新事實點火（imm/tmp/hole/store/deref/call/else），不擾 v0.3 語料。
- **前端**：`let imm` / `let hole` / `tmp` / `slot` / `set *T` / `mv *T` / `store` / `call` / `callmv` / `fn f(imm p)` / `if { } else { }` 菱形 CFG。
- **引擎**：否定可作用於內建謂詞（`!subpath` / `!path_conflict`）。
- **圓示**：else 琥珀色圓、法則③、E04/E16/E28 逸出箭頭；32 張 `examples/out/rXX_*.svg`。
- **文件**：`RULES32.md` 規則目錄、`gallery.html` 32 則畫廊。
- **測試**：46 項全綠（v0.3 回歸 + v0.4 32 則對應）。

### v0.3（2026-08-22）
- **if 分支** CFG（作用域樹遍歷生邊；`after := reach`）；NLL 分支活度語義（ex12–ex13）。
- **`lexical` 活度等級**（`liveness_lexical.dl`＋`scope_last`）；`--liveness` 三級。
- **多 fn 檔案**：每 fn 獨立 Datalog 檢查；錯誤/JSON/SVG 帶 fn 標記（ex16）。
- **字段路徑 / split borrow**：`path_conflict`/`covers`/`subpath` 內建＋**有限域枚舉**；E01–E08 路徑化；E06 字段雙子句（ex14–ex15、3 個 oracle 語料）。
- **代理工具**：`--explain`（規則原文+證明+幾何+修法）、`--rules`（規格速覽）；`--json` regions/errors 帶 fn。
- 引擎：規則保存原文（供 --explain）；內建未綁定參數域枚舉；SVG 支援 if 顏色/多 fn 標籤/all_stmts 版面。
- 測試 16→19 項（34 語料）；oracle 18→26 例（**26/26 一致**）；範例 12→17。

### v0.2（2026-08-22）
- 前端 guards（E00 全數情境）；`--json`；`test_chordlaw.py`（16 項）；`oracle_check.py`（18/18）。
- E09 改**使用點**攔截（`erefuse`）；E03 對照 E0503（MIR 讀取消除發現）。
- E05/E06 `readp`＋`!reinited`（重初始化豁免）；`tuse` 別名鏈活度閉包。
- `cur()` 解析 bug 修正＋回歸測試；liveness 參數序統一 (S,T)。

### v0.1（2026-08-22）
- 首版：mini Datalog（分層+證明樹）、`.cl` 前端（fn/loop/block/let/&/&mut/use/set/mv/dp/ret）、圓示 SVG、E01–E10、nll/referent 兩級、7 範例。

---

## 13. 參考

1. SafeTrans（LLM 修借用錯誤：引導修復 ~74.2%；「編譯器訊息不夠」）— arxiv.org/html/2505.10708v2
2. Polonius — Datalog 風格 loan 分析、Naive/Opt/LI 分級 — rust-lang.github.io/polonius/rules/loans.html
3. AkiraRust（結構化反饋 75%→100%）— arxiv.org/html/2602.21681v1
4. RustAssistant（LLM 修編譯錯誤 ~74% 峰值）— news.ycombinator.com/item?id=43851143
5. Common Rust Compiler Errors（borrow 最高頻）— reintech.io/blog/common-rust-compiler-errors-and-how-to-fix-them
6. 錯誤碼占比表 — knowledgelib.io/software/debugging/rust-borrow-checker/2026
7–8. Soufflé（Datalog 生產引擎＋provenance）— souffle-lang.github.io
9. RustViz（VL/HCC 2022，ownership 時間軸視覺化）— github.com/rustviz/rustviz
10. REVIS（生命週期錯誤視覺化）— arxiv.org/pdf/2309.06640
11. RFC 2094 — Non-lexical lifetimes — rust-lang.github.io/rfcs/2094-nll.html
