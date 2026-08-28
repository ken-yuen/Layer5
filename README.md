# 弦律 ChordLaw（工作原型 v0.7）

簡化 Rust 借用/生命週期檢查器：Datalog 規則 × DAG 拓撲 × 圓示（縱點節圖）。

- **[RULES32.md](RULES32.md)** — **32 則規則完整目錄**（Datalog + DAG + 幾何 + 範例）
- **[gallery.html](gallery.html)** — 32 則圓示（縱點節圖）畫廊
- **[DOCS.md](DOCS.md)** — 專案全面說明（語言規格、架構、接口、測試、限制）
- [PLAN.md](PLAN.md) — 完整計畫與論證、roadmap
- **[MCP.md](MCP.md)** — P3a：MCP × 持久化 × 評分／報告
- **[AHPBB.md](AHPBB.md)** — P1/P2：意圖樹 + syn/quote + AutoHPBorrowBase 工廠
- [reports/helper_v0.7.md](reports/helper_v0.7.md) — 本倉庫小幫手評分與建議

## 運行

```bash
python3 chordlaw.py                       # 全部範例（含 r01–r32）
python3 chordlaw.py --liveness referent    # 保守級（pre-NLL 行為）
python3 chordlaw.py --liveness lexical     # 最保守（作用域級）
python3 chordlaw.py examples/r01_eclash.cl
python3 chordlaw.py --json examples/r18_ebranchmove.cl
python3 chordlaw.py --explain examples/r15_etemp.cl
python3 chordlaw.py --rules                  # 規則規格速覽＋完整規則檔
python3 chordlaw.py --check [DIR]            # 專案檢查（.cl；.rs 略過）
python3 chordlaw.py --report [DIR] -o reports/latest.md
python3 chordlaw.py --history [DIR]          # 會話 diff
python3 chordlaw.py --mcp                    # stdio MCP（掛代理用）
python3 chordlaw.py --factory                # AHPBB：多線產出正確 Rust → 新資料夾
python3 chordlaw.py --from-rs FILE.rs        # syn 子集 → .cl → 檢查
```

無外部依賴（純 Python 標準庫）。輸出：終端報告（判決＋**證明樹**＋區域表）、`examples/out/*.svg`（圓示）、`--json`（供代理/CI 的結構化輸出）。

## 測試

```bash
python3 test_chordlaw.py   # 回歸測試（語料 + 32 則 + TestGeometry）
python3 test_helper.py     # P3a：評分 / 持久化 / MCP
python3 test_ahpbb.py      # P1/P2 + 工廠（syn/quote/生命週期/批量出貨）
python3 oracle_check.py    # 差異測試: .cl→Rust 忠實翻譯 vs 真 rustc（需 rustc）
```

## 32 則規則（E01–E32）一覽

| 區間 | 主題 | 代碼 |
|---|---|---|
| E01–E03 | 排他 vs 共享（紅弧孤立） | `eclash` `ewrite` `eread` |
| E04 / E10 | 不懸垂 / 逸出（弧在圓內） | `edangle` `ereturn` |
| E05–E08 | 單一所有權 / 消耗 | `eadrop` `emove` `eloanmove` `eloandrop` |
| E09 | 別名層 / 2-phase | `erefuse` |
| E11–E12 / E22–E23 / E31–E32 | 不可變（地方 / 字段 / 參數） | `eimmut` `enotmut` `eimmfield` `enotmutfield` `eparamimmut` `eparamnotmut` |
| E13–E14 | 經參考寫 / 移 | `eassignsh` `emoveout` |
| E15 / E17 / E28 | 暫存生命週期 | `etemp` `estoretemp` `erettemp` |
| E16 / E21 | 存槽逸出 / 別名環 | `eescape` `ealiascyc` |
| E18 / E24 | 分支 join 後的 maybe-move | `ebranchmove` `eelsejoin` |
| E19–E20 | 雙重釋放 | `edoubledrop` `edropmoved` |
| E25–E27 | 呼叫 | `ecallmut` `ecallmove` `ecalloan` |
| E29–E30 | 未初始化 | `euninit` `euninitret` |

每則配 `examples/rXX_*.cl` 反例＋`examples/out/rXX_*.svg` 圓示。詳見 [RULES32.md](RULES32.md)。

## `.cl` 玩具語言（v0.4）

```
fn name(args) { ... }      # 參數範圍 = ROOT; 參數可標 imm
loop { ... }               # 迴圈 = 後向邊
if { ... } else { ... }    # 菱形 CFG（無 else 時不旁路）
{ ... }                    # 區塊
let x                      # 可變已初始化地方
let imm x                  # 不可變地方
let hole x                 # 未初始化洞
tmp t                      # 暫存（不過句）
slot s                     # 可存參考的槽
let a = &x / &mut x        # sh / mut 出借（x 可為字段路徑）
use / set / mv / dp P      # 讀 / 寫 / move / drop
set *a / mv *a             # 經參考寫 / 移
store s = a                # 把參考存入槽
call f(x) / callmv f(x)    # 呼叫讀取 / 呼叫消耗
ret a                      # 回傳參考 → 逸出檢查
```

慣例：Datalog 變數大寫/`_` 開頭，常數小寫開頭（陳述 `s1…`、作用域 `n0…`）。

## 圓示速讀

- 縱軸＝時間（向下）；點＝陳述；弧＝借用（藍 sh / 紅 mut）；虛線圓＝作用域（同心＝巢狀）；紫色迴路＝迴圈後向邊；琥珀色圓＝else。
- **法則① 紅弧孤立**：紅弧弧跨內不得含他弧端點、不得與他弧弦交越。
- **法則② 弧在圓內**：弧端點須落在被借者之作用域圓內；回傳者被借者須活得比呼叫者久。
- **法則③ 點序守紀**：move/drop 之後的點不得再讀同路徑；紅弧跨內不得有寫/移/呼叫消耗。

十條對內公理（同構、著色、同心包含、後向閉包、菱形、路徑前綴、別名投影、權限點）見 [GEOMETRY.md](GEOMETRY.md)。
