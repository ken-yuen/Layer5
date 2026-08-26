# 弦律幾何法則（縱點節圖公理）

> 版本 v0.5-geo · 2026-08-27
> 文獻綜述 ＋ 法則訂立 ＋ 驗證契約
>
> **一句話**：圓示不是插圖，是借用不變式的**幾何模型**；法則是該模型上的**可判定謂詞**，與 Datalog 事實集**同構**。

配套：[`rules.dl`](rules.dl)（真值）· [`RULES32.md`](RULES32.md)（32 則錯誤）· [`gallery.html`](gallery.html)（32 張圓示）

---

## 0. 學到咩（文獻地圖）

查閱範圍：借用視覺化、區域推斷、所有權型、區間／弦圖論、圓形可視化。下面只記**可轉成法則**的結論。

### 0.1 視覺化工具：詞彙不同，幾何核相同

| 系統 | 文獻 | 幾何詞彙 | 對弦律的啟示 |
|---|---|---|---|
| **RustViz** | Luo et al., arXiv:2011.09012；VL/HCC 2022；後續 rustc 整合版 | 每變數一條時間軸；**點**=事件、**實/空心線段**=可否改綁、**箭頭**=move/copy、**右側曲線**=借用 | 曲線＝弧；空心/實心＝sh/mut。教學向、原需人工標註；弦律改為**檢查器輸出**。 |
| **REVIS** | Wang, MacLaren, Coblenz, HATRA 2023 / arXiv:2309.06640 | 三件套：**區域**（藍垂直區間）＋**箭頭**（紅＝違規使用、紫＝生命終結原因）＋**提示** | **錯誤＝使用點落在藍區之外**。開放區域（一端箭頭）＝逸出。詞彙刻意少，只畫錯誤。 |
| **Aquascope** | Crichton, Gray, Krishnamurthi, OOPSLA 2023 | 流敏感**權限**畫在路徑上（讀／寫／擁有） | 幾何應畫「檢查器怎麼想」，不是「程式怎麼跑」。權限＝弧跨內允許的點操作。 |
| **RustOwl** | cordx56/rustowl | 底線色：綠＝活、藍＝sh、紫＝mut、橙＝move、紅＝錯；波浪＝maybe-live | **確定活 / 可能活**要能畫。弦律 `onregion` 是確定活（CFG 達達）；分支 maybe-move 是 E18/E24。 |
| **BORIS / VRLifeTime** | Schott 碩論；Zhang et al. CCS 2020 demo | 生命週期著色、臨界區粉紅 | 區間可視化能抓編譯器**抓不到**的邏輯錯（雙鎖）。弦律刻意不進 unsafe／鎖。 |

**共同核（五家都承認）**：

1. **生命週期是區間**（代碼行或 CFG 點的子集），不是標籤。
2. **違規是點與區間的幾何關係**（點在區外、兩區重疊且互斥）。
3. **顏色編碼權限**：藍共享、紅／紫排他、灰死。
4. **視覺必須對齊源碼順序**（時間軸）。弦律選**時間向下**（閱讀方向），與 REVIS 垂直區間一致。

### 0.2 區域推斷：區間從哪來

| 來源 | 幾何含義 |
|---|---|
| **NLL RFC 2094**（Matsakis） | 生命週期＝**CFG 點集合**。變數在 P 活 ⇒ 其型內所有區域含 P。`'a : 'b @ P` 是**點敏感**的 outlives：從 P 出發、不離開 `'b` 的可達點都要進 `'a`。允許**空洞**（異於 RFC 396 的「支配樹前綴、不可斷」）。 |
| **NLL liveness+location**（baby steps 2017） | 不用連續區域保證健全，改用**活度約束**保證「從出借走到解參考的所有路徑」都在區域內。 |
| **Polonius** | 區域改讀成**借貸集合**（非點集合）。`'a : 'b`＝貸款子集。活貸款＝某活變數的區域含該貸。 |
| **RustCompCert / Polonius 形式化**（2026） | 區域＝參考**可能指向的地方集合**（別名分析）。 |

**弦律設計定理（已在 PLAN／DOCS）**正是 RFC 2094 的有限化：

\[
\mathrm{onregion}(L,Q) \iff \mathrm{reach}(L,Q) \land \mathrm{reach}(Q,E)
\]

即「出借點到終端使用的路徑上的點」＝NLL 區域。Datalog `reach` 取代 fixpoint。

### 0.3 所有權型：圓＝封裝邊界

Clarke, Potter, Noble（OOPSLA 1998）**Ownership Types**；後續 Universe Types、Ownership Domains（Aldrich & Chambers, ECOOP 2004）：

- **owners-as-dominators**：從根到物件的所有路徑必經其 owner。
- 圖上畫一圈**想像邊界**，禁止外→內的直接引用（虛線箭頭＝違規）。
- `inside`＝擁有樹的巢狀；`world`＝根。

**這就是弦律的同心圓**：作用域圓＝owner 邊界；弧端點掉出圓＝穿過邊界的引用＝E04/E10。ROOT（`n0`）＝`world`。

### 0.4 圖論：弦交越 ＝ 區間重疊（定理，不是比喻）

Gavril（1973）／Golumbic *Algorithmic Graph Theory*：

> **圓的相交弦 ≡ 線上的重疊區間。**
> 兩條弦相交 ⟺ 對應區間**真重疊**（非包含的端點相觸可不算交）。

這給法則①一個**數學身分**：

- 把每條借用畫成時間軸上的區間 \([L, E]\)；
- 再把區間「包」到圓上成弦；
- **弦交越當且僅當區間重疊**。

因此「紅弧不得與他弧交越」＝「mut 區間不得與衝突路徑的他區間真重疊」＝區間圖上的獨立集約束。

區間圖本身來自**暫存器分配**（Chaitin 等）：變數同時活 ⇒ 相鄰 ⇒ 著色數＝暫存器數。借用檢查是**帶顏色約束的區間著色**：

- sh = 可共享的色（同一色可重疊）；
- mut = 獨佔色（與任何色都不准重疊）。

### 0.5 圓形圖（chord diagram）的視覺規律

Process-mining chord diagrams、CHORDination（ACM 2024）：

- 節點＝弧段，互動＝弦；厚度／方向編碼流量。
- **交越是認知負荷**：弦越多越難讀，要分 lane、降透明度、或只標衝突弦。
- 弦律已用「每被借者一 lane、凸度錯開」降低重疊——這是圖論建議，不是美學。

### 0.6 對弦律的總結（學到的設計約束）

1. **事實在先、圖在後**（RustViz 2 / Aquascope）：圖必須是檢查器的渲染，否則代理會修錯圖。
2. **錯誤＝點∉區 或 兩區非法重疊**（REVIS）：詞彙保持三種（點、弧、圓）就夠。
3. **區間允許空洞**（NLL）：分支不延長活度＝弧在 if 圓處「斷開」；不要畫成實心包住整個 fn。
4. **圓＝支配邊界**（ownership types）：巢狀圓的包含＝`inside`。
5. **弦交越＝區間重疊**（Gavril）：✕ 是定理實例，不是裝飾。
6. **maybe-live 要另畫**（RustOwl 波浪線）：E18/E24 是「一條路徑消耗、join 後仍用」——應標虛弧或分叉色，而非假裝整段確定活。

---

## 1. 模型：縱點節圖是什麼

四元組 \(\mathcal{G} = (V, P, A, C)\)，外加 CFG 邊 \(E\) 與作用域樹 \(T\)。

| 符號 | 名稱 | 定義 | 視覺 |
|---|---|---|---|
| \(V\) | 陳述 | `.cl` 的每句 | 縱軸上的**點**（時間向下） |
| \(P \subseteq V\) | 點集 | \(V\) 本身 | 灰／出借藍紅／錯紅環 |
| \(A\) | 弧 | 每條 `lend(L, Q, T, K)` 配終端 \(E\)，得弦 \((L\!\to\!E, K, Q)\) | 貝茲弦：sh 細藍、mut 粗紅 |
| \(C\) | 圓 | 每個作用域 \(N\) 的陳述子樹包成橢圓 | 虛線；fn/block 灰、loop 紫、if 青、else 琥珀 |
| \(E\) | CFG | `edge`／`reach` | 隱含於點序；loop **後向邊**畫左側紫迴路 |
| \(T\) | 作用域樹 | `parent`／`inside` | **同心＝巢狀** |

**時間嵌入**：\(\tau : V \to \mathbb{R}\)，\(\tau(s_i) = i\)（嚴格遞增）。弧的**區間**為

\[
I(a) = \{ Q \in V \mid \mathrm{onregion}(L_a, Q) \}
\]

注意 \(I(a)\) 不必是 \([\tau(L), \tau(E)]\) 的實心閉包——分支可造成**空洞**（NLL）。圓示目前用實心貝茲近似；法則以 \(I(a)\) 為準，圖為投影。

**路徑衝突**：\(Q_1 \bowtie Q_2\) 當且僅當 `path_conflict`（相等或字段前綴）。

---

## 2. 十條幾何法則（正式訂立）

每條：**陳述** · **幾何** · **Datalog** · **對應錯誤** · **文獻來源** · **驗證**。

### 法則 0　同構（圖不立法）

> \(\mathcal{G}\) 是事實集 \(F\) 的渲染。任何 ✕、逸出箭頭、紅環**必須**由證明樹產生，禁止圖上獨立推導。

- **為何**：RustViz 1 人工標註會與 rustc 漂移；RustViz 2 / Aquascope 改為編譯器視圖。代理修圖＝修錯世界。
- **驗證**：每個錯誤標記 \(\leftrightarrow\) `ERROR_CODES` 中一條已證事實。

### 法則 1　紅弧孤立（Exclusive Chord Isolation）

> 對路徑衝突的兩弧 \(a,b\)（\(Q_a \bowtie Q_b\)），若至少一者 \(K=\mathrm{mut}\)，則 \(I(a) \cap I(b) = \emptyset\)。
> 幾何：mut 弦不得與衝突弦**交越**，其弧跨內不得含他弧端點。

- **圖論**：衝突 mut 弧在區間圖上是獨立集。Gavril：弦交 ⟺ 區間真重疊。
- **Datalog**：`eclash` / `overlap` / `onregion`（E01）；寫／讀落弧跨＝E02/E03；別名層＝E09；呼叫讀＝E25。
- **文獻**：RustViz「mut 活時 owner 無線段」；REVIS「已 mut 再借」；NLL「貸款條款」。
- **驗證**：對每對衝突弧，若 `overlap` 則 SVG 必有 ✕；無 `overlap` 則禁止 ✕。v0.5：✕ 只標 `eclash` 證明（法則 0）。

### 法則 2　弧在圓內（Chord-in-Circle / Dominators）

> 弧 \(a\) 的兩端點（出借點、終端使用）的作用域必須 `inside` 被借者 \(Q\) 的作用域圓。
> 回傳／存槽之被借者必須活在 ROOT 圓（`n0`）——即 caller 的邊界之外仍有效。

- **型理論**：owners-as-dominators。圓＝封裝膜；穿膜＝representation exposure。
- **Datalog**：`edangle`（E04）、`ereturn`（E10）、`eescape`（E16）、`erettemp`（E28）、`estoretemp`（E17）。
- **文獻**：REVIS 開放區域＋向上箭頭；Clarke et al. 虛線越界引用。
- **驗證**：E04/E10/E16/E28 畫逸出箭頭；端點 \(\tau\) 必落在對應橢圓的 \(y\) 範圍內（除逸出弧）。

### 法則 3　點序守紀（Point-Order / Consume-Prefix）

> 對路徑 \(P\)，在 CFG 上一條從消耗點 \(D\)（`move`/`drop`/`callmv`）到其後點 \(S\) 的路徑上，若無覆蓋寫入（`reinited`），則 \(S\) 不得讀 \(P\)。

- **幾何**：每個地方的「有效區間」是從最近一次 init 到下一次 consume 的**前綴**；consume 之後的點是區外。
- **Datalog**：E05/E06/E07/E08/E18/E19/E20/E24/E26/E27/E29/E30。
- **文獻**：REVIS 圖 1「紅箭頭在藍區外」；RustViz move 後無線段；區間圖的左端＝alloc、右端＝free。
- **驗證**：每個 E06/E05 的 \(S\) 滿足 \(\mathrm{after}(D,S)\)。

### 法則 4　區間著色（Interval Chromatic / 2-permission）

> 在任一 CFG 點 \(Q\)，令 \(L_Q = \{ a \in A \mid Q \in I(a) \}\)。對每個路徑衝突類：
> - mut 弧至多 1 條；
> - 若已有 mut，則不得再有任何弧；
> - 任意多條 sh 可共存。

- **這是法則 1 的點切片**。著色：sh＝可重疊色、mut＝獨佔色。
- **文獻**：暫存器分配區間著色；Rust「XOR mut，OR sh」。
- **對應**：E01 在出借點切片；E02/E03 在寫／讀點切片。
- **驗證**：對每個 \(Q\)，統計 `onregion` 計數。

### 法則 5　同心圓包含（Nested Circles = Region Inclusion）

> 作用域 \(N\) 在 \(M\) 之內 \(\iff\) 圓 \(C_N \subseteq C_M\)（同心、半徑更小）。
> 詞法級：參考區域 \(\subseteq\) 被借者圓。
> NLL 級：\(I(a) \subseteq \{Q \mid \mathrm{reach}(\mathrm{decl}_Q, Q)\}\)（被借者仍可達）。

- **文獻**：ownership `inside`；NLL `'a : 'b` 的點敏感子集。
- **Datalog**：`inside`/`anc`/`same`；lexical 的 `span_end = scope_last`。
- **驗證**：SVG 橢圓半徑隨 `depth_of` 遞減；子樹陳述的 \(y\) 落在父橢圓內。

### 法則 6　後向邊閉包（Back-edge Closure）

> 迴圈後向邊 \(e^\hookleftarrow \in E\) 進入 `reach` 後，凡終端使用在迴圈子樹內的弧，其 \(I(a)\) **自動覆蓋整圈**。

- **幾何**：紫迴路不是裝飾，是把弧的區間沿環**向前閉包**。
- **文獻**：NLL「迴圈內使用的借用活於整個迴圈」；RFC 2094 的 DFS 不離開區域。
- **對應**：ex5 / r02 在 loop 內寫＝E02，無需專門規則。
- **驗證**：`loop` 子樹末→首有 `edge`；`onregion` 含迴圈內所有點。

### 法則 7　菱形不串線（Diamond Non-sequencing）

> if 與 else 是**兄弟**，無 sequential 邊。一分支內的使用**不得**經兄弟把活度延長到另一支，也不得單獨延長過 join（NLL 益處：無 else 時分支內使用不延長過分支）。

- **幾何**：if 青圓、else 琥珀圓並排；弧若只在一圓內使用，出圓即死。
- **文獻**：RFC 2094 Problem Case #2（條件控制流）；弦律 ex12。
- **對應**：E18/E24 是「消耗在一支、使用在 join 後」——maybe-live，**不是**確定重疊。
- **驗證**：兄弟作用域之間無 `edge`；`after` 只沿菱形的兩腰。

### 法則 8　路徑前綴衝突（Path-Prefix Conflict / Split）

> 兩弧衝突當且僅當其被借者滿足前綴關係。異字段（`x.f` 與 `x.g`）**不衝突**，可同時畫兩條不交越的弧（split borrow）。

- **幾何**：整體弧的 lane 覆蓋子字段 lane；子字段各有獨立 lane。
- **文獻**：NLL place projection；Polonius path。
- **對應**：E01 用 `path_conflict`；ex14 PASS、ex15 FAIL；E22/E23 是 imm 的字段切片。
- **驗證**：`path_conflict(x.f, x.g)` 為假。

### 法則 9　別名鏈投影（Alias-Chain Projection）

> 使用參考 \(T\) ＝ 使用 `ref_of(T)` 最終被借者。圖上：對 \(T\) 的點，垂直投影到被借者的弧跨上。

- **文獻**：Aquascope 路徑權限；Polonius origin 活度；弦律 `tuse`。
- **對應**：E09（參考自身被借時不可排他用）；NLL `span_end` 經別名閉包。
- **驗證**：`use_ref` 能推出 `usep`／`tuse`。

### 法則 10　權限點（Permission Points / Aquascope）

> 點 \(Q\) 落在弧 \(a\) 的 \(I(a)\) 內時，允許的操作取決於 \(K_a\)：
>
> | \(K\) | 允許 | 禁止 |
> |---|---|---|
> | sh | 讀 \(Q_a\)、再 sh 出借 | 寫、move、drop、mut 出借、`set *`、`callmv` |
> | mut | 經該參考讀寫 | 直接讀寫被借者、任何其他出借、move/drop/`call` 被借者 |
>
> 經 sh 寫＝E13；經參考移出＝E14——**弧不是所有權**，不能把點從弧上抽走。

- **文獻**：Aquascope 流敏感權限；RustViz mut 時 owner 無線段。
- **驗證**：E02/E03/E07/E08/E13/E14/E25/E27 各是表中一格。

---

## 3. 三條口訣 ↔ 十條法則

圓示底部印的三條是**對外口訣**；十條是**對內公理**。

| 口訣 | 覆蓋法則 | 代理可執行的修法 |
|---|---|---|
| ① 紅弧孤立 | 1, 4, 8, 10 | 縮短其中一弧（把終端使用移到交越點前）或 clone |
| ② 弧在圓內 | 2, 5, 9 | 把端點移進圓，或讓被借者升到 ROOT（參數化） |
| ③ 點序守紀 | 3, 6, 7 | 先用再消耗；消耗後先 `set` 再讀；分支外重初始化 |

法則 0 永遠成立（元法則）。

---

## 4. 規劃：怎樣「訂立」而不只是「寫下」

訂立＝可審查、可測試、可演化。分四層。

### P-G0　規格凍結（本檔，已完成草稿）

- [x] 文獻地圖
- [x] 十條法則 ↔ Datalog ↔ 錯誤碼
- [x] 法則 1/2/5/6/7/8 的 `TestGeometry` 守衛（見 P-G1）

### P-G1　幾何測試（已完成，v0.5）

在 `test_chordlaw.py` 加 `TestGeometry`：

| 測試 | 斷言 |
|---|---|
| `test_law1_cross_iff_overlap` | `eclash` 存在 ⇒ 兩弧區間在 \(\tau\) 上真重疊 |
| `test_law1_no_false_x` | 兩 sh 重疊（ex9）⇒ 無 ✕ |
| `test_law2_endpoints_in_circle` | 非逸出弧的 \(L,E\) 之 \(y\) 落在被借者作用域橢圓 |
| `test_law5_nested_radii` | `parent(N,M)` ⇒ \(r_N < r_M\) |
| `test_law6_backedge_closes` | loop 內 `use` ⇒ `onregion` 含 loop 首 |
| `test_law7_no_sibling_edge` | if/else 兄弟無 `edge` |
| `test_law8_split_no_conflict` | `x.f`/`x.g` 兩弧可 `onregion` 同點 |

**出口**：法則 1/2/5/6/7/8 有自動守衛；✕ 改為由證明驅動（去「純幾何猜交越」）。✅

### P-G2　圖與事實對齊（已完成，v0.5）

- [x] SVG ✕ 只畫在 `eclash` 的兩弧樣本點，不掃所有 mut 對；無幾何交點則標在首個重疊點。
- [x] maybe-live（E18/E24）畫琥珀虛線（RustOwl 波浪）＋「可能活」標籤。
- [x] 空洞區間：實心只覆蓋 I(a)，文本跨度內的空洞改虛線。

### P-G3　形式化（可執行引理已落地；Coq/Lean 仍屬 PLAN P4）

引理（擬）：

1. **健全性（幾何 ⇒ 邏輯）**：若 \(\mathcal{G}\) 滿足法則 1–10，則 Datalog 不推出 E01–E32。
2. **完備性（邏輯 ⇒ 幾何）**：每條 E0k 對應恰好一條法則的一次違反（允許伴隨錯誤，如 E07+E06）。
3. **Gavril 引理**：在無空洞、單一 lane 的嵌入下，兩衝突弧弦交越 ⟺ `overlap`。

載體：`TestGeometry.test_lemma_{soundness,completeness,gavril}` 為可執行引理；P4 再 Coq/Lean。

### P-G4　代理接口（已完成，v0.5）

`--explain` 已有「幾何: 點落在弧跨內」。補：

```
幾何違反: 法則① 紅弧孤立
  弧 a : &x   I = {s2,s3,s4,s5}
  弧 b : &mut x  I = {s4,s5,s6}
  I(a) ∩ I(b) = {s4,s5} ≠ ∅
  修: 把 a 的終端使用從 s5 移到 s3（縮短弧）
```

讓代理的操作是**縮弧／移點／升圓**，不是猜 rustc 訊息。`geometry_report()` 產出此塊。

---

## 5. 與 32 則錯誤的對照表

| 法則 | E 碼 |
|---|---|
| 0 同構 | （全部標記） |
| 1 紅弧孤立 | E01 E09 |
| 2 弧在圓內 | E04 E10 E16 E17 E28 |
| 3 點序 | E05 E06 E18 E19 E20 E24 E26 E29 E30 |
| 4 區間著色 | E01 E02 E03 E25 |
| 5 同心包含 | E04 E10（詞法切片） |
| 6 後向閉包 | E02/E03 在 loop 內 |
| 7 菱形 | E18 E24；ex12 PASS |
| 8 路徑前綴 | E01 字段；E22 E23；ex14/15 |
| 9 別名投影 | E09；`tuse` |
| 10 權限點 | E02 E03 E07 E08 E13 E14 E25 E27 |

E11/E12/E31/E32（imm）是**點上的顏色約束**（空心線段不可寫），歸入法則 10 的「綁定可變性」切片；E21 是別名圖成環，歸入法則 9 的對偶（投影不可指向自身）。

---

## 6. 參考（本檔用過的）

1. Luo, Reddy, Almeida, Zhu, Du, Omar. *RustViz: Interactively Visualizing Ownership and Borrowing*. arXiv:2011.09012, 2020. （後續 rustviz/rustviz rustc 整合版）
2. Wang, MacLaren, Coblenz. *REVIS: An Error Visualization Tool for Rust*. HATRA 2023 / arXiv:2309.06640.
3. Crichton, Gray, Krishnamurthi. *A Grounded Conceptual Model for Ownership Types in Rust*（Aquascope）. PACMPL OOPSLA 2023.
4. Matsakis. RFC 2094 — Non-lexical lifetimes. https://rust-lang.github.io/rfcs/2094-nll.html
5. Matsakis. *Non-lexical lifetimes using liveness and location*. baby steps, 2017.
6. Matsakis. *Polonius and region errors*. baby steps, 2019.
7. Clarke, Potter, Noble. *Ownership Types for Flexible Alias Protection*. OOPSLA 1998.
8. Aldrich, Chambers. *Ownership Domains: Separating Aliasing Policy from Mechanism*. ECOOP 2004.
9. Clarke, Drossopoulou. *Ownership, Encapsulation and the Disjointness of Type and Effect*. OOPSLA 2002.（owners-as-dominators）
10. Gavril. *Algorithms for a maximum clique and a maximum independent set of a circle graph*. Networks, 1973.（弦交 ≡ 區間重疊）
11. Golumbic. *Algorithmic Graph Theory and Perfect Graphs*.（circle graph / overlap graph）
12. Chaitin et al. Register allocation via coloring.（區間圖著色）
13. RustOwl — visualize ownership and lifetimes（確定活／maybe-live 底線）
14. Zhang, Qin, Chen, Song, Zhang. *VRLifeTime*. CCS 2020 demo.
15. Jalali. *Supporting Social Network Analysis Using Chord Diagram in Process Mining*. 2016.
16. *CHORDination: Evaluating Visual Design Choices in Chord Diagrams*. ACM 2024.

---

## 7. 現況與下一承諾

**已訂立**：本檔十條法則＋三條口訣＋32 則對照。真值仍以 `rules.dl` 為準（法則 0）。

**v0.5 已落地**：P-G1 `TestGeometry` 七項＋✕ 由證明驅動；P-G2 虛線空洞／可能活；P-G3 可執行引理；P-G4 `--explain` 幾何違反塊。下一刀屬 PLAN P4（Coq/Lean）與多 fn 圓示。
