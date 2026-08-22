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

它會自動：裝環境（Go+Rust 到 `~/.ykc`，**零 sudo**）→ 建全部二進制 → 啟動控制面板 → **自動開瀏覽器**。

**面板 = YKC 的「控制 + 觀察台」**：選擇運行對象（專案）→ 按動作（煙測/除錯/閘門/護欄）→ 開始/停止 → 全程實時日誌與狀態，加上唯讀的專案完整性、信任等級、判決證據與即時事實流。

> 不想用瀏覽器的話，也可在終端機直接下命令（見第 4 節）。

## 3. 一鍵跑全部功能實測

```bash
make verify-all
```

預期看到 **9 項**全 ✅（第 ②⑦ 項的 FAIL / TAKEOVER 是**故意**的——證明反欺騙引擎正確揪出謊報、並把撒謊代理降級到 T0 全面接管）。

## 4. 各功能分別體驗

```bash
make smoke          # 煙測引擎：驗證 demo CLI 能編譯/能跑/介面符合聲明
make judge          # 除錯閉環：把「有錯專案」自動修到能編譯
make lsp            # LSP：驅動 rust-analyzer 取診斷
make guard-verify   # 反欺騙：誠實 vs 撒謊代理的聲明比對
make guard-score    # 信任棘輪：撒謊代理被降級到 T0 全面接管
make guard-mcp      # MCP：把 YKC 當成 agent 可呼叫的工具
```

## 5. 對你自己的 Rust 專案用 YKC

```bash
# 煙測（產出簽名收據 ykc-receipt.json）
./bin/ykc -dir /path/to/你的專案

# 除錯閉環（cargo fix 機械修復 + 語意錯誤官方說明）
./bin/ykc-judge -dir /path/to/你的專案

# 閘門（可放進 git pre-commit，編譯/測試不過就不准提交）
./bin/ykc-judge -dir /path/to/你的專案 -gate

# 反欺騙（把你的代理「聲明」寫成 claims.json 後比對）
./bin/ykc-guard verify -dir /path/to/你的專案 -claims claims.json
```

## 6. 疑難排解

| 症狀 | 解法 |
|---|---|
| `make: command not found` | 安裝 make：macOS `xcode-select --install`；Ubuntu `sudo apt install make` |
| 下載 Go/Rust 很慢 | 換網路；或自行安裝 go 與 rustup 後再跑 `bash dev-setup.sh`（會自動偵測沿用） |
| go.dev 被封鎖（內網/受限環境） | `make bootstrap-go`：自 GitHub 源碼六級 bootstrap 鏈自建 Go 1.27（實測 ~18 分鐘/2C3G，見 `bootstrap-go.sh`） |
| 面板暴露到網路 | 加 `-token <密鑰>`（或 env `YKC_PANEL_TOKEN`）；預設已綁 127.0.0.1，暴露前務必讀 README「面板安全邊界」 |
| `⚠️ rust-analyzer 下載失敗` | 僅影響 `make lsp` 展示；其他功能不受影響。稍後可重跑 `bash dev-setup.sh` 重試 |
| Windows 無法執行 | 請用 WSL（Ubuntu），原生 cmd/PowerShell 不支援 |

## 7. 更完整的文件

| 文件 | 內容 |
|---|---|
| `YKC_00_構圖與路線圖.md` | 總體構圖、里程碑、進度 |
| `YKC_03_代碼健檢報告.md` | 代碼品質、遺留事項 |
| `YKC_01_容器化方案分析.md` | Docker/Podman/gVisor 選型 |
| `README.md` | 目錄結構與執行架構 |
