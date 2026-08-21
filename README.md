# YieldKeyCode (YKC) — Rust 編程輔助器

> **使命**：讓 AI agent 在使用 YKC 後，能真實開發出「可運行、可驗證、可維護」的企業級 Rust 專案；同時讓非技術用家**無法再被代理欺騙**。
>
> **第一性原理**：真相由環境產生，不由代理敘述產生。YKC 把「驗證權」從代理手中拿走，做成環境屬性——抗欺騙能力與代理是誰無關。

## 快速開始（任何裝置）

```bash
# 方式 A：像普通程式——雙擊啟動器（macOS 雙擊 launch.command；Linux launch.sh；Windows launch.bat→WSL）
#   自動：裝環境 → 建二進制 → 起控制面板 → 開瀏覽器

# 方式 B：命令行（原生，無需容器）
make verify-all     # 一鍵跑全部功能實測
make panel          # 啟動 YKC Trust Console（控制 + 觀察台）

# 方式 C：容器（Podman 或 Docker 通用）
make image && make up
```

## YKC Trust Console（控制 + 觀察台）

`make panel` 或雙擊啟動器後，瀏覽器開 <http://localhost:8080>（本機啟動器綁 127.0.0.1）：

- **控制（人類觸發）**：選擇運行對象（專案）→ 煙測/除錯/閘門/護欄動作 → 開始/停止 → 實時日誌與狀態。
- **觀察（唯讀）**：專案完整性、信任等級（T0–T3）、判決與證據、即時事實流（hash 串鏈）——全部直接讀自 `.ykc/ledger.jsonl`，不改寫。
- **AI 看（機器可讀）**：`GET /api/state`、`GET /api/raw?project=<dir>`、`GET /api/projects`、`GET/POST /api/jobs`、`GET /healthz`。

## 文件索引

| 文件 | 內容 |
|---|---|
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
├── cmd/                           ← 四個命令（單一 module，共用 internal/）
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
│   └── ykc-panel/                 ← Trust Console 唯讀觀察台 ✅
│       ├── main.go      (HTTP 伺服器 + JSON API)
│       ├── state.go     (狀態聚合：專案/事實/信任/收據)
│       └── dashboard.html (內嵌面板，零外部依賴)
├── internal/                      ← 共享包（去重後唯一實作）
│   ├── ledger/ledger.go           ← 事實帳本（judge/guard 共用，消除 drift）
│   ├── atomicfile/                 ← 原子寫入 primitives
│   ├── eventstore/                 ← immutable per-event JSON store
│   ├── eventledger/                ← eventstore → ledger hash-chain bridge
│   ├── monitor/                    ← workspace snapshot/diff
│   ├── guardrail/                  ← 行為驅動動態護欄 policy
│   ├── enforcement/                ← block/smoke takeover 狀態落盤
│   ├── smoke/                      ← reusable smoke runner
│   ├── sandbox/                    ← gVisor/bwrap/native execution abstraction
│   ├── precompile/                 ← cargo check / rustc metadata pipeline
│   └── rustutil/rustutil.go       ← 執行/解析/簽名/雜湊通用工具
├── core/interfaces.go             ← 五層窄介面 + Executor 骨架 ✅
├── demo-rust-cli/                 ← 健康示範專案（clap CLI，煙測用）
├── demo-broken-cli/               ← 有錯專案（除錯閉環用）
├── demo-semantic-cli/             ← 語意錯誤專案（E0425，剩餘錯誤路徑用）
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

## 執行架構（「任何裝置可運行」四層）

| Tier | 載體 | 保證 |
|---|---|---|
| T0 | 3.3MB 靜態 Go 二進制（零依賴） | 無容器也能跑 |
| T1 | 鎖版 OCI 鏡像（rust-toolchain.toml） | 工具鏈位元組一致 |
| T2 | gVisor / Firecracker | 隔離不可信代碼（build script/proc-macro） |
| T3 | Kubernetes | 雲端多租戶 SaaS |

## 授權

Core 引擎 **MPL-2.0** + CLA；企業功能與數據城河閉源（見構圖文件 A.4）。
