# YKC_20 — L1/L2 能力包解耦與可驗證組合架構
> 日期：2026-08-23（依 git 提交時刻 ebfaf38 08-22 22:03 UTC = HKT 08-23；與 YKC_19／YKC_21 自述日期一致——2026-08-25 審計回填）

> 目標：**T0 成品保持細小；需要時才取 L1/L2；取回後仍可在同一套 anchored ledger、面板、MCP 與 gate 中組合成一個可信裁判。**  
> 執行更新：manifest/JSONL/Core-only composition、L1/L2 MVP 已落地；見 `YKC_21_T19T20能力包MVP執行報告.md`。

---

## 1. 結論：不要用 Go plugin；採「Core + Capability Pack + 事實協定」

```text
                 ┌──────────────────────────────────────┐
                 │              ykc-core（T0）            │
                 │ ledger / policy / MCP / panel / pack mgr│
                 │ schema verifier / artifact hasher       │
                 └───────────┬───────────────────────────┘
                             │ versioned local JSONL RPC
       ┌─────────────────────┼──────────────────────┐
       ▼                     ▼                      ▼
┌──────────────┐    ┌──────────────────┐    ┌───────────────────┐
│ ykc-pack-deps│    │ ykc-pack-struct  │    │ future capability │
│ cargo metadata│   │ Rust/Go grammar  │    │ L5/NLL/enterprise │
│ audit / deny │    │ AST facts        │    │                   │
└──────┬───────┘    └────────┬─────────┘    └───────────────────┘
       │                     │
       └──────────────┬──────┘
                      ▼
          Core 驗證 schema / hash / path / bounds
                      ▼
       EventStore → anchored ledger → panel / MCP / gate
```

**不要採 Go `plugin`：**它要求主程式與 plugin 的 Go toolchain／ABI 精確相容，跨平台能力弱，Windows 不適合，亦難以把不可信輸出收窄在子程序邊界。L1/L2 應是獨立能力包（sidecar worker），以穩定資料協定組合，而不是 runtime linking。

---

## 2. 產品拆分

### 2.1 `ykc-core`：永遠細小、靜態、零重工具依賴

保留在預設下載：

```text
ledger + eventstore + head anchor
Datalog guardrail + policy evaluator
panel / serve / MCP / CLI routing
KB core（可再拆為預設 embedded + 外掛 blob）
capability resolver、manifest verifier、JSON schema/bounds validator
```

不放入預設核心：

```text
cargo-audit / cargo-deny 執行檔
RustSec advisory DB
206 個 Tree-sitter grammar
SCIP / DuckDB / NLL facts 工具鏈
企業專用政策與私有 registry credentials
```

### 2.2 L1 `ykc-pack-deps`

內容：

```text
worker: ykc-deps
cargo metadata adapter
pinned cargo-audit / cargo-deny
optional local RustSec DB cache
policy template / schema（不是未審核的 license allow-list）
```

優點：cargo tool 與 advisory DB 不會令 T0 二進制或基本下載膨脹；需要供應鏈掃描的環境才下載此 pack。

### 2.3 L2 `ykc-pack-structure-rustgo`

第一個 grammar pack 只含 Rust + Go：

```text
ykc-structure worker
pure-Go gotreesitter runtime
Rust grammar
Go grammar
AST query / extractor rules
```

不先把所有語言塞進來。擴展語言改為獨立 pack：

```text
ykc-pack-structure-web      TypeScript / JavaScript / HTML / CSS
ykc-pack-structure-python   Python
ykc-pack-structure-jvm      Java / Kotlin
```

---

## 3. 兩種 grammar 載入方式

| 模式 | 做法 | 優點 | 風險／決策 |
|---|---|---|---|
| **A. 分開 worker binary（MVP 建議）** | 每種 grammar set 編成獨立靜態 `ykc-structure-*` | 最穩、cross-platform、無 runtime ABI | 下載檔案較多，但只下載所需 pack |
| **B. signed grammar asset（PoC 後）** | core worker 動態讀 gotreesitter 相容 grammar blob | grammar 可獨立更新、去除 worker 重複 | 必須驗證 gotreesitter loader API／ABI、資產 hash、資源上限 |

T-20 首輪採 **A**。`gotreesitter` 的「純 Go + grammar table」能力先做 admission PoC；確認 loader、binary delta、AST golden 後才決定是否升級 B。

---

## 4. Pack manifest：組合時的契約

每個 pack 都有一份內容定址 manifest：

```json
{
  "schema": "ykc-capability-pack/v1",
  "id": "structure-rustgo",
  "version": "0.1.0",
  "core_api": ">=1.0.0 <2.0.0",
  "os_arch": "linux-amd64",
  "entrypoint": "bin/ykc-structure",
  "entrypoint_sha256": "...",
  "artifacts": [
    {"path": "grammars/rust.gts", "sha256": "..."},
    {"path": "grammars/go.gts", "sha256": "..."}
  ],
  "tools": [
    {"name": "cargo-audit", "version": "0.22.2", "sha256": "..."},
    {"name": "cargo-deny", "version": "0.20.2", "sha256": "..."}
  ],
  "output_schema": "ykc.structure.snapshot/v1"
}
```

安裝後目錄：

```text
$YKC_HOME/packs/
  deps/0.1.0/linux-amd64/
    manifest.json
    bin/ykc-deps
    tools/cargo-audit
    tools/cargo-deny
  structure-rustgo/0.1.0/linux-amd64/
    manifest.json
    bin/ykc-structure
```

`ykc-core` 只執行 manifest 已列、hash 已核對、API 版本相容的 entrypoint。首次先採 SHA-256 + HTTPS release manifest；企業／雲端再加 cosign/minisign 簽章與 registry policy。

---

## 5. 組合方法：RPC 給結果，Core 才能寫事實

### 5.1 單次 CLI

```text
ykc deps scan
  Core → spawn pack worker
  Core → stdin: ScanRequest JSONL
  Worker → stdout: ScanResponse JSONL
  Core → 驗證 / hash artifact / append event
```

### 5.2 `ykc serve` 常駐模式

```text
Cargo.toml / Cargo.lock changed  → ykc-pack-deps one-shot scan
.rs / .go changed batch          → long-lived ykc-pack-structure worker
                                  → changed-file request / SHA reuse
```

常駐 L2 worker 可保留 parser／grammar memory，取得 incremental 效益；但它仍不可直接寫 event store 或 ledger。

### 5.3 固定協定（示意）

```json
{"schema":"ykc.capability.request/v1","request_id":"…","kind":"structure.scan",
 "project":"/verified/path","snapshot":{"files_sha256":"…"},"limits":{"max_file_bytes":2097152}}
```

```json
{"schema":"ykc.capability.response/v1","request_id":"…","status":"ok",
 "facts":[…],"artifacts":[{"sha256":"…","bytes":1234}],"diagnostics":[…]}
```

Core 強制：

- request ID 對應；
- JSONL 單行／總輸出上限；
- project path 必須白名單；
- artifact 必須在 worker temp root、由 Core 重算 SHA；
- finding severity 只是**建議**，真正 block/warn 由 Core policy 決定；
- worker stderr 永遠只作 diagnostics，不能當結論。

如此「重新組合」不是把程式碼 link 回核心，而是把**已驗證的事實**組回同一條 anchored ledger。面板、MCP、Datalog、gate 因而不需要知道結果原先來自哪個 binary。

---

## 6. 發行方式：Thin / Standard / Full 三種 profile

| Profile | 包含 | 適用 |
|---|---|---|
| `thin` | `ykc-core` | T0、離線、只做 judge/guard/KB 基礎能力 |
| `rust` | core + `structure-rustgo` | Rust 開發／YKC dogfood |
| `secure` | core + `deps` + `structure-rustgo` | CI／團隊供應鏈治理 |
| `full` OCI | 全部 pack 以 OCI layer 合併 | 可重現容器、企業交付 |

OCI 可採 layer 共用：core layer、Rust tool layer、deps pack layer、grammar pack layer 分開。使用者拿 `ykc-full` 時體驗是一個產品，但 registry／node cache 只會下載缺少的 layer。

---

## 7. 安全與體積的關鍵取捨

1. **把 toolchain 與 grammar 拆走，不是把驗證權拆走。** Core 仍擁有 path、policy、hash、ledger、anchor 與 gate。
2. **L1 refresh 不等於普通 scan。** advisory DB 更新、registry/network 一律人類明確觸發，必要時放沙盒／OCI。
3. **L2 parser 雖不執行專案程式碼，仍處理不可信輸入。** 所以有單檔、總量、遞迴、parser timeout／worker restart 的資源限制。
4. **能力包可缺席但不可假綠。** 沒安裝 `deps` 時回 `capability_missing` finding；不能把「無工具」顯示成「無漏洞」。
5. **Go plugin 不採用。** 用 subprocess + versioned protocol 犧牲少量 IPC，換來跨平台、獨立升級、可 sandbox、可審計。

---

## 8. 實作順序

```text
P0  pack manifest verifier + local JSONL worker harness（先不下載任何第三方）
P1  ykc-pack-deps：cargo metadata fixture adapter
P2  ykc-pack-structure-rustgo：gotreesitter Rust/Go admission PoC
P3  artifact store + event/ledger integration + panel capability health
P4  pinned tool installer / OCI layers / CI matrix
P5  policy gate、serve persistent worker、MCP reports
```

這個順序確保即使 grammar 或 cargo tool admission 失敗，T0 core 仍不受污染；只需拒絕某一 pack manifest，不需回滾整個產品。
