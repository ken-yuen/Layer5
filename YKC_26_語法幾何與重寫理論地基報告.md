# YKC_26 — 語法・幾何・重寫的理論地基:結構 Worker 的六大基礎

> 日期:2026-08-25 ｜ 基線:`ykc-serve-datalog` ｜ 承接:`YKC_25`、`YKC_20_能力包解耦與組合架構.md`、`YKC_19_T19_L1依賴對齊與T20_L2結構統計實作規劃.md`
> 性質:**理論報告**——為 T-20 結構 worker(`ykc-structure`,尚未落庫)立定律法與驗收準則;本輪不落代碼。
> 載體:雙軌——**CL0** 演示語言(定律驗證用,附錄 A)+ **R₀** Rust 子集(實用接線用,附錄 B)。

## 0. 一張圖:六大基礎為什麼是一件事

```
形式語言          邊界在哪裡——哪些性質可判定、哪些必須交給 oracle
  └─ 自動機理論    用什麼機器讀——DFA 讀 token、(G)LR PDA 讀結構
      └─ 表面語法樹  讀出來的結構是什麼——無損、具名、帶位址的樹
          └─ 抽象代數  結構服從什麼定律——無損回環、增量正確、ERROR 包攝
              ├─ 幾何拓撲  定律在哪裡變成可見的幾何——區間、嵌套、弦圖、紅邊
              └─ 重寫系統  幾何如何被規則收斂——修法菜單的終止性與合流性
```

六層不是六門課,是**同一條信任鏈的六個斷面**:語言層級決定判定權歸屬(YKC 第一性原理的數學形式);
自動機決定了樹是怎麼被生產的;樹的代數律決定了它可不可以被增量維護而不出錯;
幾何把代數律投影成人與代理都看得見的圖形(YKC_12 的「代理可讀幾何」);
重寫系統保證「看得見的錯誤」能被**確定性地**修掉。

與 YKC 現有資產的對應(全部已存在,本報告只是把它們的數學內容顯式化):

| 基礎 | 已存在的對應物 | 本報告新增 |
|---|---|---|
| 表面語法樹 | `internal/borrow/extract.go`(文字拓撲抽取,啟發式) | 形式定義 + 無損性公理;R₀ CST 為其原則化升級路徑 |
| 抽象代數 | `internal/eventstore`(冪等=內容決定論)、`internal/datalog`(決定論排序) | 樹操作的九條定律 L1–L9 = 未來 worker 的驗收矩陣 |
| 幾何拓撲 | `internal/borrow`(區間代數+衝突圖+紅邊)、`l5/chordlaw`(rules.dl) | 嵌套定理、弦圖/完美圖定位、「紅邊清零」的數學化 |
| 重寫系統 | `internal/borrow/rulecard.go`(封閉修法菜單)、judge 的 cargo fix 閉環 | 遞減測度 + Newman 引理 ⇒ 終止且合流 ⇒ 修法確定性 |
| 自動機 | `internal/toolchain`(cargo JSON = 上游 oracle 輸出) | lexer/parser 機器模型與增量重析的配置快照語義 |
| 形式語言 | YKC_22「判定權不轉移」紀律 | Rice 定理:該紀律是數學必然,不是設計偏好 |

---

## 1. 表面語法樹(Surface Syntax Tree)——無損事實層

### 1.1 CST 不是 AST:為什麼 YKC 必須要「表面」

抽象語法樹(AST)丟棄標點、括號、註釋、空白——這對編譯器夠用,對 YKC 不夠,因為 YKC 的三個核心承諾都錨定在**逐字節的源碼位置**上:

1. **證據錨定**:L5 解釋的每個語句節點都帶 `sN ← file:line` 錨(YKC_12);AST 丟行號精度則錨失效。
2. **無損重寫**:修法菜單的輸出要能直接回填源碼(或給出最小 diff);丟 trivia 的重寫會重排用家的代碼——不可接受。
3. **代理可讀**:代理與人類讀同一份文本;樹與文本必須逐字節互推(見 L1 定律)。

因此 T-20 worker 的底層表示是**表面語法樹**(concrete syntax tree, tree-sitter 語境的 "syntax tree"):每個 token 都在樹上,trivia(空白/註釋)作為附著資訊保留。

### 1.2 形式定義

一棵表面語法樹是四元組 `T = (V, E, ℓ, σ)`:

- `V`:節點集;`E ⊆ V×V`:父子邊(樹:連通、無環、每節點至多一父)。
- `ℓ: V → K`:標籤(節點種類,如 `fn_item`、`let`、`"+"`)。`K` 分兩類:**具名**(named,如 `fn_item`)與**匿名**(anonymous,如 `"("`、`"+"`)。
- `σ: V → ℤ×ℤ`:位址映象,`σ(v) = [a,b)` 為源碼字節區間(半開)。

**連續性公理**(surface 的實質內容):對每個內部節點 `v`,設其子節點按序為 `c₁…c_k`,則

```
σ(v) = [σ(c₁).start, σ(c_k).end)   且   ∀i: σ(cᵢ).end ≤ σ(cᵢ₊₁).start
```

即:父區間恰好覆蓋子區間的並(允許子節點之間有 trivia 縫隙)。trivia 本身要嘛建模為節點(tree-sitter 的 extras),要嘛附著在 token 上——兩種建模都必須滿足:**所有葉子的文本按序拼接 = 源碼原文**(這就是 L1,見 §2)。

### 1.3 具名投影

具名節點樹 = 把匿名節點投影掉:`named(T)` 保留 `ℓ(v)` 為具名的節點,父子關係取「最近的具名祖先」。投影 `π` 是樹同態:保持根、保持具名節點間的祖先序。

**為什麼要分兩層**:裁判事實(結構統計、借用抽取)只依賴具名層——`"+"` 有沒有括號不改變 `binary_expression` 事實;但重寫與錨定依賴全層。投影定律(L6)保證兩層永遠一致:**具名層是表面層的函數,不存在第二套抽取**。

> 對賬現有代碼:`internal/borrow/extract.go` 目前直接從文本啟發式抽取語句拓撲。R₀ CST 落地後,抽取改為 `named(parse(src))` 上的結構遞歸——同一輸出格式(`.cl` 事實),但每個事實自帶精確 span。這正是 YKC_20 所說「worker 輸出必須可錨定」的具體化。

---

## 2. 三種樹的抽象代數

三種樹——具名節點樹、增量樹、ERROR 節點樹——不是三個功能,是同一代數結構的三個面:**對象是樹,態射是編輯/投影/重寫,定律是它們的交換圖**。

### 2.1 編輯單體(edit monoid)

一次編輯 `e = (start, old_end, new_end, text)`。定義複合 `e₁·e₂`:先施 `e₁`,再把 `e₂` 的位址經 `e₁` 的位移函數平移。位移函數:

```
shift_e(p) = p                            若 p ≤ start
           = p + (new_end − old_end)      若 p ≥ old_end
           = ⊥(落在被替換區內,未定義)
```

**命題(單體律)**:編輯在 `·` 下構成單體(單位元 = 空編輯;結合律在位移一致意義下成立)。
證明要點:位移函數的複合恰為平移量之和,結合律化為整數加法結合律。∎

這條律的實用意義:**去抖批次內的多個檔案事件可以按任意順序歸併成一個總編輯**(watch 層的去抖在語法層有代數依據,而不是「看起來沒事」)。

### 2.2 增量樹:reparse 是單體作用

增量重析 `reparse: T × Src × Edit → T′` Reuse 準則:子樹 `S` 可重用,當且僅當

1. `σ(S)` 與編輯區不相交;且
2. `S` 左邊界處的**解析器配置**(自動機狀態 + 棧摘要,見 §5.3)與新源碼在該處的 configuration 相同。

**定律 L3(增量正確性)**:若 reuse 只發生在準則成立處,則

```
reparse(parse(s), s, e) ≡ parse(edit(s, e))
```

`≡` 指**序列化樹逐字節相等**(結構共享是實現細節,序列化不可見)。這是整個增量機制唯一需要證明的性質——其餘(性能、共享率)只是優化。

**定律 L4(作用的單體相容)**:`reparse(T, e₁·e₂) ≡ reparse(reparse(T, e₁), e₂)`。
L3 + L4 合起來說:**增量路徑與全量路徑對任意編輯序列收斂到同一棵樹**——裁判重放對賬(YKC_18 哲學)在語法層的對應物。

### 2.3 ERROR 節點樹:全化(totalization)

解析器必須是**全函數** `parse: Src → T`——任何輸入(包括半句代碼、代理寫到一半的檔案)都產出樹。做不到全化的解析器會讓 serve 的監看循環在代理每次儲存半成品時崩潰,這是常駐進程不可接受的。

全化的代價是 ERROR 節點:自動機卡死時,把最小不可解析區間封成 `ℓ(v) = ERROR` 的子樹掛在原位。三條包攝律:

- **L7a(不假報)**:`s` 可完整解析 ⇒ `parse(s)` 無 ERROR 節點。
- **L7b(良構極大)**:把每個 ERROR 子樹的 span 挖掉後,剩餘森林全部良構可解析。
- **L7c(ERROR 極小,設計目標)**:不存在 ERROR span 的真子區間滿足 L7b。實作上 best-effort(panic-mode 恢復不保證嚴格極小),但**必須保證 L7a/L7b**——ERROR 是「如實申報的卡死點」,不是垃圾場。

### 2.4 九條定律 = 驗收矩陣

| # | 定律 | 形式化 | 載體 |
|---|---|---|---|
| L1 | 無損回環 | `unparse(parse(s)) ≡ s`(逐字節) | CL0 + R₀ |
| L2 | 決定論 | `s = s′ ⇒ parse(s) = parse(s′)`(序列化相等) | CL0 + R₀ |
| L3 | 增量正確 | `reparse(parse(s), s, e) ≡ parse(edit(s,e))` | CL0 + R₀ |
| L4 | 編輯單體作用 | `reparse(T, e₁·e₂) ≡ reparse(reparse(T,e₁), e₂)` | CL0 |
| L5 | 區間嵌套 | 任意兩節點 span 要嘛嵌套要嘛不交(§3.1 定理) | CL0 + R₀ |
| L6 | 投影一致 | `named(reparse(…)) ≡ named(parse(edit(…)))`(具名層繼承 L3) | CL0 + R₀ |
| L7 | ERROR 包攝 | L7a 不假報 + L7b 良構極大(注入式測試) | CL0(窮舉小樣)+ R₀(真實半截檔) |
| L8 | 紅邊遞減 | 修法菜單每條規則嚴格遞減測度 μ(§4.2) | CL0 |
| L9 | 合流唯一 | 菜單規則終止 + 局部合流 ⇒ 唯一正規形(Newman) | CL0 |

**紀律**:這九條律的每一條在 T-20 落庫時都是**具名測試**(如 `TestLawL3IncrementalEquivalence`);律先於碼——測試矩陣就是 worker 的驗收合同。這與 YKC_22 的契約測試(cargo JSON 不漂移)同一方法論:**把口頭承諾變成機械可判**。

---

## 3. 幾何拓撲——從嵌套到弦圖

### 3.1 跨度區間與嵌套定理

**定理(區間嵌套/laminar)**:任何滿足連續性公理的樹,其節點 span 族是 laminar 族——對任意 `u,v ∈ V`:

```
σ(u) ⊆ σ(v)  ∨  σ(v) ⊆ σ(u)  ∨  σ(u) ∩ σ(v) = ∅
```

證明(反證):設 `σ(u)、σ(v)` 部分重疊且互不包含。取 `w = lca(u,v)`;由連續性,`w` 的子節點區間按序不交,`u、v` 必分屬 `w` 的兩個不同子樹,於是 `σ(u)、σ(v)` 不交——矛盾。∎

這就是 L5 定律,也是「樹 = 嵌套括號幾何」的精確內容。**語法樹永遠不會出現部分重疊**——部分重疊只出現在語義層(見下),而部分重疊正是衝突的形狀。

### 3.2 語義區間:liveness 投影

`l5/chordlaw/liveness_*.dl` 為每個引用/存取事件算出**活躍區間**(lexical:塊範圍;NLL:最後使用點;referent:被借物壽命)。這些區間投影到與語法 span **同一條線**(源碼字節軸)上。於是:

- 語法層:span 族 laminar(定理)——**合法形狀**。
- 語義層:兩個 mutable 存取的活躍區間若相交(含部分重疊)——**非法形狀**。

借用檢查的幾何本質一句話說盡:**借用錯誤 = 語義區間違反了語法區間天生享有的 laminarity**。代理修 borrow 錯誤,就是把相交的區間推成不交(縮短壽命/複製/重排)——這正是 `internal/borrow` 幾何規則卡「兩條法則 + 封閉修法菜單」的數學內容。

### 3.3 衝突圖是區間圖,區間圖是弦圖

構造衝突圖 `G = (V, E)`:`V` = 存取事件;`{u,v} ∈ E` 當且僅當活躍區間相交且相容性規則被違反(`&mut ↔ &mut` 或 `&mut ↔ &`)。

**定理**:`G` 是**區間圖**(interval graph),而區間圖 ⊂ **弦圖**(chordal graph,每個 ≥4 的環都有弦),弦圖 ⊂ **完美圖**(perfect graph)。

三個推論,每個都有 YKC 語義:

1. **完美圖定理 `χ(G) = ω(G)`**:最大團(同一時刻最多幾個互斥借用共存)恰好決定著色數(需要幾條「可用通道」)。代理問「要複製幾次才夠」,幾何直接回答:**數最大團,不多不少**。
2. **最大團 = 掃描線算法**:區間圖的最大團可在 `O(n log n)` 內按端點掃描求出——面板上「紅邊計數」的收斂過程因此有單調可驗的中間量。
3. **命名迴響**:弦(chord)正是「把環補上弦」的那條邊——衝突圖在數學上是 chordal 的,`ChordLaw` 之名與之共鳴(非考據;數學事實如此)。

### 3.4 樹是 1 維 CW 複形;編輯是手術

把樹視為 1 維 CW 複形(節點 = 0 胞腔,邊 = 1 胞腔):樹可縮(contractible),歐拉示性數 `χ = |V| − |E| = 1`。編輯(reparse 的子樹替換)是**手術**(surgery):切開一個子複形、換入同邊界的另一個。L3 增量正確性在拓撲語言裡就是:**手術後的重建與整體重造同胚且同址**——幾何不會因為「只動了一小塊」而漂移。

### 3.5 「紅邊清零」的數學化

YKC_12 的驗收判據「紅邊清零 = 幾何收斂」現在有了精確陳述:

```
收斂 ⇔ E(G) = ∅ ⇔ 活躍區間族恢復 laminar 相容 ⇔ 借用規則全部滿足(rustc 認同的必要幾何面)
```

注意方向:紅邊清零是**必要幾海面**,不是充分判定(判定權在 rustc,§6.3)——但它是**機械可驗的中間里程碑**,這正是它對代理有用的原因:代理看不見類型系統,看得見區間相交。

---

## 4. 重寫系統——修法菜單的終止與合流

### 4.1 抽象重寫系統(ARS)

一個 ARS 是 `(A, →)`,`A` = 程序(樹)集合,`→` = 單步重寫。修法菜單(`internal/borrow/rulecard.go` 的封閉菜單)是一組**有限**的樹重寫規則 `l ⇒ r`,帶幾何側條件(命中哪個幾何族)。judge 的除錯閉環 = 策略(strategy):反覆選規則施用到紅邊,直到正規形或預算耗盡。

### 4.2 終止性:良基測度

定義測度 `μ(P) = (|E_red(P)|, |Err_rustc(P)|)`(紅邊數,再 rustc 錯誤數),字典序。

**L8(遞減律)**:菜單中每條規則的每個合法施用都嚴格遞減 `μ`。

逐規則檢查(菜單封閉 ⇒ 有限檢查):縮短壽命(把使用點移出區間)嚴格減相交數;複製(`clone`)把共享邊轉為獨立區間;重排語句改變端點順序,側條件要求相交數下降;`RefCell` 類運行期內飾把編譯期邊移出 `E_red` 並在事實層記 runtime-borrow 標記(不減 rustc 面時由第二分量兜底)。`μ` 取值於 `ℕ×ℕ` 字典序——**良基**,故重寫序列不可能無窮:**菜單施用必終止**。∎

### 4.3 合流性:臨界對 + Newman 引理

**Newman 引理**:ARS 若終止且局部合流(所有單步分歧可重新會合),則合流——正規形唯一。

菜單封閉 ⇒ 規則對有限 ⇒ **臨界對**(兩條規則重疊施用的所有情形)可窮舉檢查。CL0 載體上這是可機械窮舉的測試(L9):對每個臨界對 `(r₁, r₂)`,驗證任意分歧 `P → P₁, P → P₂` 存在會合點。R₀ 上抽樣 + 側條件收窄。

**為什麼 YKC 在乎合流**:合流 + 終止 ⇒ **修法結果與施用順序無關** ⇒ 裁判輸出可重放對賬(YKC_17 紀律)。不合流的修法菜單會讓「同一個錯誤、兩次修復、兩種結果」——代理與人類互相指責的開端。

### 4.4 歸約是同態:Rust → .cl 的錨定保持

judge 的 L5 路徑把錯誤現場的 Rust 片段**歸約**為 `.cl` 事實(`sN ← file:line`)。形式化:歸約 `ρ` 是保持 span 錨的樹同態——`ρ` 把 R₀ 樹映到 ChordLaw 事實集,且每個事實項都攜帶源樹節點的 span 引用。

**錨定保持律**:對每個事實 `f ∈ ρ(P)` 與每個代理可見的解釋 `x`,存在 `sN` 使 `x` 中每個語句都能回跳源碼行。這條律是「解釋可信」的機械準則:**解釋裡的每一句話都能被點回源碼驗證**——與帳本「每個結論都能回答『是誰說的』」(T-21c attest)同一紀律,只是錨從「工具鏈版本」換成「源碼位置」。

---

## 5. 自動機理論——用什麼機器讀

### 5.1 詞法層:DFA 與 Rust 的兩個非正則點

token 流由 DFA 產生(Rust 詞法基本是正則的:標識符、字面量、生命週期 `'a`、raw string `r#"…"#`)。兩個已知非正則/上下文點:

1. **`>>` 拆分**:泛型嵌套 `Vec<Vec<u>>` 的結尾在詞法層是一個 Shl token,在語法層必須讀成兩個 `>`。正解是**解析器回饋**(lexer 按當前語法狀態重切)或 GLR 歧義消解——tree-sitter 採後者。
2. **raw identifier / 字符串轉義邊界**:仍在正則內,但狀態數大——DFA 最小化後可控。

### 5.2 語法層:PDA、LR(1) 家族、GLR

CFG 的識別機是下推自動機(PDA)。工程上取 LR(1)/LALR(1) 表驅動(線性、決定、可增量快照)。Rust 語法有 LR(1) 不夠的點(表達式位置的 `<` 到底是比較還是泛型實參——需要看更遠的 token),故 tree-sitter 用 **Tomita GLR**:非確定 PDA 的確定性模擬,配置集用**圖結構棧**(graph-structured stack)共享公共後綴。

複雜度:一般 CFG 上 GLR 最壞 `O(n³)`;在 LALR(1)-乾淨片段上退化為近線性。R₀ 刻意選在 LALR(1) 可處理的片段內(附錄 B),**歧義點(泛型實參 vs 比較)在 R₀ 中用側條件排除**——這是「先落可判定片段」紀律的又一次套用。

### 5.3 增量 = 自動機配置快照

§2.2 reuse 準則的「解析器配置」現在有了精確含義:子樹左邊界處的 `(LR 狀態, 棧高度摘要)`(GLR 下為配置集的規範化指紋)。**增量重析 = 在邊界處回放快照、只重跑受影響區段**。L3 的證明義務因此變成:快照回放與從頭跑產生相同配置序列——這是可以在 CL0 上窮舉驗證的有限檢查(狀態數有限)。

### 5.4 ERROR = 自動機卡死點

ERROR 節點的精確定義:自動機在位置 `p` 無合法轉移,恢復例程把 `[p, q)` 封為 ERROR 並從 `q` 的同步點(語句邊界)重新入棧。L7a/L7b(§2.3)在機器語言裡就是:**恢復例程不改變卡死點之外的接受性**——卡死點之外,機器行為與完整解析逐配置一致。

---

## 6. 形式語言——判定權為什麼不轉移

### 6.1 Chomsky 層級與 YKC 五層的映對

| 層級 | 語言類 | 識別機 | YKC 對應 | 判定性 |
|---|---|---|---|---|
| Type-3 | 正則 | DFA | token 層;L1 依賴事實(cargo metadata JSON 的詞法) | 可判定 |
| Type-2 | 上下文無關 | PDA / (G)LR | **L2 結構事實(R₀/CL0 CST,T-20 worker)** | 可判定 |
| Type-1 | 上下文相關 | LBA | 名字解析、生命週期約束——**L5 ChordLaw 的 Datalog 不動點落在這層的可判定片段** | 片段可判定 |
| Type-0 | 遞歸可枚舉 | TM | 完整語義(單態化、trait 求解、mir 優化)——**rustc = 有限 oracle** | 一般不可判定 |

### 6.2 Datalog = Knaster–Tarski 不動點,必然終止

`internal/datalog` 的 naive 求值是 monotone 算子在有限冪集格上的迭代——由 **Knaster–Tarski 定理**收斂到最小不動點,迭代數 ≤ 基事實數。已有的 `MaxDerivedFacts`/`MaxRounds` 保險絲不是「防 bug」,是**把數學上的必然終止顯式化為資源上限**(觸頂 = 程式錯誤而非理論風險)。L5 的全部輸出因此**可重放、可對賬、決定論**——這是它能進裁判鏈而 rustc 輸出只能當 oracle 引用的原因:Datalog 的事實是 YKC 自己能重算的,rustc 的事實只能信其簽名收據。

### 6.3 Rice 定理:oracle 紀律是數學必然

**Rice 定理**:對 Type-0 語言的任何非平凡語義性質,判定問題不可判定。推論鏈:

1. 「這段 Rust 語義正確嗎」是非平凡語義性質 ⇒ 不存在一般判定程序;
2. 任何靜態分析(含 L5)只能是**保守近似**(sound approximation):說「有錯」可信,說「沒錯」不可信;
3. 因此**判定權必須留在會停機、會給出確定答案的執行體上**——對單個輸入,rustc 就是這個有限 oracle(YKC_22 的「判定權不轉移」)。

YKC 五層因此不是權宜分層,是**判定性邊界的直接拓印**:worker 產事實(Type-2/3,可重算)、Datalog 推幾何(不動點,可重放)、rustc 下結論(oracle,收據入帳)、L5 給解釋(錨定回源,§4.4)。每一層的權力恰好等於它的判定能力——**多一分是欺騙,少一分是失職**。

---

## 7. 雙載體規格

### 7.1 CL0——定律載體(完整 EBNF 見附錄 A)

設計準則:**小到可以窮舉,全到可以違法**。CL0 刻意只保留:塊作用域(嵌套區間)、`let/mut`、`&`/`&mut`、`*` 解引用、`if/while`(分支與循環的活躍區間)、函數調用(移動語義)。它同時是:

- L1–L7 的測試語言(文法小 ⇒ 樹空間可窮舉/密集採樣);
- L8/L9 的重寫沙盤(菜單規則全部可在 CL0 上機械窮舉臨界對);
- `l5/chordlaw/examples/ex1–ex16` 的對齊對象——16 個樣例族每個都能在 CL0 寫出最小對應體。

### 7.2 R₀——實用載體(完整 EBNF 見附錄 B)

Rust 的子集,覆蓋 judge/borrow 實際需要的現場:items(`fn`/`struct`)、語句(`let`/`expr`/`return`)、表達式(借用/解引用/賦值/調用/字段/索引/`if`/`while`/`loop`)。**排除**:泛型實參歧義、宏、閉包、模式匹配全集、trait——每個排除項在報告中都是「側條件」,在 worker 中都是如實申報的 `unsupported`(不假裝覆蓋,`internal/toolchain` 的 Available() 紀律)。

### 7.3 定律 × 載體矩陣

| 定律 | CL0 | R₀ | 備註 |
|---|---|---|---|
| L1 無損回環 | ✅ 窮舉 | ✅ 真實檔案語料 | R₀ 語料 = rustc test suite 的 R₀ 片段 |
| L2 決定論 | ✅ | ✅ | 同輸入千次重跑序列化相等 |
| L3 增量正確 | ✅ 隨機編輯序列 | ✅ 編輯腳本 | property-based |
| L4 單體作用 | ✅ 隨機編輯對 | — | CL0 窮舉足夠 |
| L5 區間嵌套 | ✅ | ✅ | 定理級,抽樣即可 |
| L6 投影一致 | ✅ | ✅ | 繼承 L3 |
| L7 ERROR 包攝 | ✅ 截斷窮舉 | ✅ 半截檔注入 | L7c best-effort 如實標記 |
| L8 紅邊遞減 | ✅ 全菜單窮舉 | ⚠️ 側條件抽樣 | R₀ 上以 CL0 證明 + 側條件收窄 |
| L9 合流唯一 | ✅ 臨界對窮舉 | ⚠️ 同上 | Newman 的前提在 CL0 全驗 |

---

## 8. 與現有代碼的對照與 T-20 路線

| 現有資產 | 現狀 | 理論地基給出的升級路徑 |
|---|---|---|
| `internal/borrow/extract.go` | 文本啟發式抽取語句拓撲 | R₀ CST 上的結構遞歸抽取;輸出格式不變,事實自帶 span |
| `internal/borrow/topo.go`/`reduce.go` | 區間代數 + 歸約 | §3 定理保証區間族性質;§4.4 錨定保持律成為具名測試 |
| `internal/borrow/rulecard.go` | 封閉修法菜單 | §4:L8 遞減測度逐規則檢查 + L9 臨界對窮舉(CL0) |
| `internal/datalog` | naive 不動點 + 保險絲 | §6.2:Knaster–Tarski 定位;保險絲語義註記補強 |
| `l5/chordlaw/*.dl` | liveness 三軌規則 | §3.2:三軌 = 三種區間語義;衝突圖弦圖性質獨立於軌選擇 |
| `internal/watch` 去抖 | 時間窗合併 | §2.1:編輯單體給「任意順序歸併」代數依據 |
| YKC_20 worker 紀律 | manifest + JSONL 組合 | L1–L9 矩陣 = worker 驗收合同;事實 schema 必含 span 錨 |

**路線(與 YKC_20 的 admission 紀律一致,全部在 worker 側,T0 核心零新增依賴)**:

- **T-20a**:CL0 載體——lexer(DFA)+ PDA parser + 九律測試矩陣中的 L1/L2/L5(不涉及增量的靜態律)。
- **T-20b**:增量層——配置快照 + L3/L4/L6;ERROR 恢復 + L7。
- **T-20c**:R₀ 載體 + 與 `internal/borrow` 的抽取對接(輸出兼容,逐步替換啟發式路徑);L8/L9 在 CL0 全驗、R₀ 側條件抽樣。
- **T-20d**:pack 化(manifest/SHA/admission)——沿用 YKC_21 已驗收的組合協議。

每一步的驗收 = 對應定律的具名測試全綠;**律不過,碼不合**。

## 9. 邊界與非目標

- **本報告不落代碼**:九律的可執行形式(測試)隨 T-20a 落庫;本輪只交付定律、證明草圖與驗收合同。
- **不承諾完整 Rust**:R₀ 之外的語法(宏、泛型歧義、模式全集)由 rustc oracle 兜底,worker 如實申報 unsupported——覆蓋面擴張是數據問題(grammar 擴充),不是理論問題。
- **不重造 rustc**:L5/重寫的全部輸出是解釋與建議;`μ` 遞減保證的是「修法過程終止且順序無關」,不是「修完必過編譯」——過不過,oracle 說不算數的部分永遠留給 oracle。
- **GLR 全歧義支持非目標**:R₀ 選在 LALR(1) 片段內;若未來 grammar 擴張需要真 GLR,T-20b 的配置快照語義已按 GLR 配置集表述,升級路徑預留。

---

## 附錄 A:CL0 完整 EBNF

```ebnf
(* CL0 — Chord-L0:定律載體。設計準則:小到可窮舉,全到可違法。 *)
program   = { item } ;
item      = "fn" IDENT "(" [ params ] ")" block ;
params    = param { "," param } ;
param     = IDENT [ ":" type ] ;
type      = IDENT | "&" [ "mut" ] type ;
block     = "{" { stmt } "}" ;
stmt      = "let" [ "mut" ] IDENT [ "=" expr ] ";"
          | "if" expr block [ "else" block ]
          | "while" expr block
          | expr ";" ;
expr      = unary { binop unary } ;                 (* 平坦優先級:載體不需要 precedence 全貌 *)
binop     = "+" | "-" | "*" | "==" | "<" ;
unary     = [ borrow | "*" ] primary ;
borrow    = "&" [ "mut" ] ;
primary   = NUMBER | "true" | "false" | IDENT
          | IDENT "(" [ args ] ")"                  (* 調用:移動語義載體 *)
          | block ;                                 (* 塊表達式:值與作用域同體 *)
args      = expr { "," expr } ;

(* 詞法(正則):IDENT = letter {letter|digit|"_"}; NUMBER = digit {digit};
   空白與 // 註釋為 trivia,附著 token,參與 L1 無損回環。 *)
(* 語義面(餵 ChordLaw):let 綁定 → 聲明事實;&/&mut → 借用事實(含可變性);
   賦值/調用實參 → 使用/移動事實;塊邊界 → 壽命端點;while → 循環活躍區間。 *)
```

## 附錄 B:R₀ 子集 EBNF

```ebnf
(* R₀ — Rust 子集:judge/borrow 實用載體。排除項一律側條件申報 unsupported。 *)
Crate     = { Item } ;
Item      = FnItem | StructItem ;
FnItem    = [ "pub" ] "fn" Ident "(" [ Params ] ")" [ "->" Type ] Block ;
StructItem= [ "pub" ] "struct" Ident "{" [ FieldDefs ] "}" ;
Params    = Param { "," Param } ;
Param     = [ "&" [ Lifetime ] [ "mut" ] ] Ident ":" Type ;
Lifetime  = "'" Ident ;
Type      = Path | "&" [ Lifetime ] [ "mut" ] Type | Ident ;   (* 無泛型實參——歧義側條件 *)
Block     = "{" { Stmt } "}" ;
Stmt      = LetStmt | ExprStmt | ReturnStmt | Item ;           (* 嵌套 item 允許 *)
LetStmt   = "let" [ "mut" ] Ident [ ":" Type ] [ "=" Expr ] ";" ;
ReturnStmt= "return" [ Expr ] ";" ;
ExprStmt  = Expr ";" ;
Expr      = BinExpr | UnaryExpr | CallExpr | FieldExpr | IndexExpr
          | BorrowExpr | DerefExpr | AssignExpr | PathExpr | LitExpr
          | BlockExpr | IfExpr | WhileExpr | LoopExpr ;
BinExpr   = Expr BinOp Expr ;
BinOp     = "+" | "-" | "*" | "/" | "%" | "==" | "!=" | "<" | ">" | "<=" | ">="
          | "&&" | "||" ;
UnaryExpr = "-" Expr | "!" Expr ;
BorrowExpr= "&" [ "mut" ] Expr ;
DerefExpr = "*" Expr ;
AssignExpr= Expr "=" Expr ;                                    (* 含複合賦位置排除 *)
CallExpr  = Expr "(" [ Args ] ")" ;
FieldExpr = Expr "." Ident ;
IndexExpr = Expr "[" Expr "]" ;
IfExpr    = "if" Expr Block [ "else" ( Block | IfExpr ) ] ;
WhileExpr = "while" Expr Block ;
LoopExpr  = "loop" Block ;
(* 排除(側條件申報):宏、閉包、match/模式全集、泛型實參、trait/impl、
   運算符重載、async、unsafe 塊內部語義。*)
```

## 附錄 C:定律證明義務清單(T-20 落庫時的具名測試)

| 定律 | 具名測試 | 證明/檢查方式 |
|---|---|---|
| L1 | `TestLawL1LosslessRoundtrip` | 語料逐檔 `unparse(parse(s)) == s` |
| L2 | `TestLawL2Determinism` | 同輸入 N 次序列化相等 |
| L3 | `TestLawL3IncrementalEquivalence` | 隨機編輯序列,增量 vs 全量樹相等 |
| L4 | `TestLawL4EditMonoidAction` | 隨機編輯對,兩種複合路徑相等 |
| L5 | `TestLawL5SpanLaminarity` | 全樹節點對抽樣驗證嵌套/不交 |
| L6 | `TestLawL6NamedProjectionStability` | 具名層繼承 L3 相等 |
| L7 | `TestLawL7ErrorContainment` | 截斷/注入窮舉:不假報 + 良構極大 |
| L8 | `TestLawL8RedEdgeDecrease` | 全菜單 × 全幾何族:μ 嚴格遞減 |
| L9 | `TestLawL9Confluence` | 臨界對窮舉會合(Newman 前提完備) |
