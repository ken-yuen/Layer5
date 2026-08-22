# YKC 總體構圖與開發路線圖
### （進度參照物 ｜ 版次 v3.0 ｜ 2026-08-21 起持續更新）

> 本文件是 YKC 專案的**唯一進度參照物**：構圖（A/B）不常變，進度表（D）每週更新。任何階段性開發完成後，請把 D 表對應列的狀態改為 ✅ 並補上日期。
>
> 配套文件：
> - `YKC_YieldKeyCode_深度分析報告.md`（技術五層、依賴清單、評分）
> - `YKC_反欺騙核心設計_v2.md`（原子監控 × 動態護欄 × 信任棘輪）
> - `YKC_煙測引擎設計與PoC.md`（煙測引擎設計與實證）
> - `YKC_01_容器化方案分析.md`（本輪新增：Docker 類替代品深度分析）
> - `deploy/`（本輪新增：Dockerfile / compose / K8s / 沙盒配置）

---

## A. 遠見構圖

### A.0 開發環境備註（本平台，務必遵守）

| 項目 | 事實 | 對策 |
|---|---|---|
| `/tmp` | **tmpfs（RAM 磁碟）僅 ~1GB** | 工具鏈/大型暫存**絕不放 /tmp** |
| 工具鏈位置 | `/usr/local/ykc`（rustup+cargo，25G 磁碟） | 用 `make setup` 或 `source dev-setup.sh` 一鍵還原 |
| RAM | 2GB、無 swap | 大型編譯 minimal profile、避免多活並行 |
| CPU | 2 核 | 夠用 |
| 磁碟 `/` | 25G（餘 20G） | 充裕 |
| 已驗證 | rust-analyzer 在 RAM 釋放後**完整可用**（E0425 全診斷） | LSP 客戶端 T-09 全通 |

> 教訓（2026-08-21）：曾把 700MB 工具鏈放 /tmp，吃光 RAM 導致 rust-analyzer 被 SIGKILL。移回磁碟後 RAM 可用從 660MiB 升至 1.5GiB，LSP 全管線恢復。**開發不因平台限制暫緩。**

### A.1 使命與第一性原理

**使命**：讓使用者的 AI agent 在使用 YKC 後，能真實開發出「可運行、可驗證、可維護」的企業級 Rust 專案；同時讓非技術用家**無法再被代理欺騙**。

**第一性原理**（不可動搖的根基）：

> 真相必須由環境產生，不由代理敘述產生。代理可以誇大、偽造、隱瞞、欺騙，但 rustc、檔案 hash、測試執行器、二進制退出碼不會。YKC 把「驗證權」從代理手中拿走，做成環境屬性——因此抗欺騙能力**與代理是誰無關**。

**兩大目標合流**：反欺騙（可信裁判）與除錯（提昇成功率）是同一件事的兩面——當代理被迫交出「真能編譯、真能跑、真覆蓋需求」的成果時，「可運行大型專案」就是閘門的必然輸出。

### A.2 系統總覽（一圖構圖）

```
                        ┌──────────────────────────────────────────┐
                        │           用家（可非技術背景）              │
                        │     ┌──────────────┐  ┌───────────────┐   │
                        │     │  AI Agent     │  │ 用家控制台     │   │
                        │     │ (Claude/Cursor│  │ 紅黃綠+證據+   │   │
                        │     │ /自研框架…)   │  │ 接管按鈕      │   │
                        │     └──────┬───────┘  └───────┬───────┘   │
                        └────────────┼───────────────────┼──────────┘
                                     │ MCP (真實用家接口)  │
                        ┌────────────▼───────────────────▼──────────┐
                        │            YKC 介面層                      │
                        │   LSP(內測) → MCP → CLI → IDE插件          │
                        └────────────┬──────────────────────────────┘
                                     │
                        ┌────────────▼──────────────────────────────┐
                        │          YKC Core（Go，單一二進制）          │
                        │                                            │
                        │  L1 依賴對齊預警   L2 結構統計報告           │
                        │  L3 原子監聽+護欄  L4 沙盒預編譯除錯         │
                        │  L5 borrow代數仿構(datalog+節點圖)          │
                        │                                            │
                        │  ┌──────────────────────────────────────┐  │
                        │  │ 事實帳本 Fact Ledger                  │  │
                        │  │ (append-only + hash鏈 + HMAC簽名 +    │  │
                        │  │  唯讀 bbolt 快照 + 聲明-事實比對器)    │  │
                        │  └──────────────────────────────────────┘  │
                        └────────────┬──────────────────────────────┘
                                     │ 以「環境事實」為唯一真相
                        ┌────────────▼──────────────────────────────┐
                        │        沙盒執行層（隔離可信邊界）            │
                        │  gVisor/Firecracker 隔離 cargo build/build │
                        │  script/proc-macro（不可信代碼執行處）        │
                        └────────────┬──────────────────────────────┘
                                     │
                        ┌────────────▼──────────────────────────────┐
                        │         Rust 工具鏈（版本鎖定）             │
                        │  rustc/cargo/rust-analyzer/cargo-audit/    │
                        │  cargo-deny/polonius(nightly sidecar)      │
                        │  全部由 rust-toolchain.toml + 鎖版鏡像保證  │
                        └───────────────────────────────────────────┘
```

### A.3 五層職能定位（v2 後重定位）

| 層 | 職能 | 在「反欺騙」中的角色 |
|---|---|---|
| L1 依賴對齊 | 版本/漏洞/授權/重複依賴預警 | 證據富化：攔下「偷偷引入惡意/侵權依賴」 |
| L2 結構統計 | AST 遍歷、場景化報告 | 證據富化：真實現況 vs 聲稱現況 |
| L3 原子監聽+護欄 | 事實帳本、聲明-事實比對、信任棘輪、首擊棘輪 | **價值心臟**（見 v2 文件） |
| L4 沙盒預編譯除錯 | cargo fix→check→LLM 修復→test 閉環 | 除錯引擎 + 簽名 receipt 來源 |
| L5 borrow 仿構 | `-Znll-facts`→datalog→節點圖 | 「先解釋、後判定」的高階能力 |

> **實作狀態如實標記（2026-08-22 代碼審視後）**：`core/interfaces.go` 的五層窄介面 /
> `Core` / `Executor` 目前是**骨架（未被任何命令接線）**——實際實作為「六個 CLI 共享
> internal/ 包」的聯合體（ledger/eventstore/guardrail/sandbox/smoke/claimview…）。
> 這不影響功能與安全（CLI 聯合體更簡單可靠），但 P3 若要接線「模組化單體」（YKC_02），
> 需先決定：以現行 internal 包為實作、把 core 介面當測試替身/文件，或反向把 CLI 邏輯搬進
> 介面實作。**勿再在文檔中以「已接線」表述。**

### A.4 商業化構圖（open-core）

- **開源線**：Core 引擎 MPL-2.0 + CLA（Day 1 簽）。個人版用家控制台隨開源。
- **商業線**：企業版控制台（多代理監督/棘輪管理/審計匯出）、數據城河、SLA。分叉策略合法可行（版權擁有人可對新版本改授權）。
- **數據城河**：`(修前事實, 錯誤集, 修復 patch, 驗證結果)` 五元組 + 錯誤碼成功率統計 + borrow 事實帶標籤。**存事實不存代碼**、opt-in、去識別、合成數據優先。

---

## B. 「任何裝置都能運行」的執行架構（四層 Tier）

> 這是本輪新增的核心。目標：**同一份 YKC，在 macOS/Windows/Linux/CI/雲端都以位元組一致的環境運行。**

| Tier | 載體 | 保證什麼 | 用在哪 |
|---|---|---|---|
| **T0 原生** | 3.3MB 靜態 Go 二進制（CGO_ENABLED=0，零依賴） | 無任何容器也能跑 | 單機、CI、內測 |
| **T1 鎖版鏡像** | OCI 鏡像（Podman/Docker 通用）：rust-toolchain.toml 鎖 rustc 1.98.0 + rust-analyzer + cargo-audit + cargo-deny 鎖版 | 工具鏈位元組一致 | 日常開發、交付 |
| **T2 沙盒 runtime** | gVisor（runsc）/ Firecracker microVM | 隔離**不可信**的 cargo build script/proc-macro | 編譯代理產生的代碼（安全邊界） |
| **T3 編排** | Kubernetes（RuntimeClass: gvisor）+ Podman K8s 原生 pod | 多租戶、水平擴展 | 雲端 SaaS 商業版 |

**分層理由（關鍵）**：YKC 有**兩種截然不同的隔離需求**，不能混為一談——
1. **YKC 自己**（可信裁判）：要「確定可重現」，靠 T0/T1（靜態二進制 + 鎖版鏡像）。
2. **被編譯的代碼**（不可信：代理生成的、含 build script/proc-macro 的任意代碼）：要「安全隔離」，靠 T2（gVisor/Firecracker）。標準 Docker/runc 共享宿主核心，**不足以隔離不可信代碼**（2026 業界共識，見容器分析文件）。

---

## C. 里程碑（先實現部分 + 往後流程）

> 順序原則：**以「能運行的可信裁判」為軸心**，先煙測（已證低成本高質），再除錯閉環，再護欄，再擴張。

| 階段 | 名稱 | 核心交付 | 狀態 |
|---|---|---|---|
| **P0** | 命脈驗證 | Go 驅動 rust-analyzer（LSP）；解析 `cargo check --json`、`cargo metadata` | ⬜ |
| **P0.5** | 煙測引擎 | 分層煙測 T0–T3 + 簽名收據 + 反欺騙比對 | ✅ 2026-08-21 |
| **P1** | 裁判 MVP | L4 除錯閉環（cargo fix→check→LLM→test）+ 事實帳本 + 軌道 B git 閘門 | ✅ 核心已實裝（2026-08-21；LLM 接入留待 P2） |
| **P2** | 動態護欄 | 聲明抽取器+確定性比對器、行為分、信任棘輪、首擊棘輪、用家控制台 v1 | ✅ 核心已實裝（2026-08-21；MCP server 亦已實裝） |
| **P3** | 接管+擴展 | T0 全面接管、需求可追溯、L1/L2 併入、MCP/CLI/IDE 三介面；**常駐化已先行**（T-25/T-26：ykc serve + datalog 護欄，2026-08-22） | 🟨 常駐化完成 |
| **P4** | 城河+商業 | 欺騙行為語料、企業版控制台、混合上傳、K8s SaaS | ⬜ |

---

## D. 進度追蹤表（每週更新此表）

| ID | 任務 | 依賴 | 狀態 | 完成日 |
|---|---|---|---|---|
| T-01 | 裝 Go+rustup 工具鏈 | — | ✅ | 2026-08-21 |
| T-02 | 靜態編譯驗證（CGO_ENABLED=0） | T-01 | ✅ | 2026-08-21 |
| T-03 | 煙測引擎 PoC（main.go, 350 行, 零依賴） | T-01 | ✅ | 2026-08-21 |
| T-04 | 反欺騙聲明比對實證（2 謊報揪出） | T-03 | ✅ | 2026-08-21 |
| T-05 | panic 探測實證（exit=101） | T-03 | ✅ | 2026-08-21 |
| T-06 | 鎖版鏡像 Dockerfile（multi-stage） | T-02 | ✅ | 2026-08-21 |
| T-07 | rust-toolchain.toml 鎖版 + compose.yaml | — | ✅ | 2026-08-21 |
| T-08 | K8s manifest + gVisor RuntimeClass | — | ✅ | 2026-08-21 |
| T-09 | LSP 客戶端（Go↔rust-analyzer stdio） | — | ✅（Content-Length 框定 + initialize + publishDiagnostics 全通，E0425 完整診斷） | 2026-08-21 |
| T-10 | cargo check --json 解析器 + 錯誤指紋 | — | ✅ | 2026-08-21 |
| T-11 | 事實帳本（append-only + hash 鏈 + HMAC，bbolt 版後補） | — | ✅ | 2026-08-21 |
| T-12 | L4 除錯閉環 MVP | T-09,T-10,T-11 | ✅（機械修復 + 語意錯誤官方說明 + 簽名收據） | 2026-08-21 |
| T-13 | git 閘門（軌道 B） | T-12 | ✅（pre-commit 端到端攔截實證） | 2026-08-21 |
| T-14 | 聲明抽取器 + 確定性比對器 | T-11 | ✅（六類確定性比對 flag/subcommand/file/function/compiles/tests/receipt） | 2026-08-21 |
| T-15 | 信任棘輪 + 首擊棘輪 | T-14 | ✅（T3→T2→T0 只降不升；偽造首擊即接管） | 2026-08-21 |
| T-16 | 用家控制台 v1（個人版） | T-12,T-14 | ✅（紅黃綠摘要 + 證據報告 + 人類 reset 唯一回升入口） | 2026-08-21 |
| T-17 | MCP server（minimal，零依賴） | T-12 | ✅（initialize/tools/list/tools/call；ykc.check/verify_claims/trust_status） | 2026-08-21 |
| T-18a | L5 借用幾何解釋（vendored ChordLaw + internal/borrow: 文字拓撲/區間代數/衝突圖/幾何規則卡; judge 掛鉤 + MCP ×2 + 帳本 borrow.analysis） | — | ✅ | 2026-08-22 |
| T-18a2 | L5 P2 增量（真實 Rust→.cl 驗證式歸約 + sN←file:line 錨定; L5 報告落盤 persist.go; panel 紅邊視圖+快取鍵; MCP 紅邊; release 補 L5 引擎; 第二輪審計 7 項修復; staticcheck 0 告警） | T-18a | ✅ | 2026-08-22 |
| T-18a3 | L5 真實專案驗測（7 專案 ~20 萬行/10,255 fn 全掃描: 0 panic/0 不合法產物; 8 項邊角修復+5 條回歸; 真實形態注入 E2E; rustlings 語料; loop 複雜度防線 MaxLoopStmts。見 YKC_13） | T-18a2 | ✅ | 2026-08-22 |
| T-18b | L5 官方事實路線（-Znll-facts→Go datalog; 以 T-18a golden 做 differential testing） | T-18a | ⬜ | |
| T-19 | L1 依賴對齊（包 cargo-audit/deny） | — | ⬜ | |
| T-20 | L2 結構統計（tree-sitter） | — | ⬜ | |
| T-21 | 企業版控制台 + K8s SaaS | P4 | ⬜ | |
| T-22 | 修復與邊界加固（S1–S6、D1–D10、path traversal、MCP/claims 有界、CI/release 釘版） | — | ✅ | 2026-08-22 |
| T-23 | bootstrap-go.sh 受限環境安裝路徑 | — | ✅ | 2026-08-22 |
| T-24 | 帳本 head 錨定（截斷/末行重簽偵測） | T-22 | ⬜ | |
| T-25 | ykc serve 常駐進程（合併 atom+judge+guard+panel 運行時：inotify/fsnotify 事件流 + 去抖 + file.change 帳本序列化 + /api/claims + /api/watch + /api/rules；ErrLocked 退避重試；SIGTERM 優雅退出） | T-11,T-17 | ✅ | 2026-08-22 |
| T-26 | 護欄規則 datalog 化（internal/datalog 迷你引擎：分層否定+neq+安全檢查+確定性輸出；EvaluateClaim 遷移為規則即數據，policy_test 4 條原語義回歸全綠；-rules 附加集） | T-14 | ✅ | 2026-08-22 |

---

## E. 開發流程（往後每週節奏）

1. **單主幹（trunk-based）**：`main` 永遠可運行、可過閘門；功能用短分支，24–48h 內合回。
2. **Dogfooding**：用 YKC 開發 YKC；每週公佈「YKC 開發 YKC 的 compile-pass 率 / 煙測全綠率」——自證循環 = 活廣告。
3. **驗證門檻（合入標準）**：任何合入必須附 YKC 自簽 receipt（compile + test + smoke 全綠）。YKC 自己第一個遵守。
4. **不可信代碼永不分離例外**：凡執行「代理產生/第三方」代碼，一律走 Tier 2 沙盒（gVisor/Firecracker），禁止在裁判進程內裸跑 build script。
5. **每週一更**：更新本文件 D 表；每次里程碑完成寫 release note。
6. **版本策略**：語意化版本；rustc/rust-analyzer/polonius 升級走「鎖版→測試矩陣→再升」三步，永不「追最新」。

---

## F. 決策紀錄（累積，只增不改）

| # | 事項 | 決定 |
|---|---|---|
| D1 | 98% 除錯 | 分層 SLA（L0 捕獲 100%／L1 高頻錯誤 90%+／L2 深層不承諾） |
| D2 | 第 5 層 | 吃官方事實、先做解釋器（`-Znll-facts`→datalog→節點圖） |
| D3 | 執行位置 | 雙軌制（MCP + git 驗證閘門）+ 動態護欄決定主導多寡 |
| D4 | 驗收閘門 | compile+test+smoke+需求覆蓋+差量完整+無篡改，全由 YKC 簽發 receipt |
| D5 | 欺騙處置 | 行為分（嚴重度×意圖）、首擊棘輪、只降不升、人類才可恢復 |
| D6 | 部署 | 混合：本機 + 可選去識別匯總上傳 |
| D7 | 介面演進 | LSP(內測)→MCP(真實用家)→CLI→IDE |
| D8 | 授權 | MPL-2.0 + CLA + 商業分叉；數據層閉源 |
| D9 | 人類專屬功能 | 用家控制台；個人版隨開源、企業版商業 |
| D10 | CPI | = **CLI** |
| D11 | 煙測 | 早期重點開發；oracle 驅動、確定性判定（已 PoC） |
| D12 | 可運行保證 | 四層 Tier：靜態二進制→鎖版鏡像→gVisor/Firecracker→K8s |
| D13 | 架構整合 | 單一 module + cmd/ + internal/ 共享包（帳本/執行器/工具唯一實作，消除 drift）；見 YKC_02 |
| D14 | 代碼健檢 | 兩輪完成（2026-08-21）：修 2 bug、減 6 債、去 5 重、補 6 漏；見 YKC_03 |
| D15 | 發布可用性 | 「朋友下載」乾淨實測 9 項全通（2026-08-21）；修 3 處：① verify-all 帳本順序 ② rust-analyzer 不再隨 rustup（改 GitHub 下載） ③ setup 跨平台零 sudo |
| D16 | 觀察台 | YKC Trust Console（`cmd/ykc-panel`）：唯讀、零外部依賴、人類面板 + 機器可讀 API（/api/state、/api/raw）；帳本事實加 ts 時間戳（不進 hash，向後相容） |
| D17 | 控制台 | 面板升級「控制 + 觀察」：/api/projects（揀對象）+ /api/jobs（啟動/停止/實時日誌）+ 工具鏈環境自癒（PATH/RUSTUP_HOME/CARGO_HOME 自動補齊）；雙擊啟動器 launch.command/.sh/.bat |
| D18 | 五層落實藍圖 | L1 cargo-vet/audit/deny；L2 gotreesitter+SCIP+bbolt/DuckDB；L3 differential dataflow+datalog 護欄；L4 rustfix+nextest+miette+LLM 閉環；L5 -Znll-facts+eqlog+節點圖。見 YKC_04 |
| D19 | 修復與邊界加固 | 6 處 S 級 + 10 處 D 級全修（面板白名單/token/限長、煙測引擎合一 internal/smoke、DiagnosticSummary 統一 domain schema、帳本 OpenVerified+flock+行長有界、CI 釘 1.98.0/rust-analyzer 2026-08-17.4、release 補 7 二進制、guard 走 bridge、claimview 雙格式解碼、file: path traversal 攔截、claims 有界、MCP claims_path 約束、sandbox env key 過濾、tail buffer 合一 internal/tail）。見 YKC_11 |
| D20 | 受限環境安裝路徑 | go.dev 不可達環境（內網/受限 CI）的官方備援：`bootstrap-go.sh` 六級 bootstrap 鏈（gcc→1.4.3→…→1.27.0，實測 18 分鐘/2C3G），`make bootstrap-go` 一鍵。見 YKC_11 §1.2 |
| D21 | 帳本頭錨定（殘留風險） | hash 鏈無外錨時「截斷/末行重簽」不可偵測（OpenVerified 偵測的是中間行竄改/插入）。P3 前以「panel 定期外發 chain head + 人工核對」過渡；P3 做 head 錨定（獨立儲存/遠端存證） |
| D22 | L5 雙軌 | L5 採雙軌：**軌一（T-18a, 已落地）**= vendored ChordLaw 作「解釋層」——SVG 給人看、幾何給代理讀（文字拓撲+區間代數+衝突圖紅邊+幾何規則卡 E01–E10↔rustc）；判定權不轉移（一切輸出是 explanation, 判定以 rustc 為準）；解釋 sha256 上帳本（borrow.analysis）可審計；L5 是可選能力（無 python3 降級為純規則卡, 守 T0 承諾）。**軌二（T-18b）**= D2 原路線不變（-Znll-facts→Go datalog），以軌一 17 範例 golden + 26 oracle 作 differential testing 基準收斂。vendor 記錄見 l5/chordlaw/VENDOR.md |

---

*版次：v10.2（2026-08-22 L5 真實專案驗測：T-18a3 ✅ 7 專案 20 萬行掃描全綠；見 YKC_13）。*
