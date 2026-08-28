# YKC_25 — 全庫審計修復與報告雙 CLI 實作報告

> 日期：2026-08-25（審計執行日；依 git 提交時刻換算 HKT）
> 基線：`ykc-serve-datalog`（承接 YKC_24）
> 任務：找錯／找債／找重／找缺／找漏——全庫修復 ＋ YKC 報告重構雙 CLI

## 1. 行為結論

本輪對全庫（22,146 行 Go ＋ 28 份報告 ＋ Makefile/CI/文檔）做了一輪完整審計，
修復 **4 個代碼錯、1 個格式債、2 個測試衛生債、9 處文檔錯漏**，並以雙 CLI
（`ykc-reports` 健檢 ＋ `ykc-reportbook` 重構）把 28 份 YKC 報告變成
**結構化、可審計、機械可驗**的資產。修復後：

- `go test ./...`（含 `-race`）全綠；`gofmt`／`go vet`／`staticcheck -checks=all`／`toolchain-guard` 零告警；
- `ykc-reports check -strict`：**28 份報告 0 error 0 warn**；
- `ykc-reportbook verify`：產物與報告現狀一致（漂移閘門已入 CI core-lane）；
- serve 端到端實測：healthz、claims 駁回、`/api/toolchain`、優雅退出全過。

## 2. 代碼修復清單（錯）

| # | 位置 | 問題 | 後果 | 修復 |
|---|---|---|---|---|
| B1 | `internal/eventstore` | `Append` 對空 ID envelope 每次生成**隨機** ID | `ErrLocked` 退避重試時 outbox 寫入重複事件；首條成孤兒，`SyncMissing` 其後**雙重投影**同一內容——直接違反 YKC_08 的冪等承諾（`TestAppendIsIdempotentForSameEnvelope` 失敗） | ID 改為**內容確定性派生**（timestamp + sha256(kind,at,epoch,root,payload) 前 16 位）：重試同 envelope → 同 ID → 走既有冪等分支 |
| B2 | `internal/serve` | `/api/claims` 回應的 `claim_id` **恆為空** | `evaluateClaim` 回傳傳入 envelope 的空 ID（ID 只在 store 提交時派生）；調用方無法回鏈帳本 | `appendWithRetry` 改回傳已提交 envelope；`claim_id` 用真實 ID；新增回歸測試 `TestClaimsResponseCarriesClaimID`（斷言 ID 可在事件庫重放中找到） |
| B3 | `internal/serve` | `Handler()`（測試面）與 `Start()`（生產面）各自手寫路由表，`Handler()` **漏掛 `/api/toolchain`** | 測試永遠測不到該端點；路由漂移無人察覺 | 抽 `routes()` 唯一路由表，兩面共用 |
| B4 | `internal/panel` | `EnsureToolchainPath` 在 `ykcHome` 為空時 `filepath.Join("", "bin")` → 相對路徑 `"bin"`／`"rustup"` | CWD 恰好有同名目錄時會被塞進 PATH/RUSTUP_HOME——環境污染 | 空 base 一律跳過 |

## 3. 債／重／衛生

| # | 位置 | 問題 | 修復 |
|---|---|---|---|
| D1 | `internal/watch/watcher.go` | 未 gofmt（欄位對齊）——`make fmt-check` 本應擋住，說明上次合入未跑閘門 | gofmt；本輪起 `reports-check`/`reportbook-verify` 也入 CI，閘門面擴大 |
| D2 | `internal/serve`、`internal/eventledger` 測試 | `go test` 把數十個測試 anchor 寫進操作者真實 `~/.ykc/anchors`（ledger 包早已自我隔離，此二包漏了） | 補 `TestMain` 以 `YKC_ANCHOR_DIR` 隔離到臨時目錄；實測全跑後 anchors=0 |

## 4. 文檔修復清單（錯／漏）

1. **README/QUICKSTART 教用家跑不存在的 make 目標**（`deps`/`deps-setup`/`structure`/`pack-*`/`thin-core-test`/`structure-size`——YKC_21 未落庫）：全部加「待落庫」狀態注記，命令保留但明示現況。
2. **README 目錄樹三處失實**：`cmd/ykc-cap ✅`（不存在）、列了 `internal/capability|deps|structure`（不存在）、漏列 `ykc-doctor`/`ykc-rustd`/`internal/toolchain|lsp|tail|claimview|domain|borrow`——重寫為與實際 13 個 cmd、22 個 internal 包一致。
3. **QUICKSTART「verify-all 預期 13 項」**：實際 12 項（⑬ 不存在）——更正。
4. **CI 注記過期**（稱 verify-all 依賴 capability-test）：更新為真實原因（需完整 Rust + curl + python3 + 常駐進程）。
5. **11 份報告缺日期元數據**：依 git 首次提交時刻（**UTC→HKT 換算**，與報告自述日期同一時區慣例）如實回填，逐條註明依據；YKC_04 等自帶副標題日期者以**文檔自述優先**，YKC_10 副標題（受命日 08-21）與提交日（HKT 08-22）並存時兩者皆保留並註明語義。

## 5. 報告雙 CLI（本輪新增）

```
internal/reports/     唯一實作：解析(Scan/Parse) + 審計(Audit) + 確定性生成(Artifacts/Build/Verify)
cmd/ykc-reports/      健檢 CLI：list / check / show
cmd/ykc-reportbook/   重構 CLI：build / verify
```

### 5.1 解析模型（ykc-reports/v1）

每份 `YKC_*.md` 解析為：編號（series/foundation 二類）、標題、日期（元數據行
與副標題行兩種慣例）、基線、任務 ID（T-nn）、sha256、大綱（fence-aware 標題樹）、
對其他報告的引用、相對鏈接、首段摘要。全部輸出僅為內容的函數——**無時間戳、無隨機數**。

### 5.2 審計規則（check）

`series-gap`（編號斷層）、`dup-number`、`missing-title`、`missing-date`、
`date-order`（日期倒掛）、`broken-ref`（引用不存在的報告）、`broken-link`、
`dup-title`。error 退出 1；`-strict` 連 warn 也擋。

### 5.3 重構產物（build/verify）

`docs/reports/` 三件產物：`INDEX.md`（統計＋系列表＋奠基表＋完整性指紋）、
`manifest.json`（機器可讀全量索引）、`OUTLINE.md`（每份報告標題大綱）。
`verify` 重生成並逐字節比對——「報告改了但產物沒重生成」是機械可判的失敗，
與帳本可重放（YKC_18）同一哲學。

### 5.4 接線

- Makefile：`reports-test` / `reports-check` / `reportbook` / `reportbook-verify`；`binaries` 含雙 CLI。
- CI core-lane：`make reports-check` + `make reportbook-verify`（漂移閘門）。
- release assets：雙 CLI 二進制入包。

## 6. 驗收紀錄

```text
go build ./... && go vet ./...            通過
gofmt -l cmd internal core                （空）
staticcheck -checks=all ./...             零告警
make toolchain-guard                      ✅ cargo/rustc 全部經 port 出入
go test -count=1 ./...                    全部 ok（21 個測試包）
go test -race（eventstore/eventledger/serve/panel/reports）全部 ok
ykc-reports check -strict                 28 份 0 error 0 warn
ykc-reportbook build + verify             三件產物一致
serve 端到端（-port 18096）               healthz ok；tests_passed 聲明被
                                          fake_test_claim 駁回（claim_id 非空）；
                                          /api/toolchain 200；SIGTERM 優雅退出
GOOS=windows/darwin 交叉編譯（雙 CLI）    通過
```

## 7. 邊界與未完成

- 本環境無 Rust 工具鏈：`make verify-all` 的 ①–⑩（需 cargo/rustc）未跑；
  核心路徑以 core-lane 等價閘門（零 Rust 全綠）覆蓋，與 YKC_22 的解耦不變式一致。
- `date-order` 規則對「同批提交、自述日期與提交鐘不一致」的報告會如實示警——
  本輪以 HKT 換算回填後全部對齊；未來新增報告請遵循頭部「日期：YYYY-MM-DD」慣例。
- YKC_21 能力包落庫後：README/QUICKSTART 的狀態注記可移除，`ykc-reports` 的
  `broken-ref` 規則會自動開始守衛新報告的互引。
