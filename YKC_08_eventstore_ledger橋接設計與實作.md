# YKC 08 — EventStore 與 Ledger 橋接設計與實作

## 目標

YKC 原有兩類事實保存方式：

1. `eventstore`：每個 event 以獨立 JSON 檔案 atomic rename 寫入，優點是**不會有半行 JSONL / 半寫入事件**。
2. `ledger`：append-only JSONL + hash chain，優點是**可審計、防竄改、可驗證鏈頭**。

橋接目標是把兩者合併成單一路徑：

```text
YKC event → atomic eventstore commit → ledger hash-chain projection
```

若 ledger append 在 event 已提交後失敗，eventstore 會成為 durable outbox，之後可用 `sync-ledger` 補回 ledger projection。

## 新增實作

| 路徑 | 功能 |
|---|---|
| `internal/eventledger/bridge.go` | eventstore → ledger bridge，append / replay / sync / verify |
| `internal/eventledger/bridge_test.go` | append projection 與 sync idempotency 測試 |
| `cmd/ykc-atom` | 改用 bridge append event，同時寫 eventstore + ledger |
| `cmd/ykc-precompile` | 預編譯 report 寫入 `.ykc/precompile/report.json` 後，同步提交 `precompile.report` event + ledger fact |

## 資料流

```text
Agent / YKC command
    ↓
domain.Envelope
    ↓
eventstore.Append()       ← 原子完整事件
    ↓
ledger.Append(event.*)    ← hash-chain 審計投影
    ↓
. ykc/events/YYYYMMDD/*.json
. ykc/ledger.jsonl
```

## Ledger fact 格式

Fact type：

```text
event.<event_kind>
```

例：

```text
event.agent.claim
event.guardrail.decision
event.command.result
event.precompile.report
```

Payload：

```json
{
  "bridge_version": 1,
  "event_id": "...",
  "event_kind": "precompile.report",
  "event_at": "...",
  "epoch": "...",
  "workspace_root": "...",
  "payload_sha256": "...",
  "envelope": { }
}
```

## 實際功能

### 1. 原子 append + ledger projection

`Bridge.Append(e)`：

1. 先把 event 寫入 eventstore。
2. 再檢查 ledger 是否已有該 event id。
3. 沒有則 append hash-chain fact。
4. 回傳 event id、ledger seq、ledger head。

### 2. 缺失補投影

`Bridge.SyncMissing()`：

1. Replay eventstore 所有事件。
2. Replay ledger 已投影 event id。
3. 將缺失 event 補 append 到 ledger。
4. 可重複執行，不會重複投影同一 event。

CLI：

```bash
bin/ykc-atom sync-ledger -root . -state .ykc
```

### 3. 預編譯審計

`ykc-precompile` 現在會在 state 非空時寫三份資料：

```text
.ykc/precompile/report.json       ← 人類 / 工具直接讀
.ykc/events/YYYYMMDD/*.json       ← 原子事件
.ykc/ledger.jsonl                 ← hash-chain 審計
```

### 4. ledger 驗證

`Bridge.VerifyLedger()` 使用原 `internal/ledger.VerifyChain` 驗證鏈頭與 hash。

## 應用方式

### Agent 行為審計

```bash
bin/ykc-atom snapshot -root . -state .ykc
bin/ykc-atom claim -root . -state .ykc -kind tests_passed -text "all tests pass"
bin/ykc-atom sync-ledger -root . -state .ykc
```

結果：

- `.ykc/events` 有完整事件。
- `.ykc/ledger.jsonl` 有 `event.workspace.snapshot`、`event.agent.claim`、`event.guardrail.decision` 等投影。
- 如果 agent 假測，decision 也被 hash-chain 留證。

### Rust 預編譯審計

```bash
bin/ykc-precompile -project ./demo-rust-cli -sandbox native -allow-native
```

結果：

- 預編譯 report 可直接給 AI agent / panel 使用。
- 同一 report 也成為 `event.precompile.report`，被 ledger hash-chain 固化。

## 帶來的提升

| 方面 | 提升 |
|---|---|
| 原子完整性 | eventstore 保證 event 不會半寫入 |
| 可審計性 | ledger 保證 event 投影可驗證、防竄改 |
| 故障恢復 | ledger append 失敗可由 eventstore replay 後補投影 |
| AI 管控 | agent claim / guardrail decision / precompile report 均可留不可抵賴證據 |
| 舊新功能穩定 | 不改舊 ledger hash 算法，只新增 projection 層，降低破壞風險 |
| Panel / MCP 擴展 | 之後 panel 可同時讀 eventstore 最新狀態與 ledger 鏈頭完整性 |

## 已驗證

```bash
go test ./internal/eventledger
go test ./...
go vet ./...
go build -o bin/ykc-atom ./cmd/ykc-atom
go build -o bin/ykc-precompile ./cmd/ykc-precompile
YKC_HOME=/home/user/.cache/ykc-tools make verify-all
```

實測：

- `ykc-atom snapshot/claim/sync-ledger` 會產生相同數量 event files 與 ledger facts。
- `ykc-precompile` 會產生 `event.precompile.report` ledger fact。
- `make verify-all` 全通過。
