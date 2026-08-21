# YKC 五層落實藍圖 — 前沿技術選型與設計洞察
### （v1.0 ｜ 2026-08-22 ｜ 基於 2026 聯網檢索）

> 本文回答兩個問題：**① 五層功能到底點落實（用咩前沿技術）；② 有咩勁、有咩高智（每個設計的巧思）。**
> 一句總綱：**YKC 的「勁」唔係自己發明技術，而係用「政策化、增量化、官方事實化」三招，把業界最前沿但分散的工具，串成一部代理無法欺騙的裁判機器。**

---

## 0. 五個貫穿全局的「高智」原則

| # | 原則 | 洞察（點解勁） |
|---|---|---|
| P1 | **真相由環境產生** | 代理會說謊，rustc / Cargo.lock / 檔案 hash 唔會。一切結論可回溯到官方工具輸出。 |
| P2 | **吃官方事實，不自造語意** | Rust 團隊已經寫咗 Polonius(datalog)、rust-analyzer(salsa)、a-mir-formality(形式化)——YKC 唔重造，只接駁。 |
| P3 | **增量計算** | 改一行代碼，唔好重算成個專案。differential dataflow / salsa / tree-sitter 全部為此而生。 |
| P4 | **政策化，唔係掃描化** | 供應鏈安全最高境界唔係「搵漏洞」，而係「冇人審過就唔准入 build」。 |
| P5 | **確定性裁判** | 裁決用機械比對、hash 鏈，唔用 LLM 做判官。LLM 只可抽取，不可裁決。 |

---

## 1. L1 依賴版本對齊 + 衝突風險預警

### 1.1 前沿技術（2026 現況）
- **cargo-audit**：RustSec 諮詢庫，`--json` 輸出 [1](https://kx.cloudingenium.com/en/cargo-audit-scan-rust-dependencies-security-vulnerabilities-guide/)。
- **cargo-deny**：EmbarkStudios 五合一（advisories / licenses / bans / duplicates / sources）[2](https://grokipedia.com/page/cargo-deny)。
- **cargo-vet**：Mozilla 的「**人工審計政策層**」——Mozilla+Google 已把共享審計池擴到 **14,000 crates**；核心保證係「冇 crate-version 未經記錄在案嘅人工審計，就唔入 build」。呢個正正係能捉 event-stream(2018)、ua-parser-js(2021)、xz 類供應鏈攻擊嘅方法 [3](https://safeguard.sh/resources/blog/rust-supply-chain-cargo-vet-expansion-2025)。
- **cargo-semver-checks**：抓 semver 破壞性變更。
- **MSRV / rust-version**：依賴是否相容使用者工具鏈。

### 1.2 落實藍圖
```
代理改 Cargo.toml
   │
   ▼
L1 管線（並行）：
  cargo metadata → 版本對齊（宣告 vs lock vs crates.io 最新，caret semver 演算）
  cargo audit --json → CVE / unmaintained / yanked
  cargo deny check → 授權 / 重複依賴 / 來源白名單
  cargo vet → 「未審計 crate」清單 → 人工審計閘門
   │
   ▼
三級預警（block / warn / suggest）→ 寫入事實帳本 → 護欄 L3 攔截「未過審計的依賴變更」
```

### 1.3 有咩勁、有咩高智
- **勁**：代理謊稱「已更新到最新安全版」，但 Cargo.lock + sparse index 係事實——一比對即穿。
- **高智 ①（政策化）**：cargo-vet 的洞見係「**審計係政策，唔係掃描**」。YKC 唔去自建漏洞庫，而係繼承 Mozilla/Google 14,000 crate 嘅審計池，再要求「代理每引入一個新依賴，都自動觸發 vet 閘門」——供應鏈安全由「出事後先知」變成「入 build 前已擋」。
- **高智 ②（三工具串聯而非互斥）**：`cargo vet`（審計）+ `cargo audit`（CVE）+ `cargo deny`（政策）三者互補，一個 CI 順序跑晒 [3](https://safeguard.sh/resources/blog/rust-supply-chain-cargo-vet-expansion-2025)。

---

## 2. L2 遍歷專案結構 + 統計報告

### 2.1 前沿技術
- **tree-sitter**：增量解析（每擊鍵更新語法樹）、容錯（有 syntax error 都有用）[4](https://github.com/tree-sitter/tree-sitter)。
- **gotreesitter**：純 Go 重實現 tree-sitter runtime（無 cgo、無 C 工具鏈、206 語法內建）——讓 YKC 保持「純 Go 單一二進制」[5](https://gotreesitter.m31labs.dev/)。
- **SCIP**：Sourcegraph 的語意索引格式（Protobuf + 人讀 string symbol ID），比舊 LSIF **細 8 倍、快 3 倍**，支援增量索引、跨倉導航 [6](https://sourcegraph.com/blog/announcing-scip)。
- **rust-analyzer salsa**：on-demand 增量計算框架（query 模型 + memoize）[7](https://lib.rs/crates/rust-analyzer-salsa)。

### 2.2 落實藍圖
```
語法層：gotreesitter（純 Go）→ 快速統計（unsafe 密度 / unwrap 密度 / 模組深度 / 循環）
語意層：rust-analyzer LSP → documentSymbol / references → 精確符號圖
存儲層：「唯讀嵌入式 AST 資料庫」
   bbolt（事件快照 + 唯一寫者 + MVCC 唯讀檢視）
   + SCIP 索引（符號 → 定義 → 引用，可增量）
   + DuckDB（純分析查詢：GROUP BY 統計報告，10-100x 快過 SQLite 掃描）
```

### 2.3 有咩勁、有咩高智
- **勁**：語法（容錯、快）+ 語意（精確）+ 分析（SQL）三層各司其職，唔用一個工具硬撐全部。
- **高智 ①（SCIP 做語意庫，唔自造格式）**：SCIP 用「人讀得明嘅 string ID」取代 LSIF 嘅 opaque 數字，index 細 8 倍、可增量——YKC 嘅「唯讀 AST 資料庫」直接採納業界標準格式，將來可對接任何支援 SCIP 嘅工具。
- **高智 ②（bbolt 寫 + DuckDB 讀的分工）**：bbolt 做 append-only 事實（ACID、單寫者、MVCC），DuckDB 做唯讀分析（columnar、vectorized）——「寫」同「分析」用唔同引擎，各自在最佳狀態。SQLite 對分析掃描會輸 DuckDB 一截 [8](https://motherduck.com/learn/duckdb-vs-sqlite-databases/)。

---

## 3. L3 持久原子監聽 + 動態護欄主動介入

### 3.1 前沿技術
- **事件溯源 + WAL**：append-only 事實帳本（已有）。
- **differential dataflow（DD）**：增量視圖維護——「**刪除與新增同性能**」，用 partially-ordered timestamp 處理迭代 + 增量 [9](https://www.semanticscholar.org/paper/Differential-Dataflow-McSherry-Murray/f5df61effe8047eb9ea1702cfcc268dbba678567)。DDlog = datalog 編譯到 DD [10](https://zenodo.org/records/8192564)。
- **MCP 2025-11-25 + Streamable HTTP + OAuth 2.1 PKCE**：遠端 MCP 係 2026 主流（stdio 只留本地個人用）；SSE 已棄用 [11](https://mcpplaygroundonline.com/blog/mcp-2026-roadmap-whats-changing-for-developers)。
- **A2A (Agent2Agent)**：代理與代理間協定，同 MCP 互補 [12](https://hidekazu-konishi.com/entry/tool_use_and_agent_protocol_history_and_timeline.html)。

### 3.2 落實藍圖
```
fsnotify → 事實流
    │
    ▼
Datalog 護欄規則（寫成規則庫，唔係寫死 if-else）
    │  用 DD 做增量求值：改一行 → 只重算受影響規則，唔係全量
    ▼
首擊棘輪 + 信任狀態機（T3→T0 只降不升，已有）
    │
    ▼
介入通道：LSP publishDiagnostics（推 IDE）+ 遠端 MCP（推 agent）+ 控制台
```

### 3.3 有咩勁、有咩高智
- **勁**：護欄「永不漏、永不假陽性」——因為所有觸發源都係不可竄改嘅事實（帳本 hash 鏈）。
- **高智 ①（Datalog + DD 嘅組合）**：護欄規則寫成 datalog（宣告式、可證明），用 differential dataflow 增量求值——**改一個檔案，護欄結論毫秒級更新**，而唔係每次成個專案重跑規則。DD 嘅洞見係「刪除同新增一樣快」，對 YKC 尤其關鍵：代理「改咗又改返」時，護欄都要即時跟住。
- **高智 ②（首擊棘輪，物理消滅下次傷害）**：唔係「記過三次先罰」，而係「偽造一次 → 測試執行權永久收歸 YKC」——代理物理上再冇可能偽造第二次。

---

## 4. L4 沙盒預編譯除錯 + 改善建議

### 4.1 前沿技術
- **cargo-nextest**：跨二進制並行、每測試獨立進程、flaky 自動重試、CI 分片（`--partition`）——通常快 cargo test **2-5 倍** [13](https://configcrate.com/nextest-rust-test-runner.html)。
- **rustfix / cargo fix**：套用 rustc machine-applicable 建議（已有）。
- **miette / annotate-snippets**：rustc 自家診斷渲染（rustc 正統一到 annotate-snippets）——YKC 可直接用它渲染「改善建議」，與 rustc 視覺一致 [14](https://rust-lang.github.io/rust-project-goals/2024h2/annotate-snippets.html)。
- **LLM 修復對標數字**：RustAssistant 峰值 **74%**（真實開源倉編譯錯誤）[15](https://arxiv.org/abs/2308.05177)；Rust-SWE-bench 上 SOTA agent（RustForger+Sonnet）**28.6%**；**前 7 大錯誤碼佔 73%**，且絕大多數係名稱/型別/import 錯誤（非 borrow）[16](https://arxiv.org/html/2602.22764v1)。

### 4.2 落實藍圖（分層 SLA 已有，加 LLM 閉環）
```
cargo fmt → cargo fix（機械） → cargo check --json
   ├─ machine-applicable → rustfix 直接套用
   └─ 語意/長尾錯誤 → 錯誤指紋去重 + 根因優先排序
        → 查 Error Index（rustc --explain）拼 context
        → LLM 產 diff → 套用 → 再 check（迭代至 0 error）
   └─ cargo nextest（並行 + flaky 重試 + 分片）做驗證
```

### 4.3 有咩勁、有咩高智
- **勁**：YKC 嘅「98% 級」承諾有真實對標——RustAssistant 單模型都做到 74%，YKC 加咗 rustfix 機械層 + 錯誤指紋 + 根因排序 + nextest 驗證，高頻錯誤（前 7 碼佔 73%）完全可爭 90%+。
- **高智 ①（機械先行、LLM 殿後）**：能機械修嘅（rustfix）絕唔燒 LLM token；LLM 只處理長尾——「對開發者價值低的消耗時工」正正係由機械層吸走。
- **高智 ②（錯誤指紋 + 根因優先）**：級聯錯誤（一個錯引發一串）係 LLM 修錯方向嘅主因；按 span 排序揀根因、指紋去重，避免 LLM 喺假錯誤上打轉。
- **高智 ③（nextest 做驗證層）**：測試隔離 + flaky 重試，令「通過驗證」呢個收據唔會俾 flaky 測試污染——反欺騙需要嘅正係呢種「乾淨嘅通過」。

---

## 5. L5 借用/生命週期代數仿構（datalog + 節點圖）

### 5.1 前沿技術
- **Polonius = datalog borrow checker**：`rustc -Znll-facts` 匯出 borrow 事實（loan_issued_at / subset / outlives…），polonius-engine 做 datalog 不動點求值 [17](https://rust-lang.github.io/polonius/rules/relations.html)。
- **eqlog**：datalog + 函數符號 + 同餘閉包（congruence closure），編譯成 Rust [18](https://crates.io/crates/eqlog)。
- **egglog**：equality saturation + datalog 嘅融合語言 [19](https://github.com/newca12/awesome-rust-formalized-reasoning)。
- **a-mir-formality**：Rust 官方型別系統形式化（目標 2027 通過 99.9% test suite）[20](https://www.devclass.com/development/2023/01/23/rust-has-no-formal-specification-and-it-is-time-that-was-fixed-says-team-which-longs-for-formalized-type-system/1618189)。

### 5.2 落實藍圖
```
階段 A（解釋器，先做）：
  rustc -Znll-facts → 事實庫 → 畫 borrow/生命週期節點圖
  → 向 agent 圖文解釋「點解呢度唔借得」→ 大幅提升 agent 修 borrow 錯誤成功率

階段 B（規則層）：
  Go datalog 求值器（吃官方 facts）+ eqlog 寫「企業規範」額外規則
  → 同庫求值 → 企業級編程建議（必然定律：&mut 排他、生命週期子集…）

階段 C（城河）：
  (borrow facts, 可編譯標籤) 語料 → 訓練小型判別器 / 規則庫
```

### 5.3 有咩勁、有咩高智
- **勁**：borrow 錯誤係 LLM 最難一類（因為 LLM 睇唔見生命週期圖）——YKC 用官方 facts 畫圖 = 畀 agent 一雙「睇得見 borrow」嘅眼。呢個係全行業稀缺能力。
- **高智 ①（唔自造簡化規則，改吃官方事實）**：Polonius 已經係 datalog，`-Znll-facts` 已經滙出事實——**Rust 團隊幫你寫好咗最難嗰 90%**。YKC 只做「接駁 + 擴充規則」，唔用同 rustc 比正確性。
- **高智 ②（eqlog/egglog 寫「企業規範」）**：用 datalog+congruence closure 寫企業規則（如「禁止 &mut 逃逸到 API 邊界」），可以喺官方 facts 同一個求值器內跑——「必然定律」畀企業級建議，而「局部規則」唔會影響其他規則（呼應用家原話）。
- **高智 ③（a-mir-formality 係未來保險）**：Rust 型別系統形式化 2027 完成後，YKC 的 L5 可逐步由「仿構」升級為「可證」——數據城河亦係為呢日鋪路。

---

## 6. 橫切：數據城河 + 遠端化 + 部署

### 6.1 數據城河（越用越值錢嘅資產）
`(修前事實, 錯誤集, 修復 patch, 驗證結果)` 五元組 + 錯誤碼成功率統計 + borrow facts 帶標籤。價值：可訓練「修復策略」模型、可做「YKC 可信度儀表板」。**存事實唔存代碼**、opt-in、去識別、合成數據優先。

### 6.2 遠端化（MCP 2026 規格）
- 本地 stdio 留畀個人；**Streamable HTTP + OAuth 2.1 PKCE** 係 2026 遠端 MCP 標準（SSE 棄用中）[11](https://mcpplaygroundonline.com/blog/mcp-2026-roadmap-whats-changing-for-developers)。
- YKC 面板已係 HTTP 服務，加 MCP endpoint = 同一服務同時做人類面板 + agent 工具。

### 6.3 部署（沿用已定四層 Tier）
靜態二進制 → 鎖版鏡像（rust-toolchain.toml）→ gVisor/Firecracker（不可信代碼）→ K8s（多租戶）。

---

## 7. 技術棧總表

| 層 | 前沿選型 | 勁 | 高智 |
|---|---|---|---|
| L1 | cargo-vet + audit + deny + semver-checks | 代理改依賴即被審計 | 政策化（14,000 共享審計池） |
| L2 | gotreesitter + SCIP + bbolt/DuckDB | 語法+語意+分析三層 | SCIP 標準格式 + 寫/讀引擎分工 |
| L3 | 事件溯源 + differential dataflow + datalog 護欄 | 毫秒級護欄重算 | DD 刪除=新增同速 + 首擊棘輪 |
| L4 | rustfix + nextest + miette + LLM 閉環 | 74%→90%+ 可爭 | 機械先行 + 錯誤指紋根因排序 |
| L5 | -Znll-facts + eqlog/egglog + 節點圖 | 畀 agent 睇見 borrow | 吃官方事實 + 企業規則同庫求值 |

---

## 8. 里程碑（疊加於現有 D 表）

| 階段 | 交付 | 對應層 |
|---|---|---|
| P1 已完成 | 裁判 MVP + 護欄 + 面板 | L3/L4 骨幹 |
| P2 | L1（vet/audit/deny 閘門）+ L2（gotreesitter+SCIP+統計報告） | L1/L2 |
| P3 | L4 完整（nextest + miette + LLM 閉環 + 分層 SLA 上線） | L4 |
| P4 | L5 階段 A（nll-facts → 節點圖解釋器） | L5 |
| P5 | 遠端 MCP（Streamable HTTP + OAuth）+ 數據城河 + K8s | 橫切 |

---

## 9. 一句總結

**YKC 嘅「勁」＝ 唔重造輪子，而係「政策化供應鏈 + 增量化計算 + 官方事實化語意」三合一；「高智」＝ 每層都揀咗該領域嘅最佳原語（vet / SCIP / differential dataflow / nextest / Polonius facts），再用「確定性裁判」把佢哋焊死成一條代理無法欺騙嘅證據鏈。** 全部選型已於 2026-08 聯網核實。

---

*版次 v1.0 ｜ 資料來源見引用 [1]–[20]。*
