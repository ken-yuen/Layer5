# YieldKeyCode (YKC) — Rust 編程輔助器 深度分析報告

> 版本 1.0 ｜ 2026-08-21 ｜ 以聯網最新技術現況為基礎（rustc / Polonius / MCP / LSP 生態 2026-08 狀態）

---

## 0. 執行摘要

**結論：概念優秀、時機正確、技術上完全可行，但「98% 除錯」與「先開源後改閉源」兩處必須修正，否則價值主張不具說服力、商業化亦有法律風險。修正後整體評分可從 7.0 提升至 8.5+。**

| 維度 | 評分 (/10) | 一句話評價 |
|---|---|---|
| 願景與市場時機 | 8.0 | AI agent 寫 Rust 是剛性痛點，時機極佳 |
| 定位清晰度（命題） | 6.0 | 想法對，但文字表述混亂、指標模糊 |
| 技術可行性（5 層） | 8.5 | 每層都有現成原語可對接，無需重造輪子 |
| 架構設計（Go / 解耦 / LSP-first） | 8.0 | 方向正確，Go 做編排、Rust 生態做語意 |
| 指標定義與可驗證性（98%） | 3.5 | 目前不可量測、不可證偽，是最大硬傷 |
| 商業化與授權策略 | 5.5 | open-core 對，但「事後改閉源」法律上不可行 |
| 護城河數據策略 | 7.0 | 方向對，但應存「事實/規則」而非「代碼」 |
| 風險管理 | 6.0 | nightly 依賴、級聯誤差、外部二進制需納管 |
| **整體** | **7.0** | **好構想，需在指標、授權、範圍三處收緊** |

---

## 1. 技術現況盤點（聯網最新）與依賴建議

以下為 2026-08 聯網搜尋確認的關鍵事實，直接決定 YKC 各層的技術選型。

### 1.1 命脈事實

1. **rust-analyzer 本質是「語意分析庫」，LSP 只是它的接口**。任何語言都能用 JSON-RPC over stdio 驅動它，在編輯器外做靜態分析（已有成功案例）[1](https://medium.com/@selfint/lsp-outside-the-editor-431f77a9a4be)。→ 這正是 YKC「以 LSP 作為首階段接駁」的正確性依據：**YKC 不需要自己維護一份 Rust 前端**，透過 LSP 即可免費取得 name resolution、型別推論、診斷、重構能力。

2. **`cargo metadata --format-version=1`** 直接輸出 workspace 完整依賴圖（`packages` + `resolve.nodes`），是第 1 層的資料基座 [2](https://rustwiki.org/en/cargo/commands/cargo-metadata.html)。

3. **`cargo check --message-format=json`** 輸出帶 span、error code、`suggestions`（機器可套用修復）的結構化診斷，是第 4 層的 ground truth [3](https://doc.rust-lang.org/cargo/commands/cargo-fix.html)。

4. **`cargo fix` / rustfix** 能自動套用 rustc 的 `machine-applicable` 建議；`--broken-code` 可在程式碼有錯時也套用 [3](https://doc.rust-lang.org/cargo/commands/cargo-fix.html)。→ 第 4 層的機械化前置步驟。

5. **供應鏈安全不用自建**：`cargo-audit`（RustSec advisory DB，`--json` 輸出）+ `cargo-deny`（EmbarkStudios：advisories / licenses / bans / duplicates / sources 五合一）已覆蓋第 1 層 80% 需求 [4](https://kx.cloudingenium.com/en/cargo-audit-scan-rust-dependencies-security-vulnerabilities-guide/) [5](https://grokipedia.com/page/cargo-deny)。

6. **Polonius 就是 datalog**：rust-lang/polonius 用「事實 (facts) + 規則 (rules) + 不動點求解」實現 borrow checker，且可用 `rustc -Znll-facts` 匯出事實 [6](https://github.com/rust-lang/polonius)。**2026-08-04 起 Polonius Alpha 已在 nightly 預設啟用，穩定化在數月內** [7](https://byteiota.com/polonius-rust-borrow-checker-nightly/)。→ 你的第 5 層直覺（datalog + 代數仿構 borrow 邏輯）**已被 Rust 官方以同一思路實現**，這是最重要的好消息：第 5 層應「借用官方事實」，而非「自創一套簡化規則」。

7. **Go 生態 MCP SDK 已成熟**：`mark3labs/mcp-go`（MIT，v0.57.0，支援 stdio / SSE / streamable-HTTP，實現 MCP 規格 2025-11-25）[8](https://pkg.go.dev/github.com/mark3labs/mcp-go)，另有官方 `modelcontextprotocol/go-sdk`。

8. **Go 插件沙盒**：`wazero`（WebAssembly 零依賴沙盒 runtime）是 Go 插件擴展的推薦方案；`hashicorp/go-plugin`（RPC 型）為備選 [9](https://www.reddit.com/r/golang/comments/1kswbfc/whats_your_experience_with_go_plugins/)。

9. **成效基線已有對標物**：Rust-SWE-bench 上，SOTA agent（RustForger + Claude-Sonnet-3.7）任務解決率約 **28.6%**；錯誤碼分佈顯示前 7 大錯誤（E0599/E0433/E0432/E0425/E0308/E0277/E0412）佔 **~73%**，且**絕大多數是名稱解析/型別/import 錯誤，而非 borrow 錯誤** [10](https://arxiv.org/html/2602.22764v1)。→ 這直接影響「98% 除錯」的可行性論證（見 §6）。

### 1.2 推薦依賴清單

**Go 側（YKC 本體）**

| 用途 | 依賴 | 授權 | 說明 |
|---|---|---|---|
| LSP 客戶端 | `github.com/tliron/glsp` 或 `go.lsp.dev/protocol` | Apache-2.0 / MIT | 驅動 rust-analyzer；若不滿意可自建 stdio JSON-RPC（約 300 行） |
| TOML 解析 | `github.com/pelletier/go-toml/v2` | MIT | 解析 Cargo.toml（快速、流式） |
| 語法 AST | `github.com/smacker/go-tree-sitter` + `tree-sitter-rust` 語法 | MIT | 增量、容錯的語法樹，第 2 層主力 |
| 檔案監聽 | `github.com/fsnotify/fsnotify` | BSD-3 | 第 3 層觸發源 |
| 嵌入式 KV / 快照 | `go.etcd.io/bbolt` | MIT | read-only snapshot、MVCC、單檔，做「唯讀 AST 資料庫」的事件/快照層 |
| 嵌入式分析 DB | `modernc.org/sqlite`（純 Go，無 cgo）或 `github.com/marcboeker/go-duckdb` | BSD / MIT | 對 AST 事實做 SQL/分析查詢；DuckDB 適合大量「統計報告」查詢 |
| MCP | `github.com/mark3labs/mcp-go`（或官方 `go-sdk`） | MIT | 第 2 階段把 YKC 能力包裝成 agent 工具 |
| 插件沙盒 | `github.com/tetratelabs/wazero` | Apache-2.0 | 後期 IDE/插件/第三方擴展的沙盒執行 |
| 事件匯流排 | `github.com/nats-io/nats.go`（可嵌入式）或 `connectrpc` | Apache-2.0 | 解耦各層、並行歧異操作的骨幹 |
| 併發 | 標準庫 `errgroup` / `x/sync` | BSD | — |
| 可觀測 | `go.opentelemetry.io/otel` | Apache-2.0 | 指標（這正是你「說服力指標」的來源） |

**Rust 側（作為外部二進制/sidecar，用 rustup 管理 toolchain）**

| 用途 | 工具 | 說明 |
|---|---|---|
| 語意分析 | `rust-analyzer` | LSP server，YKC 的最重依賴 |
| 編譯 ground truth | `rustc` / `cargo check --message-format=json` | 第 4 層 |
| 依賴圖 | `cargo metadata --format-version=1` | 第 1 層 |
| 安全/政策 | `cargo-audit`、`cargo-deny` | 第 1 層 |
| 機械修復 | `cargo fix`（rustfix） | 第 4 層前置 |
| 錯誤說明 | `rustc --explain E0xxx` + Compiler Error Index | 第 4 層「改善建議」文案來源 |
| borrow 事實 | `rustc -Znll-facts`（nightly）/ Polonius | 第 5 層 |
| 教學文檔 | rust-lang.org/learn 全套（Book / Reference / Nomicon / Error Index / Cargo Book / rustc Book / Unstable Book） | 第 4、5 層知識庫語料 |

---

## 2. 五層達成方法（逐層技術方案）

> 建議實現順序**不是 1→2→3→4→5**，而是 **4→1→2→3→5**（理由見 §4）。以下按層級編號說明。

### 第 1 層：依賴版本對齊與衝突風險預警

**資料來源（全部現成，不要重造）**
- `cargo metadata --format-version=1` → 解析出完整 `resolve` 圖。
- 解析 `Cargo.lock`（用 go-toml 讀 TOML 即可）。
- 查 crates.io 最新版：sparse index（`index.crates.io`）或 `crates.io/api/v1/crates/{name}`。
- `cargo-audit --json`（RustSec advisories）＋ `cargo-deny check advisories|licenses|bans|sources`。

**檢測項目**
1. 版本對齊：每個直接依賴「宣告版本 vs lock 實際解析 vs crates.io 最新版」三者差；MSRV 是否符合專案 `rust-version`。
2. 衝突風險：同一 crate 多版本並存（duplicates）、yanked 版本、RUSTSEC 漏洞、license 衝突（GPL 污染）、feature 衝突、build script 網路行為。
3. 預警輸出：分「阻塞（block）/警告（warn）/建議（suggest）」三級，附「升到哪個版本安全」的 semver 區間建議（`^` caret 相容性演算）。

**Go 實作要點**：包一層 `cargo-deny`/`cargo-audit` 子行程 + 自寫 semver 比較（`golang.org/x/mod/semver`）。「高兼容規格」= 不接管 cargo 的解析邏輯，只讀官方輸出。

### 第 2 層：遍歷專案數據結構與統計

**雙引擎**
- **語法層**：tree-sitter-rust 增量解析 → 快速統計（struct/enum/impl/fn/trait/unsafe block/`unwrap()`/`panic!`/module 深度/循環 import）。
- **語意層**：rust-analyzer LSP（`textDocument/documentSymbol`、`references`、hover 型別）→ 精確的符號與呼叫圖。

**輸出場景化**
- 「新人上手報告」：模組地圖、公開 API 清單。
- 「重構評估」：循環依賴、超深模組、巨型 struct、`unsafe` 集中點。
- 「安全審計」：unsafe 區塊、`unwrap`/`expect` 密度、FFI 呼叫點。
- 格式：JSON / Markdown / HTML / 直接在 AST DB 上跑 SQL。

**唯讀嵌入式 AST 資料庫**：設計為「**追加式事實庫**」——每次變更把 tree-sitter 節點 + rust-analyzer 符號寫入 bbolt（snapshot/MVCC，天然唯讀檢視）或 DuckDB（分析查詢）。**唯讀**體現在：分析層永遠只開 read transaction，寫入只由索引器單一寫者進行 → 滿足「並行歧異操作」與「解耦」。

### 第 3 層：持久原子監聽 + 動態護欄主動介入

- **監聽**：fsnotify 監聽 `.rs`/`Cargo.toml` 變更 → 事件溯源（append-only log + 序列號）。
- **持久原子**：狀態機 + WAL + 冪等重放；快照存 bbolt read-only view。「原子」= 每次狀態轉換要嘛完整落地、要嘛回滾，這是 Go 的強項（單一寫者 + 事務）。
- **動態護欄（guardrails）**：宣告式規則引擎，例如：
  - `borrowck_errors > 0 → 阻止 agent 宣告「完成」，並推送修復指引`
  - `cargo check 未通過 → 攔截 agent 的「提交/PR」動作`
  - `新增依賴未過 audit → 警示並要求人工確認`
- **主動介入通道**：LSP `publishDiagnostics`（推給 IDE）＋ MCP tool（推給 agent）＋ CLI 事件流。

### 第 4 層：沙盒預編譯 + 改善建議（價值核心，先做）

**沙盒**（按成本遞增，先做輕量版）
1. 隔離工作目錄 + 獨立 `CARGO_HOME`/`target` + 網路 egress 白名單 + 資源限制（CPU/mem/超時）。
2. 需要更強隔離時再上容器（runc / gVisor / firecracker）。

**除錯循環（這是「98%」的真正引擎）**
```
cargo fmt ─► cargo fix (機械修復) ─► cargo check --message-format=json
    ▲                                        │
    │                                        ▼
    └── 重複直到 0 error ── 解析 spans/error code/suggestions
                                            │
                ┌───────────────────────────┴───────────────┐
                ▼                                            ▼
      machine-applicable 建議                   語意/借用/邏輯錯誤
      → 直接套用（rustfix）                     → 查 Error Index + rustc --explain
                                                → 附型別/符號資訊（LSP）
                                                → LLM 生成帶 diff 的修復
                └───────────────► cargo test（可運行性驗證）──► clippy（品質）
```
**改善建議回報**：每個錯誤附「錯誤碼 → 官方解釋（`rustc --explain` + Compiler Error Index）→ 定位資訊（span + 相關型別）→ 具體修復 diff → 為何這樣修的說明」。這正是 rust-lang.org/learn 的 Compiler Error Index 的直接價值。

### 第 5 層：借用/生命週期代數仿構（datalog + 節點圖）

**核心洞察：這條路 Rust 官方已經走通了，你的直覺正確，但方向要反過來**——不要「自己簡化規則」，而要「**吃官方事實**」：

1. **事實匯出**：`rustc -Znll-facts`（nightly）把每個函式的 borrow 事實（`borrow_region`、`outlives`、`subset`、`loan_issued_at` 等）匯出成事實檔 [6](https://github.com/rust-lang/polonius)。
2. **求值**：三選一——
   - (a) 直接用 `polonius-engine`（Rust sidecar，CLI 呼叫）；
   - (b) **Go 自寫一個 ~1,000 行的 semi-naïve datalog 求值器**（這是護城河：既吃官方事實，又可加入 YKC 自有的「擴充規則」，如企業編碼規範規則）；
   - (c) 混搭：官方事實 + YKC 規則在同一事實庫上求值。
3. **節點圖**：把事實轉成 **MIR/CFG/borrow 圖視覺化**（節點 = 位置/區域，邊 = 借用/存活/子集關係）。**這才是第 5 層最早能變現的產物**：borrow 錯誤對 LLM agent 是「最難」的一類，因為它看不見圖；YKC 把「為什麼這裡不能借」畫成圖 + 文字，能顯著提升 agent 修 borrow 錯誤的成功率。
4. **企業級建議**：「必然定律」（如 `&mut` 排他性、生命週期子集關係）做成確定性 lint，直接給企業級編程建議。
5. **注意**：`-Znll-facts` / Polonius 都在 nightly，且近期變動快（Polonius Alpha 2026-08 才上 nightly [7](https://byteiota.com/polonius-rust-borrow-checker-nightly/)）。**必須封裝成 sidecar + rustup toolchain 鎖版本**，否則會碎。

---

## 3. 整體架構設計（Go）

```
┌──────────────────────────────────────────────────────────────┐
│                        YKC Core (Go)                          │
│                                                              │
│  ┌───────────┐  ┌────────────┐  ┌─────────────┐             │
│  │ LSP Client │  │ Tree-sitter │  │ Cargo 子行程 │  (邊車)     │
│  │(rust-analyz)│  │  (AST)     │  │(metadata/    │            │
│  └─────┬─────┘  └─────┬──────┘  │ check/fix/   │            │
│        │              │         │ audit/deny)  │            │
│        ▼              ▼         └──────┬───────┘            │
│  ┌──────────────────────────────────────┴─────────┐         │
│  │         事件匯流排 (NATS embedded / pub-sub)      │         │
│  └───────┬─────────────────────────────────────────┘         │
│          ▼                                                    │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────────┐    │
│  │ L1 依賴檢查  │  │ L2 統計/報告   │  │ L3 監聽/護欄       │    │
│  ├─────────────┤  ├──────────────┤  ├───────────────────┤    │
│  │ L4 沙盒除錯  │  │ L5 borrow 分析│  │ 規則引擎 + 狀態機   │    │
│  └──────┬──────┘  └──────┬───────┘  └────────┬──────────┘    │
│         ▼                 ▼                   ▼               │
│  ┌──────────────────────────────────────────────────────┐    │
│  │ 唯讀嵌入式 AST 資料庫 (bbolt 快照 + DuckDB 分析)         │    │
│  └──────────────────────────────────────────────────────┘    │
│                              │                                │
│        ┌─────────────┬───────┴───────┬──────────────┐        │
│        ▼             ▼               ▼              ▼        │
│   LSP server    MCP server      CLI / gRPC    WASM 插件(wazero)│
│  (接 IDE)      (接 AI agent)    (接 CI/人)     (第三方擴展)      │
└──────────────────────────────────────────────────────────────┘
```

**設計原則**
- **Go 做編排，Rust 生態做語意**：YKC 不重寫編譯器/型別系統，所有「Rust 事實」都來自 rustc/rust-analyzer/polonius。這是「高兼容規格」的本質——**rustc 是 single source of truth**。
- **解耦**：五層之間只透過事件匯流排 + 資料庫介面通訊，任一一層可獨立替換（呼應「高擴展與解偶性」）。
- **並行歧異操作**：goroutine 天然契合；每個檔案的分析是獨立任務，用 errgroup 並行、結果冪等合併。
- **接駁演進**：LSP（階段一，接 IDE/編輯器）→ MCP（階段二，接 AI agent，把 L1–L5 包裝成 agent 可呼叫的 tools）→ 插件（wazero WASM）→ IDE 插件（分發渠道）。四者共用同一個 Core，互不依賴。

---

## 4. 我會如何開發（路線圖）

> **核心原則：以第 4 層為價值引擎，先做出可量測成果，再向外擴張。** 你原文也點出了這個邏輯——「除錯不能實現，其他根本亦難實現」。

**Phase 0 — 命脈驗證（1–2 週）**
PoC 只做三件事，證明三條命脈可行：
1. Go 驅動 rust-analyzer（LSP stdio）拿到診斷 + 型別。
2. `cargo check --message-format=json` 解析 spans + suggestions。
3. `cargo metadata` 解析依賴圖。
→ 這三步通了，YKC 就成立；不通，一切都免談。**先做這個，別急著搭五層框架。**

**Phase 1 — MVP = 第 4 層 + 第 1 層（1–2 月）**
- 沙盒編譯 + 除錯循環（cargo fmt → cargo fix → cargo check → LLM 修復 → 重複 → cargo test）。
- 第 1 層包裝 cargo-audit/cargo-deny（成本低、立刻有商業賣點）。
- 產出第一個可對外宣傳的指標：「在基準語料上的 compile-pass 率、平均修復輪數、剩餘錯誤數」。

**Phase 2 — 第 2 層 + 第 3 層（2–3 月）**
- tree-sitter 統計 + 場景化報告；bbolt 快照 + fsnotify 監聽 + 護欄規則引擎。
- 此時 YKC 已能「常駐守護」agent 的開發流程。

**Phase 3 — 第 5 層 PoC + MCP + 插件（3–6 月）**
- `-Znll-facts` → Go datalog 求值 → borrow 節點圖 + 文字解釋（先做「解釋器」，再做「自訂規則」）。
- MCP server 上線（`mark3labs/mcp-go`），YKC 成為 agent 的工具集。
- wazero 插件沙盒，開放第三方規則/報告插件。

**Phase 4 — 數據城河 + 商業化（6 月後）**
- 累積「(修前代碼事實, 錯誤集, 修復 patch, 驗證結果)」語料（去識別、opt-in、合成優先）。
- open-core 分層授權（見 §6.3）。

**為什麼這方案能達成願景**
1. **第 4 層先做 = 直接命中「98% 除錯」價值主張**，且 rustc 給的是 100% 的錯誤清單（ground truth），YKC 提供的是「把這 100% 修到 0」的**編排與加速**，而不是重新發明診斷——可量測、可證偽、可宣傳。
2. **五層全部建立在官方原語上**（cargo/rustc/rust-analyzer/polonius），不走「自研簡化版 Rust 語意」的險路，既保「高兼容」，又天然「高擴展」。
3. **LSP→MCP→插件→IDE 的接駁演進**與 AI agent 生態的成長節奏同頻：今天 agent 走 MCP，明天 IDE 內嵌 agent，YKC 兩邊都通吃。
4. **數據城河**不是空談：YKC 每次修復都產出「帶驗證標籤」的結構化事實，這是別人沒有、且越用越值錢的資產。

---

## 5. 客觀深度分析（逐項）

### 5.1 亮點（真實優勢）
- **時機**：AI agent 寫 Rust 的最大失敗點就是編譯不過（Rust-SWE-bench SOTA 也才 28.6% [10](https://arxiv.org/html/2602.22764v1)），YKC 正好卡在「編譯成功率」這個最痛的斷點。
- **「以 LSP 首階段接駁」是內行判斷**：rust-analyzer 是現成、免費、持續進化的 Rust 語意引擎，用它等於站在巨人的肩膀上 [1](https://medium.com/@selfint/lsp-outside-the-editor-431f77a9a4be)。
- **第 5 層的 datalog 直覺被官方驗證**：Polonius 就是 datalog borrow checker，你的「代數仿構」其實有官方事實可吃，風險遠低於自創規則。
- **Go 選型合理**：單一靜態二進制、goroutine 併發、生態成熟，適合做「編排層」與開發者工具分發。

### 5.2 硬傷（必須修正）
1. **「98% 除錯率」不可操作化**：目前既無分母定義（什麼算一個「錯誤」？）、也無語料範圍（任意企業代碼？）、更無證偽方法。**這會成為對外宣傳的致命傷**——客戶一問「98% 怎麼測的」就穿幫。詳解與修正見 §6.1。
2. **「先全開源、數據爆發後改商業 license」法律上不可行**：已以開源授權發布的程式碼，授權不可單方撤回；社群 fork 會立刻分走。「後改閉源」唯一可行路徑是 Day 1 就 dual-license + CLA（見 §6.3）。
3. **「100% 找出阻截錯誤」要精確表述**：rustc 確實會回報**全部**錯誤，但存在**級聯誤差**（一個錯引發一串錯）。正確表述應是「以 rustc 診斷為 100% ground truth，YKC 保證不遺漏 rustc 已回報的任何 blocker」，而非「YKC 自己 100% 找出所有錯」。
4. **第 5 層過度承諾風險**：自研 borrow 分析想「完全仿照真實規則」工程量極大（rustc 的 borrowck 是數年累積）。**先做「解釋器/視覺化」而非「判定器」**，價值更快、風險更低（見 §6.2）。
5. **外部二進制依賴納管**：rust-analyzer/rustc nightly/Polonius 版本變動會打碎 YKC；需 rustup toolchain 鎖版 + sidecar 封裝 + 啟動時自檢。

---

## 6. 現實使用的高質優化建議（核心交付）

### 6.1 把「98%」改造成可證偽的指標體系（最重要）
**放棄單一口號，改用三層 SLA + 公開 benchmark：**

- **L0（機械層，承諾可近 100%）**：「rustc 回報的錯誤 100% 被 YKC 捕獲、歸檔、並標註可否機械修復」——這個 YKC 穩拿。
- **L1（高頻錯誤層，承諾 90%+）**：前 10 大錯誤碼（E0599/E0433/E0432/E0425/E0308/E0277/E0412/E0412/E0282…，佔總量 ~80% [10](https://arxiv.org/html/2602.22764v1)）在**受限語料**（如 rustlings + 自建基準）上的單輪修復成功率。這些多是名稱/型別/import 錯誤，**高度可機械化**，90% 是可爭取的。
- **L2（深層語意層，不承諾數字）**：borrow/生命週期/邏輯錯誤，提供「多輪迭代 + 節點圖解釋」的編排，報告「N 輪內消除比例」。

**對外呈現**：發一張「錯誤消除曲線」（第 0…N 輪 vs 剩餘錯誤數）＋「有 YKC vs 無 YKC 的 compile-pass 對照」（以 Rust-SWE-bench 或自建基準）。**有基線的對照圖，比「98%」有說服力一百倍。**

### 6.2 第 5 層改為「先解釋、後判定」
1. 階段 A（3–6 月）：`-Znll-facts` 事實 → 節點圖 → **向 agent 圖文並茂解釋 borrow 錯誤**。這是全行業稀缺、立刻可賣的能力。
2. 階段 B（6–12 月）：Go datalog 求值器吃官方事實 + YKC 自訂「企業規範規則」（如「禁止 `&mut` 逃逸到 API 邊界」）。
3. 階段 C（長期）：把「事實 → 可編譯性」標籤數據做成判別模型/規則庫。
**切勿一開始就做「判定器」**，那會陷入與 rustc 比正確性的泥潭。

### 6.3 授權：Day 1 就定 open-core，放棄「事後閉源」
- **核心引擎**：MPL-2.0 或 AGPL-3.0（MPL 檔案級 copyleft，最適合工具類專案）。
- **企業功能**（沙盒叢集、團隊護欄、數據城河、SLA）：閉源商業版。
- **數據庫/規則庫**：單獨閉源或訂閱授權（這才是你要「爆發價值」的資產）。
- **貢獻者協議 (CLA)**：沒有 CLA，你連「改雙授權」都做不到。
- 若想社群聲量最大化：核心 MIT/Apache（學 rust-analyzer），**但數據層從頭就閉源**。

### 6.4 數據城河：存「事實與規則」，不存「代碼」
真正有長期價值的數據（價值由高到低）：
1. **(修前事實, 錯誤集, 修復 patch, 驗證結果) 五元組**——唯一可訓練「修復策略」的資產。
2. **錯誤碼 × 修復成功率統計**——可產品化為「YKC 可信度儀表板」。
3. **borrow 事實 + 可編譯標籤**——可訓練小型判別器。
4. 匿名化的專案結構/依賴圖譜聚合。

**法律與隱私**：優先 **opt-in + 去識別 + 合成數據**（自建「故意打錯→修復」生成器，批量產出乾淨的 broken→fixed 對）；避免吞入 GPL 程式碼污染訓練集。**「存代碼」價值密度低、法律風險高；「存事實」反之。**

### 6.5 工程與產品
- **Dogfooding**：用 YKC 開發 YKC（你原文提到），這是最強的自證循環與活廣告——每週公佈「YKC 開發 YKC 的編譯成功率」。
- **CLI/MCP 優先於 IDE 插件**：agent 是首要客戶，MCP tool 體驗是生死線；IDE 插件後置。
- **增量與快取**：只重分析髒檔案；sandbox 目錄/`CARGO_HOME` 重用；錯誤指紋去重（避免同一錯誤反覆送 LLM）。
- **rustc 是唯一真相**：任何 YKC 結論必須能回溯到 rustc/rust-analyzer 輸出，否則會失去「高兼容」信用。
- **成本控制**：LLM 呼叫前先過「機械修復 + 高頻模板」，只有長尾錯誤才進 LLM——這正是「對開發者價值很低的消耗時工減輕」的落點。

### 6.6 風險清單（納入每週檢查）
| 風險 | 緩解 |
|---|---|
| Polonius/`-Znll-facts` nightly 變動 | sidecar 封裝 + rustup 鎖版 + CI 追蹤 nightly |
| rust-analyzer 升級行為變化 | 鎖定版本、升級走測試矩陣 |
| 級聯誤差導致 LLM 修錯方向 | 每輪先修「根因錯誤」（依 span 排序） |
| 大型專案分析耗時 | tree-sitter 增量 + 只分析髒檔案 + 並行 |
| 客戶程式碼外洩（沙盒/LLM） | 沙盒 egress 白名單、LLM 去識別、on-prem 部署選項 |

---

## 7. 結語

YKC 的底層直覺——「**讓 AI agent 先把 Rust 程式碼編譯過，是一切價值的入口**」——是正確且稀缺的。五層架構每一層都能對接到現成的官方原語（cargo/rustc/rust-analyzer/polonius），技術路徑不存在原理性障礙；Go 做編排、Rust 生態做語意、LSP→MCP→插件的接駁演進，也都是成熟且正確的選擇。

**真正阻礙 YKC 成為「神器」的，不是技術，而是三件事**：
1. 把「98%」從口號改造成有基線、可證偽的分層指標；
2. 把授權從「事後改閉源」改成 Day 1 的 open-core + CLA；
3. 把第 5 層從「自製簡化規則」改成「吃官方事實、先做解釋器」。

這三件事做完，YKC 的整體評分可由 7.0 提升至 8.5+，且具備真實的企業級與商業化潛力。

---

*報告完 ｜ 資料來源為 2026-08 聯網檢索；rustc/Polonius/MCP 等快速演進項目，落地前請以官方最新文檔複核。*
