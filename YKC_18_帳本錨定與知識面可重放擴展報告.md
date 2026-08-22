# YKC_18 帳本 Head Anchor、KB 可重放 Release 與繁中知識面擴展

> 日期：2026-08-23  
> 承接：`YKC_17_知識庫鎖版與代理可追溯開發執行報告.md`  
> 狀態：**Sprint A 完成；Sprint B 的繁中卡片、面板與跨程序快取已完成第一個可用閉環**

---

## 1. 本輪執行範圍

使用者已接納 YKC_17 的後續建議。本輪按「先封閉信任根、再擴充可重放知識面、最後改善人與代理的使用面」執行：

| 任務 | 交付 | 狀態 |
|---|---|---|
| **T-24 / A1** | 專案外、HMAC 簽章的 ledger head anchor；可選 HTTP witness | ✅ |
| **A2** | import manifest：URL、ETag、Last-Modified、來源 SHA、原子／blob 指紋 | ✅ |
| **R8 / A3** | `ykc-know diff`：metadata、內容原子、knowledge graph refs 差異 | ✅ |
| **R5 / B1 MVP** | 60 張 tier-1 rustc error card 的繁中摘要；英文原文保留 | ✅ |
| **R9 / B2** | Trust Console 內嵌唯讀「知識庫」搜尋頁 | ✅ |
| **R7 / B3** | 可選、私有、跨程序的 KB context disk cache | ✅ |

R6 向量檢索、R10 core 介面收斂、T-18b NLL facts 仍遵守 YKC_17 的**進場條件**：先建立可量測的失敗案例／矩陣，不以「接受建議」取代驗證依據。

---

## 2. T-24：把帳本鏈頭移出被監督專案

### 2.1 問題與不變式

原本的 JSONL hash chain 能偵測中間行竄改，卻不能單獨偵測：

1. 攻擊者把帳本截斷到一段舊但合法的前綴；
2. 攻擊者改寫最後一行並重新計算 hash；
3. 攻擊者以另一條內部自洽的鏈取代原鏈。

現已新增一份**不放在 `<project>/.ykc/` 下**的獨立 head anchor：

```text
ledger:       <project>/.ykc/ledger.jsonl
anchor:       $YKC_HOME/anchors/<sha256(canonical-ledger-path)>.json
HMAC key:     $YKC_HOME/anchor.key          (0600)
```

每次成功 `Ledger.Append` 的順序是：

```text
write ledger fact → fsync ledger → 原子寫 HMAC-signed head anchor →（可選）POST remote witness
```

寫入前的 `ledger.Open`／`OpenVerified` 則完整重放鏈並比對 anchor：

| 狀態 | 行為 |
|---|---|
| `anchored` | 鏈在 anchor 的 seq 仍含同一 hash；允許寫入 |
| `unanchored` | 舊帳本第一次升級；可讀／可寫，下一次 Append 自動 bootstrap anchor |
| `rollback` | 現在 seq 小於 anchor seq；**fail closed** |
| `mismatch` | anchor seq 的 hash 改變；**fail closed** |
| `invalid` | anchor metadata/HMAC/key 不合法；**fail closed** |

`ykc-judge -verify`、event bridge、Trust Console 的 `/api/state` 都改走 anchor-aware verification。面板卡片現會顯示 **Head anchor**；anchor 不一致時即使裸 hash chain 看似完整，也會標為 tampered。

### 2.2 可選遠端 witness

本機 anchor 已能防止「只取得專案工作目錄權限」的攻擊。若操作者要防護本機 anchor 被刪除／回滾，可設定：

```bash
export YKC_ANCHOR_WITNESS_URL='https://witness.example/ykc/anchor'
export YKC_ANCHOR_WITNESS_TOKEN='…'              # 可選 Bearer token
export YKC_ANCHOR_WITNESS_VERIFY=true             # 可選 GET 對賬
export YKC_ANCHOR_WITNESS_REQUIRED=true           # 可選：witness 無法記錄即令 Append 回報失敗
```

協定刻意小而可替換：

```text
POST <URL>                         ← HeadAnchor JSON，端點按 ledger_id 單調保存
GET  <URL>?ledger_id=<sha256-path> ← HeadAnchor JSON 或 {"anchor": HeadAnchor}
```

遠端回傳的 anchor 仍以本機 HMAC key 驗證；遠端較新的 seq 或同 seq 不同 head 會被判為 mismatch。未設定或暫時不可用的遠端 witness **不會降低**已存在的本機獨立 anchor；只有設定 `REQUIRED` 才會把遠端可用性納入寫入門檻。

### 2.3 驗收測試

`internal/ledger/anchor_test.go` 覆蓋：

- 完整鏈＋anchor round-trip；
- 合法前綴截斷 → `ErrAnchorRollback`；
- 最後一行重簽、裸鏈仍合法 → `ErrAnchorMismatch`；
- anchor JSON/HMAC 竄改；
- 舊帳本 bootstrap；
- seq gap；
- HTTP remote witness 記錄／核對，以及 required witness 故障時回報已提交 seq。

```bash
make anchor-test
```

---

## 3. KB import manifest 與可重放 release

### 3.1 新工作流

```bash
# 下載官方對應版本，寫 blob + manifest
./bin/ykc-know import "$(rustc --version)" -o bin/kb.ykc
# 預設同時產生 bin/kb.ykc.manifest.json

# 在乾淨環境重抓相同來源並逐項比對
./bin/ykc-know replay bin/kb.ykc.manifest.json -o bin/kb-replay.ykc
cmp bin/kb.ykc bin/kb-replay.ykc

# release / 審計差異
./bin/ykc-know diff old.ykc new.ykc -limit 200
./bin/ykc-know diff old.ykc new.ykc -json
```

manifest 不含建置時間，內容包括：

```text
manifest/blob 格式版本
rustc 版本
error index URL、內容 SHA-256、ETag、Last-Modified
繁中翻譯層版本
錯誤碼數、原子數、dataset version
blob SHA-256、blob bytes
```

`replay` 會重新下載，核對來源 hash、可選 ETag／Last-Modified、錯誤碼數、原子數、dataset version 與 blob SHA；任一漂移即失敗，避免「成功下載了不同內容」被誤報為可重放。

### 3.2 差異語意

`diff` 的 logical key 是 `(kind, code)`，分為：

- `added`／`removed`：原子存在性改變；
- `content_changed`：原子內容／內容定址 ID 改變；
- `references_changed`：knowledge graph `Refs` 改變。

最後一項不可省略：`Atom.contentHash()` 有意不把 `Refs` 算入內容 ID，故只看 ID 會漏掉「答案卡不變、前置規則／出處閉包變了」的知識面漂移。

---

## 4. R5 MVP：繁中錯誤卡，英文來源不被覆寫

新增 `zh-Hant-tier1-60@2026-08-23` 翻譯層，覆蓋 boost tier-1 的 60 張高頻 rustc error card（如 E0308、E0277、E0382、E0499、E0597、E0728）。

- `Atom.ZH` 是獨立欄位，內容定址會納入此欄位；英文 `Title`／`Body`／官方 source URL 完整保留；
- `docs/rust_terms_zh_hant.md` 固定 ownership、borrow、lifetime、trait、orphan rule 等譯法，作為下一批翻譯的人工審核基線；
- CLI、MCP、HTTP、Trust Console 會顯示「繁中摘要」並保留英文官方正文；
- translation version 寫入 blob metadata／manifest；
- 測試強制 tier-1 覆蓋率為 `60/60`，避免後續資料更新靜默漏譯。

這是可審核的第一個語言層，不宣稱已翻完 518 卡或 91 章文檔；下一批翻譯仍應先走術語表、golden 與人工抽樣。

---

## 5. R9/R7：面板知識頁與可選跨程序快取

### Trust Console

`ykc-panel` 和 `ykc-serve` 現共用 `/api/know/*`：

```text
GET /api/know/state
GET /api/know/search?q=...&k=...&expand=...&budget=...
GET /api/know/code/{E0382}
GET /api/know/rule/{BRW-01}
…
```

面板新增 **KNOWLEDGE** 區：顯示 dataset、rustc、原子數，並以相對 URL 查詢知識卡。它是唯讀、無 token 的觀察面，並不給面板任何新控制權。

### 跨程序 cache

預設維持進程內 LRU，避免未經同意把代理問題／程式片段落盤。要啟用跨程序快取：

```bash
export YKC_KB_CACHE_DIR="$HOME/.ykc/kb-context-cache"
```

快取檔案會：

- 放在操作者指定目錄（0700），檔案 0600；
- key = 原有 query fingerprint（已含 dataset version）之 SHA-256；
- 只保存 atom IDs／scores／上下文結構，讀取後一定重新綁定當前唯讀 Store；
- 版本不符、原子遺失、格式損毀一律刪除並安全 miss；
- 上限 256 檔／8 MiB，原子 rename 避免跨程序半寫入。

因此 cache 永遠不是知識或判定的權威來源；它只可加速可由 Store 決定論重建的結果。

---

## 6. 驗證矩陣

```text
make lint                                      ✅
go vet ./...                                   ✅
staticcheck -checks=all ./...                  ✅
go test ./...                                  ✅
make l5-test                                   ✅ 19/19
make anchor-test                               ✅ 截斷／重簽／witness
import → manifest → replay → cmp               ✅ byte-identical
DiffFiles / manifest / persistent cache tests  ✅
Panel /api/know state/search/POST-boundary      ✅
```

`make verify-all` 現新增第 ⑫ 項 head-anchor 回歸，連同既有 smoke、judge、MCP、LSP、precompile、serve 全功能驗證一起執行。

---

## 7. 下一階段（保留既定門檻）

1. **繁中第二批**：先以錯誤出現頻率與 user feedback 排序，擴至 tier-2／book；每批有術語表和 golden。
2. **遠端 witness 產品化**：提供受管 append-only witness、retention、mTLS／OIDC、外部可驗證 checkpoint；不把簡易 HTTP 協定誤稱為不可竄改 SaaS。
3. **R6 向量檢索**：先建立 BM25/shingle 失敗集、精確率／可重放率 baseline，才以可選本地模型接入。
4. **R10 / T-18b**：先完成 interface 真實接線決策與 nightly differential matrix，再開始大規模架構搬遷。

---

## 8. 決策增補

- **D31**：鏈頭安全必須有 project 外狀態；hash chain 不是 rollback 防護。anchor 的 HMAC key 不可與受監督專案一起存放。
- **D32**：remote witness 是強化層，不應讓未配置網路成為本機可驗證性的單點故障；required mode 必須由操作者明確選擇。
- **D33**：KB release 的版本號不足以重放；來源 HTTP witness、內容 hash、原子圖版本和產物 hash 都是 release 證據。
- **D34**：翻譯是資料層，不是 UI overlay；必須有版本、coverage test、原文共存與內容定址。
- **D35**：持久 cache 一律視為非權威效能層；版本綁定、私有檔權限、原子寫入和 corruption-as-miss 是最低要求。
