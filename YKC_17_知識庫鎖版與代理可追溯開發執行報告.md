# YKC_17 知識庫鎖版、代理可追溯與品質閘門

> 日期：2026-08-23  
> 分支：`ykc-serve-datalog`  
> 狀態：**P0 R1–R4 已實作並驗證**

---

## 1. 本輪目標與結論

承接 `YKC_16_全代碼健檢修復與後續開發規劃.md` 的最高優先級 R1–R4，本輪先把「代理使用的知識」變成可鎖版、可驗證、可重放的事實，再把品質檢查變成不可跳過的合入閘門。

| 項目 | 交付 | 結果 |
|---|---|---|
| **R1** 錯誤碼索引鎖版 | `ykc-know import <rustc版本>`、v2 KB blob metadata 標頭與校驗 | ✅ |
| **R2** 知識面可追溯 | judge 將錯誤碼、KB 版本、原子 ID、上下文 hash 寫進帳本 | ✅ |
| **R3** KB MCP 工具 | `ykc.kb_search`、`ykc.kb_explain` | ✅ |
| **R4** 供應鏈／品質鎖定 | `make lint`、`verify-all` 前置、CI staticcheck、setup 安裝 | ✅ |

**核心結果**：同一輪 judge 現在可以回答「E0425 當時用了哪個 rustc 版本的 KB、哪幾個內容定址原子、實際提供的上下文是甚麼 hash」，而不是只留下不可追溯的人類敘述。

---

## 2. 先做的基線驗證

乾淨拉取後，先以專案腳本安裝並固定以下工具鏈：

```text
Go              1.27.0
rustc / cargo   1.98.0
rust-analyzer   0.3.3016-standalone（release 2026-08-17.4，SHA-256 校驗）
staticcheck     2026.2.1 (0.8.1)
```

改動前與改動後均通過：

```text
gofmt -l cmd internal core              ✅ 無輸出
go vet ./...                            ✅
staticcheck -checks=all ./...           ✅
go test ./...                           ✅
go build ./...                          ✅
make lint                               ✅
python3 l5/chordlaw/test_chordlaw.py    ✅ 19/19
```

---

## 3. 已落地的設計

### 3.1 R1 — 可追溯的版本鎖定 KB

#### 新指令

```bash
# 從正在使用的 rustc 取版本；下載對應的官方 print.html，產出可驗證 blob
make know-import

# 或明確指定版本／完整 rustc --version 輸出
./bin/ykc-know import "$(rustc --version)" -o bin/kb.ykc
./bin/ykc-know open bin/kb.ykc stats
```

`import` 會：

1. 正規化及驗證版本（接受 `1.98.0`、`v1.98.0` 或 `rustc --version` 輸出）；
2. 抓取 `https://doc.rust-lang.org/<version>/error_codes/print.html`；
3. 從官方 mdBook 結構抽取所有 `error-code-E####` 卡與錯誤／正解範例；
4. 保留既有 54 條規則與 91 章官方教學文檔，組合成完整原子圖；
5. 把下列資料寫入 blob v2 **已校驗標頭**：
   - `rustc_version`
   - `error_index_url`
   - `error_index_sha256`

v2 的 SHA-256 覆蓋 metadata、record count 與 payload，故不能只篡改標頭把一個舊資料集偽稱為新版。舊 v1 blob 仍可讀取，但其 `rustc_version` 會明確顯示為未知，提示重新 import。

本輪實測：

```text
rustc 1.98.0
518 條錯誤碼 + 54 規則 + 91 章 + 19 部 + 1 TOC = 683 原子
官方 print.html SHA-256:
7d7def835759efa57023e6742e2e8bb581879f1cf4826f8208260ac2d6af1954
```

內嵌資料的 metadata 亦由測試對齊 `deploy/rust-toolchain.toml` 的 `channel = "1.98.0"`；未來更新工具鏈時，測試會強迫同步更新 KB 鎖定資訊。

### 3.2 R2 — judge 的 KB provenance 事實

`ykc-judge` 新增：

```bash
./bin/ykc-judge -dir /path/to/project -kb bin/kb.ykc
```

- 沒有 `-kb` 時，使用內嵌、鎖版資料集；
- 有剩餘 rustc 診斷時，對每個唯一錯誤碼做精確 KB `Retrieve()`；
- 每輪最多記錄 16 個不同錯誤碼，避免壞掉的大型專案無界膨脹帳本；
- 結果以 `type="kb.analysis"`、actor=`ykc-judge` 寫進 `.ykc/ledger.jsonl`。

Payload 的關鍵欄位：

```json
{
  "kb_version": "92b37f3c4bf9f858",
  "rustc_version": "1.98.0",
  "error_index_sha256": "7d7d…1954",
  "usages": [{
    "code": "E0425",
    "root_atom_id": "kb-20cdc50bd6750f86",
    "atom_ids": ["kb-20cdc50bd6750f86"],
    "context_sha256": "f761…73309"
  }]
}
```

這是**解釋／修復上下文的審計證據**；它不改變 D22：唯一判定權仍在 rustc。

### 3.3 R3 — 外部代理可用的 KB MCP

`ykc-guard mcp` 的工具清單新增：

| 工具 | 作用 | 安全／資源邊界 |
|---|---|---|
| `ykc.kb_search` | 以錯誤、訊息或程式片段查詢最小知識閉包 | query ≤ 8 KiB；`k` 1–20；圖深度 0–4；預算 512–32768 bytes |
| `ykc.kb_explain` | 精確取得 `E0382`、`BRW-01` 或章節 ID 的可追溯解釋 | code ≤ 256 bytes；同樣的展開／預算上限 |

兩者回傳 Markdown，上方附 `dataset`、`rustc` 與來源；MCP 長駐進程以 `sync.Once` 共用唯讀 Store，不會為每個呼叫重建索引。

### 3.4 R4 — 把 lint 變成真正的門檻

新增指令與流程：

```bash
make lint        # gofmt + go vet + staticcheck -checks=all
make verify-all  # 現在先執行 lint，失敗即不跑功能驗證
make health      # 同樣以 lint 作前置
```

- `dev-setup.sh` 安裝固定的 `staticcheck v0.8.1` 至 `$YKC_HOME/bin`；
- Rust 不再跟隨浮動 `stable`：setup 與 CI 固定 `rustc 1.98.0`；rust-analyzer 固定 release `2026-08-17.4`，下載時校驗 SHA-256；
- Makefile 對自行管理 Go 的用家也會尋找 `$(go env GOPATH)/bin/staticcheck`；
- GitHub Actions 在 build 前安裝相同 staticcheck 並執行 `make lint`；
- release asset 補上先前遺漏的 `ykc-know` 二進制。

---

## 4. 後續開發排序（執行準則）

### Sprint A — 先處理仍未封閉的安全不變式

| 優先 | 任務 | 為何先做 | 驗收 |
|---|---|---|---|
| **A1** | **T-24 帳本 head 錨定** | 現有 hash 鏈仍不能獨力偵測「截斷後重新簽最後一行」；這比新 UI 更接近信任根 | 本機獨立 anchor + 可選遠端 witness；截斷、回滾、重簽三種攻擊測試必攔截 |
| **A2** | KB import manifest / 可重放更新流程 | 將官方下載的 URL、ETag、SHA、原子數和產物 SHA 寫為 release manifest | 乾淨環境依 manifest 可重建同一 blob；差異有明確報告 |
| **A3** | `ykc-know diff`（R8） | 有兩個 KB 版本後必須能知道原子增加、刪除、內容改變 | 兩 blob 產出決定論 diff；輸出可直接掛到 release／審計 |

### Sprint B — 讓人與代理真正用得到知識面

| 優先 | 任務 | 設計邊界 | 驗收 |
|---|---|---|---|
| **B1** | R5 錯誤碼／文檔中文回填 | 英文官方原文保留；翻譯以獨立欄位與來源版本綁定；先覆蓋 top 60 tier-1 | 人工抽樣、術語表、繁中 golden；無原文覆蓋或不可追溯改寫 |
| **B2** | R9 Trust Console「知識庫」頁 | 只讀、無 token，沿用 `/api/know/*`；顯示資料集／rustc／原子 ID | 搜尋、卡片、出處、版本與 blob metadata 可見；無新增控制面權限 |
| **B3** | R7 跨程序 context cache | key 必須含 KB version；快取可刪、可失效、沒有權威性 | 多進程命中、版本切換失效、損毀快取安全降級 |

### Sprint C — 只在基線量測後才引入複雜度

| 任務 | 進場條件 | 不變式 |
|---|---|---|
| R6 可插拔向量檢索 | BM25/shingle 失敗案例與離線評測集已量化 | 預設關閉；BM25 為決定論回退；模型與索引版本入審計 |
| R10 core 介面收斂 | 先決定 L1/L2 是否落地，避免把骨架誤稱已接線 | 文件、程式與執行路徑一致 |
| T-18b 官方 NLL facts | 有可重放的 rustc nightly 矩陣與差異測試預算 | L5 仍只作解釋，rustc 保有判定權 |

### 每個後續 PR 的 Definition of Done

1. `make lint && go test ./... && make l5-test`；
2. 有新資料格式時：round-trip、tamper、legacy compatibility 與 deterministic tests；
3. 有新帳本 fact 時：payload schema、最大尺寸、重放／對賬測試；
4. 有新 HTTP/MCP 輸入時：白名單、長度、型別、路徑與資源上限測試；
5. 更新本文件與 `YKC_00_構圖與路線圖.md` 的任務狀態。

---

## 5. 本輪改動檔案

```text
internal/kb/{metadata,import}.go        版本 metadata、官方 error-index import
internal/kb/{store,seed,http}.go        blob v2、來源 metadata、HTTP state
internal/kb/*_test.go                   import、標頭防竄改、v1 相容、工具鏈對齊
cmd/ykc-know/main.go                    import / metadata stats / 可選 blob serve
cmd/ykc-judge/{main,kb}.go              kb.analysis provenance 與 -kb 選項
cmd/ykc-guard/mcp.go                    ykc.kb_search / ykc.kb_explain
cmd/ykc-guard/mcp_kb_test.go            MCP 工具與界限測試
Makefile / dev-setup.sh                 lint 前置、know-import、固定 staticcheck
.github/workflows/ci.yml                CI lint、release 納入 ykc-know
README.md / QUICKSTART.md                使用入口同步（本輪）
YKC_00_構圖與路線圖.md                  T-27 / T-28 進度同步（本輪）
```

---

## 6. 決策補充

- **D28 — KB 來源版本是資料的一部分**：Rust error code 的內容會隨工具鏈演進，故版本、來源 URL 與 source hash 必須隨 blob 被校驗，不可只寫在 release note。
- **D29 — 知識上下文也必須有 provenance**：代理的「我根據文件修好了」不是證據；`kb.analysis` 的 dataset version、atom IDs、context hash 才是可重放證據。
- **D30 — lint 不可只是文件建議**：`staticcheck -checks=all` 與 vet 進入 Make/CI 前置，缺工具即失敗，不允許靜默跳過。

---

*完成條件：R1–R4 的程式、測試、實際 rustc 1.98.0 import、judge `kb.analysis` ledger payload，以及 MCP KB 工具皆已於本分支驗證。後續已執行的 T-24/T-29/T-30 見 `YKC_18_帳本錨定與知識面可重放擴展報告.md`。*
