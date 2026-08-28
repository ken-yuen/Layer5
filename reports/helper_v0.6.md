# 弦律小幫手報告 v0.6

> 內環預檢，**rustc 終審**。分數不是 rustc 品質，是「代理能不能用幾何動詞修完」。
>
> 專案：`/home/user/Layer5` · 活度 `nll` · 會話 `s_20260827_014553_ce0e` · 2026-08-27T01:45:53+08:00 · 162 ms

## 1. 產品就緒（本倉庫當 Rust 小幫手）

| 維度 | 分 | 說明 |
|---|---:|---|
| engine | 88 | 32 則 + 幾何可測 + 92 回歸 |
| soundness | 90 | 合約 S：oracle 26/26，0 虛假放行 |
| agent_api | 80 | MCP 六工具 + --json/--explain/--report |
| persistence | 75 | .chordlaw/state.json 會話 + diff |
| rust_coverage | 25 | 僅 .cl；.rs 直通 rustc（P2 未做） |
| repair_loop | 70 | 縮弧/移點/升圓建議可執行；尚未自動改碼 |
| **overall** | **71** | 產品就緒分 ≠ 專案檢查分。內環預檢，rustc 終審。 |

**解讀（本倉庫）**：71 = 「當 Rust 小幫手」的產品分。引擎與健全性已夠當內環；短板是 **無 .rs 前端（25）** 與 **不能自動改碼（70）**。

**專案檢查 37 / blocked 是預期**：`examples/r01–r32` 與多數 `ex*` 是**故意 FAIL 的規則反例**，不是用戶 crate。explainability=100 表示每條錯都有幾何修法。對真實專案應只掃業務 `.cl`，不要把語料當品質。

### 作為小幫手的下一步（優先序）

| 優先 | 建議 | 理由 |
|---|---|---|
| P0 | P2：`syn` → IR，`.rs` 不再 skipped | 否則代理無法在真 Rust 倉用內環 |
| P0 | 合約 S 繼續用 oracle 守；禁止把 .rs 假譯成 .cl | 虛假放行致命 |
| P1 | P3c：`chordlaw_apply` 執行縮弧／移點 | 關掉「建議看了不改」的環 |
| P1 | 增量（檔 hash → 只重算變更 fn） | M3 p95 < 500ms |
| P2 | Cursor/Claude Desktop 掛 `--mcp` 做一次代理在環 demo | 量 M2 |

## 2. 專案檢查分（語料倉，非用戶 crate）

- **overall 37 / 100** · 帶 **`blocked`**（反例語料，見上）
- 已檢 49（PASS 8 / FAIL 41）· 錯誤 50 · 略過 .rs 0

| 維度 | 分 |
|---|---:|
| correctness | 16 |
| severity | 0 |
| explainability | 100 |
| coverage | 100 |

### 錯誤碼分布

| 碼 | 次 | 法則 | 優先 |
|---|---:|---|---|
| E01 | 5 | 法則① 紅弧孤立 | P0 |
| E02 | 3 | 法則④ 區間著色 | P1 |
| E03 | 1 | 法則④ 區間著色 | P1 |
| E04 | 1 | 法則② 弧在圓內 | P2 |
| E05 | 2 | 法則③ 點序守紀 | P2 |
| E06 | 5 | 法則③ 點序守紀 | P0 |
| E07 | 2 | 法則⑩ 權限點 | P0 |
| E08 | 1 | 法則⑩ 權限點 | P2 |
| E09 | 3 | 法則① 紅弧孤立 | P1 |
| E10 | 2 | 法則② 弧在圓內 | P0 |
| E11 | 1 | 法則⑩ 權限點 | P2 |
| E12 | 1 | 法則⑩ 權限點 | P2 |
| E13 | 1 | 法則⑩ 權限點 | P2 |
| E14 | 1 | 法則⑩ 權限點 | P2 |
| E15 | 1 | 法則③ 點序守紀 | P2 |
| E16 | 3 | 法則② 弧在圓內 | P2 |
| E17 | 1 | 法則② 弧在圓內 | P2 |
| E18 | 1 | 法則③ 點序守紀 | P1 |
| E19 | 1 | 法則③ 點序守紀 | P2 |
| E20 | 1 | 法則③ 點序守紀 | P2 |
| E21 | 1 | 法則⑨ 別名鏈投影 | P2 |
| E22 | 1 | 法則⑩ 權限點 | P2 |
| E23 | 1 | 法則⑩ 權限點 | P2 |
| E24 | 1 | 法則③ 點序守紀 | P1 |
| E25 | 1 | 法則④ 區間著色 | P2 |
| E26 | 2 | 法則③ 點序守紀 | P2 |
| E27 | 1 | 法則⑩ 權限點 | P2 |
| E28 | 1 | 法則② 弧在圓內 | P2 |
| E29 | 1 | 法則③ 點序守紀 | P2 |
| E30 | 1 | 法則③ 點序守紀 | P2 |
| E31 | 1 | 法則⑩ 權限點 | P2 |
| E32 | 1 | 法則⑩ 權限點 | P2 |

## 3. 建議（按 P0 → P2，幾何動詞）

| 優先 | 操作 | 檔 | 碼 | 點 | 修 |
|---|---|---|---|---|---|
| P0 | `shrink_arc` | examples/ex11_alias_chain.cl | E01 | s5 | 修: 縮弧 — 把 a 的終端使用從 s7 移到 s4 (後弧出借點 s5 之前) |
| P1 | `shrink_arc` | examples/ex11_alias_chain.cl | E09 | s4 | 修: 縮弧 — 把 a 的終端使用從 s7 移到 s4 (後弧出借點 s5 之前) |
| P0 | `shrink_arc` | examples/ex13_if_clash.cl | E01 | s3 | 修: 縮弧 — 把 a 的終端使用從 s4 移到 s2 (後弧出借點 s3 之前) |
| P0 | `shrink_arc` | examples/ex15_field_clash.cl | E01 | s3 | 修: 縮弧 — 把 a 的終端使用從 s4 移到 s2 (後弧出借點 s3 之前) |
| P1 | `shrink_arc` | examples/ex16_two_fn.cl | E02 | s6 | 修: 縮弧 — 令該點不再落在衝突弧跨內 |
| P0 | `shrink_arc` | examples/ex1_clash.cl | E01 | s4 | 修: 縮弧 — 把 a 的終端使用從 s5 移到 s3 (後弧出借點 s4 之前) |
| P0 | `raise_circle` | examples/ex3_return.cl | E10 | s3 | 修: 升圓 — 令被借者來自參數 (ROOT), 或把使用移進圓內 |
| P1 | `shrink_arc` | examples/ex5_loop_write.cl | E02 | s4 | 修: 縮弧 — 令該點不再落在衝突弧跨內 |
| P0 | `move_point` | examples/ex6_move.cl | E06 | s4 | 修: 移點 — 先使用再消耗, 或消耗後先 set 再讀 |
| P0 | `change_perm` | examples/ex6_move.cl | E07 | s3 | 修: 改經引用存取, 或把操作移出弧跨 |
| P1 | `shrink_arc` | examples/ex7_alias_use.cl | E09 | s4 | 拆成兩句: 先終結對該引用的借用 (其最終使用), 再排他使用該引用 (2-phase 於陳述粒度不可表達) |
| P0 | `shrink_arc` | examples/r01_eclash.cl | E01 | s4 | 修: 縮弧 — 把 a 的終端使用從 s5 移到 s3 (後弧出借點 s4 之前) |
| P1 | `shrink_arc` | examples/r02_ewrite.cl | E02 | s3 | 修: 縮弧 — 令該點不再落在衝突弧跨內 |
| P1 | `shrink_arc` | examples/r03_eread.cl | E03 | s4 | 修: 縮弧 — 令該點不再落在衝突弧跨內 |
| P2 | `raise_circle` | examples/r04_edangle.cl | E04 | s5 | 修: 升圓 — 令被借者來自參數 (ROOT), 或把使用移進圓內 |
| P2 | `raise_circle` | examples/r04_edangle.cl | E16 | s4 | 修: 升圓 — 令被借者來自參數 (ROOT), 或把使用移進圓內 |
| P2 | `move_point` | examples/r05_eadrop.cl | E05 | s3 | 修: 移點 — 先使用再消耗, 或消耗後先 set 再讀 |
| P0 | `move_point` | examples/r06_emove.cl | E06 | s3 | 修: 移點 — 先使用再消耗, 或消耗後先 set 再讀 |
| P0 | `move_point` | examples/r07_eloanmove.cl | E06 | s4 | 修: 移點 — 先使用再消耗, 或消耗後先 set 再讀 |
| P0 | `change_perm` | examples/r07_eloanmove.cl | E07 | s3 | 修: 改經引用存取, 或把操作移出弧跨 |
| P2 | `move_point` | examples/r08_eloandrop.cl | E05 | s4 | 修: 移點 — 先使用再消耗, 或消耗後先 set 再讀 |
| P2 | `change_perm` | examples/r08_eloandrop.cl | E08 | s3 | 修: 改經引用存取, 或把操作移出弧跨 |
| P1 | `shrink_arc` | examples/r09_erefuse.cl | E09 | s4 | 拆成兩句: 先終結對該引用的借用 (其最終使用), 再排他使用該引用 (2-phase 於陳述粒度不可表達) |
| P0 | `raise_circle` | examples/r10_ereturn.cl | E10 | s3 | 修: 升圓 — 令被借者來自參數 (ROOT), 或把使用移進圓內 |
| P2 | `change_perm` | examples/r11_eimmut.cl | E11 | s2 | 修: 改經引用存取, 或把操作移出弧跨 |
| P2 | `change_perm` | examples/r12_enotmut.cl | E12 | s2 | 修: 改經引用存取, 或把操作移出弧跨 |
| P2 | `change_perm` | examples/r13_eassignsh.cl | E13 | s3 | 修: 改經引用存取, 或把操作移出弧跨 |
| P2 | `change_perm` | examples/r14_emoveout.cl | E14 | s3 | 修: 改經引用存取, 或把操作移出弧跨 |
| P2 | `move_point` | examples/r15_etemp.cl | E15 | s4 | 修: 移點 — 先使用再消耗, 或消耗後先 set 再讀 |
| P2 | `raise_circle` | examples/r16_eescape.cl | E16 | s4 | 修: 升圓 — 令被借者來自參數 (ROOT), 或把使用移進圓內 |
| P2 | `raise_circle` | examples/r17_estoretemp.cl | E16 | s4 | 修: 升圓 — 令被借者來自參數 (ROOT), 或把使用移進圓內 |
| P2 | `raise_circle` | examples/r17_estoretemp.cl | E17 | s4 | 修: 升圓 — 令被借者來自參數 (ROOT), 或把使用移進圓內 |
| P0 | `move_point` | examples/r18_ebranchmove.cl | E06 | s3 | 修: 移點 — 先使用再消耗, 或消耗後先 set 再讀 |
| P1 | `move_point` | examples/r18_ebranchmove.cl | E18 | s3 | 修: 移點 — 先使用再消耗, 或消耗後先 set 再讀 |
| P2 | `move_point` | examples/r19_edoubledrop.cl | E19 | s3 | 修: 移點 — 先使用再消耗, 或消耗後先 set 再讀 |
| P2 | `move_point` | examples/r20_edropmoved.cl | E20 | s3 | 修: 移點 — 先使用再消耗, 或消耗後先 set 再讀 |
| P2 | `clone_or_reinit` | examples/r21_ealiascyc.cl | E21 | s3 | 修: 不要把指向槽自身的參考存回槽 |
| P2 | `change_perm` | examples/r22_eimmfield.cl | E22 | s2 | 修: 改經引用存取, 或把操作移出弧跨 |
| P2 | `change_perm` | examples/r23_enotmutfield.cl | E23 | s2 | 修: 改經引用存取, 或把操作移出弧跨 |
| P0 | `move_point` | examples/r24_eelsejoin.cl | E06 | s4 | 修: 移點 — 先使用再消耗, 或消耗後先 set 再讀 |

… 其餘 10 條略（見 state.json）

## 5. 檔案判決

| 檔 | 判決 | 錯誤 |
|---|---|---|
| examples/ex10_mut_seq.cl | PASS | — |
| examples/ex11_alias_chain.cl | FAIL | E09, E01 |
| examples/ex12_if_branch.cl | PASS | — |
| examples/ex13_if_clash.cl | FAIL | E01 |
| examples/ex14_field_split.cl | PASS | — |
| examples/ex15_field_clash.cl | FAIL | E01 |
| examples/ex16_two_fn.cl | FAIL | E02 |
| examples/ex1_clash.cl | FAIL | E01 |
| examples/ex2_sequential.cl | PASS | — |
| examples/ex3_return.cl | FAIL | E10 |
| examples/ex3b_return_param.cl | PASS | — |
| examples/ex4_loop_ok.cl | PASS | — |
| examples/ex5_loop_write.cl | FAIL | E02 |
| examples/ex6_move.cl | FAIL | E07, E06 |
| examples/ex7_alias_use.cl | FAIL | E09 |
| examples/ex8_reassign.cl | PASS | — |
| examples/ex9_two_sh.cl | PASS | — |
| examples/r01_eclash.cl | FAIL | E01 |
| examples/r02_ewrite.cl | FAIL | E02 |
| examples/r03_eread.cl | FAIL | E03 |
| examples/r04_edangle.cl | FAIL | E16, E04 |
| examples/r05_eadrop.cl | FAIL | E05 |
| examples/r06_emove.cl | FAIL | E06 |
| examples/r07_eloanmove.cl | FAIL | E07, E06 |
| examples/r08_eloandrop.cl | FAIL | E08, E05 |
| examples/r09_erefuse.cl | FAIL | E09 |
| examples/r10_ereturn.cl | FAIL | E10 |
| examples/r11_eimmut.cl | FAIL | E11 |
| examples/r12_enotmut.cl | FAIL | E12 |
| examples/r13_eassignsh.cl | FAIL | E13 |
| examples/r14_emoveout.cl | FAIL | E14 |
| examples/r15_etemp.cl | FAIL | E15 |
| examples/r16_eescape.cl | FAIL | E16 |
| examples/r17_estoretemp.cl | FAIL | E16, E17 |
| examples/r18_ebranchmove.cl | FAIL | E06, E18 |
| examples/r19_edoubledrop.cl | FAIL | E19 |
| examples/r20_edropmoved.cl | FAIL | E20 |
| examples/r21_ealiascyc.cl | FAIL | E21 |
| examples/r22_eimmfield.cl | FAIL | E22 |
| examples/r23_enotmutfield.cl | FAIL | E23 |
| examples/r24_eelsejoin.cl | FAIL | E06, E24 |
| examples/r25_ecallmut.cl | FAIL | E25 |
| examples/r26_ecallmove.cl | FAIL | E26 |
| examples/r27_ecalloan.cl | FAIL | E27, E26 |
| examples/r28_erettemp.cl | FAIL | E28 |
| examples/r29_euninit.cl | FAIL | E29 |
| examples/r30_euninitret.cl | FAIL | E30 |
| examples/r31_eparamimmut.cl | FAIL | E31 |
| examples/r32_eparamnotmut.cl | FAIL | E32 |

---
產生器：弦律 v0.6 · helper P3a · 狀態 `/home/user/Layer5/.chordlaw/state.json`
