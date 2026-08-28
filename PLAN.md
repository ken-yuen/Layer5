# 弦律 ChordLaw — 簡化 Rust 借用/生命週期檢查器

**面向 AI 代理開發 Rust 專案的「核心邏輯一致、複雜度簡化」規則重構**
**開發計畫 · 設計論證 · 成功論證 · 可運行原型**

> 版本 v0.3 · 2026-08-22 · 附可運行原型（`chordlaw.py` + `rules.dl` + 17 個範例 + 圓示 SVG + 19 項回歸測試 + rustc oracle 差異測試 26/26 一致）

---

## 0. 一頁摘要（TL;DR）

**我們要做什麼**：仿照 Rust 的借用/生命週期（borrow/lifetime）規則，**重構**出一個核心邏輯一致、但複雜度大幅簡化的檢查器「弦律」。它不取代 `rustc`，而是作為 **AI 代理在寫 Rust 時的高頻、局部、可視化內層反饋環**，讓代理「看得懂、改得對、收斂得快」，從而**提升代理開發 Rust 專案的成功率**。

**核心洞察（一句話）**：
> 把「**迭代 fixpoint 推導**（NLL/Polonius 的區域推斷）」重構成「**有向無環圖上的達達性閉包 ＋ Datalog 單調規則 ＋ 圓示幾何視覺**」。
> 規則可讀（Datalog 檔）、反例可證（證明樹）、圖形可看（圓示）、`rustc` 為最終門。

**為什麼代理在 Rust 上失敗、以及我們如何對症下藥**：
- 借用/生命週期錯誤是 Rust 編譯失敗中**最高頻且最難修**的類別：E0382（use-after-move）約 30%、E0502（mut 與 sh 衝突）約 20%、E0505/E0597 各約 12%、E0499 約 10% [6]。
- LLM 修這類錯誤特別吃力：直接給編譯器訊息**不夠**，需要引導式反饋，跨六個模型借用違規的引導修復成功率也只有約 **74%** [1]；即便最強模型裸修 ownership/lifetime/aliasing 也只在 ~75% pass [3]；代理常「在兩三個仍編譯不過的修法之間來回循環」[4]。
- **根因**：NLL 要求心智模型去**模擬一個全域、迭代、不透明的 dataflow fixpoint**。代理拿到的是全域、分散、語義壓縮的錯誤碼，修復空間是組合爆炸。**代理需要的不是「更精確的檢查器」，而是「局部可定位、幾何可直覺、規則可讀、反饋極快」的檢查器。**

**本倉庫已證明可行性**：一個純標準庫的 mini Datalog 引擎（分層＋證明樹＋有限域內建）＋玩具前端（**if 分支、迴圈、多 fn、字段路徑/split borrow**）＋圓示 SVG 渲染器，17 個範例判定全部與 NLL 語義一致；`test_chordlaw.py`（19 項）全綠；`oracle_check.py` 以**真實 rustc**（1.98.0）為 oracle 做差異測試，**26/26 判定一致**（合約 S：0 虛假放行）；`python3 chordlaw.py` 即可運行。

---

## 1. 問題陳述：為何 AI 代理寫 Rust 會卡在借用檢查

### 1.1 錯誤分布（為什麼這是「那個」瓶頸）

借用/生命週期是 Rust 初學者與 LLM 遇到**最多**的編譯錯誤 [5][6]。按活躍修復成本排序的代表性代碼 [6]：

| 代碼 | 語義 | 近似占比 | 弦律規則 |
|---|---|---|---|
| **E0382** | use of moved value（move 後使用） | ~30% | E06 `emove` |
| **E0502** | mut 借用活躍期間再 sh 借用/讀取 | ~20% | E01 `eclash` / E03 `eread` |
| **E0505** | move out of borrowed value | ~12% | E07 `eloanmove` |
| **E0597** | value does not live long enough | ~12% | E04 `edangle` / E10 `ereturn` |
| **E0499** | 多次 mut 借用同一值 | ~10% | E01 `eclash` |
| **E0515** | return reference to local | ~6% | E10 `ereturn` |
| E0716 / E0507 / E0506 | 暫存被 drop / 經 sh 取出 / 借用期間寫入 | <10% | E05 / E02 |

> **關鍵事實**：代理最常撞牆的 6~8 個錯誤碼，**全部**被弦律 E01–E10 的十條規則覆蓋。我們不是在做一個「通用」檢查器，而是在做一個**針對代理最高失敗率的精煉檢查器**。

### 1.2 LLM 特有的失敗模式（為什麼「給編譯器訊息」不夠）

- **SafeTrans**（15,918 次 C→Rust 轉譯、6 個模型）：「只提供編譯器錯誤訊息與反饋**不足以**修復許多錯誤」；借用違規即便用 few-shot 引導修復，跨模型平均成功率僅 **74.2%**；整體轉譯成功率從 54% 提升到 80% [1]。
- **AkiraRust**：通用 LLM 裸修（GPT-4/5、Claude 3.5）的語義 exec rate 只有 25–50%；引入**狀態機＋波形引導的結構化反饋**後，pass rate 從 ~75% 提到 100%、exec rate 提到 85–95% [3]。
- **RustAssistant**（Microsoft）：對真實 repo 的編譯錯誤峰值準確率約 **74%**；社群反饋：LLM「無法真正修好 borrow 相關問題」「在兩三個仍編譯不過的修法之間循環」[4]。

### 1.3 根因分析

把上述現象歸到一個機制上：

| 維度 | NLL/Polonius 現狀 | 代理需要的 |
|---|---|---|
| **計算模型** | 全域迭代 dataflow **fixpoint**（區域推斷、2-phase、three-set） | 單遍、確定、可緩存 |
| **反饋粒度** | 全域、分散、語義壓縮的錯誤碼＋span | 局部、精確到「哪兩句、哪兩條弧」 |
| **心智模型** | 要模擬一個迭代算符的收斂 | 幾何直覺＋可讀規則 |
| **修復空間** | 組合爆炸、常循環不收斂 | 「改一條弧」的局部操作 |
| **可解釋性** | 黑盒（为何活著/死掉） | 每個反例附**證明樹** |

**結論**：代理在 Rust 上的瓶頸不是「不懂 Rust 語法」，而是**借用檢查的反饋結構與 LLM 的修復機制（局部、迭代、需要可定位訊號）錯配**。

---

## 2. 設計哲學：仿照與重構

### 2.1 「核心邏輯一致」＝ 健全性合約（soundness contract）

我們**不聲稱**與 `rustc` 接受集相同，而是聲稱一個**單向、可驗證**的合約：

> **合約 S（無虛假放行 / no false pass）**：凡弦律**接受**的程式，`rustc` 也接受。
> 即 弦律接受集 ⊆ rustc 接受集（弦律是 rustc 的**保守精化**）。

- **方向正確性**：對「提升代理成功率」而言，**虛假放行**（弦律放行、rustc 拒絕/未定義行為）是致命的；**保守拒絕**（弦律拒絕、rustc 接受）只是讓代理多重構一次，且 `rustc` 仍是最終門。因此「**接受得更少**」恰是正確方向——**用接受集缺口（acceptance gap）換健全性與可解釋性**。
- **可驗證**：合約 S 用「以 `rustc` 為 oracle 的差異模糊測試」（Phase P4）持續守衛，任何「弦律接受但 rustc 拒絕」都是 P0 級 bug。

### 2.2 「複雜度簡化」＝ 四刀

| # | 重構 | 從 | 到 | 效果 |
|---|---|---|---|---|
| 1 | **區域推斷 → DAG 達達性閉包** | 迭代 fixpoint dataflow | 單遍 Datalog 遞推（`reach`） | 無需 fixpoint 引擎；終止性由 Datalog 單調性保證 |
| 2 | **活度（liveness）分級** | NLL 全域活度 | 三個可切換等級：`nll`（預設）/ `referent`（保守）/ `lexical`（最保守，P2） | 像 Polonius 的 Naive/Opt/LI 分級 [2]；預設即 NLL 等價 |
| 3 | **裁除罕用/高危機制** | 2-phase borrow、Deref  coercion、暫存生命週期延展、closure 捕捉、unsafe | v0.1 一律**禁止或延後**（E09 攔截 2-phase） | 規則數從「rustc 數千行」降到「~40 行可讀 Datalog」 |
| 4 | **函數內局部化** | 全域/跨程序區域 | v0.1 聚焦**函數內 CFG**（跨程序 P2） | 圖小、確定、可緩存 |

### 2.3 四條核心不變式（弦律逐條編碼）

Rust 借用系統的**核心邏輯**可歸約為四條不變式；弦律每條都有對應規則，且**不新增、不放松**任何一條：

1. **單一所有權 / 移動消耗**：值被 `move` 後原位置不可再使用 → E06/E07。
2. **排他 vs 共享**：同一被借者，`&mut` 與**任何**其他借用（含 `&`）不得同時活躍；`&` 與 `&` 可共存 → E01/E02/E03。
3. **不懸垂（no dangling）**：引用不得活過被借者 → E04/E10。
4. **區域包含（region inclusion）**：引用的區域 ⊆ 被借者的區域；回傳引用須活得比呼叫者久 → E10。

> 「核心邏輯一致」在此有明確含義：**這四條不變式與 rustc 相同，弦律只是把「驗證它們」的算法從 fixpoint 推斷簡化為 DAG 上的有限檢查。**

---

## 3. 四大可視化/形式化支柱

### 3.1 Datalog：規則即代碼

**為什麼是 Datalog**（而非手寫 dataflow、SMT 或直接調 rustc）：

1. **終止性免費**：Datalog 是**單調**、**有限全域**的邏輯，定點必收斂——我們不需要像 NLL 那樣管理迭代收斂、殺死（kill）函數、fixpoint 不發散的證明。終止性是語言性質，不是工程成就。
2. **證明樹（provenance）= 診斷**：每個推出的事實都記錄「由哪條規則、哪些事實推出」。於是**每個錯誤都自帶證明樹**——這正是代理需要的「局部、可定位、可解釋」訊號。原型已實現並輸出（見 §7）。
3. **規則檔是版本化的規格**：`rules.dl` ~40 行，**人與代理都讀得懂**。代理可以**讀規則**（「為什麼被拒」）、甚至**提議改規則**（以健全性測試集為護欄：新規則若放行已知不健全樣本即拒）。這是「自我演化工具鏈」的種子。
4. **增量天然**：Datalog 有成熟的增量更新（update rules）。代理改一行 → 只重算受影響的事實閉包 → 反饋在**毫秒級**，而非重編整個 crate。
5. **生態對齊**：
   - **Polonius 本身就用 Datalog 風格規則撰寫 loan 分析**（`origin_contains_loan_on_entry :- loan_issued_at, cfg_edge, !loan_killed_at …`），且明確分 Naive/Opt/**LocationInsensitive** 三個精化等級，並提出「LI 作為快速預檢：若無路徑/流敏感都無錯，完整分析必無錯」[2]。**我們的「簡化 Datalog 內層環＋rustc 完整外層環」正是 Polonius 官方路線的通用化與代理化。**
   - **Soufflé**：把 Datalog 單調計算編譯成高效（可並行）C++，支持大規模靜態分析與 provenance [7][8]。生產化引擎直接可用。

**NLL 精化「只值三行」**：NLL 相對於保守「被借者活度」的差別，在弦律裡只是**換一個 `span_end` 定義**（`liveness_nll.dl` vs `liveness_referent.dl`，各 ~3 行）。這說明我們把 NLL 的複雜度**隔離在了最小面**——預設即 NLL 等價，想更保守就換檔。

### 3.2 圖形拓撲：拓撲即語義

弦律把程式抽象成**三張緊耦合的圖**，拓撲結構直接承載語義：

1. **控制流圖（CFG）**：直線陳述是前向邊；`if` 是**分支**（無 else）；`loop` 是**後向邊**（back edge）。邊由作用域樹遍歷生成（fn 之間、兄弟分支之間無邊）。整圖是「一個 DAG ＋ 若干後向邊」；`after` := CFG 達達性（嚴格在前），使分支活度語義精確（**分支內使用不延長活度過分支** — ex12 與 rustc 一致）。
2. **作用域樹（scope tree）**：`fn`/`block`/`loop` 巢狀為樹；參數與呼叫者掛在 **ROOT**（`n0`）。區域 = 子樹。
3. **借用/別名圖（borrow/alias graph）**：`lend`/`borrow_of` 形成別名鏈（`ref_of` 遞推）；同一被借者的多條借用是「弧」的集合。

**設計定理（核心主張，待 P4 形式化證明）**：
> 對**函數內** CFG，一條借貸（loan）在某點 Q 的「活躍性」（NLL 區域語義：Q 是否落在該 loan 的區域內）等於
> `{ Q : reach(lend, Q) ∧ reach(Q, E) }`，其中 `E` 是該參考的**終端使用**。
> 即 **NLL 的 loan 活度 = 出借點到終端使用點的路徑上的點 = 活度點之向前閉包**。

- NLL 用**迭代 fixpoint** 算出這個向前閉包；弦律用 **Datalog 的 `reach` 遞推**算出**同一個集合**——*同結果、不同載體*，且 Datalog 的終止性/證明樹是免費的。
- **迴圈不需要任何專門規則**：後向邊進入 `reach`，區域就**自動向前閉包**環過迴圈。原型 `ex5_loop_write` 驗證：`r` 的區間經後向邊覆蓋到迴圈內的寫入點，精準觸發 E02（與 NLL「迴圈內使用的借用活於整個迴圈」一致）。

**確定性紅利**：分析是「圖的純函數」→ 可**緩存**（子圖 hash→判定）、可**測試**（每條規則配 pass/fail 樣本）、可**增量**。

### 3.3 DAG：無環紀律

- **達達性只在切掉後向邊的 DAG 上定義**；後向邊是「語義標記」（下一輪迭代），不是達達性邊本身。這保證拓撲序存在、單遍可行。
- **別名環 = 結構錯誤**：`alias(a,a)`（所有權環）在 Datalog 裡是一條規則即偵測，語義是「雙重所有權」。
- **與未來對齊**：Polonius 的方向就是「per-CFG 圖＋達達性」[2]；我們押注在「借用檢查器的未來形狀」，且把最難的 fixpoint 收斂問題用 DAG＋Datalog 消解。

### 3.4 圓示（縱點節圖）：把規則變成「看得見的幾何」

**自研圓示**的形式定義——一個四元組 `(V, P, A, C)`：

- **P（點, points）**：陳述，沿**縱軸由上而下**排布（**時間向下**，符合閱讀方向）。「縱點節圖」即此：一列縱向點節。
- **A（弧, arcs/chords）**：每條借用 = 一條從「出借點」到「終端使用點」的貝茲**弦弧**，向右凸出，凸度 = 被借者的 lane（不同被借者不疊弧）。`sh` 細藍、`mut` 粗紅。
- **C（圓, circles）**：每個作用域 = 一個**同心橢圓**環抱其陳述；**巢狀作用域 = 同心圓**（區域的包含關係直接是圓的包含關係）。`loop` 額外畫出紫色**後向邊迴路＋箭頭**（「下輪」）。

**兩條幾何法則（規則的幾何化身）**：

> **法則①（紅弧孤立）**：`mut`（紅）弧的弧跨內**不得含**任何其他弧的端點，且**不得與**任何其他弧的弦**交越**。
> ⟺ E01/E02/E03（排他衝突）。
>
> **法則②（弧在圓內）**：任何弧的**端點**須落在**被借者**的作用域圓內；回傳者之被借者須活得比**呼叫者**（ROOT）久。
> ⟺ E04/E10（懸垂/逸出）。

- **視覺等價**：違規 = 一眼可讀的幾何矛盾（紅弧交越＝✕；弧端點掉出圓＝逸出箭頭）。
- **幾何是直覺、事實是真值**：✕ 與錯誤標記**由檢查器的證明樹產生**，不是重新推導；圖是「同一事實集的渲染」，**無轉譯損失**。
- **對代理的價值**：
  - 有視覺能力的代理可**直接看圖**：「紅弧交越了 → 把其中一條弧縮短/拆句」——修復變成**局部幾何操作**。
  - 無視覺的代理拿**同一事實集**的**文字弧表**（`a : &x 區間 s2→s5`）與 JSON，接口一致。
- **與先前工作差異**：
  - **RustViz**（VL/HCC 2022）：時間軸視覺化，證實能幫學生建立「準確的所有權心智模型」[9]，但它是**教學向、人工標註/人看**的工具。
  - **REVIS**：把生命週期錯誤畫成 region 箭頭給**新手人**看 [10]。
  - **弦律圓示**是**檢查器的輸出**、**代理在環（agent-in-the-loop）修復用**、**規則=幾何**、與 Datalog 證明**同一事實集**——定位不同。

---

## 4. 規則集 v0.1（仿照 Rust 核心，重構簡化）

### 4.1 十條錯誤規則（E01–E10）

| 弦律 | 語義 | ~ rustc | 佔比 | 幾何化身 |
|---|---|---|---|---|
| **E01** `eclash` | 同一被借者兩借用重疊、至少一者 mut | E0499/E0502 | ~30% | 紅弧交越 |
| **E02** `ewrite` | 借用活躍期間寫入被借者 | E0506 | — | 寫入點落在弧跨內 |
| **E03** `eread` | mut 活躍期間直接讀被借者 | E0503 | — | 讀點落在紅弧跨內 |
| **E04** `edangle` | 使用點落在被借者作用域外 | E0597 | ~12% | 弧端點掉出圓 |
| **E05** `eadrop` | drop 後使用 | — | — | 使用點在 drop 之後 |
| **E06** `emove` | move 後使用（v0.3: 整體 move 後讀/寫子字段、字段 move 後讀整體，皆犯；整體重初始化豁免） | E0382 | ~30% | 使用點在 move 之後 |
| **E07** `eloanmove` | 借用活躍期間 move 被借者 | E0505 | ~12% | move 點落在弧跨內 |
| **E08** `eloandrop` | 借用活躍期間 drop 被借者 | — | — | drop 點落在弧跨內 |
| **E09** `erefuse` | 別名層衝突：參考 t 的使用點，t 自身被借用活躍（2-phase 借用在陳述粒度不可表達，於**使用點**攔截） | E0502/E0499（別名層） | — | 紅弧跨內排他使用 |
| **E10** `ereturn` | 回傳參考之被借者不活得比呼叫者久 | E0106/E0515 | ~6% | 弧逸出作用域圓 |

### 4.2 結構規則（非錯誤，承載語義）

- `reach(A,B)`：控制流達達性（DAG＋後向邊）——**NLL fixpoint 的替身**。
- `anc/inside`：作用域樹的祖孫/包含——區域包含的替身。
- `ref_of`：別名鏈遞推——Deref 前的別名。
- `usep`：「使用參考＝使用被借者」——把引用使用歸約到 place 使用。
- `readp`：**讀取性**使用（排除寫入）——E05/E06 的判據（寫入＝重新初始化，非「使用」）。
- `reinited(D,S,P)`：move/drop 與使用之間對 P 之寫入——豁免 E05/E06（`mv x; set x; use x` 合法，與 rustc 一致）。
- `tuse`（`liveness_nll.dl`）：對參考 T 之使用**含對其別名鏈之使用**（NLL origin 活度）——別名鏈活度的落點。
- `onregion(L,Q)`：借貸 L 於 Q 活躍 = `reach(L,Q) ∧ reach(Q, span_end)`——**設計定理的落點**。
- `overlap`：兩借貸重疊 = 其一出借點落另一區域。
- `span_end`：**活度等級**掛鉤（`nll`/`referent`/`lexical`）。

### 4.3 活度（liveness）三個等級（= 複雜度/精確度旋鈕）

| 等級 | `span_end` 定義 | 接受集 | 用途 |
|---|---|---|---|
| `nll`（預設） | 該**參考自身**的終端使用（經 `tuse` 含別名鏈使用閉包） | ⊇ referent | 與 NLL RFC 2094 區域語義一致 [11] |
| `referent`（保守） | **被借者**的終端使用 | 較小 | 更直覺、更安全的內層環 |
| `lexical`（**v0.3 已實作**） | **參考宣告之作用域**之終端陳述 | 最小 | 最保守，類 pre-NLL / 快速預檢 / 教學 |

> 對應 Polonius 的 Naive/Opt/**LI** 分級與「LI 快速預檢」思路 [2]。代理可視情境選級：要快用 `lexical` 預檢，要準用 `nll`，`rustc` 終審。

### 4.4 刻意裁除（接受集缺口，全部是「更安全方向」）

| 裁除 | v0.1 行為 | 為何安全 |
|---|---|---|
| 2-phase borrow | 陳述粒度不可表達 → E09 於**使用點**攔截，要求拆成兩句 | 只拒絕更多（保守） |
| Deref coercion | 不建模（需顯式） | 保守 |
| 暫存生命週期延展 | 不建模 | 保守 |
| closure 捕捉 | P2 | 保守（v0.1 不含 closure） |
| 跨程序/命名生命週期/variance | 區域＝詞法作用域＋ROOT | 保守 |
| 字段級/split 借用（place projection） | **v0.3 已實作**：字段路徑 `x.f`（整體/子字段前綴衝突；不同字段可共存）；經參考取字段（deref coercion）仍不支持 | 保守（deref 除外） |
| `unsafe` | 不在域內 | 獨立健全性域 |

> 所有裁除都是「**接受得更少**」，符合合約 S 的方向；缺口大小在 P2 用量體化，`rustc` 終審兜底。

---

## 5. 開發計畫（Roadmap）

> 現狀：**P1 核心已在本倉庫實現並跑通**（mini Datalog 引擎＋前端（if/迴圈/多 fn/字段路徑）＋圓示＋17 範例＋3 活度等級＋19 項回歸測試＋rustc oracle 26/26 一致＋`--explain`/`--rules` 代理工具）。下表 P1 為「已驗證骨架 → 工程化」。

| 階段 | 週期 | 目標 | 交付物 | 出口準則 |
|---|---|---|---|---|
| **P0** 規則規格 | 2 週 | 把 E01–E10 與設計定理寫成可審查規格＋一致性模型 | `rules.dl` 評審版、Alloy 模型、健全性陳述 | Alloy 模型檢查通過；規則↔不變式對應表簽核 |
| **P1** Datalog 核心 | 4 週（**骨架已完成**） | 引擎＋前端＋圓示工程化 | 分層引擎、證明樹、增量、CLI、`rules.dl`、`--json` 機器接口、`--explain`/`--rules`、測試＋oracle CI | 17 範例＋19 項回歸測試全綠；rustc oracle 26/26 一致（M5 目前=0）；增量 p95 < 500ms |
| **P2** Rust 子集前端 | 4 週（**Python syn 子集 + AHPBB 已落地**） | `syn`/`quote` → IR（let/expr/if/loop、字段、暫存、回傳） | `syn_subset.py` / `quote_tpl.py` / `intent.py` / `ahpbb.py`（多線出貨正確 Rust） | 子集：oracle 26 例判決不變。全量 syn crate + 200 snippet 仍待 cargo/rustc |
| **P3** 代理工具鏈 | 3 週（**P3a 已落地**） | MCP/CLI 工具＋增量引擎 | `chordlaw_check` / `diagram` / `explain` / `rules` / `report` / `history`（MCP stdio）＋`.chordlaw/state.json` | P3a：六工具＋評分報告。P3b 增量／P3c apply／代理 demo（M2≤2）未做 |
| **P4** 健全性證明 | 3 週（並行） | 形式化＋模糊測試 | Coq/Lean 核心引理；以 `rustc` 為 oracle 的差異模糊測試 CI | 核心引理證明；模糊測試 **0** 虛假放行 |
| **P5** 量測 | 3 週 | A/B 對照實證 | 200 任務 benchmark、M1–M5 報告 | 出「有/無弦律」成功率對照報告 |

**總計**：約 **4 個月到 v1.0**（3 人）；**P1 兩週即可用**（本倉庫已達）。

### 5.1 量測指標（success = 可測）

| 指標 | 定義 | 目標 |
|---|---|---|
| **M1** 首次通過率 | 代理一次生成都通過（弦律內環＋rustc 終審） | 基線 +20 點 |
| **M2** 借用錯誤修復迭代 | 從首次報錯到收斂的中位迭代數 | ≤ 2 |
| **M3** 檢查延遲 | 增量檢查 p95 | < 500ms |
| **M4** 診斷精度 | 證明樹指向「正確兩句/兩弧」的比例 | 人工/LLM 評 ≥ 90% |
| **M5** 健全性 | 「弦律接受但 rustc 拒絕」數 | **0**（差異模糊測試；oracle 18 例現為 0） |

### 5.2 風險與對策

| 風險 | 影響 | 對策 |
|---|---|---|
| 接受集缺口過大（代理寫 idiomatic rustc 代碼被弦律拒） | 代理空轉 | `lexical`/`referent` 寬鬆級＋`rustc` 終審＋語料驅動調規則；P2 量體化缺口 |
| 前端覆蓋不足（全量 Rust） | 部分代碼查不了 | 子集優先；無法解析即**直通 rustc**，只從其錯誤學習 |
| **健全性 bug（虛假放行）** | 致命 | 差異模糊測試（`rustc` oracle）是生命線；Coq 核心；任何虛假放行=P0 |
| 代理不願意用工具（行為） | 效果打折 | 工具回傳「視覺＋局部規則」降低認知負載；MCP 預設掛載；system prompt few-shot |
| 非視覺模型誤讀 SVG | 誤修 | 文字弧表＋JSON（同一事實集）雙接口 |
| Datalog 規則檔維護成本 | 腐化 | 規則檔即規格、版本化；**每條規則配 pass/fail 測試** |

---

## 6. 為何能成功（論證）

**總綱**：我們不是把借用檢查器變「簡單」，而是把借用檢查器的**「推理」變得可讀、可證、可見**——讓 LLM 的局部迭代修復機制第一次與借用規則的反饋結構**對齊**。

1. **失敗訊號重設計（最核心）**：把「全域算法性反饋（fixpoint 黑盒）」換成「**局部幾何事實＋證明樹**」。代理的修復從「重推整個 fixpoint」變成「**改一條弧**」。這直接命中 SafeTrans「光給編譯器訊息不夠」[1] 與 AkiraRust「結構化反饋把 75%→100%」[3] 的实证。
2. **用保守性換健全性**：接受集 ⊆ rustc 是**功能**不是缺陷（類似 strict mode / clippy pedantic）。`rustc` 是終審門，弦律是**快內環**；虛假放行由差異模糊測試守成 0（M5）。對「提升成功率」，**寧可多拒、不可錯放**。
3. **Datalog 的工程紅利**：終止性（語言性質）、證明樹（=診斷）、增量（毫秒反饋）、**代理可讀可改規則檔**（自我演化工具鏈的種子）。且 Polonius 官方已用 Datalog 規則＋分級預檢走同一路 [2]，我們是把它**通用化、代理化、視覺化**。
4. **感知接口**：圓示給視覺代理**幾何直覺**（紅弧交越＝修）；給非視覺代理**同一事實集**的文字弧表/JSON。圖與文**無轉譯損失**。RustViz 已證視覺化能建立正確心智模型 [9]，我們把它做成**代理在環**的修復接口。
5. **生態對齊（順勢而非造勢）**：Polonius 的 graph-based＋Datalog＋分級方向 [2]；Soufflé 的生產級 Datalog＋provenance [7][8]；rustc 的 HIR/MIR 基建供 P2 前端。我們押在「借用檢查器的未來形狀」上。
6. **实证方向正確**：跨多模型，**結構/反饋**比「裸模型強度」更能提升 Rust 修復（AkiraRust [3]、SafeTrans [1]、RustAssistant [4]）。弦律提供的是**更好的反饋結構**，而非更強的模型——這正是可持續、可規模化的槓桿。

**一句話**：
> 讓代理**看得見**（圓示）、**讀得懂**（Datalog 規則）、**證得明**（證明樹）、**收斂得快**（局部幾何修復＋增量），而 `rustc` 永遠是**最終的門**——成功率自然上去。

---

## 7. 原型驗證（本倉庫，已跑通）

**運行**：
```bash
python3 chordlaw.py                  # 跑全部 17 個範例（預設 liveness=nll）
python3 chordlaw.py --liveness referent|lexical   # 保守級 / 最保守級
python3 chordlaw.py examples/ex1_clash.cl # 單檔
python3 chordlaw.py --json examples/ex7_alias_use.cl  # 機器接口 (verdict/errors+proof/regions)
python3 chordlaw.py --explain examples/ex15_field_clash.cl  # 代理工具: 規則原文+證明+幾何+修法
python3 chordlaw.py --rules             # 規則規格速覽＋完整規則檔
python3 test_chordlaw.py             # 19 項回歸測試 (34 語料 + 前端 + 解析器作用域 + v0.3 功能 + 引擎單元)
python3 oracle_check.py              # rustc 差異測試: 26 例 弦律 vs 真 rustc 判定核對
```

**組件**：
- `chordlaw.py`：mini Datalog 引擎（**分層 stratified**＋單調定點＋**證明樹**＋**有限域內建** `path_conflict`/`covers`/`subpath`）、`.cl` 前端（**if 分支／迴圈／多 fn／字段路徑**）、圓示 SVG 渲染器、CLI＋`--json`/`--explain`/`--rules`（純標準庫）。
- `rules.dl`（~60 行）＋`liveness_nll.dl` / `liveness_referent.dl`：規則即代碼。
- `examples/*.cl`：12 個範例（交越、順序、逸出×2、迴圈×2、move、別名×2、重初始化、sh 重疊、mut 依序）。
- `examples/out/*.svg`：每個範例的圓示。
- `test_chordlaw.py`：回歸測試（34 語料：PASS 防守虛假拒絕／FAIL 防守虛假放行且錯誤碼+位置須完全相符、前端 E00、解析器作用域回歸、v0.3 功能、引擎單元）。
- `oracle_check.py`：**以真實 rustc 為 oracle 的差異測試**（18 例忠實 .cl→Rust 翻譯，accept/reject 判定核對＋rustc 錯誤碼記錄）。

**判定結果（liveness=nll，全部符合 NLL 語義，且經 rustc oracle 核對）**：

| 範例 | 情節 | 判定 | 錯誤 |
|---|---|---|---|
| ex1_clash | `&x` 未死又 `&mut x`（紅弧交越） | **FAIL** | E01@s4 |
| ex2_sequential | `&x` 用盡後再 `&mut x` | **PASS** | — |
| ex3_return | 回傳指向局部值的 `&` | **FAIL** | E10@s3 |
| ex3b_return_param | 回傳指向**參數**的 `&` | **PASS** | — |
| ex4_loop_ok | 外部借用在迴圈內使用 | **PASS** | — |
| ex5_loop_write | 迴圈內寫被借者（後向邊閉包） | **FAIL** | E02@s4 |
| ex6_move | `&x` 活躍期 move＋move 後用 | **FAIL** | E07@s3＋E06@s4 |
| ex7_alias_use | 別名層：`b=&a` 活躍期間排他用 `a` | **FAIL** | E09@s4 |
| ex8_reassign | move 後重新初始化並使用 | **PASS** | — |
| ex9_two_sh | 兩條 sh 重疊共存 | **PASS** | — |
| ex10_mut_seq | mut 依序借用（前貸死後再借） | **PASS** | — |
| ex11_alias_chain | 別名鏈活度＋衝突 | **FAIL** | E09@s4＋E01@s5 |
| ex12_if_branch | 分支內使用不延長活度過分支（NLL 益處） | **PASS** | — |
| ex13_if_clash | 分支內交越（a 於分支內仍被用） | **FAIL** | E01@s3 |
| ex14_field_split | split borrow：`x.f` sh 與 `x.g` mut 共存 | **PASS** | — |
| ex15_field_clash | 整體 `x` mut 與 `x.f` sh 路徑衝突 | **FAIL** | E01@s3 |
| ex16_two_fn | 多 fn 檔案（僅第二 fn 違規；同名 place 不干擾） | **FAIL** | E02@s6 |

**oracle 差異測試（`oracle_check.py`，rustc 1.98.0）**：26 例（17 範例＋9 額外語料）判定 **26/26 一致**，0 虛假放行（M5=0）。rustc 錯誤碼對照：E01→E0502/E0499、E02→E0506、E03→E0503、E06→E0382/E0594 系、E07→E0505、E09→E0502/E0506（別名層）、E10→E0106。測試中驗證到四個重要的 NLL 實作細節：
1. **MIR 讀取消除**：`let _ = x;`（無副作用讀取）會被 MIR 消除，rustc 看不到該讀取；`let y = x;`（真實 MIR 讀取）才受 E0503 檢查。弦律 `use x` 對應後者。
2. **讀取 vs 寫入**：mut 借貸活躍期間，對被借者的**讀取**觸發 E0503（E03），**寫入**觸發 E0506（E02）；讀取不與 sh 借貸衝突。
3. **分支活度**：`if` 分支內的使用**不**延長活度過分支（ex12 與 rustc 一致）——`after`:=CFG 達達性使此語義自動成立，無需專門規則。
4. **place projection**：字段借用按路徑前綴判定衝突（整體 vs 子字段衝突、異字段共存）；整體 move 後寫子字段＝E0382（E06 字段子句，經 rustc 驗證）。

**證明樹範例（ex1，E01）**——代理拿到的「為什麼」：
```
eclash(s4)
└─ 規則 R20 eclash:
   lend(s2, x, a, sh)   (事實)
   lend(s4, x, b, mut)   (事實)
   after(s2, s4)   (事實)
   overlap(s2, s4, x)
   └─ 規則 R16 overlap:
      onregion(s2, s4)
      └─ 規則 R15 onregion:
         lend(s2, x, a, sh)   span_end(s2, a, s5)
         └─ 規則 R35 span_end:
            last_use_ref(s5, a)   reach(s2, s5)
```
每個錯誤都能**回溯到具體事實與規則**——這就是「局部、可定位、可解釋」反饋的最小實現。

**圓示**（`examples/out/ex1_clash.svg`）：藍弧 `a:&x`（s2→s5）與紅弧 `b:&mut x`（s4→s6）**交越**，交點標 ✕，出借點 s4 紅環；底部兩條法則。`ex5` 顯示紫色**後向邊迴路**把 `r` 的區間閉包到寫入點。

---

## 8. 與現有工作對照（定位）

| 系統 | 定位 | 與弦律差異 |
|---|---|---|
| **NLL / Polonius**（rustc） | 生產級、全域、fixpoint | 弦律是其**函數內保守精化**＋Datalog 規則化＋視覺化，作代理**內環**，非取代 |
| **RustViz** [9] | 教學、時間軸、人看 | 弦律圓示是**檢查器輸出**、**代理在環**、規則=幾何 |
| **REVIS** [10] | 生命週期錯誤視覺化給新手 | 弦律面向**代理修復**，附**證明樹**與**同一事實集** |
| **Soufflé** [7][8] | Datalog 引擎/基礎設施 | 弦律 P1+ 直接採用為生產引擎 |
| **SafeTrans / AkiraRust / RustAssistant** [1][3][4] | LLM 修 Rust 的**方法學** | 它們證「更好的反饋結構」有效；弦律提供**那個更好的結構** |

---

## 9. 參考

1. SafeTrans: LLM-assisted Transpilation from C to Rust. https://arxiv.org/html/2505.10708v2
2. Polonius — Loan analysis（Datalog 風格規則、Naive/Opt/LI 分級、LI 快速預檢）. https://rust-lang.github.io/polonius/rules/loans.html ；rustc_borrowck::polonius. https://doc.rust-lang.org/nightly/nightly-rustc/rustc_borrowck/polonius/index.html
3. AkiraRust: Re-thinking LLM-aided Rust Repair Using a Feedback-guided Thinking Switch. https://arxiv.org/html/2602.21681v1
4. RustAssistant: Using LLMs to Fix Compilation Errors in Rust Code（＋社群反饋）. https://news.ycombinator.com/item?id=43851143
5. Common Rust Compiler Errors（borrow 為最高頻）. https://reintech.io/blog/common-rust-compiler-errors-and-how-to-fix-them
6. Fixing Common Rust Borrow Checker Errors（錯誤碼占比表）. https://knowledgelib.io/software/debugging/rust-borrow-checker/2026 ；JetBrains 借用錯誤清單. https://youtrack.jetbrains.com/issue/RUST-10897/Borrow-checker-errors
7. Soufflé: On Synthesis of Program Analyzers. https://www.researchgate.net/publication/305258489_Souffle_On_Synthesis_of_Program_Analyzers
8. On fast large-scale program analysis in Datalog. https://souffle-lang.github.io/cc-paper
9. RustViz: Interactively Visualizing Ownership and Borrowing（VL/HCC 2022）. https://github.com/rustviz/rustviz ；https://ui.adsabs.harvard.edu/abs/2020arXiv201109012G/abstract
10. REVIS: An Error Visualization Tool for Rust. https://arxiv.org/pdf/2309.06640
11. RFC 2094 — Non-lexical lifetimes. https://rust-lang.github.io/rfcs/2094-nll.html
