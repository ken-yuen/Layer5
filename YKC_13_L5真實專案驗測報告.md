# YKC_13 — L5 真實專案驗測報告

**日期**：2026-08-22 ｜ **對象**：v0.0.2 L5 全管線（歸約器→引擎→judge→panel→MCP）
**方法**：以 7 個真實開源 Rust 專案（合計 601 檔、~202,000 行）做四層驗測：
T1 全量掃描（穩健性）→ T2 真實形態錯誤注入（端到端）→ T3 rustlings 天然錯誤語料 → T4 panel/MCP 出口。

## 1. 驗測對象

| 專案 | .rs 檔 | 行數 | 性質 |
|---|---|---|---|
| BurntSushi/ripgrep | 110 | 56,386 | 搜尋工具（重度迭代器/泛型） |
| rtk-ai/rtk | 130 | 87,872 | AI 代理工具鏈 |
| alacritty/alacritty | 88 | 33,710 | GPU 終端（重度 unsafe/生命週期） |
| rust-lang/rustlings | 220 | 11,678 | 教學練習（天然 borrow 錯誤語料） |
| sharkdp/fd | 24 | 8,281 | 檔案搜尋 |
| ajeetdsouza/zoxide | 25 | 2,698 | 目錄跳轉 |
| rust-unofficial/awesome-rust | 4 | 1,356 | 清單庫（少量代碼） |

## 2. T1 全量掃描（歸約器穩健性）

方法：對每個專案全部 `.rs` 檔的**每個 fn** 跑歸約器（10,255 個 fn），檢驗：
①不 panic ②歸約產物必須是引擎可解析的合法 `.cl`（E00=不合格）③統計放棄原因。

**最終結果（修復後）**：

| 專案 | fn 數 | 歸約成功 | 成功率 | panic | 不合法產物 |
|---|---|---|---|---|---|
| ripgrep | 2,992 | 1,207 | 40.3% | 0 | **0** |
| rtk | 4,316 | 2,112 | 48.9% | 0 | **0** |
| alacritty | 1,594 | 856 | 53.7% | 0 | **0** |
| rustlings | 839 | 343 | 40.9% | 0 | **0** |
| fd | 352 | 183 | 52.0% | 0 | **0** |
| zoxide | 140 | 51 | 36.4% | 0 | **0** |
| awesome-rust | 22 | 2 | 9.1% | 0 | **0** |
| **合計** | **10,255** | **4,754** | **46.4%** | **0** | **0** |

放棄原因主要為（符合設計——寧退模板勿失真）：closure/match/else 等不可歸約結構（~60%）、
fn 體無可翻譯陳述、fn 過長、產物過大防線。

## 3. 驗測驅動的修復（8 項, 全部沉澱為回歸測試）

初掃時 fd/zoxide 兩專案即暴露 36 例不合法產物；修復後七專案全部清零。

| # | 發現（真實專案暴露） | 修復 |
|---|---|---|
| 1 | `*b += 1` 解引用被誤當塊註釋延續行殺掉（fd walk.rs） | 註釋判定改 `/* `、`* `、`*/` 精確前綴 |
| 2 | 大寫構造子/型別（`Ok/Err/Some/Database`）被當 place（zoxide） | `isNoiseName`: 大寫開頭=噪音；`_`=噪音 |
| 3 | 方法呼叫名被當字段路徑（`dirs.swap_remove(x)` → use dirs.swap_remove）| 呼叫名剝除, 受者保留（`→ use dirs, use x`） |
| 4 | 靜默塊（結構體字面量換行）多發 `}` 致括號失衡（fd new()） | 塊棧配平：發射塊/靜默塊分離 |
| 5 | 塊內宣告塊外引用 E00（ripgrep version.rs 5 例） | 塊內宣告**提升**：頂部 let＋原位 set；塊內參考出塊即死 |
| 6 | `for atlas in &mut atlas` 自引用 shadowing（alacritty atlas.rs）；`let x = &x`；經參考取字段 `&r.field` | 三態全退化為 use＋宣告（.cl 不可表達, 不硬譯） |
| 7 | **loop 內大產物令 naive 引擎 OOM 被 kill**（rustlings hashmaps3, fd tests.rs） | 實測復雜度邊界（loop 內 16 陳述≈5s, 22≈20s, 28 超時）→ `MaxLoopStmts=12` ＋ `MaxReduceStmts=40` 雙防線 |
| 8 | shadowing 重 let（.cl 禁止）多處 | `declare()`: 已知名譯 set（重初始化語義） |

## 4. T2 端到端（真實形態錯誤注入）

從 zoxide/fd/ripgrep/alacritty 的真實代碼形態提取樣板、注入 borrow 錯誤成獨立 crate，跑 `ykc-judge` 全管線：

| 注入 | rustc | L5 行為 | 結果 |
|---|---|---|---|
| zoxide db 形態（`&self.dirs[0]`＋push） | E0502 | 索引不可表達 → **誠實回退模板**（標明「現場歸約未通過驗證, 已回退」） | ✅ |
| fd WorkerState 形態（借 config 後寫 state.config） | E0506 | **真實歸約成功**：`s3 ← main.rs:7` 錨定, 引擎驗證通過, 帳本 `reduced=1` | ✅ |
| alacritty Term 形態（借 rows 後整體重賦） | E0506 | **真實歸約成功**＋sN←行號對照 | ✅ |
| ripgrep 形態（E0505 借用期間 move） | E0505 | 先被 `cargo fix` 機械修復（L4 正確行為）；改注不可機修變體後 L5 正常接手 | ✅ |

附帶驗證：`cargo fix` 能機修的 borrow 錯不會到 L5——L4/L5 分工正確。

## 5. T3 rustlings 天然語料

move_semantics×5＋lifetimes×3 練習（教學用天生錯誤）：

- E0382（move 後使用）→ L5 出 E05/E06 規則卡＋模板拓撲 ✅
- E0106×2（缺生命週期標注）→ E10 引用逸出／法則② ✅
- E0597（活得不夠久）→ E04／法則② ✅
- E0596/E0308（非 borrow 類）→ 正確**不觸發** L5 ✅

## 6. T4 出口驗證

- **panel**：`-root /tmp/t2 -dir …` 聚合 5 個驗測專案，`/api/state` 全部帶 `l5` 節點（codes/red_edges/reduced），紅邊徽章正常 ✅
- **MCP**：以 zoxide dedup 真實形態的 .cl 調 `ykc.borrow_explain`——區間拓撲＋`OVERLAP[s4..s5] ←違法`＋紅邊=1 ✅

## 7. 結論與遺留

**結論**：L5 全管線在 ~20 萬行真實代碼上零 panic、零不合法產物；歸約成功率 46.4%（其餘誠實回退模板）；判定權歸屬（rustc）與 L4/L5 分工經注入實測正確。

**遺留（如實申報）**：
1. 歸約成功率受行級啟發法本質限制（closure/match 即放棄）——P3 syn AST 前端的動機更堅實；
2. `e0502族驗證通過`在盲掃下命中極低（預期內：掃描器對任意行硬套 E0502 幾何族，非真實錯誤現場）；真實錯誤注入下驗證通過率 2/3（不可表達者誠實回退）；
3. naive 引擎的 loop 複雜度牆（>12 陳述）是 T-18b Go 引擎重寫的又一動機；
4. 8 項修復全部沉澱為 `extract_test.go` 回歸測試（TestReduceBlockCommentVsDeref 等 5 條新增）。
