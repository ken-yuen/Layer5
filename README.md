# YieldKeyCode (YKC) — Rust 編程輔助器

> **使命**：讓 AI agent 在使用 YKC 後，能真實開發出「可運行、可驗證、可維護」的企業級 Rust 專案；同時讓非技術用家**無法再被代理欺騙**。
>
> **第一性原理**：真相由環境產生，不由代理敘述產生。YKC 把「驗證權」從代理手中拿走，做成環境屬性——抗欺騙能力與代理是誰無關。

## 快速開始（任何裝置）

```bash
# 方式 A：像普通程式——雙擊啟動器（macOS 雙擊 launch.command；Linux launch.sh；Windows launch.bat→WSL）
#   自動：裝環境 → 建二進制 → 起控制面板 → 開瀏覽器

# 方式 B：命令行（原生，無需容器）
make lint           # gofmt + vet + staticcheck -checks=all（合入前置）
make verify-all     # lint 後一鍵跑全部功能實測
make panel          # 啟動 YKC Trust Console（控制 + 觀察台）
make serve          # 啟動 ykc-serve 常駐進程（監看+聲明評估+面板合一）
# （T-19/T-20 能力包的 make deps / make structure 目標，待 YKC_21 代碼落庫後恢復；見下文）

# 方式 C：容器（Podman 或 Docker 通用）
make image && make up
```

## YKC Trust Console（控制 + 觀察台）

`make panel` 或雙擊啟動器後，瀏覽器開 <http://localhost:8080>：

- **控制（人類觸發）**：選擇運行對象（專案）→ 煙測/除錯/閘門/護欄動作 → 開始/停止 → 實時日誌與狀態。
- **觀察（唯讀）**：專案完整性、信任等級（T0–T3）、判決與證據、即時事實流（hash 串鏈）——全部直接讀自 `.ykc/ledger.jsonl`，不改寫；專案卡另顯示 project 外的 **Head anchor**，可攔截截斷／末行重簽。
- **知識庫（唯讀）**：直接在面板搜尋鎖版 Rust 錯誤碼、規則與文檔閉包；顯示 dataset/rustc metadata、繁中摘要與官方來源，不新增任何控制權。
- **L1/L2（可選能力包）**：建置 `ykc-deps`／`ykc-structure` 後，面板可顯示依賴 block/warn 與 Rust/Go 結構觀察；核心只讀其已驗證、anchored 的事實，不會把工具鏈或 grammar 塞進 T0。
- **AI 看（機器可讀）**：`GET /api/state`、`GET /api/raw?project=<dir>`、`GET /api/know/*`、`GET /api/projects`、`GET/POST /api/jobs`、`GET /healthz`。
- `make panel`／啟動器現在使用 `ykc-serve`，所以 Rust 預譯會主動執行；只想開不監看的薄面板時才直接使用 `ykc-panel`。

### 面板安全邊界（2026-08 加固）

- **預設綁 127.0.0.1（本機）**；要暴露到網路須顯式 `-addr 0.0.0.0:8080`，且未設 token 時啟動即打印安全警告。
- **控制端點可加 Bearer token**：`-token <密鑰>` 或 `YKC_PANEL_TOKEN` env；未授權 401。前端 401 時會提示輸入並記憶。
- **任務 project 白名單**：`POST /api/jobs` 的 project 必須在已發現的 Cargo 專案內，任意路徑（如 `/etc`）一律 400 拒收——杜絕經面板在攻擊者目錄觸發 cargo（build.rs → 任意代碼執行）。
- **claims 路徑約束**：guard-verify/guard-score 的 claims 檔必須在專案目錄或面板根內（防任意檔讀取）。
- **請求體限長**（1MB，`MaxBytesReader`）。
- 觀察端（`/api/state`、`/api/know/*` 等）唯讀、無需 token，可安全供 AI agent 拉取。

### 帳本 Head anchor（T-24）

`.ykc/ledger.jsonl` 的 hash chain 會在每次成功 append 後，把最新 `(seq, head)` 寫到
**專案外**的 `$YKC_HOME/anchors/<ledger-id>.json`；該 anchor 由 `$YKC_HOME/anchor.key`
（0600）作 HMAC 簽章。因此僅取得專案目錄權限的攻擊者不能靠「截斷到合法前綴」或
「重簽最後一行」躲過驗證。`ykc-judge -verify` 與 Trust Console 會顯示 anchor 狀態。

可選 remote witness（預設不需要網路）：

```bash
export YKC_ANCHOR_WITNESS_URL='https://witness.example/ykc/anchor'
export YKC_ANCHOR_WITNESS_VERIFY=true       # GET 對賬
# export YKC_ANCHOR_WITNESS_REQUIRED=true    # witness 不可用時讓 Append 回報失敗
```

## ykc serve — 常駐進程（YKC_14）

`make serve`、`make panel` 或 `./bin/ykc-serve -root . -port 8080` 啟動單一常駐進程，合併四個 CLI 的**運行時監督面**：

- **監看**（atom 之職）：inotify（Linux；他平台 stat 輪詢）遞迴監看專案 → 去抖（預設 300ms，最後操作勝）→ `file.change` 事件批次入事實帳本（hash 鏈）；`.ykc`/`target`/`.git` 必排（防自身寫入回環）。
- **主動預譯**（L4 之職）：啟動時對每個已發現 Rust 專案先跑一次 `cargo metadata → fetch → check --all-targets → test --no-run`；其後每個 `.rs`、`Cargo.toml`、`Cargo.lock` 變更批次自動重跑。單一專案單飛，編譯期間的新變更會合併成下一輪，不會把最新編輯吞掉。完整 report 寫入 `.ykc/precompile/report.json`，並以 `precompile.report` 事件入帳本。
- **沙盒安全**：主動預譯的 `auto` 順序是 gVisor/runsc → bubblewrap；compile stage 斷網且設 `CARGO_NET_OFFLINE=true`。找不到隔離能力時只記錄 `unsupported.sandbox_required`，不會偷偷 native 執行 `build.rs` 或 proc-macro。可信本地專案如需例外，必須明示 `-precompile-allow-native`。
- **聲明評估**（guard 之職）：`POST /api/claims {project, kind, text, run_smoke?}` → **宣告式 Datalog 護欄**（規則即數據，`/api/rules` 可審計全文）→ 裁決入帳本 + enforcement 落盤。
- **觸發**（judge 之職）：`POST /api/jobs`（既有面板任務）或 `-auto-judge`（.rs 變更批次後單飛觸發 ykc-judge）。
- **面板**（panel 之職）：全部既有端點（/api/state、/api/raw、/api/projects、/api/jobs、/healthz）+ 新增 `/api/watch`（監看與主動預譯狀態）。
- 四個 CLI 全部保留（git 閘門、MCP、一次性除錯）；ykc-guard 的 MCP 維持獨立 stdio 進程。
- 帳本單一寫者紀律：serve 與 judge 子行程共用 `.ykc/ledger.jsonl`，衝突時 serve 以指數退避重試。

主動預譯預設開啟；只有明示退出才關閉：

```bash
# 預設：啟動掃描 + 每次 Rust 工作區變更自動預譯
./bin/ykc-serve -root .

# 顯式關閉（例如只想使用觀察面）
./bin/ykc-serve -root . -no-auto-precompile

# 明示使用指定 sandbox；native 例外必須兩個旗標同時表達意圖
./bin/ykc-serve -root . -precompile-sandbox bwrap
./bin/ykc-serve -root . -precompile-sandbox native -precompile-allow-native
```

`GET /api/watch` 會回報每個專案的 `running`/`pending`、預譯次數、最後整體結果、sandbox/isolation、診斷數與 report 路徑；`GET /api/state` 則投影最後一份 report 的有限摘要。詳見 `YKC_24_主動rustc預編譯報告.md`。

## T-19 / T-20 Capability Packs（L1/L2 可選）

> ⚠️ **狀態注記（2026-08-25 審計）**：本節命令（`make deps-setup` / `make deps` /
> `make structure` / `make pack-deps` / `make pack-structure` / `make thin-core-test` /
> `make structure-size` 與 `cmd/ykc-cap`、`cmd/ykc-deps`、`cmd/ykc-structure`）對應的
> YKC_21 代碼**尚未落庫**（見 `YKC_23_T21執行報告.md` §風險）。落庫前執行會得到
> `No rule to make target`。設計與驗收紀律以下列兩份報告為準。

為保持 `ykc` T0 核心細小，cargo-audit/cargo-deny、RustSec DB 與 Tree-sitter grammar 不會連結進
預設核心。它們以獨立 worker + SHA-verified manifest 組合；**只有 `ykc-cap` / Core 可將 worker
輸出寫進 EventStore 與 anchored ledger**。

```bash
# （待 YKC_21 落庫後可用）
# L1：先安裝固定 cargo 工具，再建 worker；scan 預設 report-first + offline
make deps-setup
make deps
./bin/ykc-deps scan -dir /path/to/rust-project
./bin/ykc-deps init-policy -o /path/to/rust-project/.ykc/deps-policy.toml

# L2：只嵌 Rust + Go grammar subset（不是 206 grammar 全包）
make structure
./bin/ykc-structure scan -dir /path/to/project

# 建 development pack、驗證 SHA、由 Core 側運行 worker
make pack-deps pack-structure
./bin/ykc-cap verify -manifest packs/structure-rustgo/0.1.0/<os-arch>/manifest.json
```

`make thin-core-test` 會保證 T0 `ykc` 不連結 gotreesitter；`make structure-size` 鎖定 Rust+Go
worker 的 25 MiB 預算。詳見 `YKC_20_能力包解耦與組合架構.md` 與 `YKC_21_T19T20能力包MVP執行報告.md`。

## YKC 報告雙 CLI（健檢 + 重構）

根目錄的 28 份系列報告（`YKC_00…YKC_27`）+ 3 份奠基文檔由兩個零依賴 CLI 管理（共用 `internal/reports`）：

```bash
# 報告健檢（ykc-reports）：解析元數據 + 審計
./bin/ykc-reports list                    # 一覽：編號/標題/日期/基線/任務
./bin/ykc-reports check                   # 審計：編號斷層、斷鏈引用、日期倒掛、標題缺失（error 退出 1）
./bin/ykc-reports check -strict           # warn 也擋（合入前最嚴）
./bin/ykc-reports show 24                 # 單一報告詳情（大綱/引用/sha256）

# 報告重構（ykc-reportbook）：確定性生成三件產物到 docs/reports/
./bin/ykc-reportbook build                # INDEX.md（總索引）+ manifest.json（機器可讀）+ OUTLINE.md（大綱）
./bin/ykc-reportbook verify               # 漂移閘門：報告改了但產物沒重生成 → 退出 1（CI 用）
```

**決定論紀律**：產物僅為報告內容的函數（無時間戳、無隨機數）——`verify` 因此能把
「文檔漂移」變成機械可判的失敗，與帳本可重放同一哲學。`make reports-check` 與
`make reportbook-verify` 已入 CI core-lane。

## 文件索引

> 全量索引（含日期/基線/任務/摘要/指紋，機器可讀）：[`docs/reports/INDEX.md`](docs/reports/INDEX.md)（`ykc-reportbook` 生成）。

| 文件 | 內容 |
|---|---|
| **`YKC_14_常駐進程與宣告式護欄報告.md`** | **ykc serve 合併 + 護欄 datalog 化設計、驗收與紀律** |
| **`YKC_15_知識庫與代理上下文引擎方案.md`** | **嵌入式唯讀知識庫：518 錯誤碼（例子+正解）、54 規則抽象、官方教學文檔、精準檢索/依賴項圖/上下文緩存/原子化** |
| **`YKC_16_全代碼健檢修復與後續開發規劃.md`** | **全代碼健檢（staticcheck/vet/race 零告警）、錯漏債重死修復清單、後續 roadmap** |
| **`YKC_17_知識庫鎖版與代理可追溯開發執行報告.md`** | **KB v2 鎖版／import、judge 知識 provenance、MCP KB 工具、lint/CI 閘門與後續排程** |
| **`YKC_18_帳本錨定與知識面可重放擴展報告.md`** | **T-24 Head anchor／remote witness、KB manifest/replay/diff、tier-1 繁中卡、面板知識面與跨程序 cache** |
| **`YKC_19_T19_L1依賴對齊與T20_L2結構統計實作規劃.md`** | **Cargo audit/deny、pure-Go Tree-sitter、政策與 grammar admission 設計** |
| **`YKC_20_能力包解耦與組合架構.md`** | **Core + capability pack + verified JSONL composition、thin/secure/full profiles** |
| **`YKC_21_T19T20能力包MVP執行報告.md`** | **T-19/T-20 MVP、pack manifest、實際 cargo tool/grammar admission 與驗收** |
| **`YKC_24_主動rustc預編譯報告.md`** | **主動預譯預設、啟動全掃、檔案變更觸發、單飛合併、sandbox fail-closed、帳本與面板狀態** |
| **`YKC_25_全庫審計修復與報告雙CLI實作報告.md`** | **全庫審計（4 代碼錯/3 債/9 文檔錯漏全修）+ 報告雙 CLI（ykc-reports 健檢 / ykc-reportbook 確定性重構）** |
| **`YKC_26_語法幾何與重寫理論地基報告.md`** | **T-20 理論地基：表面語法樹/抽象代數九律/幾何拓撲（弦圖）/重寫系統（Newman）/自動機/形式語言；CL0+R₀ 雙載體** |
| **`YKC_27_專案現況審計與競品價值分析報告.md`** | **審計第二輪（judge 收據修復/latestEpoch 收斂/快照恢復 runbook）+ 底層邏輯引擎評分 7.6/10 + 競品矩陣（shipgate/Shipmoor）與可吸收項（in-toto/Kani/Proof-or-Stop）** |
| **`YKC_00_構圖與路線圖.md`** | **總體構圖 + 里程碑 + 進度追蹤表（進度參照物）** |
| **`YKC_01_容器化方案分析.md`** | Docker 類替代品深度分析（Podman/gVisor/Firecracker/Nix…）與建議 |
| `YKC_YieldKeyCode_深度分析報告.md` | 技術五層、依賴清單、整體評分（v1.0） |
| `YKC_反欺騙核心設計_v2.md` | 原子監控 × 動態護欄 × 信任棘輪（v2.0） |
| `YKC_煙測引擎設計與PoC.md` | 煙測引擎設計與實證 |
| `YKC_05_原子監控與動態護欄對齊報告.md` | GitHub 原檔與上一輪原子監控 / 動態護欄開發內容對齊結果 |
| `YKC_06_沙盒與rustc預編譯設計實作.md` | 沙盒選型、rustc/cargo 預編譯 pipeline、無法預編譯場景安排 |
| `YKC_07_新增功能技術債審計與優化報告.md` | 新增原子監控 / 動態護欄 / 沙盒預編譯後的技術債審計與修復記錄 |
| `YKC_08_eventstore_ledger橋接設計與實作.md` | EventStore 原子事件與 Ledger hash-chain 橋接設計、實作、應用 |
| `YKC_09_panel工作視覺與審計健康優化報告.md` | Trust Console 工作中動態視覺、precompile/sync 控制、event/ledger 健康顯示 |

## 目錄結構

```
.
├── README.md                      ← 本檔（入口）
├── go.mod                         ← 單一 module（ykc）
├── Makefile                       ← 一鍵命令（setup/build/smoke/judge/lsp/guard/health/…）
├── dev-setup.sh                   ← 環境一鍵還原（冪等）
├── YKC_00_構圖與路線圖.md          ← 進度參照物
├── YKC_01_容器化方案分析.md
├── YKC_02_架構整合與效能設計.md
├── YKC_05_原子監控與動態護欄對齊報告.md
├── YKC_06_沙盒與rustc預編譯設計實作.md
├── YKC_07_新增功能技術債審計與優化報告.md
├── YKC_08_eventstore_ledger橋接設計與實作.md
├── YKC_09_panel工作視覺與審計健康優化報告.md
├── YKC_24_主動rustc預編譯報告.md
├── cmd/                           ← 十三個命令（全部已落庫；共用 internal/）
│   ├── ykc-smoke/main.go          ← 煙測引擎 ✅
│   ├── ykc-atom/main.go           ← 原子監控 + 動態護欄 enforcement CLI ✅
│   ├── ykc-precompile/             ← 沙盒 rustc/cargo 預編譯 ✅
│   ├── ykc-judge/                 ← L4 除錯閉環 ✅
│   │   ├── main.go      (judge/gate/verify 三模式)
│   │   └── cargocheck.go (cargo check --json 解析 + 錯誤指紋)
│   ├── ykc-guard/                 ← 動態護欄（反欺騙裁判）✅
│   │   ├── main.go      (verify/score/console/reset/mcp)
│   │   ├── claims.go    (聲明模型 + 抽取)
│   │   ├── verify.go    (確定性比對器)
│   │   ├── ratchet.go   (信任棘輪 + 首擊棘輪)
│   │   ├── console.go   (用家控制台 + 證據報告)
│   │   ├── mcp.go       (最小 MCP server)
│   │   └── guardledger.go (帳本路徑包裝)
│   ├── ykc-lsp/main.go            ← LSP 客戶端 ✅
│   ├── ykc-panel/main.go          ← Trust Console 薄殼（實作在 internal/panel）✅
│   ├── ykc-serve/main.go          ← 常駐進程（監看+聲明評估+面板合一；internal/serve）✅
│   ├── ykc-know/main.go           ← 嵌入式唯讀知識庫 CLI（錯誤碼/規則/教學文檔檢索）✅
│   ├── ykc-doctor/main.go         ← 工具鏈顯性探測（ykc-doctor/v1 JSON；-strict fail-fast）✅
│   ├── ykc-rustd/main.go          ← rust-analyzer 常駐 daemon（前哨診斷；判定權仍在 cargo）✅
│   ├── ykc-reports/main.go        ← 報告健檢 CLI（list/check/show；internal/reports）✅
│   └── ykc-reportbook/main.go     ← 報告重構 CLI（build/verify 確定性產物 + 漂移閘門）✅
│   （ykc-cap / ykc-deps / ykc-structure 屬 YKC_21 能力包——代碼尚未落庫，落庫後補列）
├── internal/                      ← 共享包（去重後唯一實作）
│   ├── ledger/                    ← 事實帳本 + project 外 HMAC head anchor（judge/guard 共用）
│   ├── atomicfile/                ← 原子寫入 primitives
│   ├── eventstore/                ← immutable per-event JSON store
│   ├── eventledger/               ← eventstore → ledger hash-chain bridge
│   ├── monitor/                   ← workspace snapshot/diff
│   ├── guardrail/                 ← 行為驅動動態護欄 policy + datalog 規則
│   ├── enforcement/               ← block/smoke takeover 狀態落盤
│   ├── smoke/                     ← reusable smoke runner
│   ├── watch/                     ← 事件監看（inotify/poll + 去抖 + 過濾）
│   ├── datalog/                   ← 迷你 Datalog 引擎（分層否定；護欄規則用）
│   ├── domain/                    ← 事件域穩定 wire 型別（Envelope/Claim/CommandResult）
│   ├── claimview/                 ← 帳本事實投影（判決/信任視圖）
│   ├── toolchain/                 ← T-21 工具鏈 port（replay/unavailable adapter + 契約測試）
│   ├── lsp/                       ← LSP 客戶端 session 管理（ykc-lsp / ykc-rustd 共用）
│   ├── tail/                      ← 全量 hash + 尾部保留輸出緩衝（子行程輸出唯一實作）
│   ├── panel/                     ← Trust Console 唯一實作（ykc-panel 與 ykc-serve 共用）
│   ├── serve/                     ← 常駐進程核心（事件循環/claims/auto-judge/主動預譯/attest）
│   ├── sandbox/                   ← gVisor/bwrap/native execution abstraction
│   ├── precompile/                ← cargo check / rustc metadata pipeline
│   ├── borrow/                    ← L5 接線：文字拓撲 + 區間代數 + 衝突圖 + 幾何規則卡
│   ├── reports/                   ← YKC 報告解析/審計/確定性重構（雙 CLI 共用）
│   ├── kb/                        ← 嵌入式唯讀知識庫 + 代理上下文引擎（YKC_15）
│   │   ├── store.go / manifest.go  ← 內容定址 Store、blob v2、release manifest/replay
│   │   ├── diff.go / zh.go          ← 原子／Refs diff、tier-1 繁中摘要層
│   │   ├── index.go / token.go     ← 倒排索引 + BM25 + char-shingle 模糊檢索
│   │   ├── graph.go                ← 依賴項圖（展開/反向/SCC）
│   │   ├── cache.go / persistent.go ← 記憶體 LRU + 可選私有跨程序 context cache
│   │   ├── context.go              ← Retrieve 管線（檢索→展開→預算截斷→渲染）
│   │   └── data/*.json.gz          ← 518 錯誤碼 + 54 規則 + 官方教學文檔（go:embed）
│   └── rustutil/rustutil.go       ← 執行/解析/簽名/雜湊通用工具
│   （internal/capability、internal/deps、internal/structure 屬 YKC_21——尚未落庫）
├── core/interfaces.go             ← 五層窄介面 + Executor 骨架 ✅
├── l5/chordlaw/                   ← L5 引擎：vendored ChordLaw（Datalog 借用檢查器，26/26 rustc oracle）
├── demo-rust-cli/                 ← 健康示範專案（clap CLI，煙測用）
├── demo-broken-cli/               ← 有錯專案（除錯閉環用）
├── demo-semantic-cli/             ← 語意錯誤專案（E0425，剩餘錯誤路徑用）
├── docs/rust_terms_zh_hant.md     ← tier-1 繁中卡術語表與翻譯紀律
├── docs/reports/                  ← ykc-reportbook 確定性產物（INDEX/manifest/OUTLINE；勿手改）
├── claims.json                    ← 代理謊報聲明樣本（煙測反欺騙比對用）
├── demo-agent-honest.json         ← 誠實代理聲明（T-14/T-15 用）
├── demo-agent-lying.json          ← 撒謊代理聲明（T-14/T-15 用）
├── test-mcp-client.py             ← MCP server 測試客戶端（T-17 用）
└── deploy/
    ├── Dockerfile                 ← 多階段鎖版鏡像
    ├── rust-toolchain.toml        ← 鎖 rustc 1.98.0（位元組一致關鍵）
    ├── compose.yaml               ← 本機一鍵運行
    ├── .dockerignore
    ├── git/pre-commit             ← YKC 閘門 git 掛鉤（軌道 B）
    ├── k8s/
    │   ├── deployment.yaml        ← 裁判 K8s 部署
    │   └── sandbox-runtime.yaml   ← gVisor RuntimeClass + 沙盒 Pod
    └── sandbox/
        └── README.md              ← gVisor/Firecracker 安裝指引
```

## L5 借用幾何解釋（2026-08-22 新增，T-18a）

borrow 錯誤是 LLM 代理最難修的一類——因為代理「睇唔見生命週期圖」。L5 把借用規則變成**代理可讀的幾何**：

- **judge 自動掛鉤**：`ykc-judge` 遇到 borrow 類錯誤碼（E0499/E0502/E0503/E0505/E0506/E0382/E0597/E0106…）自動附「📐 L5 借用幾何解釋」：幾何規則卡（兩條法則＋封閉修法菜單）＋**真實歸約拓撲**——把錯誤現場的 fn 行級歸約為 `.cl` 並經引擎驗證（幾何族命中才用，附 `sN ← file:line` 對照錨回源碼），驗證不過回退 canonical 樣例；解釋 sha256 上帳本（`borrow.analysis` 事實）可審計。
- **Trust Console 展示**：專案卡顯示「L5 紅邊」——衝突圖中違法重疊的數目，**紅邊清零 = 幾何收斂**，是代理修復的機械可驗判據；`/api/state` 帶 `l5` 節點供機器讀取。
- **MCP 工具 ×2**：`ykc.borrow_rules`（規則卡，純靜態永遠可用）、`ykc.borrow_explain`（.cl 最小樣例 → 區間拓撲＋代數事實＋紅邊數＋修法）。
- **判定權不轉移**：一切 L5 輸出都是 explanation，判定以 rustc 為準（決策 D22）。
- **可選能力**：無 python3 時自動降級（規則卡仍可用），符合 T0 零依賴承諾。
- 引擎：vendored [ChordLaw](l5/chordlaw/VENDOR.md)（Datalog 借用檢查器；19 項回歸＋26/26 rustc 1.98.0 oracle 差異測試一致）。

```bash
make l5-test      # vendored ChordLaw 上游 19 項回歸
make borrow-test  # Go 接線層測試（拓撲/規則卡/17 範例 golden）
```

## 嵌入式唯讀知識庫（2026-08-22 新增，YKC_15）

把代理最需要的 Rust 知識做成**零依賴、內容定址、唯讀**的知識面（683 原子）：

- **資料**：rustc 官方**全部 518 條錯誤碼**（含錯誤範例 + 正解）、**54 條規則抽象**（17 領域）、**官方教學文檔**（19 部 / 91 章）——以 `go:embed` 編入二進制，離線可用；另有 **tier-1 60 張錯誤卡繁中摘要**，英文官方原文保留。
- **精準檢索**：精確碼短路 → BM25（程式碼也參與檢索）→ char-shingle 模糊（拼錯可召回）→ 領域加權；全決定論。
- **代理優化**：`Retrieve()` = 檢索 → 依賴項圖展開（答案+規則+出處閉包）→ 預算截斷 → 可貼入提示的 Markdown；LRU 上下文緩存以「查詢指紋+資料版本」為鍵，亦可選用私有跨程序 disk cache。
- **可重放 release**：`import` 同時產出 manifest（來源 URL/ETag/SHA、dataset/blob hash）；`replay` 逐項核對、`diff` 顯示原子與 Refs 漂移。
- **唯讀資料庫**：`ykc-know build` 產出單一 blob（sha256 防竄改），開檔即驗；HTTP 端點全 GET。
- 已掛入 `ykc-panel`／`ykc serve`（`/api/know/*`，唯讀無 token）與獨立 `ykc-know serve`（`/api/kb/*`）。

```bash
make know-test    # KB 鎖版／manifest／diff／翻譯／cache 驗收測試
make know         # 建 ykc-know + stats
./bin/ykc-know search "cannot borrow as mutable"   # 精準檢索
./bin/ykc-know graph E0382 -depth 2                # 依賴項圖展開
./bin/ykc-know build -o bin/kb.ykc                 # 由內嵌鎖版種子建唯讀 blob
./bin/ykc-know import "$(rustc --version)" -o bin/kb.ykc # 產生 blob + .manifest.json
./bin/ykc-know replay bin/kb.ykc.manifest.json -o bin/kb-replay.ykc # 可重放驗證
./bin/ykc-know diff bin/kb.ykc bin/kb-replay.ykc   # release／知識圖差異
./bin/ykc-know open bin/kb.ykc stats               # 顯示 rustc／翻譯／來源 metadata
```

`import` 是顯式的離線建庫動作：blob v2 會把 rustc 版本、翻譯層版本、官方 error-index URL 與
來源 SHA-256 寫進**校驗過的標頭**，並產出可重放 manifest；可用 `ykc-judge -kb bin/kb.ykc`
讓除錯閉環把實際命中的錯誤碼、原子 ID 與上下文 hash 追加為 `kb.analysis` 帳本事實。MCP 另提供
`ykc.kb_search`／`ykc.kb_explain`，讓外部代理取得同一份可追溯知識面。

若需跨程序快取（預設關閉，避免未經同意落盤代理查詢），顯式設定：

```bash
install -d -m 700 "$HOME/.ykc/kb-context-cache"
export YKC_KB_CACHE_DIR="$HOME/.ykc/kb-context-cache"
```

## 執行架構（「任何裝置可運行」四層）

| Tier | 載體 | 保證 |
|---|---|---|
| T0 | 3.3MB 靜態 Go 二進制（零依賴） | 無容器也能跑 |
| T1 | 鎖版 OCI 鏡像（rust-toolchain.toml） | 工具鏈位元組一致 |
| T2 | gVisor / Firecracker | 隔離不可信代碼（build script/proc-macro） |
| T3 | Kubernetes | 雲端多租戶 SaaS |

## 授權

Core 引擎 **MPL-2.0** + CLA；企業功能與數據城河閉源（見構圖文件 A.4）。
