# YKC 11 — 修復與邊界加固報告

### （2026-08-22 ｜ 針對 YKC_10 所發現之 S1–S6 / D1–D10 / 風險登記，全部修復 + 邊界加固 + 回歸實證）

> 本輪職責：**「請修復找到的問題、債務、風險。之後進行邊界加固。」**
> 結果：**6 處 S 級、10 處 D 級、9 項風險（RK1–RK9）全部處理完畢**；另完成 8 組邊界加固，
> 新增 **7 個測試包 / 30+ 斷言**（測試包從 6 增至 12），全數通過。

---

## 0. 執行摘要

| 類別 | 項目 | 狀態 |
|---|---|---|
| S1 | 面板暴露＋任意路徑 | ✅ 白名單＋預設 127.0.0.1＋Bearer token（timing-safe）＋請求體限長；實測 `/etc`→400、無 token→401 |
| S2 | 雙套煙測引擎 drift | ✅ `cmd/ykc-smoke` 全部命令改經 `internal/smoke.Runner`（全專案唯一執行核心）；tail buffer 合一 `internal/tail` |
| S3 | DiagnosticSummary 同名字段斷層 | ✅ `precompile` 改輸出 `domain.DiagnosticSummary`（正規化）+ 明細獨立欄位；**集成測試鎖死**「precompile 診斷→事件→護欄判定」全鏈 |
| S4 | 帳本開鏈不校驗 | ✅ `ledger.OpenVerified`（開帳本即重放全鏈）；judge 寫入路徑改走之；實測竄改帳本→拒絕寫入 |
| S5 | CI 工具鏈浮動 | ✅（見下註）rust 釘 `1.98.0`（與 rust-toolchain.toml 一致）、rust-analyzer 釘 `2026-08-17.4` |
| S6 | release 漏 2 二進制 | ✅（見下註）release job 改 `make binaries` 單一來源（7 二進制全含） |

> **S5/S6 推送狀態註記**：S5/S6 的變更全在 `.github/workflows/ci.yml`（8 行 diff），已提交於本地分支
> **`ci-fix-local`（commit `52ba0a3`）**；因本環境 GitHub App 未獲 `workflows` 權限，含 workflow
> 變更的推送被 GitHub 拒絕（diff 級檢查）。**解法二擇一**：① 於 Arena GitHub 整合設定為 App 開啟
> `workflows` 權限後 `git push origin ci-fix-local:arena/01a02698-kyc --force-with-lease` 前先合入主分支；
> ② 倉庫擁有者直接 cherry-pick `52ba0a3`（或依下文 diff 手動套用）。diff 全文見 `git show 52ba0a3`。
| D1 | 帳本無檔案鎖 | ✅ flock 排他鎖（單寫者強制，第二寫者 `ErrLocked`）；測試鎖定 |
| D2 | Scanner 1MB 靜默截斷 | ✅ 行長有界讀取器（16MB 上限，超限顯式報錯）；測試鎖定 |
| D3 | eventledger O(n²) 補償 | ✅ 單次讀表建索引＋單次開帳本批量 append（O(n)）；去重檢查移入鎖內（消 TOCTOU） |
| D4 | guard 直寫舊 ledger | ✅ guard score/reset 全改經 `eventledger` bridge；讀端以 `internal/claimview` 同時解碼舊（扁平）/新（信封）格式 |
| D5 | tailBuffer 重複兩份 | ✅ 收斂 `internal/tail`（smoke/sandbox 共用，含「全量 hash 與尾部長度無關」測試） |
| D6 | panel `itoa()` 繞路 | ✅ 已移除（json.Marshal 繞路改 `strconv` 直用） |
| D7 | /api/state 每 poll 全量重讀 | ✅ 伺服器端快取：鍵 =（帳本 mtime, size, 事件檔數）——append-only 下三者不變 ⇒ 狀態不變 |
| D8 | 發現只掃一層 | ✅ `-depth` 參數（預設 1，0=僅根） |
| D9 | LSP pipe 錯誤未檢查 | ✅ 已補 err 檢查 |
| D10 | （YKC_10 已更正：Cargo.lock 在位） | 隨 S5 處理，無獨動作業 |
| RK1–RK9 | 風險登記 | 對應 S/D 項全數落地；RK8 以 YKC_00 A.3 如實標記（core 骨架未接線）＋ D19–D21 決策紀錄 |
| 新 | bootstrap-go.sh 受限環境安裝路徑 | ✅ 收進 repo（D20/T-23），`make bootstrap-go`；實測 18 分鐘全鏈 |

**邊界加固（新，YKC_10 未列）**：

| # | 邊界 | 加固 | 實證 |
|---|---|---|---|
| B1 | guard `file:` 聲明 path traversal | `file:../../etc/passwd`、絕對路徑、NULL 字元 → 一律 contradicted（不 verified、不讀檔）；`insideProject` Clean 後前綴比對（含 `/a/bc` 非 `/a/b` 內之陷阱測試） | 測試 5 組路徑全拒 |
| B2 | guard claims 輸入 | 檔案 ≤1MB、messages ≤100 則×64KB、聲明 ≤1000 條、**agent_id 必填**（匿名代理不入信任體系） | 測試鎖定 |
| B3 | MCP `ykc.verify_claims` 任意檔讀取 | claims_path 必須在專案目錄內 | 代碼＋insideProject 測試 |
| B4 | MCP `ykc.trust_status` | agent_id 必填 | 401 級錯誤回傳 |
| B5 | sandbox 環境變數注入 | `cfg.Env` 的 key 必須是合法 POSIX 變數名（字母/數字/底線、非數字開頭），否則丟棄——防容器 `-e` 參數注入 | 測試 5 種非法 key 全攔 |
| B6 | 全部 CLI 目錄參數 | judge/guard/atom/precompile 的 dir/root/project 必須存在且為目錄 | 各 CLI 實測 |
| B7 | 面板請求體 | `MaxBytesReader` 1MB（實測 2MB body→400） | 實測 |
| B8 | 面板暴露警告 | 非本機位址＋無 token 啟動時打印醒目安全警告 | 啟動日誌 |

---

## 1. 關鍵修復細節

### 1.1 S4 + D1 + D2：`internal/ledger` 三合一升級

- **OpenVerified**：`Open` 後 `VerifyChain` 全鏈重放；judge 寫入路徑一律經之。偵測範圍：任何**中間行**竄改（含攻擊者重簽該行——後續 PrevHash 斷裂）、插入、序號跳號。**殘留**（見 §3 與 YKC_00 D21）：截斷／**末行**重簽需 head 錨定方能偵測，P3（T-24）處理。
- **flock**：`Open` 即取 `LOCK_EX|LOCK_NB`，第二程序開啟同帳本 → `ErrLocked`（unix build tag；非 unix 降級不鎖並註明）。
- **行長有界**：手動逐行讀取（`bufio.Reader.ReadBytes`），`MaxLineBytes=16MB`，超限顯式錯誤；同時修正 `Append` 對過大 payload 的上限檢查。
- 新測試 7 項：roundtrip、中間行竄改、重簽式竄改、超限行、無結尾換行、flock 第二寫者、OpenVerified 接受淨帳本。

### 1.2 S3：診斷 schema 統一

`precompile.ParseDiagnostics` 回傳 `ParseResult{Summary domain.DiagnosticSummary, Messages []Diagnostic, ...}`：
- `Summary` 即 `domain` 的 wire 型別（ErrorCount/WarningCount/BuildBlockingCount/Digest）——接事件流零轉換；
- 明細（行/欄位/rendered）留 `Stage.Messages`；
- **鎖死測試** `TestDiagnosticsFeedGuardrailNoErrorsClaim`：E0425 JSON → ParseDiagnostics → `diagnostic.summary` 事件 → `guardrail.EvaluateClaim(ClaimNoErrors)` → `fake_no_errors_claim` + block_writes。兩套 schema 若再走樣，此測試必紅。

### 1.3 S2 + D5：煙測執行核心合一

- `internal/smoke`：新增 `CommandSpec.ID`（收據稳定 id）、`Runner.FailFast`（atom takeover 保持 fail-fast；ykc-smoke 用 run-all 列齊分層結果）、`Report.ResultOf(id)`；
- `cmd/ykc-smoke`：不再自行 exec/處理輸出——T0/T1/T2 全部命令經 runner（超時、全量 SHA、1MB 尾部保留）；本檔只留「檢查語意＋收據」；
- `internal/tail`：兩份 tailBuffer 收斂為一份，並補「全量 hash 與保留長度無關」測試（收據可比性根基）。

### 1.4 S1：面板安全邊界

- `validateProject`：任務 project 必須解析到白名單**同一目錄**（相對路徑以面板 root 為基準；不做 base 名模糊比對）；
- `validateClaims`：guard-verify/guard-score 的 claims 必須在專案目錄或面板根內；
- token：`-token`/`YKC_PANEL_TOKEN`，`subtle.ConstantTimeCompare`；唯讀端點不需 token（AI agent 可安全拉取）；前端 401 提示＋記憶；
- 預設綁 127.0.0.1；非本機＋無 token 打印警告；`MaxBytesReader` 1MB。

### 1.5 D4：guard 走 bridge ＋ claimview 雙格式解碼

- `domain` 新增 `EventClaimVerdict/EventTrustEvent/EventTrustReset`；
- guard score/reset 經 `eventledger` 寫入（事件庫原子 commit → hash 鏈投影）；
- `internal/claimview.Parse/TrustLevel`：舊（扁平）/新（信封）格式統一解碼，guard console 與 panel 共用（讀端 drift 消除）；
- 實證：同一帳本混存舊新事實，panel `agents={'agent-honest-01': 0}`、trust_events=4、verdicts=4 全正確。

---

## 2. 驗證結果（本沙盒實跑）

```
gofmt -l cmd internal core        → 空（0 違規）
go vet ./...                      → 通過
go test -count=1 ./...            → 12/12 測試包全過
make binaries                     → 7/7 二進制
make health                       → vet/build 過；smoke/judge/ledger 三項因本沙盒無 cargo 而 FAIL（環境限制，非回歸；
                                    有網機器 make verify-all 一鍵全測）
```

**邊界實證（全部通過）**：

| 測試 | 結果 |
|---|---|
| 面板 `POST /api/jobs {project:"/etc"}` | 400「project 不在已發現專案清單內」 |
| 面板 `POST /api/jobs {project:"/tmp"}` | 400 同上 |
| 面板合法專案（絕對路徑） | 200，任務完成 exit=0 |
| 無 token / 錯 token / 對 token（控制端點） | 401 / 401 / 200 |
| 唯讀 /api/state（無 token） | 200 |
| 2MB 請求體 | 400（MaxBytesReader） |
| 竄改帳本 → `ykc-judge`（寫入路徑） | `ledger chain verification failed: tamper or corruption detected`，拒寫 |
| 竄改帳本 → `ykc-judge -verify` | `🛑 帳本遭竄改！hash 鏈斷裂。` |
| 撒謊代理（零證據）`ykc-atom claim` | `evidence_only_smoke_takeover`＋block_writes（行為未變，寫入改經 bridge） |
| 帳本 hash 鏈（Python 獨立重算 vs Go 簽發） | 4/4 一致（跨實作驗證，維持） |

**新增測試清單**（7 個測試包）：

```
internal/ledger/ledger_test.go      7 項（含 flock、重簽竄改、超限行）
internal/tail/tail_test.go          3 項（含全量 hash 不變性）
internal/rustutil/rustutil_test.go  2 項（clap help 格式快照——RK4 鎖定）
cmd/ykc-guard/verify_test.go        5 項（path traversal 5 路徑、棘輪矩陣）
cmd/ykc-guard/claims_test.go        4 項（抽取快照、agent_id 必填、超限、嚴重度矩陣）
internal/claimview/claimview_test.go 4 項（雙格式、混合格式順序、空 agent 拒收）
cmd/ykc-panel/jobs_test.go          3 項（白名單、claims 約束、inside 前綴陷阱）
internal/precompile/precompile_test.go +1 項（S3 集成鎖死）
internal/sandbox/sandbox_test.go    +1 項（env key 注入攔截）
```

---

## 3. 殘留風險（誠實記錄，附處置）

| # | 風險 | 處置 |
|---|---|---|
| R1 | hash 鏈無外錨：截斷／末行重簽不可偵測（OpenVerified 偵測中間行） | YKC_00 D21/T-24：P3 做 head 錨定（獨立儲存/遠端存證）；過渡期 panel 可觀測 chain head，人工/外部系統定期核對 |
| R2 | 本沙盒無 cargo：verify-all ①③④⑨⑩ 未實跑 | 命令與預期全備妥（YKC_10 §3.3）；CI（已釘版）在有網環境跑全量 |
| R3 | `core/interfaces.go` 仍為骨架（未接線） | 已如實標記（YKC_00 A.3 + D19）；P3 接線前先定方向（介面當測試替身 vs 反向搬移） |
| R4 | 面板快取以（mtime,size,事件數）為鍵：極端併發下同 ns 雙 append 理論上可漏更 | 實務不可達（size 必變）；P3 若上 ETag/seq 增量一併取代 |
| R5 | bwrap 僅 moderate trust（設計如此，非本輪變更） | 維持現狀；SaaS 走 gVisor/Firecracker |

---

## 4. 對 YKC_10 路線圖的落點

- §7.1 加固週（H1–H6）：**本輪全部完成**（H1=S1、H2=S4+D1+D2、H3=S3、H4=RK4 快照測試+CI 釘版、H5=bootstrap-go.sh 收進 repo、H6=CI/release 修補）。
- 下一步依 YKC_10 §7.2：M1 已隨 S2 完成；**M2（atom 常駐化 daemon）與 M4（L1 依賴對齊）為下輪主軸**。

---

## 附錄：變更檔案清單

```
新增  internal/tail/{tail.go,tail_test.go}
新增  internal/claimview/{claimview.go,claimview_test.go}
新增  internal/ledger/flock_unix.go, flock_other.go, ledger_test.go
新增  internal/rustutil/rustutil_test.go
新增  cmd/ykc-guard/{verify_test.go,claims_test.go}
新增  cmd/ykc-panel/jobs_test.go
新增  bootstrap-go.sh（D20：受限環境 Go 安裝備援）
修改  internal/ledger/ledger.go（OpenVerified/flock/行長有界/Path()）
修改  internal/eventledger/bridge.go（O(n) sync、鎖內去重、Close）
修改  internal/domain/events.go（+3 事件型別）
修改  internal/precompile/precompile.go（S3 schema 統一）+ 測試
修改  internal/smoke/smoke.go（ID/FailFast/ResultOf）
修改  internal/sandbox/sandbox.go（tail 收斂、env key 過濾）+ 測試
修改  cmd/ykc-smoke/main.go（S2 執行核心合一）
修改  cmd/ykc-guard/{main,guardledger,claims,verify,ratchet,console,mcp}.go
修改  cmd/ykc-judge/main.go（OpenVerified、目錄驗證）
修改  cmd/ykc-atom/main.go（目錄驗證、FailFast 顯式化）
修改  cmd/ykc-lsp/main.go（pipe err 檢查）
修改  cmd/ykc-panel/{main,state,jobs}.go + dashboard.html（S1/D6/D7/D8/token UI）
修改  .github/workflows/ci.yml（S5 釘版、S6 release 7 二進制）
修改  Makefile（bootstrap-go target）、README.md（面板安全邊界）、QUICKSTART.md（疑難排解）
修改  YKC_00_構圖與路線圖.md（A.3 如實標記、D19–D21、T-22/23/24）
```

*本輪全部變更以 gofmt/vet/test 全綠＋上表邊界實證收口；未污染 demo 工作樹（運行產物均在 .gitignore 覆蓋範圍）。*
