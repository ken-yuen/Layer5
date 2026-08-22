# YKC_16 全代碼健檢、修復與後續開發規劃

> 日期：2026-08-22
> 範圍：全專案（`core/`、`cmd/`、`internal/` 全部 Go 源碼 + 腳本/掛鉤檔案模式）
> 方法：`staticcheck -checks=all`、`go vet`、`go test -race`、人工逐檔審查 + 可執行位元審計
> 結果：**零告警**（staticcheck / vet 全綠），全測試通過（含 race）。

---

## 1. 健檢結論

YKC 的代碼基底品質高：帳本（hash 鏈 + flock 單寫者）、Datalog 引擎（分層否定、
安全檢查、決定論排序）、eventstore→ledger 橋接（TOCTOU 消除）、面板邊界（白名單、
path traversal 防護、timing-safe token）均屬企業級。本輪健檢**沒有發現功能性錯誤**，
問題集中在四類「重死 / 錯漏 / 債」：

| 類別 | 數量 | 說明 |
|---|---|---|
| 重死（dead code / 死參數 / 重複實作） | 3 | 未用欄位、未用參數、BFS 雙實作 |
| 錯（真會咬人的） | 3 | git 掛鉤不可執行、啟動腳本不可執行、vet 誤報 build tag |
| 漏（缺失的文件級註釋） | 6 套件 | package comment 缺失 |
| 債（註釋形式 / 命名） | ~15 | ST1020/1021/1022/1003 形式債 |

---

## 2. 修復清單（錯 / 漏 / 債 / 重死）

### 2.1 錯（bug）— 已修

1. **`deploy/git/pre-commit` 不可執行（真 bug）**
   git 掛鉤必須有可執行位元才會被 git 呼叫；此檔在 HEAD 是 `100644`，等於「閘門
   靜默失效」（README 宣稱「YKC 閘門 git 掛鉤（軌道 B）」但實際不會跑）。
   → `chmod +x`，模式改為 `100755`。

2. **啟動腳本不可執行**
   `launch.sh`、`launch.command` 在 HEAD 是 `100644`：macOS Finder 雙擊
   `launch.command` 與 Linux 檔案管理器雙擊 `launch.sh`（README/QUICKSTART 承諾的
   用法）會失敗。→ 兩者改 `100755`；`dev-setup.sh` 一併改 `100755`（冪等還原腳本
   應可直接執行）。

3. **`bootstrap-go.sh` 工作樹遺失可執行位元** → 還原為 `100755`（與 HEAD 一致）。

4. **`go vet` 誤報「possible malformed +build comment」**
   `internal/guardrail/rules.go` 文件註釋中的 `"build"=check+build+smoke` 含
   `+build` 子串，命中 vet 的 build-tag 啟發式。→ 改寫為 `check/build/smoke`
   （語意不變，消除子串）。

### 2.2 重死（dead weight）— 已清

5. **`internal/kb/store.go` 未用欄位 `mu sync.RWMutex`**（含 `sync` import）→ 刪除。

6. **`panel.JobManager.Start` 未用參數 `root`** → 移除，並更新兩處呼叫端
   （`panel/server.go`、`serve/serve.go`）；root 白名單本就由 `JobManager.root`
   提供，參數是歷史殘留。

7. **依賴項圖 BFS 雙實作**：`kb/graph.go` 與 `kb/context.go` 各有一份幾乎相同的
   展開邏輯。→ 統一為 `graph.ExpandContext`（保留根的相關性順序 + 決定論），
   刪除 `context.go` 的 `expandOrdered`；新增 `TestGraphExpandPreservesRootOrder` 鎖定順序語意。

### 2.3 漏（omission）— 已補

8. **缺 package comment 的套件（6）**：`atomicfile`、`domain`、`enforcement`、
   `eventstore`、`monitor`、`guardrail` → 各補一條 `// Package X — …` 文件註釋。

9. **`internal/panel/state.go` 的 `DiscoverCargoProjects` 上方註釋重複且名稱不符**
   （寫成小寫 `discoverCargoProjects` ×2）→ 修正為單一條正確形式。

### 2.4 債（debt）— 已清

10. **註釋形式（ST1020/1021/1022）**：`ApplyAll`、`ValidateProject`、
    `ValidateClaims`、`Ledger`、`DefaultBounds`、`core` 五層窄介面
    （`DependencyChecker`…`BorrowAnalyzer`）的註釋開頭改為「符號名 …」。

11. **欄位命名（ST1003）**：`ledger.Fact.Ts` → `TS`、`panel.FactView.Ts` → `TS`
    （JSON tag `ts` 不變，外部序列化輸出零變化）。

12. **文件級註釋被誤判為 package comment（7 檔）**：`guardrail/dl.go`、
    `guardrail/rules.go`、`panel/jobs.go`、`panel/state.go`、`watch/inotify_linux.go`、
    `watch/poll.go`、`watch/watcher.go` → 在首行註釋與 `package` 之間補空行，
    使其回歸「檔案級註釋」而非 package doc（各包已另有正規 package comment）。

---

## 3. 驗證結果

```
go build ./...                       ✅ 0 錯誤
go vet ./...                         ✅ 0 告警
staticcheck ./...                    ✅ 0 告警
staticcheck -checks=all ./...        ✅ 0 告警（原 24 條 → 0）
go test ./...                        ✅ 全綠（17 套件）
go test -race ./internal/kb/panel/ledger/serve  ✅ 無資料競爭
```

新增測試：`internal/kb` 的 `TestGraphExpandPreservesRootOrder`（根順序保留 + 決定論）。

---

## 4. 後續開發規劃（roadmap）

### P0 — 本週內（正確性 / 安全面）

- **R1 錯誤碼索引鎖版**：`ykc-know import <rustc版本>` 依 `rustc --version` 抽取對應
  版本的錯誤索引，並把版本寫入 blob 頭；與 `deploy/rust-toolchain.toml`（rustc 1.98.0）
  對齊，做到「解釋與工具鏈同位元組一致」。
- **R2 知識面可追溯**：judge 修復閉環把「命中錯誤碼 → 使用到的 KB 版本 + 原子 ID」
  寫入事實帳本（同 `borrow.analysis` 款式），讓「代理靠哪份知識修好」可審計、可重放。
- **R3 KB MCP 工具**：`ykc.kb_search` / `ykc.kb_explain` 兩支 stdio MCP 工具
  （仿既有 `ykc.borrow_*`），把 KB 直接暴露給外部代理；serve 側 `/api/know/*` 已就緒。
- **R4 供應鏈鎖定**：`Makefile` 加入 `verify-all` 前的 `staticcheck`/`vet` 前置目標，
  並把本輪的 lint 門檻固化進 CI（`.github/workflows/ci.yml` 補 staticcheck 步驟）。

> **執行更新（2026-08-23）**：R1–R4 已完成，包含 KB blob v2 metadata/checksum、`ykc-know import`、judge `kb.analysis` provenance、KB MCP 工具與固定 `staticcheck v0.8.1` 的 Make/CI 閘門；驗證與接續排程見 `YKC_17_知識庫鎖版與代理可追溯開發執行報告.md`。

### P1 — 下個里程碑（代理體驗）

- **R5 錯誤碼中文陳述**：518 條錯誤碼卡 + 官方教學文檔的中文回填（規則層已中文），
  供中文代理與非技術用家；英文原文保留為 `en` 欄位。
- **R6 向量檢索（可插拔、預設關閉）**：需要語意級召回時以介面接入本地 embedding，
  維持「零依賴預設」不變；BM25 + shingle 維持為決定論回退。
- **R7 上下文緩存持久化**：跨程序共享 KB 緩存（現為進程內 LRU），
  鍵仍綁定資料版本指紋，跨版本自動失效。
- **R8 `ykc-know diff`**：比較兩個 blob 版本的原子增刪，輸出「知識面變更清單」，
  供裁判/審計對賬。

> **執行更新（2026-08-23）**：R5 已完成 tier-1 60 張錯誤卡繁中摘要 MVP（英文原文保留）；R7 完成 explicit opt-in 的跨程序 cache；R8 完成 manifest/replay/diff；R9 完成 Trust Console 唯讀知識頁。完整邊界與驗證見 `YKC_18_帳本錨定與知識面可重放擴展報告.md`。

### P2 — 架構級

- **R9 面板整合**：Trust Console 加「知識庫」頁籤（讀 `/api/know/*`），
  讓人在面板內直接查錯誤碼與規則，與 L5 紅邊視圖並列。
- **R10 可選性收斂**：評估 `core/interfaces.go` 五層骨架介面（L1/L2 現為 stub）
  是否落地或標註為 roadmap 佔位，消除「骨架與實作」認知落差。
- **R11 打包瘦身**：KB 資料改為「內嵌預設 + 可選外掛 blob」，讓 T0 二進制可選
  不攜帶知識面（零依賴承諾與功能完備的取捨開關）。

### 常規化（持續）

- **R12** 每輪開發後跑 `make health`（vet + build + 回歸）+ `staticcheck -checks=all`；
  新增 lint 告警視同回歸。
- **R13** 新套件一律帶 package comment 與決定論測試（排序/順序語意用 golden 鎖定）。

---

## 5. 本輪變更檔案

```
core/interfaces.go            （註釋形式）
internal/ledger/ledger.go     （TS 命名 + Ledger 註釋）
internal/datalog/datalog.go   （DefaultBounds 註釋）
internal/panel/jobs.go        （移除 root 死參 + 註釋）
internal/panel/server.go      （Start 呼叫端）
internal/panel/state.go       （DiscoverCargoProjects 重複註釋 + TS 命名）
internal/serve/serve.go       （Start 呼叫端）
internal/kb/{store,graph,context,kb_test}.go（清死碼、統一 BFS、新增測試）
internal/{atomicfile,domain,enforcement,eventstore,monitor,guardrail,watch,panel}/…（package comment）
cmd/ykc-guard/ratchet.go      （ApplyAll 註釋）
bootstrap-go.sh / launch.sh / launch.command / dev-setup.sh / deploy/git/pre-commit（可執行位元）
本檔 YKC_16_全代碼健檢修復與後續開發規劃.md
```
