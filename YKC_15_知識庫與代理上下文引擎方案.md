# YKC_15 知識庫與代理上下文引擎方案

> 狀態：已實作並持續擴展（`internal/kb` + `cmd/ykc-know`，26 項 KB 測試全綠）
> 日期：2026-08-22（2026-08-23 由 YKC_17／YKC_18 擴展）
> 對齊：`YKC_00_構圖與路線圖.md` 的 T 系列路線；延續決策編號 D22（「判定權不轉移」）。
> 執行更新：版本鎖定、manifest/replay/diff、tier-1 繁中摘要、面板知識面與可選跨程序 cache 已落地；詳見 `YKC_18_帳本錨定與知識面可重放擴展報告.md`。

---

## 0. 一句話

把「Rust 官方教學文檔 + 全部 518 條編譯錯誤碼（含錯誤範例與正解）+ 54 條規則抽象」，
做成一個**嵌入式、唯讀、內容定址**的知識庫，並在其上提供**精準檢索、依賴項圖、上下文原子化、
代理上下文緩存、預算截斷**——讓生成式代理在修 Rust 時，拿到「剛好夠用、可驗證、可重放」的上下文，
而不是盲目塞一整本手冊。

---

## 1. 需求 → 實作對照表

| 需求（原話） | 本方案對應 | 實作位置 |
|---|---|---|
| **go 數據庫** | 純 Go（零依賴）嵌入式資料庫：內容定址原子 + 倒排索引 + 圖 + 快取；並支援 `build` 產出**單一唯讀 blob**（magic + 版本 + sha256 尾章，開檔校驗防竄改） | `internal/kb/store.go` |
| **嵌入式** | 資料以 `go:embed` 編入二進制（`data/*.json.gz`，~151 KB 壓縮 / ~613 KB 原始），離線可用、零外部檔案 | `internal/kb/seed.go`、`internal/kb/data/` |
| **唯讀** | Store 建構後不可變：無任何寫入 API；HTTP 端點全 GET；開檔即驗 hash；`state` 回報 `read_only: true` | `internal/kb/store.go`、`http.go` |
| **AI agent（生成式代理）優化** | `Retrieve()` 一鍵產出「最小高訊號上下文」：命中排序 → 依賴展開 → 預算截斷 → 可貼入提示的 Markdown | `internal/kb/context.go` |
| **精準檢索能力** | 四層：①精確鍵短路（錯誤碼/規則 id/章節 id）②倒排索引 + BM25 ③char-shingle 模糊相似（拼寫誤差/程式碼片段）④領域詞彙加權 + 常見錯誤碼優先級；全決定論 | `internal/kb/index.go`、`token.go`、`search.go` |
| **代理上下文緩存** | 記憶體 LRU（條目 + 位元組雙上限）+ 可選私有跨程序 cache；鍵 = 查詢指紋 + 資料版本 + 參數，跨版本自然失效 | `internal/kb/cache.go`、`persistent.go` |
| **上下文原子化** | 每條知識 = 一個自足「原子」（Atom）：ID = 內容 sha256；518 錯誤碼卡 + 54 規則 + 19 部 + 91 章 + 1 目錄 = **683 原子** | `internal/kb/types.go`、`seed.go` |
| **依賴項圖（代理用）** | 原子間 Refs 邊（錯誤↔規則↔章節↔部）+ 反向索引；BFS 展開上下文、`graph`/`/api/kb/graph` 出節點+邊、SCC 環診斷 | `internal/kb/graph.go` |
| **rust 官方教學文檔** | 嵌入 The Rust Programming Language（mdBook）結構：19 部 / 91 章 + 各章摘要 + 小節標題 | `data/book.json.gz` |
| **所有編譯 error code 及例子與正解** | 嵌入 rustc 錯誤索引全文 518 條：標題 + 說明 + **錯誤範例（compile_fail）** + **正解（修正後程式）** | `data/errcodes.json.gz` |
| **rust 規則抽象** | 54 條跨 17 領域的結構化規則（中文陳述 + 為什麼 + 修法菜單 + 對應錯誤碼 + 對應章節 + 規則互引） | `data/rules.json.gz` |
| **既規劃及方案** | 本文件（架構 + 資料模型 + 演算法 + 驗收 + 路線圖） | 本檔 |

---

## 2. 架構總覽

```
                      ┌──────────────────────────────────────────────┐
   ykc-know / ykc-serve│            internal/kb（零依賴、唯讀）          │
   ┌──────────────┐   │                                              │
   │ CLI (query/   │   │  ┌────────────┐   ┌───────────────────────┐ │
   │  search/graph/│──▶│  │   Store     │──▶│  Retrieve(q, opts)     │ │
   │  build/serve) │   │  │  (唯讀實體) │   │  ①Search ②展開 ③截斷   │ │
   └──────────────┘   │  └────┬───────┘   └───────────┬───────────┘ │
   ┌──────────────┐   │       │                       │             │
   │ HTTP API     │──▶│  ┌────▼───────┐ ┌───────────┐ ┌▼──────────┐ │
   │ /api/know/*  │   │  │ 倒排索引    │ │ 依賴項圖    │ │ 上下文快取  │ │
   │ (唯讀, 無token)│  │  │ BM25        │ │ Refs+反向  │ │ LRU        │ │
   └──────────────┘   │  └────▲───────┘ └─────▲─────┘ └───────────┘ │
                      │       │               │                     │
                      │  ┌────┴───────────────┴──────────────────┐  │
                      │  │ 內容定址原子（683）                      │  │
                      │  │ error(518) rule(54) book(91) part(19)  │  │
                      │  │ toc(1)                                 │  │
                      │  └───────────────▲────────────────────────┘  │
                      └──────────────────┼───────────────────────────┘
                                     go:embed data/*.json.gz
                    （err: 官方錯誤索引 / book: 官方教學文檔 / rules: 規則抽象）
```

**資料流**：種子資料（gzip JSON）→ `buildAtoms()` 組裝原子 + 解析依賴 → `contentHash()` 內容定址 →
`newIndex()` 倒排索引 → `newGraph()` 依賴項圖 → `Retrieve()` 供查詢。整條鏈路只讀、決定論、可重放。

**零依賴紀律**：與 YKC「可信裁判自身供應鏈最小化」一致——KB 只用 Go 標準庫
（`embed`/`compress/gzip`/`crypto/sha256`/`container/list`/`net/http`），不引入任何第三方模組。
不依賴外部 embedding 模型：模糊匹配用 char-shingle（字元 3-gram FNV 雜湊）達成，純決定論。

---

## 3. go 數據庫（嵌入式、唯讀）

### 3.1 兩種載體

1. **內嵌（預設）**：`Open()` 由 `go:embed` 種子資料建構，靜態二進制離線可用。
2. **磁碟 blob**：`Build()`/`Save(path)` 產出單一不可變檔案；`OpenFile(path)` 唯讀開啟。

### 3.2 blob 格式（目前版本 2；2026-08-23 鎖版）

```
┌────────────┬─────────┬──────────────┬───────────────────┬──────────┬──────────────────────┬────────────┐
│ magic      │ version │ metadata_len │ metadata JSON     │ count    │ payload              │ sha256     │
│ "YKCKB\0"  │ u32=2   │ u32          │ rustc/source/SHA  │ u64      │ [u32 len][atom] …    │ 32 bytes   │
└────────────┴─────────┴──────────────┴───────────────────┴──────────┴──────────────────────┴────────────┘
```

- metadata 至少可帶 `rustc_version`、`error_index_url`、`error_index_sha256`、`translation_version`；`ykc-know import` 將其與對應官方 `print.html` 一起寫入。
- 開檔依序校驗 magic、格式版本、**`sha256(metadata_len + metadata + count + payload)`**；metadata 與內容均不可被單獨篡改。
- v1（只校驗 payload、沒有來源 metadata）仍可唯讀開啟，但會明確回報來源版本未知，應重新 import。
- 內容定址：`ID = "kb-" + hex(sha256(原子內容))[:8]`；同內容同 ID、異內容異 ID（Merkle-DAG 式）。
- 寫入只發生在建庫時（離線）；執行期**無寫路徑**。

### 3.3 唯讀保證

- `Store` 欄位全部私有，只暴露查詢方法；
- HTTP 端點全部 `GET`，回傳 `Cache-Control: no-store` 且不回寫任何狀態；
- 測試 `TestBlobRoundTripAndTamper` 驗證：翻轉 1 byte → 開檔失敗。

---

## 4. 上下文原子化（Atom）

每條知識切成**最小自足單元**，欄位依種類而異：

| 種類 | Code | 主要欄位 | 數量 |
|---|---|---|---|
| `error` | E0382 | Title / Body(英文官方說明) / ZH(可選繁中摘要) / Err(錯誤範例) / Fix(正解) / Source | 518（tier-1 60 張已有 ZH） |
| `rule` | BRW-01 | Title / ZH(中文陳述) / Why / Fixes(修法菜單) / Refs | 54 |
| `book` | what-is-ownership-1 | Title / Body(摘要) / Sections(小節) / Refs→part | 91 |
| `book-part` | understanding-ownership | Title | 19 |
| `toc` | toc | Refs→全部部 | 1 |

原子化帶來三項代理優化：
1. **可尋址**：`ByCode("E0382")` 直取單卡，不必載整本；
2. **可組合**：依賴項圖按需把相關原子拼成閉包；
3. **可計量**：`Atom.Size()` 供預算截斷。

---

## 5. 精準檢索能力

四層，由上而下：

1. **精確鍵短路**：查詢是錯誤碼/規則 id/章節 id（大小寫不敏感）→ 該原子置頂（分數 1e9）。
2. **BM25**（`k1=1.2, b=0.75`）於加權欄位倒排索引上：Code/Tags 3.0、Title 2.5、ZH 2.0、錯誤/正解程式碼 1.6、Body/Why 1.0。**程式碼參與檢索**：搜 `&mut`、`clone()` 可命中範例。
3. **char-shingle 模糊**：查詢與候選的字元 3-gram FNV 集合做 containment（∈[0,1]）；BM25 全無命中時切換為全量 shingle 回退（拼寫誤差、新措辭仍可召回）。
4. **領域 + 優先級加權**：查詢詞 → 領域（"borrow"→borrowing/lifetime…65 個詞表）；常見錯誤碼 tier 優先級。

**決定論**：無隨機、無模型溫度；同查詢、同資料集 → 同結果（`TestSearchDeterministic`）。

**排序公式**：`score = bm25Norm·(1 + 0.25·shingle + 0.2·domainBoost) + 0.1·priority`。

### 檢索效果（實測）

```
q = "cannot borrow as mutable"  →  E0499, E0596, E0502, E0501, E0503 …（全對）
q = "borow checkr"（拼錯）       →  回退召回 borrow/lifetime 領域知識
q = "trait not implemented"     →  TRT-01 規則、E0277 …
q = "variable used after move"  →  E0382
```

---

## 6. AI agent（生成式代理）優化：Retrieve 管線

```
Retrieve(q, opts{ K, ExpandDepth, BudgetBytes })
  ├─ 快取查詢（命中 → 直接回傳，標 CacheHit）
  ├─ Search(q, K)                       ← 精準檢索
  ├─ expandOrdered(roots, ExpandDepth)  ← 依賴項圖 BFS（根按相關性序、鄰居按 ID 序）
  ├─ 預算截斷（累加 Atom.Size()，超 BudgetBytes 即止並標 Truncated）
  └─ 渲染 Markdown（可貼入提示）/ JSON（機器讀）
```

- **高訊號最小化**：預設 12000 bytes ≈ 3000 tokens；只帶「答案 + 背後的規則 + 出處」閉包，不塞全書。
- **可驗證**：每個原子帶 `Source` URL 與內容定址 ID；版本指紋可對賬。
- **`RenderMarkdown`**：錯誤卡 → 說明 + 錯誤範例 + 正解（` ```rust ` 區塊），代理可直接貼入提示。
- **`EstTokens`**：bytes/4 估值，供呼叫方做提示預算。

---

## 7. 代理上下文緩存

- LRU，條目（256）與位元組（8 MiB）雙上限，`container/list` 實作、mutex 保護。
- **鍵 = `version + "\0" + query + "\0" + K + ExpandDepth + BudgetBytes`**：
  同查詢同參數必命中；**資料版本（全部原子 ID 的 sha256）含於鍵中**——升級知識庫後舊鍵自然失效，不會拿到過期上下文。
- 統計經 `/api/kb/state` 暴露（memory hits/misses/entries/bytes + 可選 persistent stats），供觀測。
- **跨程序層（2026-08-23）**：只在顯式設置 `YKC_KB_CACHE_DIR` 時啟用；檔案 0600、目錄 0700、key 再以 SHA-256 命名、最多 256 檔／8 MiB。資料版本、key digest、atom IDs 任一不符或損毀即安全 miss 並重建，從不把 cache 當權威資料。
- 決定論 + 快取 = 代理重複提問零成本（`TestContextCacheAndBudget`、`TestPersistentCacheSharesOnlyVersionMatchedContexts`）。

---

## 8. 依賴項圖（代理用）

**邊的語意 = 「依賴 / 前置知識」**（有向）：

```
rule  → rule.refs（規則互引）         BRW-01 → BRW-03, OWN-01, LFT-01
rule  → rule.book（出處章節）          BRW-01 → references-and-borrowing-1
rule  → rule.codes（對應錯誤碼）        BRW-01 → E0499, E0502, E0596
chapter → part（章節屬部）             what-is-ownership-1 → understanding-ownership
toc   → 全部部
```

**反向索引（Backrefs）**：誰引用我。因此查 `E0382` 展開會帶出引用它的 `OWN-*` 規則，再帶出規則引用的章節——**「答案 → 規則 → 出處」閉包**。

**展開**：在 `依賴 ∪ 被依賴` 無向化圖上 BFS（visited 去重、有界深度），
根按相關性序、鄰居按 ID 序 → 決定論；`graph` 子命令與 `/api/kb/graph` 同時輸出**節點 + 邊**，可直接畫圖。

**環診斷**：相關概念互引（BRW-01 ↔ BRW-03）成「相關性環」，屬預期；Kosaraju SCC 供診斷
（實測 11 個 SCC），BFS 不受影響。

---

## 9. 資料集（嵌入式種子）

| 資料 | 來源 | 規模 |
|---|---|---|
| 編譯錯誤碼 | rustc 官方錯誤索引（doc.rust-lang.org/error_codes，抽取自 print.html） | **518 條**（434 條含錯誤範例、414 條含正解；含「不再發出的舊碼」） |
| 官方教學文檔 | The Rust Programming Language（mdBook print.html） | 19 部 / 91 章 / 524 標題節點 |
| 規則抽象 | 手撰、與官方錯誤碼 + 章節 + 既有 L5 幾何規則卡對齊 | **54 條 / 17 領域** |
| 加權表 | 手撰 | 60 tier1 + 456 tier2 錯誤碼、65 詞→領域 |

資料抽取腳本（`/tmp`，一次性）產出 `internal/kb/data/*.json.gz`；重新抽版只需重跑並 `make know-build`。

> 說明：L5 的 ChordLaw 幾何規則卡（`internal/borrow/rulecard.go` 的 E01–E10）是「借用」領域規則的
> 幾何化壓縮；本 KB 的 BRW-* 規則與其同源互補（KB 給「廣度」，L5 給「借用幾何深度」）。

---

## 10. 對外介面

### 10.1 CLI（`cmd/ykc-know`）

```
ykc-know code E0382                 # 錯誤碼卡（錯誤範例 + 正解）
ykc-know search "borrow after move" [-k 8 -expand 2 -budget 12000 -json]
ykc-know rule BRW-01 / rules -domain borrowing
ykc-know graph E0382 -depth 2       # 依賴項圖展開
ykc-know book [id] / codes / stats
ykc-know build -o kb.ykc            # 由內嵌鎖版種子建唯讀 v2 blob
ykc-know import "$(rustc --version)" -o kb.ykc # 取官方 error index、寫 blob + release manifest
ykc-know replay kb.ykc.manifest.json -o replay.ykc # 重抓來源並逐項可重放驗證
ykc-know diff old.ykc new.ykc [-json] # 比較 metadata、內容原子與 Refs
ykc-know open kb.ykc search "..."   # 對 blob 唯讀查詢
ykc-know serve -addr -port -blob kb.ykc # 以已校驗鎖版 blob 提供唯讀 HTTP API
```

### 10.2 HTTP API（唯讀，無 token）

已掛入 **ykc-serve**（`/api/know/*`，與 `/api/state` 同屬「觀察」面）與 **ykc-know serve**（`/api/kb/*`）：

```
GET /api/know/state           資料版本、rustc／來源 SHA／translation metadata、683 原子統計、memory/persistent 緩存統計、read_only
GET /api/know/search?q=&k=&expand=&budget=&format=json|md
GET /api/know/code/{E0382}    錯誤碼卡
GET /api/know/rule/{OWN-01}   規則
GET /api/know/rules?domain=   規則列表
GET /api/know/graph?code=&depth=  節點 + 邊
GET /api/know/book [/{id}]    教學文檔目錄 / 章節
GET /api/know/codes           全部錯誤碼（一行標題）
```

---

## 11. 驗收

`go test ./internal/kb/`（26 項測試函式，全綠；含 2026-08-23 鎖版、manifest/diff、繁中與 persistent cache 增量）：

| 測試 | 驗證點 |
|---|---|
| `TestOpenCounts` | 518 錯誤 + 54 規則 + 91 章 + 19 部 + 1 toc = 683 |
| `TestVersionDeterministic` | 兩次 Open 版本一致、16 hex |
| `TestByCodeCaseInsensitive` | 錯誤碼/規則/章節三類鍵、大小寫不敏感 |
| `TestSearchExactCode` | 精確碼置頂 |
| `TestSearchSemantic` | 語意命中 + 拼錯 shingle 回退 |
| `TestSearchDeterministic` | 同查詢同結果（含分數） |
| `TestGraphExpand` | E0382 展開帶出 ownership 規則；章節→部 |
| `TestContextCacheAndBudget` | 二次查詢命中；極小預算截斷 |
| `TestRenderMarkdown` | 渲染含 ` ```rust ` 範例區塊 |
| `TestBlobRoundTripAndTamper` | blob 版本一致；竄改開檔失敗 |
| `TestContentAddressing` | 同內容同 ID、異內容異 ID |
| `TestGraphExpandPreservesRootOrder` | 多根輸入順序保留與決定論 |
| `TestImportErrorIndexWritesVersionedBlob` | 官方 print.html／受管鏡像抽取、v2 blob 與 source metadata |
| `TestImportRejectsInvalidVersionAndURL` | 版本、URL 輸入邊界 |
| `TestImportedBlobProtectsMetadata` | 篡改 v2 metadata 即 checksum 失敗 |
| `TestEmbeddedMetadataMatchesRustToolchainLock` | 內嵌 KB rustc metadata 與部署工具鏈鎖一致 |
| `TestBuildBlobCarriesEmbeddedMetadata` | `Build()` 的 v2 header round-trip |
| `TestLegacyV1BlobRemainsReadableButHasNoVersionProvenance` | 舊 blob 只讀相容與未知來源提示 |
| `TestHTTPStateReportsVersionedDatasetMetadata` | `/api/kb/state` 暴露 rustc／來源 metadata |
| `TestImportManifestRoundTripAndReplay` | URL/ETag/SHA/原子/blob manifest 與 byte-identical replay |
| `TestDiffStoresIncludesContentAndReferenceChanges` | 內容 ID 與 Refs 漂移分別可見、排序決定論 |
| `TestDiffFilesRoundTrip` | 已校驗 blob 的檔案 diff 路徑 |
| `TestTier1ErrorCardsHaveTraditionalChineseSummaries` | tier-1 60/60 繁中摘要 coverage |
| `TestErrorMarkdownRendersTraditionalChineseSeparately` | ZH 顯示且英文官方原文不被覆寫 |
| `TestPersistentCacheSharesOnlyVersionMatchedContexts` | 跨 Store 命中、私有檔權限、版本不符安全 miss |
| `TestPersistentCacheDisabledWithoutExplicitDirectory` | 未明確設定時不落盤 |

`go test ./...` 全綠；`go vet` 通過。Makefile 新增 `know / know-test / know-build / know-import / know-serve`，
`binaries` 納入 `ykc-know`。

---

## 12. 決策記錄（接續 D22）

- **D23**：KB 採「內容定址原子 + 倒排索引 + 圖 + LRU」全自研，不引入第三方（含嵌入向量模型）——維繫零依賴與決定論。
- **D24**：模糊檢索用 char-shingle 而非詞向量：決定論、可重放、可審計，且 683 文件規模下成本可忽略。
- **D25**：邊的語意定為「依賴（前置知識）」，反向用 Backrefs 補「被依賴」；相關性環（互引）視為正常，以 SCC 診斷而非報錯。
- **D26**：KB 端點一律唯讀、無 token（與 `/api/state` 同級）；控制面（claims/jobs）維持既有 token 邊界不變。
- **D27**：知識庫輸出是**解釋**，判定仍以 rustc 為準（延續 D22「判定權不轉移」）。
- **D28**（2026-08-23）：KB blob metadata（rustc／來源 URL／來源 SHA）必須與 payload 一起被校驗；v1 只讀相容但不偽稱有來源 provenance。
- **D29**（2026-08-23）：judge 寫 `kb.analysis`，記下實際錯誤碼→資料集／原子／上下文 hash，不把「代理說看過文件」當證據。
- **D30**（2026-08-23）：staticcheck 固定為開發／CI 工具，`make lint` 缺工具即失敗；不把它納入 YKC 二進制執行期供應鏈。

---

## 13. 路線圖（後續）

1. **T-KB-2 增量 — ✅ 2026-08-23**：`ykc-know import` 從 rustc 版本號／`rustc --version` 抽取對應官方 `print.html`；blob v2 把 rustc、來源 URL 與來源 SHA-256 放入已校驗標頭，且內嵌 metadata 以測試對齊 `deploy/rust-toolchain.toml`。
2. **T-KB-3 代理回填 — ✅ 2026-08-23**：judge 的 `-kb` 可使用已校驗 blob，將「錯誤碼 → KB dataset/rustc/source/atom IDs/context hash」以 `kb.analysis` 追加到事實帳本。
3. **T-KB-4 MCP 工具 — ✅ 2026-08-23**：`ykc.kb_search` / `ykc.kb_explain` 已加入 stdio MCP，並有 query、展開深度和上下文預算界限。
4. **T-KB-5 中文版 MVP — ✅ 2026-08-23**：tier-1 60 張高頻錯誤卡已有獨立 `ZH` 繁中摘要、translation version、60/60 coverage test 與英文原文共存；518 卡／91 章全文回填仍按術語表與人工抽樣分批推進。
5. **T-KB-6 manifest + `ykc-know diff` — ✅ 2026-08-23**：`import` 產出 URL/ETag/SHA/dataset/blob manifest，`replay` byte-identical 核對；`diff` 比較 metadata、內容定址原子與 Refs。
6. **T-KB-7 跨程序 cache — ✅ 2026-08-23**：以 `YKC_KB_CACHE_DIR` 顯式 opt-in，資料版本／atom ID 驗證、私有權限與 corruption-as-miss；不改 blob。
7. **T-KB-8 向量檢索（可選）**：需要語意級召回時，以可插拔介面接入本地 embedding，維持零依賴預設不變；必須先有 BM25/shingle 失敗集與離線評測基線。

---

## 14. 檔案清單

```
internal/kb/
  types.go     Atom 型別 + 內容定址
  seed.go      go:embed 種子載入 + 原子組裝 + Refs 解析
  token.go     Tokenize（駝峰/底線/錯誤碼）+ char-shingle
  index.go     倒排索引 + BM25
  graph.go     依賴項圖（Deps/Dependents/展開/SCC）
  cache.go / persistent.go  LRU + 可選私有跨程序 context cache
  metadata.go  資料集 rustc／來源／SHA／translation metadata（blob v2 標頭）
  import.go / manifest.go  官方 error-index 鎖版抽取器、release manifest/replay
  diff.go      metadata、內容原子與 Refs 的決定論 diff
  zh.go        tier-1 error card 繁中摘要層（英文來源不覆寫）
  store.go     Store（Open/Build/Save/OpenFile + v1/v2 驗證 + 查詢）
  search.go    四層精準檢索 + 排序
  context.go   Retrieve 管線 + Markdown 渲染
  http.go      唯讀 HTTP API
  *_test.go    26 項驗收（鎖版、manifest/diff、翻譯、persistent cache）
  data/        errcodes/book/rules/boost .json.gz（~151 KB）
cmd/ykc-know/main.go   CLI（code/search/rule/graph/book/codes/stats/build/import/replay/diff/open/serve）
Makefile      know / know-test / know-build / know-import / know-replay / know-diff / know-serve
internal/panel/knowledge.go  ykc-panel／ykc-serve 共用 /api/know/* 唯讀掛載
docs/rust_error_codes.md      全部 518 條錯誤碼（例子 + 正解）參考檔
docs/rust_rules_abstract.md   54 條規則抽象參考檔
docs/rust_terms_zh_hant.md   tier-1 繁中摘要術語表／翻譯紀律
```
