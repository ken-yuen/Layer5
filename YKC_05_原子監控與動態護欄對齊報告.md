# YKC 05 — 原子監控與動態護欄對齊報告

本文件記錄本次把 GitHub 原檔 `https://github.com/ken-yuen/KYC` 與上一輪開發內容對齊後的結果。

## 對齊原則

1. **不破壞原檔命令**：原有 `ykc-smoke`、`ykc-judge`、`ykc-guard`、`ykc-lsp`、`ykc-panel` 全部保留。
2. **把上一輪開發內容收斂為可重用 internal package**：原子檔案寫入、事件存儲、snapshot、guardrail、enforcement、smoke runner 皆放入 `internal/`。
3. **避免命令名衝突**：上一輪 `cmd/ykc` 改名整合為 `cmd/ykc-atom`，避免覆蓋原本由 `cmd/ykc-smoke` build 出來的 `bin/ykc`。
4. **採 fail-closed**：證據不足、payload 壞掉、agent 假聲稱，全部進入可審計的阻斷/煙測接管模式。

## 新增檔案

| 路徑 | 用途 |
|---|---|
| `cmd/ykc-atom/main.go` | 原子監控與動態護欄 CLI：`snapshot` / `claim` / `smoke` / `replay` |
| `internal/atomicfile/atomicfile.go` | same-directory temp file + fsync + atomic rename |
| `internal/domain/events.go` | 統一事件模型、AgentClaim、CommandResult、DiagnosticSummary |
| `internal/eventstore/store.go` | immutable per-event JSON store，避免 JSONL 半寫入 ambiguity |
| `internal/enforcement/enforcement.go` | 寫入 `.ykc/enforcement/state.json` 與 `AGENT_WRITES_BLOCKED` |
| `internal/guardrail/policy.go` | 行為驅動動態護欄：一次假測/假完成即 critical |
| `internal/monitor/snapshot.go` | workspace content snapshot + digest + diff |
| `internal/smoke/smoke.go` | YKC 接管 smoke command runner |
| `internal/*/*_test.go` | guardrail / snapshot / eventstore 單元測試 |
| `docs_guardrails_atomic.md` | 動態護欄規格 |

## 與原有系統的分工

| 原有模組 | 保留角色 | 新增對齊內容 |
|---|---|---|
| `cmd/ykc-guard` | 聲明抽取、確定性比對、信任棘輪、MCP、用家控制台 | 可在後續調用 `internal/guardrail` 作低層 enforcement engine |
| `internal/ledger` | hash-chain JSONL 事實帳本 | 新 `eventstore` 補充原子 per-event store；兩者可共存 |
| `cmd/ykc-smoke` | 原 smoke engine | 新 `internal/smoke` 提供可重用 command evidence runner |
| `cmd/ykc-panel` | Trust Console | 後續可讀 `.ykc/enforcement/state.json` 顯示 block 狀態 |

## 行為護欄規則

| Agent 行為 | 判斷 | Enforcement |
|---|---|---|
| `tests_passed` 無同 epoch 新鮮 test/smoke success | `fake_test_claim` critical | `smoke_takeover` + `block_agent_writes` |
| `build_passed` 無同 epoch 新鮮 check/build/smoke success | `fake_build_claim` critical | `smoke_takeover` + `block_agent_writes` |
| `no_errors` 但最新 diagnostics 仍有 blocking errors | `fake_no_errors_claim` critical | `smoke_takeover` + `block_agent_writes` |
| `work_done` 但同 epoch 無文件變化/命令/diagnostic 證據 | `unsupported_done_claim` critical | `smoke_takeover` + `block_agent_writes` |
| blocking diagnostic count 增加 | `diagnostics_regression` high | evidence required，可由 policy 升級 |

## CLI 使用

```bash
# 建立 workspace content snapshot
bin/ykc-atom snapshot -root . -state .ykc

# Agent 聲稱測試通過；無證據時立即進入 smoke takeover
bin/ykc-atom claim -root . -state .ykc -kind tests_passed -text "all tests pass"

# 由 YKC 接管 smoke 驗證，產生 command evidence
bin/ykc-atom smoke -root . -state .ykc

# 重放事件
bin/ykc-atom replay -state .ykc
```

## 後續接線建議

1. `cmd/ykc-guard score` 在出現 severity 3/4 時，同步寫入 `internal/enforcement` 狀態。
2. `cmd/ykc-panel` 新增讀取 `.ykc/enforcement/state.json`，在 UI 顯示「代理寫入已阻斷」。
3. LSP / MCP adapter 在看到 `AGENT_WRITES_BLOCKED` 時，拒絕 autonomous edit，只允許產生 patch proposal。
4. 將 `eventstore` 與 `ledger` 建立橋接：eventstore 保證原子事件完整，ledger 保證 hash-chain 審計。

## 驗證狀態

已在本 sandbox 內補齊臨時工具鏈並完成驗證：

- Go：`go1.27.0 linux/amd64`
- Rust：`rustc 1.98.0` / `cargo 1.98.0`
- rust-analyzer：`0.3.3016-standalone`

工具鏈安裝於 `.cache` 類目錄，供本次驗證使用，不污染 repository source tree。

已執行並通過：

```bash
cd KYC
gofmt -w internal/atomicfile internal/domain internal/eventstore internal/enforcement internal/guardrail internal/monitor internal/smoke cmd/ykc-atom
go test ./internal/atomicfile ./internal/domain ./internal/eventstore ./internal/enforcement ./internal/guardrail ./internal/monitor ./internal/smoke ./cmd/ykc-atom
go test ./...
go build -o bin/ykc-atom ./cmd/ykc-atom
YKC_HOME=/home/user/.cache/ykc-tools make verify-all
```

`make verify-all` 全部通過，包含：煙測、反欺騙 FAIL 預期路徑、L4 judge、帳本完整性、誠實代理 PASS、撒謊代理 TAKEOVER、MCP server、LSP initialize。

為避免把生成二進制與測試狀態納入 source snapshot，驗證後已刪除 `bin/` 與 `.ykc-test/`。
