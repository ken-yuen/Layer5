# YKC 06 — 沙盒與 rustc 預編譯設計實作

## 結論

YKC 的沙盒採分層策略：

1. **生產 / SaaS / 不可信多租戶**：Firecracker / Kata microVM 為最高安全基線。
2. **本 repo 立即可落地的 container-native 沙盒**：gVisor `runsc`，以 `docker-runsc` / `podman-runsc` backend 執行。
3. **單機開發 fallback**：Bubblewrap `bwrap`，用 Linux namespaces、network namespace、只掛載必要檔案。
4. **Native**：只准 `-allow-native` 明示開啟，標記為 `isolation=none`，不可用於不可信 build.rs / proc-macro。

## 新增實作

| 路徑 | 功能 |
|---|---|
| `internal/sandbox/` | sandbox backend 偵測與 command runner |
| `internal/precompile/` | cargo/rustc 預編譯 pipeline、diagnostic parser、無法預編譯分類 |
| `cmd/ykc-precompile/` | CLI：對 Rust 專案執行 sandboxed precompile |

## CLI

```bash
# 自動選擇 sandbox；無 sandbox 時 fail-closed
bin/ykc-precompile -project ./demo-rust-cli

# 本地 trusted demo 才可用 native
bin/ykc-precompile -project ./demo-rust-cli -sandbox native -allow-native

# 指定 gVisor / runsc
bin/ykc-precompile -project ./my-rust-project -sandbox docker-runsc -image rust:1.98
```

## 預編譯 pipeline

Cargo project：

1. `cargo metadata --format-version=1 [--locked]`
2. `cargo fetch [--locked]`（可控 network 階段；不執行 build.rs）
3. `cargo check --workspace --all-targets --message-format=json [--locked]`
4. `cargo test --workspace --all-targets --no-run --message-format=json [--locked]`
5. 可選：`cargo clippy --workspace --all-targets --message-format=json [--locked]`

Single-file Rust：

```bash
rustc --edition=2021 --error-format=json --emit=metadata -o /tmp/ykc-rustc-precompile.rmeta <file.rs>
```

## 無法預編譯或不可安全預編譯的安排

| 場景 | YKC 行為 | 後續安排 |
|---|---|---|
| 沒有 Cargo.toml，但有 `.rs` | 轉 single-file rustc metadata precheck | 報告 single-file mode |
| 沒有 Cargo.toml 也沒有 `.rs` | `unsupported.no_rust_entrypoint` | 要求建立 Cargo project |
| 無 sandbox 且未 `-allow-native` | `unsupported.sandbox_required` | fail-closed，不執行 cargo |
| 有 `build.rs` | 標記 `build_script_executes` | 只可在 network-disabled sandbox compile；要求審計 |
| 有 `proc-macro = true` | 標記 `proc_macro_executes` | 只可在 sandbox compile；列為 supply-chain risk |
| dependency 需要下載 | 先 `cargo fetch` controlled network | compile stage network none |
| native libs / pkg-config / openssl 缺失 | classify `native_dependency_missing` | 換 sandbox image 或改 pure Rust feature |
| linker / cc 缺失 | classify `system_tool_missing` | sandbox image 加 C toolchain |
| cargo check 過但 build/codegen 仍可能錯 | 報告 limitation | 後續用 full build/release stage 補足 |

## 報告輸出

預設寫入：

```text
.ykc/precompile/report.json
```

報告包含：

- selected sandbox backend
- all detected capabilities
- unsupported / warning reasons
- 每個 stage 的 exit code、timeout、stdout/stderr tail
- rustc/cargo JSON diagnostics summary
- error/warning 計數與 primary span

## 驗證

已在本 sandbox 完成：

```bash
go test ./internal/sandbox ./internal/precompile ./cmd/ykc-precompile
go test ./...
go vet ./...
go build -o bin/ykc-precompile ./cmd/ykc-precompile
bin/ykc-precompile -project ./demo-rust-cli -sandbox native -allow-native -json=false
bin/ykc-precompile -project ./demo-semantic-cli -sandbox native -allow-native -json=true  # 預期 exit=2，E0425 被解析
```

另外 `make verify-all` 仍全通過，代表新功能未破壞原有 smoke / judge / guard / MCP / LSP。

## 本輪穩定性優化補充

全面檢查後，已修復以下潛在技術債：

1. **輸出截斷方向錯誤**：原本 runner 保留輸出開頭，長 cargo build 可能丟失最後真正錯誤；已改為 tail buffer，保留最後 N bytes，並對完整 stream 計算 SHA-256。
2. **host cargo cache 污染**：預編譯現在使用 project-local `.ykc/precompile/cargo-home` 與 `.ykc/precompile/target`，避免污染開發者全局 cache。
3. **offline compile enforcement**：network-none 階段自動加入 `CARGO_NET_OFFLINE=true`，防止 compile 階段偷偷拉網絡依賴。
4. **container 權限污染**：docker/podman backend 增加 `--user uid:gid`，避免容器以 root 在 workspace 產生 root-owned files。
5. **Makefile 回歸缺口**：`verify-all` 現已包含 `ykc-precompile` 健康 demo，防止預編譯功能日後 drift。
