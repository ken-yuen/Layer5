# YKC_24 — 主動 Rust 預譯實作報告

**日期**：2026-08-23
**基線**：`ykc-serve-datalog`
**目的**：把原本需要人類在面板按下的 rustc/cargo 預編譯，提升為 YKC 常駐監督面的預設能力。

## 1. 行為結論

`ykc serve` 現在採 **active-by-default**：

1. 服務啟動後，對每一個已發現的 Rust 專案主動執行一次預譯；
2. 監看器對 `.rs`、`Cargo.toml`、`Cargo.lock` 的去抖批次，會自動排程該專案下一次預譯；
3. 同一專案同一時間只允許一輪預譯；編譯期間再發生的任意多次編輯會合併為一輪 follow-up，使用最新工作區，不吞掉最後一次修改；
4. 每輪結果同時寫入 `.ykc/precompile/report.json`，並以 `precompile.report` 事件投影到 hash-chain ledger；
5. `/api/watch` 暴露執行中／待處理／最後結果／sandbox／診斷計數，Trust Console 的 AUTO PRECOMPILE 區直接顯示。

這表示「寫入代碼」不再需要另按一次 precompile 按鈕才會得到 rustc 事實；按鈕仍保留作為人類明示的手動重跑入口。

## 2. 觸發與合併模型

```text
serve start
  ├─ discover Cargo projects
  ├─ watcher starts
  ├─ startup → requestPrecompile(project)
  └─ file batch (.rs/.toml/.lock)
       ├─ append file.change
       └─ requestPrecompile(project)
            ├─ idle   → start worker
            └─ running → pending=true（只保留一輪 follow-up）
```

`internal/serve` 的 coordinator 使用 project-scoped single-flight：

- `request_count` 是收到的觸發數；
- `run_count` 是實際啟動的預譯輪數；
- `pending=true` 表示當前 cargo/rustc 完成後會以最新內容再跑一輪；
- event loop 不會被預譯阻塞，故仍可繼續收集及記錄檔案變更。

## 3. 預譯內容

每輪沿用 `internal/precompile` 的完整 pipeline：

```text
cargo metadata [--locked]
cargo fetch [--locked]                 # controlled network phase
cargo check --workspace --all-targets --message-format=json
cargo test --workspace --all-targets --no-run --message-format=json
```

若目錄沒有 `Cargo.toml` 但包含 `.rs`，則使用 single-file `rustc --emit=metadata` 路徑。`Cargo.lock` 存在時自動採用 `--locked`。

編譯 stage 會使用 project-local：

```text
<project>/.ykc/precompile/cargo-home
<project>/.ykc/precompile/target
<project>/.ykc/precompile/home
```

因此不污染使用者全域 Cargo cache，也不會因 YKC 自己寫 report／ledger 而觸發 watcher 回環（`.ykc` 與 `target` 仍在排除清單）。

## 4. 安全策略

主動不等於放寬隔離。預設 `precompile-sandbox=auto`，選擇順序維持：

1. Docker + gVisor `runsc`；
2. Podman + `runsc`；
3. bubblewrap `bwrap`；
4. 沒有隔離能力時回報 `unsupported.sandbox_required`。

預設 **不會** 把 untrusted build.rs、proc-macro 或測試編譯產物改以 native 執行。compile stage 使用 network-none，並注入 `CARGO_NET_OFFLINE=true`；依賴下載只在 `cargo fetch` 的 controlled network stage 發生。

對可信本地專案，操作者可以明確表達例外：

```bash
./bin/ykc-serve -root . \
  -precompile-sandbox native -precompile-allow-native
```

關閉主動能力同樣必須是明示動作：

```bash
./bin/ykc-serve -root . -no-auto-precompile
```

`-no-auto-precompile` 只關閉 serve 的自動排程，不會刪除既有 report、ledger 或改變手動 `ykc-precompile` 的語義。

## 5. 可觀察與可追溯輸出

### report

```text
<project>/.ykc/precompile/report.json
```

report 增加 `trigger` 欄位，常見值為 `startup` 或 `file-change`；完整 stage、sandbox capability、JSON diagnostics、stdout/stderr tail 仍保留。

### ledger

每輪完成會追加：

```text
event.precompile.report
```

事件 payload 是同一份 report，故可以由 event store 重放並由 hash-chain 驗證。report 寫入採 atomic write，避免面板讀到半份 JSON。

### HTTP

`GET /api/watch` 的每個 project 項目包含：

- `precompile.enabled`
- `precompile.running` / `precompile.pending`
- `request_count` / `run_count`
- `last_overall`（`passed`、`failed`、`unsupported`）
- `last_sandbox` 與 isolation trust
- 最後一輪診斷 errors/warnings、report 路徑

`GET /api/state` 另外投影有限的 `precompile` 摘要，不把大型 compiler output 塞進熱狀態回應；完整內容仍以 report／ledger 為準。

## 6. 面板與啟動入口

- `launch.sh` 改由 `ykc-serve` 提供 Trust Console，因此雙擊啟動後即有主動預譯；
- `make panel` 與 `make serve` 均啟動 `ykc-serve`；
- 面板手動 precompile action 改為 `-sandbox auto`，不再暗中帶 `-allow-native`；
- `ykc-panel` 二進制仍保留作為不需要常駐監看時的薄面板入口。

## 7. 驗收重點

應至少執行：

```bash
gofmt -w cmd/ykc-serve/main.go internal/serve/serve.go \
  internal/precompile/precompile.go internal/panel/state.go \
  internal/panel/jobs.go

go test ./internal/serve ./internal/panel ./internal/precompile ./internal/sandbox
make lint
```

手動 smoke：

```bash
./bin/ykc-serve -root . -port 18099
# 另一個 terminal：
# 1. GET /api/watch，確認 auto_precompile=true；
# 2. 修改任一 demo-rust-cli/src/*.rs；
# 3. 再 GET /api/watch，確認 request_count/run_count 增加；
# 4. 確認 demo-rust-cli/.ykc/precompile/report.json 及 ledger 的
#    event.precompile.report 存在；
# 5. 無 bwrap/runsc 時，確認結果是 unsupported，而不是 native 執行。
```

## 8. 不變式

- rustc/cargo 仍是唯一編譯判定來源；主動預譯不把 LSP、datalog、KB 或 ChordLaw 提升為判定者；
- `.ykc`／`target` 仍不會成為 watcher 輸入；
- 預譯失敗照樣產生可審計 report，不會被轉成「沒有執行」或偽造成功；
- 無 sandbox 時 fail-closed，不以便利性換取 build script/proc-macro 隔離失效；
- 預譯期間的新編輯不被丟棄，最多合併成一輪最新狀態重跑。
