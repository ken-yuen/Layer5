# 弦律 ChordLaw（工作原型）

簡化 Rust 借用/生命週期檢查器：Datalog 規則 × DAG 拓撲 × 圓示（縱點節圖）。
- **[DOCS.md](DOCS.md)** — 專案全面說明（語言規格、架構、規則參考、接口、測試、限制）
- [PLAN.md](PLAN.md) — 完整計畫與論證、roadmap

## 運行

```bash
python3 chordlaw.py                       # 全部 17 範例（liveness=nll, 與 NLL 等價）
python3 chordlaw.py --liveness referent    # 保守級（pre-NLL 行為）
python3 chordlaw.py --liveness lexical     # 最保守（作用域級）
python3 chordlaw.py examples/ex1_clash.cl
python3 chordlaw.py --json examples/ex7_alias_use.cl   # 機器接口: verdict/errors(+證明樹)/regions
python3 chordlaw.py --explain examples/ex15_field_clash.cl  # 代理工具: 規則原文+證明+幾何+修法
python3 chordlaw.py --rules                  # 規則規格速覽＋完整規則檔
```

無外部依賴（純 Python 標準庫）。輸出：終端報告（判決＋**證明樹**＋區域表）、`examples/out/*.svg`（圓示）、`--json`（供代理/CI 的結構化輸出）。

## 測試

```bash
python3 test_chordlaw.py   # 19 項回歸測試: 34 語料 (PASS 防守虛假拒絕 / FAIL 防守虛假放行, 錯誤碼與位置須全符) + 前端 E00 + 解析器作用域 + v0.3 功能 + 引擎單元
python3 oracle_check.py    # 差異測試: 26 例 .cl→Rust 忠實翻譯, 弦律 vs 真 rustc 判定核對 (現況 26/26 一致, 0 虛假放行)
```

## 檔案

| 檔案 | 說明 |
|---|---|
| `chordlaw.py` | mini Datalog 引擎（分層＋單調定點＋證明樹＋有限域內建）＋ `.cl` 前端 ＋ 圓示 SVG 渲染器 ＋ CLI（`--json`/`--explain`/`--rules`） |
| `rules.dl` | 核心規則（E01–E10；純 Datalog，唯內建 `neq`） |
| `liveness_nll.dl` | 活度等級：NLL（參考自身終端使用，含別名鏈閉包 `tuse`）— 預設 |
| `liveness_referent.dl` | 活度等級：referent（被借者終端使用）— 保守 |
| `liveness_lexical.dl` | 活度等級：lexical（作用域終端）— 最保守 |
| `examples/*.cl` | 17 個範例（交越/順序/逸出×2/迴圈×2/move/別名×2/重初始化/sh 重疊/mut 依序/**if×2/字段×2/多 fn**） |
| `examples/out/*.svg` | 各範例圓示 |
| `test_chordlaw.py` | 回歸測試（`python3 test_chordlaw.py`，19 項全綠） |
| `oracle_check.py` | rustc oracle 差異測試（需 rustc；26/26 一致） |

## `.cl` 玩具語言

```
fn name(args) { ... }   # 參數之範圍 = ROOT（來自呼叫者）; 每檔可多 fn（各自獨立檢查）
loop { ... }            # 迴圈 = 後向邊
if { ... }              # 分支（無 else; 分支內使用不延長活度過分支 — NLL）
{ ... }                 # 區塊
let x                   # 宣告
let a = &x              # sh 出借（x 可為字段路徑 x.f）
let b = &mut x          # mut 出借
use a                   # 使用（place 讀取 / 參考使用; 可為 x.f）
set x                   # 寫入（可為 x.f）
mv x / dp x             # move / 顯式 drop（可為 x.f）
ret a                   # 回傳（參考 → 逸出檢查）
```

字段路徑（v0.3, split borrow）：借 `x.f` 與 `x.g` 可共存；與整體 `x` 的借用路徑衝突；整體 move 後讀/寫子字段、字段 move 後讀整體皆為 E06（整體重初始化豁免）；經參考取字段（deref）不支持。

慣例：Datalog 變數大寫/`_` 開頭，常數小寫開頭（故陳述 ID 為 `s1…`、作用域 `n0…`）。

## 圓示速讀

- 縱軸＝時間（向下）；點＝陳述；弧＝借用（藍 sh / 紅 mut）；虛線圓＝作用域（同心＝巢狀）；紫色迴路＝迴圈後向邊。
- **法則① 紅弧孤立**：紅弧弧跨內不得含他弧端點、不得與他弧弦交越。
- **法則② 弧在圓內**：弧端點須落在被借者之作用域圓內；回傳者被借者須活得比呼叫者久。
