# YKC 演化報告 — 樹／線／圖／幾何／拓撲／自動機／矩陣

> 2026-08-27 · Layer7 · 聯網文獻後的規劃與已執行項
> **YKC = Yet Kernel of Chords（弦律內核）**。不是 rustc，也不靠 rustc 才有效。

---

## 0. 先回答你的三句

| 問 | 答 |
|---|---|
| **YKC 是否需要／後台／依賴 rustc 才有效？** | **不需要。** YKC 是純 Python 內環：Datalog × DAG × 圓示 ×（本層）矩陣閉包 × 樹自動機。後台就是這個直譯器。`rustc` 是**可選終審**，合約 S 的外門，不是正確性條件。maturin／PyO3 只是將來的包裝，裝唔到也不影響內核有效。 |
| **不是完美、不是另一個 rustc** | 對。我們不追接受集＝rustc。專注：代理看得懂、改一條弧、毫秒反饋。 |
| **很貼近 Rust → 場景較小** | 這是**功能**：接受集 ⊆ rustc。子集小，所以能幾何化、能證明、能給代理局部動詞。場景小換的是健全方向。 |

---

## 1. 文獻地圖（改革用，不是裝飾）

| 領域 | 文獻 | 落到弦律的一句 |
|---|---|---|
| **線／區間** | 區間圖；借貸活躍集 = CFG 點的區間 | `onregion` 的 I(L) 是「出借→終用」路徑上的點 |
| **幾何** | 圓示弧交越 ≡ 區間重疊 | 法則① 紅弧孤立 |
| **拓撲** | Gavril 1974：弦圖 = 樹上子樹的交圖；路徑圖更窄 | NLL 區間在 DAG＋後向邊上；我們用達達性，不宣稱 chordal 識別 |
| **圖** | CFG 達達性；Polonius：loan 傳播＝圖上 reachability（[Niko 2023](https://smallcultfollowing.com/babysteps/blog/2023/09/22/polonius-part-1/)、[rustc polonius](https://doc.rust-lang.org/nightly/nightly-rustc/rustc_borrowck/polonius/index.html)） | `reach` 已是這件事；YKC 加矩陣對偶 |
| **矩陣** | Warshall／代數路徑：布林 `(∨,∧)` 半環上 A* | `R = Warshall(A∨I)` **必須等於** Datalog `reach` |
| **自動機** | 樹自動機走 LCRS；GKAT 把命令式骨架當自動機 | SIT 形狀檢查（父／兄一致）；**不**用 GKAT 取代借用規則 |
| **樹** | 左孩子右兄弟 + parent（Bali AstNode、OpenDSA） | Layer6 `sit.py` |
| **表面→語意** | CST ≠ AST；rustc AST→HIR→MIR（[Charon](https://arxiv.org/html/2410.18042v1)） | surface / semantic / CFG≈MIR 活度 |

**刻意不演化成 Polonius Alpha**：那是另一個 rustc。我們的改革是「同一設計定理的多載體」，讓代理／工廠走樹與矩陣，規則仍是 32 則。

---

## 2. 同構（執行的數學核）

對函數內 CFG，陳述編號 `0…n-1`：

```
A[i,j] = 1  ⇔  edge(si, sj)          鄰接（線）
R = A*        ⇔  reach                拓撲閉包（矩陣）
v_L[q] = R[L,q] ∧ R[q,E]  ⇔  onregion(L,Q)     區間向量
v_a ∧ v_b ≠ 0 ∧ mut       ⇔  eclash            幾何交越
SIT.left/right/up 合法    ⇔  樹自動機接受      樹
```

若 Warshall ≠ Datalog `reach` → **P0 內核 bug**（兩套演算法必須一致）。

---

## 3. 規劃（只做專注範圍內的）

| # | 項 | 狀態 |
|---|---|---|
| E1 | Layer6 LCRS SIT（表面／語意／呼叫圖） | **已做** `sit.py` |
| E2 | Warshall ≡ `reach` 回歸 | **已做** `ykc.py` / `test_ykc.py` |
| E3 | 借貸區間向量；序貫不相交、衝突相交 | **已做** |
| E4 | 樹自動機：parent／sibling 形狀 | **已做** |
| E5 | 工廠／MCP 仍走弦律 check（YKC 是對偶，不換引擎） | 保持 |
| P- | 流敏感 origin 圖（Polonius subset graph） | **不做**（那是 rustc 的路） |
| P- | GKAT 完備性、矩陣加速 Strassen | **不做**（n 為函數內陳述數，O(n³) 足夠） |
| P- | maturin 把 YKC 編成 .so | **不做**（此環境無 rustc；也不需要才有效） |

---

## 4. 使用場景（小，但是對的小）

YKC／弦律有效當且僅當：

1. 輸入落在 `.cl` 或 syn 子集；
2. 問題是函數內借用／移動／逸出；
3. 消費者是代理（要證明樹／弧／縮弧），不是要過 crates.io 全語法。

無效（直通 rustc，不假裝）：巨集、泛型 variance、async、trait 物件、unsafe。

---

## 5. 存倉策略（你的授權）

- 完成即 commit；**只開新分支**，不 force-push、不改已推的 `main`／`Layer6`。
- 本報告隨 Layer7 新支上去。
