# Layer6 — 左樹右樹語意分析器

聯網對齊後的節點形狀（[Bali AstNode](https://www.cs.utexas.edu/~schwartz/ATS/fopdocs/AST.html)、左孩子右兄弟）：

```
SNode
  left  → 第一個孩子     左樹
  right → 下一個兄弟     右樹 / Sibling_Reference
  up    → 父親           Parent_Pointer
```

n 元樹用二元指針表示：孩子走 `left`，同層走 `right`，繼承／出作用域走 `up`。

## 你列的名字 ↔ 實作

| 你寫的 | 是什麼 | 檔案 |
|---|---|---|
| **PSurface Syntax** | 表面語法／混凝土樹（行＋`{}`，關鍵字還在） | `sit.build_surface` |
| **semantic-analyzer** | 表面 → 語意意圖樹（fn / lend / use / call） | `sit.build_semantic` |
| **左樹右樹** | `left` = 首孩，`right` = 兄妹 | `sit.add_child` |
| **Sibling_Reference** | 同層 `right` 鏈 | `iter_siblings` |
| **Tree Parent_Pointer** | `up`；`has_ancestor` | `iter_ancestors` |
| **Class_Inheritance+** | 沿 `up` 做詞法繼承（param/let），不是 OOP class（`.cl` 沒有 class） | `inherit_lookup` |
| **Call_Graph_Traversal** | `call`/`callmv` 出邊，DFS | `call_graph_reachable` |

## 跟弦律的關係

- **不取代** `onregion` / 32 則。那些仍在 CFG 點集上跑。
- SIT 是 **IR**：工廠／代理用樹走，而不是掃扁平 `stmts` 列表。
- rustc 管道對照（學自 [Charon](https://arxiv.org/html/2410.18042v1)）：表面 ≈ AST/CST，語意 ≈ HIR，弦律 CFG ≈ MIR 活度。我們沒有 rustc，所以表面只是 syn／`.cl` 子集。

## 用法

```bash
python3 sit.py examples/ex2_sequential.cl --layer both
python3 test_sit.py
```

## 刻意不做

- 真 class 繼承網（無 `impl`/`trait`）
- 動態分派呼叫圖（需 Rupta 那種 MIR pointer analysis）
- 把這棵樹當 rustc 終審
