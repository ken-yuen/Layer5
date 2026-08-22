# YKC_21 — T-19/T-20 Capability Pack MVP 執行報告

> 日期：2026-08-23  
> 承接：`YKC_19_T19_L1依賴對齊與T20_L2結構統計實作規劃.md`、`YKC_20_能力包解耦與組合架構.md`  
> 狀態：**T-19 / T-20 MVP 已落地；政策 gate、serve 增量排程與多語言擴張留在下一階段**

---

## 1. 本輪交付

| ID | 交付 | 狀態 |
|---|---|---|
| T-31 | Capability manifest + local JSONL worker protocol + Core-only composition | ✅ |
| T-19a–c | Cargo metadata graph、cargo-audit JSON、cargo-deny JSON diagnostics、report-first CLI | ✅ |
| T-19d（局部） | anchored event、artifact、Trust Console L1 summary | ✅ |
| T-20a–c | pure-Go gotreesitter Rust/Go grammar admission、AST facts、Go dogfood path | ✅ |
| T-20d（局部） | content-addressed snapshot digest、anchored event、Trust Console L2 summary | ✅ |

未把未審核的 license allow-list、source allow-list 或企業 gate 假裝成安全政策；它們仍需人類審核。

---

## 2. Capability Pack Core

新增：

```text
internal/capability/
  types.go       capability manifest / request / response schema
  manifest.go    entrypoint + artifact SHA-256 validation
  protocol.go    bounded local JSONL worker server
  runner.go      verified subprocess execution + response binding
cmd/ykc-cap/
  manifest       建 manifest
  verify         SHA 驗證 pack
  run            執行 worker，只有此 Core-side 命令可寫 anchored ledger
```

核心約束：

```text
worker → 只能回傳 candidate facts/artifacts
Core   → 驗證 schema / request ID / manifest SHA / path / output bounds
Core   → 寫 EventStore + anchored ledger
panel/MCP/gate → 只讀同一套事實
```

因此 L1/L2 可以獨立下載、升級、沙盒化，但不能自行偽造「已通過」或繞過 head anchor。

### 本機 development pack

```bash
make pack-deps
make pack-structure

bin/ykc-cap verify -manifest packs/.../manifest.json
bin/ykc-cap run -manifest packs/.../manifest.json \
  -kind structure.scan -project /path/to/project
```

manifest 已覆蓋 worker entrypoint SHA；deps pack 另覆蓋 `cargo-audit`、`cargo-deny` binaries 的 SHA 與版本需求。

---

## 3. T-19 L1 MVP

新增：

```text
internal/deps/
cmd/ykc-deps/
```

### CLI

```bash
# 預設 locked + offline、report-first
bin/ykc-deps scan -dir /path/to/rust-project

# 明確允許刷新 Cargo/RustSec 資料
bin/ykc-deps scan -dir /path/to/rust-project -refresh

# 建立待人工審核的 cargo-deny 政策模板
bin/ykc-deps init-policy -o /path/to/project/.ykc/deps-policy.toml

# 預設只跑 bans,sources；法律審核 allow-list 後才加入 licenses
bin/ykc-deps scan -dir /path/to/project \
  -policy .ykc/deps-policy.toml \
  -deny-checks bans,licenses,sources

# 只對 block severity 回傳非零
bin/ykc-deps gate -dir /path/to/rust-project
```

### 事實與安全邊界

- `cargo metadata --format-version=1 --locked [--offline]` 是 resolved graph 真相源；
- `cargo audit --json --file Cargo.lock [--no-fetch]` 產生 RustSec finding；
- `cargo-deny --format json --config <policy> check ...` 產生 NDJSON diagnostic；
- 每次 command 都記錄 argv、exit code、stdout/stderr SHA-256、byte count、bounded tail；
- `Cargo.lock` 缺失時輸出 `lock_missing` finding，不靜默重新解決依賴；
- `cargo audit` / `cargo deny` 缺失時輸出 `tool_unavailable`，不假綠；
- 結果寫至：

```text
<project>/.ykc/deps/latest.json
<project>/.ykc/events/…
<project>/.ykc/ledger.jsonl       event.dependency.snapshot
```

### 版本 admission

已以 rustc 1.98.0、單 worker job 實測：

```text
cargo-audit 0.22.2  ✅
cargo-deny  0.20.2  ✅
```

Dockerfile 與 `make deps-setup` 都固定此版本，不再容許 floating latest。

---

## 4. T-20 L2 MVP

新增：

```text
internal/structure/
cmd/ykc-structure/
```

純 Go parser：

```text
github.com/odvcencio/gotreesitter v0.51.0
MIT / no CGo
```

### Rust/Go facts

```text
symbols: function/method/struct/enum/trait/interface/module/type
observations: unsafe block, macro, unwrap, expect, panic, todo, unimplemented
complexity: branch nodes, nesting, function span
structure: import/module text, node histogram, parse error range
bounds: file bytes, file count, total bytes, excluded directories
```

### 體積 admission

```text
ykc core               3.3 MiB
ykc-deps worker         9.9 MiB
ykc-structure Rust+Go  19.0 MiB
ykc-structure all      37.0 MiB  （只作量測，不作預設發行）
```

生產 build 只使用：

```bash
-tags="grammar_subset grammar_subset_rust grammar_subset_go"
```

並以 Make gate 強制：

```bash
make thin-core-test     # ykc-smoke 不得連結 gotreesitter
make structure-size     # Rust+Go worker <= 25 MiB
make structure-test
```

結果寫至：

```text
<project>/.ykc/structure/latest.json
<project>/.ykc/events/…
<project>/.ykc/ledger.jsonl       event.structure.snapshot
```

---

## 5. 面板組合

Trust Console 不 link `internal/deps` 或 `internal/structure`，只讀固定 schema 的 JSON：

```text
L1  packages / block / warn / suggest / duplicate
L2  files / functions / parse errors / unsafe / unwrap / panic
```

這保持 `ykc-panel`／`ykc-serve` 的主產品不帶 grammar blob；若 worker 尚未建置，面板只是不顯示該區，核心仍可運行。

面板新增人類控制動作：

```text
L1 依賴掃描
L1 依賴 gate
L2 結構掃描
```

若 capability binary 未建立，面板明確提示：

```bash
make deps
make structure
```

---

## 6. 驗證

```text
make deps-test                         ✅
make structure-test                    ✅
make thin-core-test                    ✅
make structure-size                    ✅ 19,721,011 bytes <= 25 MiB
make capability-test                   ✅
ykc-deps metadata/audit/deny real demo ✅
ykc-structure Rust demo                ✅
ykc-cap manifest → verify → run        ✅
go test ./...                          ✅
```

測試覆蓋：

- manifest SHA 竄改拒收；
- JSONL request/response binding；
- `cargo metadata` graph、lock missing、cargo-audit current `list` schema、cargo-deny nested diagnostic schema；
- Rust + Go scanner deterministic output、malformed source partial report、file-size bounds；
- `go test -tags='grammar_subset grammar_subset_rust grammar_subset_go'`。

---

## 7. 下一階段（不假裝完成）

1. **L1 policy gate**：exception `owner/reason/expires_at` schema、human-reviewed license/source allow-list、CI/pre-commit blocking。
2. **L1 refresh sandbox**：RustSec DB refresh 與 registry metadata 放入受控網路／OCI capability runtime。
3. **L2 content-addressed per-file reuse**：以 file SHA 重用 individual report，而非目前 snapshot 全掃。
4. **L2 serve debounce**：只在 `.rs/.go` watcher batch 後調 worker，並把 grammar worker 常駐化。
5. **Pack signing/distribution**：OCI manifest layer、cosign/minisign、enterprise registry policy。
6. **SCIP / rust-analyzer 語意層**：只在 AST baseline 和 performance matrix 已建立後引入。
