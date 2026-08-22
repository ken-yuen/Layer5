# YKC_22 — rust-analyzer / cargo：使用前解耦合方案 ＋ 生產期耦合方案

> 分支：`ykc-serve-datalog`　撰寫日期：2026-08-23
> 定位：延續 YKC_20「Core + Capability Pack + 事實協定」路線，把 **Rust 工具鏈（cargo / rustc / rust-analyzer）** 這一組外部依賴，明確拆成兩個生命週期策略：
> - **使用前（開發、建置、測試、分發期）＝解耦合**：ykc 本體不需要任何 Rust 工具鏈也能編譯、測試、發佈。
> - **生產期（裁判實際執行期）＝受控耦合**：工具鏈以「鎖版 + 內容定址 + 帳本申報」方式強耦合，版本即事實，事實可審計。

---

## 0. 現狀盤點：倉庫內全部耦合點（已逐檔核實）

| # | 位置 | 耦合對象 | 耦合方式 | 風險 |
|---|------|----------|----------|------|
| 1 | `internal/rustutil/rustutil.go` `Run()` | cargo / rustc / 任意 bin | `exec.Command` 直呼，**無介面抽象** | 唯一集中點（好事），但被 6+ 個 cmd 直接 import |
| 2 | `cmd/ykc-judge/cargocheck.go` | `cargo check --message-format=json`、`cargo fix`、`rustc --explain` | 直呼 rustutil.Run | 測試必須有真 cargo；JSON 格式無契約鎖定 |
| 3 | `cmd/ykc-guard/verify.go` | `cargo check` / `cargo test` | 直呼 | 同上 |
| 4 | `cmd/ykc-lsp/main.go` | **rust-analyzer**（stdio JSON-RPC，Content-Length 框定） | spawn + 20 秒等 `publishDiagnostics` | 每次冷啟動索引整個專案；r-a 對 toolchain 有最低版本要求 |
| 5 | `internal/precompile/precompile.go` | `cargo metadata / fetch / check / test --no-run / clippy` | runStage + 沙盒網路分級（已做得好） | 版本漂移仍可能令診斷 JSON 變化 |
| 6 | `internal/panel/server.go` `EnsureToolchainPath()` | PATH / RUSTUP_HOME / CARGO_HOME | 環境猜測（多候選目錄） | 隱式耦合：找到哪個算哪個，帳本不知道用了哪個版本 |
| 7 | `Makefile` | `cargo install cargo-audit/cargo-deny --locked`（已 pin 版本 ✅） | deps-setup 目標 | 開發者本機需先裝 Rust |
| 8 | `deploy/Dockerfile` | `rust:1.98.0-bookworm` + `rustup component add rustfmt clippy rust-analyzer` | 多階段建置（已鎖 rust-toolchain.toml ✅） | base image 只鎖 tag 未鎖 digest；`|| true` 吞掉 component 失敗 |
| 9 | `internal/sandbox/sandbox.go` | docker / podman / runsc / bwrap | `exec.LookPath` 探測 + 如實申報（好範式） | — |
| 10 | `internal/borrow/analyzer.go` | chordlaw（python） | `Available()` 如實申報 + 降級（**全倉最佳範式，應推廣**） | — |

**核心觀察**：`internal/borrow` 已示範了正確模式——`Available()` 如實申報、不可用即降級、判定權不轉移。cargo / rust-analyzer 目前反而是「假設一定存在」的硬耦合。方案就是把 #10 的模式推廣到 #1–#6。

---

## 第一部分：使用前解耦合方案（開發／建置／測試／分發期）

### 目標驗收條件

```text
A1. 在「完全沒有 Rust 工具鏈」的機器上：go build ./... 與 go test ./... 全綠
A2. CI 拆成兩條 lane：core-lane（無 Rust，秒級）＋ toolchain-lane（有 Rust，契約測試）
A3. T0 下載物（ykc-core）體積不含任何 Rust 工具鏈相關資產（延續 YKC_20 §2.1）
A4. 所有對 cargo/rust-analyzer 的呼叫都經過一個窄介面，可注入 fake/replay
```

### 1.1 引入 Toolchain Port（六邊形架構：port & adapter）

在 `core/interfaces.go` 增加窄介面，令五層與 cmd 只依賴介面、不依賴 `exec`：

```go
// RustToolchain 是對 cargo/rustc 的唯一出口（port）。
type RustToolchain interface {
    Available() (bool, string)                 // 如實申報（沿用 borrow.Analyzer 範式）
    Version(ctx context.Context) (ToolchainInfo, error) // rustc -vV / cargo --version / r-a --version
    Check(ctx context.Context, dir string) (CheckResult, error)    // --message-format=json
    Test(ctx context.Context, dir string) (TestResult, error)
    Metadata(ctx context.Context, dir string) ([]byte, error)
    Explain(ctx context.Context, code string) (string, error)      // rustc --explain
}

// DiagnosticsProvider 是對 rust-analyzer 的唯一出口。
type DiagnosticsProvider interface {
    Available() (bool, string)
    Diagnostics(ctx context.Context, file string) ([]Diagnostic, error)
}
```

三個 adapter：

| Adapter | 用途 | 所在期 |
|---|---|---|
| `nativeToolchain` | 包住現有 `rustutil.Run`，行為完全不變 | 生產期 |
| `replayToolchain` | 讀 golden fixture（真 cargo JSON / 真 LSP 訊息的錄音帶）回放 | 使用前（單元測試、無 Rust 的 CI） |
| `unavailableToolchain` | `Available() == false, why`，一律回「能力缺席」而非報錯 | T0 core-only 分發 |

**重點**：`cmd/ykc-judge`、`cmd/ykc-guard`、`internal/precompile` 改為收介面注入；`rustutil` 降級為 `nativeToolchain` 的私有實作細節，禁止 cmd 層直接 import（用 `depguard`/自訂 lint 在 CI 把關）。

### 1.2 Record / Replay：契約測試取代「測試需要真工具鏈」

- 新增 `internal/toolchain/testdata/`：
  - `cargo-check-E0502.json`、`cargo-metadata-v1.json`…（由 toolchain-lane CI 用鎖版 cargo 錄製，內容定址 sha256 入檔名）
  - `ra-publishDiagnostics-*.json`（LSP 3.17 訊息原文）
- 單元測試（core-lane）只跑 replay：解析器、指紋、降級路徑、帳本寫入全部可測，**零 Rust 依賴**（KB 已內嵌 518 條 rustc 錯誤碼樣例，可直接作 fixture 源）。
- 契約測試（toolchain-lane）：用鎖版工具鏈重新產生輸出，與 golden diff——**cargo JSON 診斷格式或 r-a 行為一旦漂移，先在 CI 爆，不在生產爆**。

### 1.3 分發解耦：延續 YKC_20 能力包，補一個 `ykc-pack-rust-toolchain`

YKC_20 已把 cargo-audit / cargo-deny 拆入 `ykc-pack-deps`。同理新增：

```text
ykc-pack-rust-toolchain（生產期才下載）
├── manifest.json          ykc-capability-pack/v1；entrypoint_sha256；tools 版本聲明
├── rust-toolchain.toml    channel 鎖死（如 1.98.0）+ components: [rust-analyzer, clippy, rustfmt]
├── bin/ykc-rustd          常駐 worker（見第二部分 2.4）
└── (可選) vendor 離線鏡像指紋清單
```

- T0 `ykc-core` 對 Rust 專案的所有功能在 pack 缺席時顯示「能力未安裝＋一鍵安裝指引」，而不是 exec 失敗。
- 不採 Go plugin（YKC_20 §1 結論不變）：sidecar worker + versioned JSONL RPC。

### 1.4 環境探測顯性化：`ykc doctor`

把 `EnsureToolchainPath()` 的隱式猜測改成顯性探測命令：

```text
$ ykc doctor
cargo        1.98.0  /home/u/.ykc/cargo/bin/cargo      sha256=ab12…  ✅
rustc        1.98.0  (rust-toolchain.toml 匹配)                        ✅
rust-analyzer 2026-08-11 (rustup component, toolchain 對齊)            ✅
cargo-audit  0.21.x  pinned                                            ✅
sandbox      bwrap                                                     ✅
```

探測結果寫入帳本（`toolchain.probe` 事實），開發者與代理讀到同一份真相。

### 1.5 CI 拆 lane（.github/workflows）

```yaml
core-lane:      # 無 Rust。gofmt / vet / staticcheck / go test（replay）/ go build 全平台交叉編譯
toolchain-lane: # dtolnay/rust-toolchain@<鎖版>；契約測試 + golden 重錄 diff + precompile E2E
nightly-drift:  # 每晚以 latest stable 跑契約測試 → 提前發現 cargo JSON / r-a 行為漂移，開 issue 不擋 merge
```

---

## 第二部分：生產期耦合方案（受控強耦合）

生產期的哲學相反：**不要鬆散，要釘死**。「rustc 是唯一真相」的前提是 rustc 本身版本可審計、可重放。

### 2.1 版本鎖定三件套

1. **`rust-toolchain.toml`（已有，deploy/ 下）補齊 components**——rust-analyzer 改用 rustup component 隨 toolchain 鎖定，保證 r-a 與 rustc 版本永遠對齊，避免「r-a 要求 toolchain ≥ X」的斷裂（rust-analyzer 已明確會拒絕過舊 toolchain）：

```toml
[toolchain]
channel    = "1.98.0"          # 精確版本，非 "stable"
components = ["rustfmt", "clippy", "rust-analyzer", "rust-src"]
profile    = "minimal"
```

   權衡：rustup component 的 r-a 比 r-a 官方週更版舊；但裁判系統要的是**確定性**而非最新功能，component 路線正確。若某 bug 必須用新版 r-a，改為在 pack manifest 內 pin 官方 release 的 sha256，兩者擇一、寫入帳本。

2. **`Cargo.lock` 全部 commit**；生產編譯一律 `cargo fetch --locked`（受控網路階段）→ `cargo check/build --frozen`（斷網階段）。precompile.go 已有 NetworkHost/NetworkNone 分級，只需把 `--frozen` 補進斷網 stage 的參數。

3. **氣隙／高保證環境**：`cargo vendor` + `.cargo/config.toml` source replacement，vendor 目錄整體 sha256 入 pack manifest。

### 2.2 容器耦合加固（deploy/Dockerfile）

```dockerfile
# tag 之外再鎖 digest：tag 可被重推，digest 不可
FROM rust:1.98.0-bookworm@sha256:<digest> AS toolchain
# 去掉 "|| true"：component 裝不上必須 fail-fast，不能靜默降級
RUN rustup component add rustfmt clippy rust-analyzer rust-src
# 建置終段自我申報：把版本指紋燒進 image label + /etc/ykc-toolchain.json
RUN rustc -vV > /etc/ykc-toolchain.json && cargo --version >> /etc/ykc-toolchain.json \
 && rust-analyzer --version >> /etc/ykc-toolchain.json
```

沙盒策略維持現有結論：**不可信代碼（build.rs / proc-macro 會在編譯期執行任意代碼）一律走 gVisor / bwrap / k8s sandbox-runtime**，native cargo 只給可信本地專案（precompile.go 的 `native_not_isolated` 申報保留）。

### 2.3 啟動握手：版本即事實，入帳本

`ykc serve` 啟動時執行一次工具鏈握手，寫入 `toolchain.attest` 事實（append-only hash 鏈）：

```json
{
  "type": "toolchain.attest",
  "payload": {
    "rustc": "1.98.0 (hash…)", "cargo": "1.98.0", "rust_analyzer": "…",
    "cargo_bin_sha256": "…", "expected": "rust-toolchain.toml@sha256:…",
    "match": true
  }
}
```

- `match == false` 時由 policy 決定：`strict`（拒絕出裁判結論）或 `degrade`（結論標記 `toolchain-mismatch`，Trust Console 顯紅）。
- 此後每一條 `crate.compile` / `test.run` 事實都可回鏈到當時的 attest 序號 → **任何裁判結論都能回答「是哪個 rustc 說的」**。

### 2.4 rust-analyzer 生產耦合形態：常駐 `ykc-rustd`

現時 `cmd/ykc-lsp` 每次 spawn 冷啟動、等 20 秒，生產不可用。改為：

1. **常駐 worker**（併入 YKC_14 常駐進程框架）：每個受測專案一個 r-a session，initialize 一次，之後 `didOpen/didChange` 增量取診斷，毫秒級回應。
2. **看門狗**：r-a 記憶體超限（可設 4GB 上限）或 5 次無回應即重啟，事件入帳本（`lsp.restart` 事實）。
3. **角色定位不變**：r-a 診斷只作**低延遲前哨**（L2/L3 快速訊號）；裁判定論仍以 `cargo check --message-format=json` 為準——r-a 與 rustc 偶有診斷差異，判定權必須留給 rustc（維持「rustc 是唯一真相」公理）。
4. 批次場景可用 `rust-analyzer diagnostics <dir>` 一次過模式作後備（無需 LSP 握手）。

### 2.5 供應鏈耦合（延續 Makefile 現狀並收緊）

- `cargo-audit` / `cargo-deny` 維持 `--locked` pin（已做 ✅），版本寫入 pack manifest 的 `tools` 欄（YKC_20 已定義 ✅）。
- 每日排程 CI 跑 `cargo audit`：新 advisory 出現時通知而非擋生產（帳本記 `advisory.new` 事實）。
- lockfile diff 視為安全相關變更，強制人審。

### 2.6 升級流程（受控解凍）

```text
1. bot 開 PR：bump rust-toolchain.toml channel（每 6–12 週一次，跟 Rust release train）
2. toolchain-lane 重錄 golden → diff 報告自動貼進 PR（cargo JSON / r-a 行為變化一目了然）
3. 人審 → merge → 重建 image（新 digest）→ 新 attest 事實
4. 帳本上兩個 attest 序號之間 = 一個「工具鏈世代」，所有裁判結論按世代可分段重放
```

---

## 第三部分：落地排序（建議切成 4 個 PR）

| PR | 內容 | 驗收 |
|----|------|------|
| **T-21a** | `RustToolchain`/`DiagnosticsProvider` 介面 + native adapter 包裹 rustutil；cmd 層改注入 | 行為零變化；depguard 禁 cmd 直呼 rustutil |
| **T-21b** | replay adapter + golden fixtures + CI 拆 core-lane / toolchain-lane | 無 Rust 機器 `go test ./...` 全綠 |
| **T-21c** | `ykc doctor` + serve 啟動 `toolchain.attest` 事實 + strict/degrade policy | 面板顯示工具鏈指紋；mismatch 走 policy |
| **T-21d** | Dockerfile digest 鎖定、去 `|| true`、r-a 改 component 鎖版；`ykc-rustd` 常駐化（併 YKC_14） | 契約測試綠；r-a 診斷 P95 < 500ms |

---

## 附：聯網搜查依據（2026-08 查證）

1. rust-analyzer 可經 `rust-toolchain.toml` 的 `components` 欄位隨 toolchain 鎖版，對任何編輯器/調用方一致生效；已 pin toolchain 的專案應同步 pin r-a component（https://v5.chriskrycho.com/notes/rust-analyzer-version-via-rust-toolchain-toml/）
2. 新版 rust-analyzer 會拒絕過舊 toolchain（例：「only supports 1.90.0 and higher」），並官方建議改用 rustup component 對齊——印證「r-a 與 rustc 必須同世代鎖定」（https://github.com/rust-lang/rust-analyzer/issues/21234）
3. rustup 分發的 r-a 相對官方週更版滯後，是確定性 vs 新鮮度的取捨（https://www.reddit.com/r/rust/comments/kvbl6n/ ; RFC 2912）
4. 離線/重現建置標準做法：`cargo fetch --locked` →（斷網）`cargo build --frozen`；`--frozen` ≡ `--locked --offline`；氣隙用 `cargo vendor` + source replacement（cargo-vendor man page；Arch Rust package guidelines）
5. 生產 supply-chain 基線：commit Cargo.lock、rust-toolchain.toml 精確 pin、cargo-audit/cargo-deny 常態化、lockfile diff 人審、build.rs/proc-macro 視為編譯期任意代碼執行（https://www.systemshardening.com/articles/cicd/rust-cargo-supply-chain-security/ ; https://swatinem.de/blog/rust-toolchain/）
