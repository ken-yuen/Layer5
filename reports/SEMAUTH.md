# 認証語義機 — 不論輸入，語義是 X 才報 X

> Layer8 · 2026-08-28 · 聯網文獻後執行
> 不依賴 rustc。不是另一個 rustc。

---

## 不變量（認証）

\[
\mathrm{Auth}(w) = X \quad\Longleftrightarrow\quad [\![w]\!] = X
\]

輸入 \(w\) 可以是任何表面字串（空白、註解、unnamed、`88÷ww`、`if=x`）。  
**只有語義等於 X 才報 X。** ERROR 節點永不發明 `lend`（不報 PASS）。

這是健全方向：虛假 X 禁止；表面噪音必須被重寫掉。

---

## 樹層（tree-sitter 詞彙）

| 層 | 名稱 | 做什麼 |
|---|---|---|
| **CST** | Concrete Syntax Tree | 全標記 |
| **SST** | Surface Syntax Tree | extra → ε（註解／空行） |
| **NNT** | Named Node Tree | 只留 named |
| **EIT** | Extra-Inclusive Tree | extra 打標，不混語義 |
| **ENT** | Error Node Tree | `ERROR`/`MISSING`；`if=x` 無 `{` 屬此 |
| **AST** | Abstract Syntax Tree | 脫糖、空白正規化 |
| **DAG** | 語義 DAG | hash-cons 正規 `.cl`（[ASG](https://en.wikipedia.org/wiki/Abstract_semantic_graph)） |

文獻：[tree-sitter named/extra/error](https://tree-sitter.github.io/py-tree-sitter/classes/tree_sitter.Node.html)、[ast-grep CST→AST](https://betterprogramming.pub/deep-dive-into-ast-greps-pattern-7efc3eefc7c3)、[Ghica 圖重寫與觀察等價](https://arxiv.org/pdf/2102.02363)、[Huet 合流](https://dl.acm.org/doi/10.1145/322217.322230)、[ASF+SDF hash-cons](https://dl.acm.org/doi/pdf/10.1145/567097.567099)。

## 形式

- **形式語言**：表面 \(L_{cst}\) 與語義正規形 \(L_{nf}\)；Auth 是 \(L_{cst}\to \{\mathrm{PASS},\mathrm{FAIL},\mathrm{ERROR}\}\)。
- **抽象代數**：重寫系統 \(R\) 地面項；意圖正交 → 合流 → 唯一 NF → 唯一 digest。
- **幾何拓撲**：NF 上的判決仍是弦律 `onregion`／區間交越（YKC 矩陣對偶）。
- **自動機**：ERROR 是拒絕態；named 路徑才進檢查自動機。
- **DAG**：相同子項共享；digest = sha1(NF)。

## `if=x` 與 `88÷ww`

| 表面 | 節點 | 語義 |
|---|---|---|
| `if=x` 無 `{` | ERROR | **不**發明 `if {`；報 ERROR |
| `88÷ww` | unnamed `divop` | extra／非借用；不產生 lend |
| `let a=&x` vs `let a = &x` | 同一 named | 同一 digest、同一 X |

## 不做

- 全量 tree-sitter crate（無 rustc／無 grammar.js 執行期）
- 把 ERROR 修成能過的程式（那是工廠，不是認証）
- 當 rustc

## 用法

```bash
python3 semauth.py examples/ex2_sequential.cl
python3 test_semauth.py
```
