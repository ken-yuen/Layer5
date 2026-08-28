# YKC_27 — 專案現況審計、底層邏輯引擎評分與競品價值分析報告

> 日期：2026-08-28
> 基線：`ykc-serve-datalog`
> 性質：審計 + 分析報告——①全庫檔案完整性核查與修復 ②本輪代碼找錯/債/重 ③底層邏輯引擎評估與評分 ④聯網競品掃描與可吸收項分析。本輪代碼改動僅兩處（見 §2），其餘為分析。

---

## 1. 專案現況：完整性核查與事故如實記錄

### 1.1 工作區快照事故（第二次）與恢復

本輪開始時發現工作區快照還原再次不完整：`.git`、`.github/`、14 個 internal 包、4 個 cmd、
Go 工具鏈全部丟失。存留：全部 30 份報告 .md、README/Makefile/QUICKSTART 修改、
`internal/eventstore` 修復、`bin/` 二進制、雙 CLI 源碼。

恢復方法（與 YKC_25 後首次事故同一套流程，已驗證兩次有效）：

1. 重裝 Go 1.27.0（官方 tarball → `~/.ykc/go`）。
2. clone 上游取得 `.git`（base `e9a1a10`），`git ls-files --deleted | xargs git checkout --`
   精準恢復 **126 個**被刪 tracked 檔（不覆蓋存留修改）。
3. `internal/reports` 四檔（上輪 commit 時仍 untracked，checkout 無法恢復）以
   **存留二進制為行為 oracle** 完整重建：三件產物（INDEX/manifest/OUTLINE）與
   oracle 輸出**逐字節一致**，`list/check/show/verify` 全部輸出一致——重建版與
   丟失版功能等價有機械證據，非「憑記憶重寫」。
4. `internal/serve/main_test.go`、`internal/eventledger/main_test.go`（anchor 測試隔離）重建。
5. CI 三處修改重施；執行位 chmod 修復。

**教訓（第二次確認）**：快照不可作為唯一備份。本輪起以遠端 push 為正式備份通道（§5）。

重建期間逆向確認的解析規則（記入此處作為文檔）：日期=含「日期」關鍵詞行∪副標題行；
任務=首 20 行掃描、lexicographic 排序；摘要=首個普通段落逐行濾元數據行、220 runes 截斷；
INDEX 系列表摘要 80 runes 截斷、奠基表不截斷；KB=總位元組/1024 一位小數。

### 1.2 檔案完整性結論

| 類別 | 狀態 |
|---|---|
| 30 份報告 .md | ✅ 齊全（27 系列 + 3 奠基） |
| internal 22 包 / cmd 13 個 / core / l5 | ✅ 齊全（126 檔恢復 + 6 檔重建） |
| .github/workflows/ci.yml | ✅ 恢復 + 3 處修改重施 |
| Makefile / README / QUICKSTART / LICENSE / .gitignore | ✅ 齊全 |
| docs/reports 三件產物 | ✅ 重生成，verify 一致 |
| bin/ 13 支二進制 | ✅ 重建（make binaries） |
| deploy/ 腳本 + git hook | ✅ 齊全，執行位已修 |

`ci.yml` 注釋提及的 `cmd/ykc-cap` 為 YKC_21 未落庫代碼（YKC_23 已註明），非缺失。

---

## 2. 本輪代碼審計：找錯、債、重

全庫 23,429 行 Go。閘門現狀：`gofmt`/`go vet`/`staticcheck -checks=all` 零告警、
`go test -count=1` 22 包全綠、`go test -race`（serve/eventstore/eventledger/panel/reports/watch/datalog）全綠、
`toolchain-guard` 通過。TODO/FIXME/HACK 全庫零條。

### 2.1 修復（本輪 2 處）

- **B1（錯）`cmd/ykc-judge`：收據靜默吞錯**。`receipt.json`（含簽名的 L4 閉環產品本體）
  序列化與寫盤錯誤被 `_ =` 吞掉——人讀摘要顯示成功而收據不存在，直接違反
  「收據=產品」的第一性原理。修復：兩處錯誤皆 fatal（exit 1）。已過測試。
- **D1（重）`latestEpoch` 逐字重複兩份**（`cmd/ykc-atom` 與 `internal/serve`，
  舊注釋稱「cmd 不可被 import 故刻意重複」）。收斂為 `domain.LatestSnapshotEpoch`
  唯一實作 + 單元測試；兩呼叫點改用。反欺騙專案自己的事實選擇邏輯更不該有兩份。

### 2.2 記錄不修（接受的債，附理由）

- 雙 CLI 薄殼輔助函數（`mustScan/orDash/truncateRunes`）兩份——Go 慣例上 cmd 層
  小工具重複優於為 12 行建共享依賴；`internal/reports` 已承載全部實質邏輯。
- `contains/firstParagraph/writeJSON/rootOf` 跨包同名——語義各異（已逐一核對），非重複。
- `internal/enforcement` 的 `_ = os.Remove`——best-effort 解除阻擋，吞錯正確。

### 2.3 測試衛生

`go test -race` 七個關鍵包全綠；anchor 測試隔離（`YKC_ANCHOR_DIR` → tempdir）已隨
main_test.go 重建恢復——測試不再可能污染宿主 `~/.ykc/anchors` 信任根。

---

## 3. 底層邏輯引擎評估與評分

「底層邏輯引擎」按六個承重组件逐一評估（10 分制；依據=代碼實讀+測試+文檔交叉）：

| 組件 | 規模 | 強項 | 弱項 | 分 |
|---|---|---|---|---|
| **Datalog 引擎**（internal/datalog） | ~700 行 | 分層否定、安全檢查（頭部/否定變數必綁定）、neq 內建、跨行規則、確定性輸出（規則宣告序）、求值保險絲（MaxDerivedFacts/MaxRounds）、非分層顯式報錯 | 無聚合、無遞歸優化（半樸素求值未做）、無增量維護；規模屬「迷你引擎」 | **7.5** |
| **Oracle 判定鏈**（rustc/cargo/test/exit-code） | judge+smoke+guard | 第一性原理正確且被市場驗證（§4.1）：真相由環境產生、判定權不轉移（YKC_22 不變式）；四層煙測分層清晰 | 機械修復覆蓋面窄（rustc suggestion 子集）；無 Kani/Miri 級深度 oracle（§4.3 可吸收） | **8.5** |
| **帳本+錨定信任根**（ledger/eventstore/anchor） | ~1,500 行 | hash 鏈 + flock 單寫者 + HMAC head anchor（專案外存放）+ witness 可選；eventstore 確定性 ID（YKC_25 B1）；可重放 | anchor rotation 手動；未對齊 in-toto/SLSA 標準詞彙（§4.4） | **8.0** |
| **常駐監督面**（serve/watch/precompile） | ~1,600 行 | inotify+去抖（create/remove 抵銷）、single-flight 預譯、ErrLocked 指數退避、SIGTERM 優雅退出、路由表唯一化（YKC_25 B3） | Windows 僅 stat 輪詢；watcher 過濾規則硬編碼副檔名 | **7.5** |
| **知識庫**（kb） | ~1,200 行 | 內容定址、鎖版、可重放 release、繁中卡片 60 張 tier-1 | 檢索無向量層（R6 自覺擱置，進場條件未滿足——正確決定） | **7.0** |
| **報告雙 CLI**（reports） | ~800 行 | 確定性產物、verify 漂移閘門入 CI、oracle 逐字節驗證方法論 | 規則集固定 8 條；無跨報告一致性檢查（如術語表） | **7.5** |

**總評：7.6 / 10 —— 夠好、方向對、可用；距「好用」差在 DX 與生態接線，不在核心邏輯。**

- **夠唔夠好**：核心判定邏輯（oracle 鏈 + 帳本信任根）是同類中最紮實的一檔——
  8.5/8.0 兩項是全庫最高分，且哲學被 2026 學術界（Proof-or-Stop）與市場數據獨立驗證。
- **夠唔夠好用**：13 支 CLI 的認知負擔、無一鍵 installer、文檔 30 份但入門坡陡
  （QUICKSTART 存在但 assume 太多背景）——DX 綜合 **6.0/10**，是最高槓桿的改進面。
- **價值**：反欺騙定位從「差异化賣點」變成「品類必要條件」（§4.1）——價值主張升值，
  但視窗期在收窄：同類工具 2026 年密集出現（§4.2）。

---

## 4. 聯網掃描：能帶給 YKC 更好體驗/價值的東西

### 4.1 市場驗證（利好）

- Sonar《2026 State of Code》：**96% 開發者不完全信任 AI 代碼**，團隊每週 ~24% 工時
  花在檢查/修復/驗證 AI 輸出（[thenewstack.io](https://thenewstack.io/agentic-ai-verification-impact/)）。
  「驗證稅」正是 YKC 要消滅的對象——市場規模有硬數據。
- Pillar Security 2026-07「Week of Sandbox Escapes」：Cursor/Codex/Gemini CLI/Antigravity
  的邊界繞過多為「代理寫出宿主側受信組件事後會加載/執行的檔案」——**不是打破沙盒，
  而是讓沙盒輸出污染更高信任層**（[developersdigest.tech](https://www.developersdigest.tech/blog/securing-ai-coding-agents)）。
  這正是 YKC 原子監控+動態護欄的威脅模型；Cursor CVE-2026-50548 已修復但品類風險常駐。

### 4.2 直接競品（比 YKC 早到或同軌）

| 競品 | 形態 | 與 YKC 對照 | YKC 差異 |
|---|---|---|---|
| [agents-shipgate](https://github.com/ThreeMoonsLab/agents-shipgate)（ThreeMoonsLab） | 確定性 merge 閘門：單一決策引擎（report.json.release_decision）+ merge_verdict 確定投影 + capability lock + verification receipt + agent-handoff.json | **哲學幾乎同構**：claim≠evidence、收據綁定、代理不可自批（「coding agent must not self-approve」） | 非 Rust 專屬、無 oracle 鏈深度（rustc 級）、無帳本 hash 鏈；YKC 的 borrow 幾何/518 錯誤碼知識庫無對應 |
| Shipmoor | 本地確定性驗證層：掃描+測試證據+binding merge verdict，不上傳源碼 | 同「本地、確定性、證據」三關鍵詞 | 語言泛用=深度淺；YKC 深耕 Rust 生態四組 oracle |
| Galley | executor/supervisor 雙配置 + repo 定義品質閘門 + 可檢查證據 | 監督面思路相近 | 無信任根（帳本/錨定）概念 |
| agentnotary | 代理公證：加密封印+運行時護衛+EU AI Act 文檔 | 合規向 | YKC 是工程向；兩者互補 |
| Braintrust/Galileo/Promptfoo 等 | LLM-as-judge 幻覺檢測 | **哲學相反**：非確定性裁判 | YKC 的賣點正是「不用 LLM 判 LLM」；可作為互補層而非競品 |

**結論：YKC 的護城河 = Rust 深度（oracle 鏈+借用幾何+錯誤碼知識庫）× 信任根（帳本+錨定）。**
泛用競品無法速成這兩項；但「確定性 merge 閘門」品類已開跑，YKC 的 claim/verdict 詞彙
應尽快對齊 shipgate/Proof-or-Stop 的公開詞彙（merge_verdict、evidence admissibility），
讓生態可以直接消費 YKC 的輸出。

### 4.3 可吸收：讓 YKC 更強的外部成果

1. **Kani 自動 harness（verify-std/Autoharness）**：自動生成 16,748 個 MIR 級 proof
   harness、11,970 個通過 UB 檢查（[arxiv 2606.17374](https://arxiv.org/html/2606.17374v1)）。
   對 YKC：unsafe 代碼可加第五組 oracle（Kani BMC），把「煙測 T4」升級為「有界模型檢查
   證據」；LLM 代理產出的 unsafe fn 正是最高風險面。限制已知：僅單態化、無並發、有界。
2. **Proof-or-Stop 論文**（[arxiv 2607.14890](https://arxiv.org/html/2607.14890v1)，2026-07）：
   「self-report 不是證據；日誌說 All tests passed 也不是證據——除非收據重新綁定到
   即將合併的精確樹」——與 YKC 收據/claim 設計逐字同構。可直接引用為理論背書，
   並採納其「evidence admissibility（證據可採性）」術語升級 YKC_14 的 claim 詞彙。
3. **tree-sitter 生態**：GLR+增量+錯誤恢復已是事實標準（VSCode/Neovim/Zed/Emacs 全採用；
   query 編譯為狀態機單遍匹配）。YKC_26 T-20 路線（CL0→增量→ERROR 包攝）方向正確；
   YKC_21 已選 pure-Go gotreesitter，無 C 依賴——與 Go 單二進制分發紀律一致。
4. **Polonius 現狀**：next 版在 rustc 樹內重寫中、nightly `-Zpolonius=next` 原型、
   未完整（NLL problem case 3 仍失敗）。**含義：borrow 判定權短期內仍屬 rustc/NLL，
   YKC 的 ChordLaw 可視化與 26/26 oracle 一致性策略繼續有效**，不必追 Polonius。
5. **SLSA/in-toto/Sigstore**：provenance 標準化已完成（in-toto 信封 + SLSA 謂詞 +
   keyless 簽名 + Rekor 透明日誌；SLSA L3=hermetic build+平台簽名）。對 YKC：帳本
   anchor 與 judge 收據**輸出 in-toto 格式.attestation**，即可被 sigstore policy
   controller/OPA 類生態直接消費——小改動、大互操作。
6. **cursor-rust-tools / Rust MCP Server**：rust-analyzer 經 MCP 餵 LLM（hover/refs/
   cargo check 輸出）已有現成生態。YKC_22 的 rust-analyzer 解耦+MCP 方向被驗證；
   差異化應放在「MCP 出口帶帳本收據」——競品餵事實但不簽名，YKC 餵**可驗證**事實。
7. **HalluSquatting**（2026-07）：幻覺包名→供應鏈投毒。對 YKC：kb 的 import manifest
   （URL/ETag/SHA 指紋，YKC_18 已有）正是對此的正確防禦，可在文檔中顯式標註此威脅名。

### 4.4 優先級建議（按 ROI 排序）

| # | 行動 | 依據 | 成本 |
|---|---|---|---|
| P1 | judge 收據 + anchor 輸出 **in-toto attestation 格式**（並保留現有格式） | §4.3-5；互操作=生態入口 | 低 |
| P2 | verdict 詞彙對齊（merge_verdict/evidence admissibility），MCP 出口帶收據 | §4.2/§4.3-2/6 | 低 |
| P3 | DX：一鍵 installer + 13 CLI 收斂為 `ykc` 單入口子命令（serve 已示範模式） | §3 總評 6.0/10 | 中 |
| P4 | unsafe oracle：Kani 適配層（第五組 oracle，Available() 如實申報+降級） | §4.3-1 | 中 |
| P5 | T-20 結構 worker 按 YKC_26 路線推進（gotreesitter 增量+ERROR 包攝） | §4.3-3 | 高 |

---

## 5. 交付與備份紀律（本輪新增）

- 遠端 `https://github.com/ken-yuen/Layer5` 為正式備份通道；每輪完成即 push。
- 快照事故恢復手冊（§1.1 五步）記入本報告作為 runbook。

## 6. 邊界與未完成

- 本環境無 Rust 工具鏈：Kani/cargo 相關建議（P4）未實跑驗證，僅文檔級分析。
- 競品資訊來自公開網頁（檢索日 2026-08-28），Shipmoor/Galley 等未安裝實測，
  功能描述以其自述為準——按 YKC 自己的標準，這屬於「代理敘述」而非環境證據，
  採納前應逐一實測（P2 執行時順帶完成）。
- §3 評分為本輪審計者判斷（依據=代碼實讀+測試+文檔交叉），非環境產生事實；
  組件分數的可重複性依賴評審準則公開（本表即準則）。
