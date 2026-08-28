# YKC 快速開始（朋友下載後 5 分鐘上手）

> 給第一次拿到 YKC 的朋友。你只需要一台能連網的電腦（Linux / macOS / Windows-WSL）。

## 0. 你需要什麼

| 需要 | 說明 |
|---|---|
| 網路 | 首次安裝會下載 Go 與 Rust 工具鏈 |
| 終端機 | macOS 用「終端機」，Windows 用「WSL」 |
| 時間 | 首次安裝約 2–5 分鐘（視網速） |

**不需要**：不需要 Docker、不需要 sudo、不需要懂 Rust。

## 1. 下載

```bash
# 假設你拿到的是 YKC 資料夾（或從 git 取得）
cd ykc        # 進入專案根目錄
```

## 2. 啟動（像普通程式：雙擊即可）

| 你的系統 | 動作 |
|---|---|
| **macOS** | 在 Finder **雙擊 `launch.command`**（自動開 Terminal） |
| **Linux** | 雙擊 `launch.sh`（或終端執行 `bash launch.sh`） |
| **Windows** | 雙擊 `launch.bat`（經 WSL 啟動；需先裝 WSL） |

它會自動：裝環境（Go + Rust + staticcheck 到 `~/.ykc`，**零 sudo**）→ 建全部二進制 → 啟動 `ykc serve`（控制面板 + 監看 + **主動 Rust 預譯**）→ **自動開瀏覽器**。

**面板 = YKC 的「控制 + 觀察台」**：選擇運行對象（專案）→ 按動作（煙測/除錯/閘門/護欄）→ 開始/停止 → 全程實時日誌與狀態，加上唯讀的專案完整性、信任等級、判決證據與即時事實流。`ykc serve` 另會在啟動時預譯所有已發現 Rust 專案，並在 `.rs`、`Cargo.toml`、`Cargo.lock` 改動後自動重跑；預設沒有 sandbox 時只記錄 `unsupported`，不會裸跑不可信編譯期程式。

> 不想用瀏覽器的話，也可在終端機直接下命令（見第 4 節）。

## 3. 一鍵跑全部功能實測

```bash
make verify-all
```

`verify-all` 會先跑 `gofmt + go vet + staticcheck -checks=all`；預期看到 **12 項**全 ✅（第 ②⑦ 項的 FAIL / TAKEOVER 是**故意**的——證明反欺騙引擎正確揪出謊報、並把撒謊代理降級到 T0 全面接管；⑫ 驗證獨立 head anchor）。

## 4. 各功能分別體驗

```bash
make lint           # gofmt + vet + staticcheck（開發／合入前置）
make smoke          # 煙測引擎：驗證 demo CLI 能編譯/能跑/介面符合聲明
make judge          # 除錯閉環：把「有錯專案」自動修到能編譯
make lsp            # LSP：驅動 rust-analyzer 取診斷
make guard-verify   # 反欺騙：誠實 vs 撒謊代理的聲明比對
make guard-score    # 信任棘輪：撒謊代理被降級到 T0 全面接管
make guard-mcp      # MCP：把 YKC 當成 agent 可呼叫的工具
make anchor-test    # head anchor：截斷／重簽／remote witness 回歸
make know-import    # 依目前 rustc 匯入官方 error index，建 blob + release manifest
# （make deps-setup / make deps / make structure 屬 YKC_21 能力包——代碼落庫後恢復）
```

## 5. 對你自己的 Rust 專案用 YKC

```bash
# 煙測（產出簽名收據 ykc-receipt.json）
./bin/ykc -dir /path/to/你的專案

# 除錯閉環（cargo fix 機械修復 + 語意錯誤官方說明）
./bin/ykc-judge -dir /path/to/你的專案

# 使用鎖版 KB；命中的錯誤碼、原子 ID 與上下文 hash 會寫入 .ykc/ledger.jsonl 的 kb.analysis
./bin/ykc-judge -dir /path/to/你的專案 -kb ./bin/kb.ykc

# 驗證 hash chain + project 外 head anchor（可攔截合法前綴截斷／末行重簽）
./bin/ykc-judge -dir /path/to/你的專案 -verify

# 閘門（可放進 git pre-commit，編譯/測試不過就不准提交）
./bin/ykc-judge -dir /path/to/你的專案 -gate

# 反欺騙（把你的代理「聲明」寫成 claims.json 後比對）
./bin/ykc-guard verify -dir /path/to/你的專案 -claims claims.json
```

### L1 依賴與 L2 結構 evidence（可選 capability pack）

> ⚠️ **狀態注記（2026-08-25 審計）**：以下命令對應的 YKC_21 代碼（`ykc-deps` /
> `ykc-structure` / `ykc-cap` 與相關 make 目標）**尚未落庫**（見 `YKC_23_T21執行報告.md`）。
> 落庫前請勿執行；設計細節見 `YKC_20_能力包解耦與組合架構.md`。

```bash
# （待 YKC_21 落庫後可用）
# L1 預設只報告、不替你決定授權政策；-refresh 是人類明確允許的網路動作
make deps-setup
make deps
./bin/ykc-deps scan -dir /path/to/你的專案
./bin/ykc-deps gate -dir /path/to/你的專案

# L2 只帶 Rust + Go grammar subset；輸出寫入 .ykc/structure/latest.json 與 anchored ledger
make structure
./bin/ykc-structure scan -dir /path/to/你的專案

# 可驗證的 pack 組合（核心驗 manifest SHA 後才允許 worker 產生事實）
make pack-deps pack-structure
```

### 鎖版 KB 的 release／重放

```bash
./bin/ykc-know import "$(rustc --version)" -o bin/kb.ykc
./bin/ykc-know replay bin/kb.ykc.manifest.json -o bin/kb-replay.ykc
./bin/ykc-know diff bin/kb.ykc bin/kb-replay.ykc

# 可選：跨程序共用 context cache（預設不落盤；目錄必須私有 0700）
install -d -m 700 "$HOME/.ykc/kb-context-cache"
export YKC_KB_CACHE_DIR="$HOME/.ykc/kb-context-cache"
```

## 6. 疑難排解

| 症狀 | 解法 |
|---|---|
| `make: command not found` | 安裝 make：macOS `xcode-select --install`；Ubuntu `sudo apt install make` |
| 下載 Go/Rust 很慢 | 換網路；或自行安裝 go 與 rustup 後再跑 `bash dev-setup.sh`（會自動偵測沿用） |
| go.dev 被封鎖（內網/受限環境） | `make bootstrap-go`：自 GitHub 源碼六級 bootstrap 鏈自建 Go 1.27（實測 ~18 分鐘/2C3G，見 `bootstrap-go.sh`） |
| 面板暴露到網路 | 加 `-token <密鑰>`（或 env `YKC_PANEL_TOKEN`）；預設已綁 127.0.0.1，暴露前務必讀 README「面板安全邊界」 |
| `🛑 ... head anchor ...` | 不要刪除 `.ykc` 或 anchor 來「修好」；先保留現場、執行 `ykc-judge -verify`，比對 `$YKC_HOME/anchors/`。這通常表示帳本被截斷、回滾或末行重簽。 |
| `⚠️ rust-analyzer 下載失敗` | 僅影響 `make lsp` 展示；其他功能不受影響。稍後可重跑 `bash dev-setup.sh` 重試 |
| Windows 無法執行 | 請用 WSL（Ubuntu），原生 cmd/PowerShell 不支援 |

## 7. 更完整的文件

| 文件 | 內容 |
|---|---|
| `YKC_00_構圖與路線圖.md` | 總體構圖、里程碑、進度 |
| `YKC_03_代碼健檢報告.md` | 代碼品質、遺留事項 |
| `YKC_17_知識庫鎖版與代理可追溯開發執行報告.md` | KB 鎖版、judge provenance、MCP 工具與接續排程 |
| `YKC_18_帳本錨定與知識面可重放擴展報告.md` | Head anchor、remote witness、manifest/replay/diff、繁中 KB 與面板知識面 |
| `YKC_19_T19_L1依賴對齊與T20_L2結構統計實作規劃.md` | L1/L2 工具、政策與 admission 計畫 |
| `YKC_20_能力包解耦與組合架構.md` | 保持 T0 細小的 Core + capability pack 協定 |
| `YKC_21_T19T20能力包MVP執行報告.md` | L1/L2 MVP 的實作、驗證與剩餘工作 |
| `YKC_01_容器化方案分析.md` | Docker/Podman/gVisor 選型 |
| `README.md` | 目錄結構與執行架構 |
