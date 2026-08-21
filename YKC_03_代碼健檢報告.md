# YKC 全專案代碼健檢報告（兩輪）
### （2026-08-21 ｜ 找錯／找債／找重／找缺陷 → 除錯、減債、去重、補漏）

> 結論：**兩輪完成。12 個 Go 檔案全部通讀審查，修復 2 個真實 bug、清除 4 類技術債、消除 5 處重複代碼（含跨模組 hash 演算法 drift 風險）、修補 6 處缺陷；重構後 8 項回歸 + 5 項邊界測試全通過。** 代碼從「4 個獨立 module、多份重複實作」收斂為「單一 module + internal/ 共享包」。

---

## 一、第一輪：發現的問題清單

### A. 真實 bug（2）
| # | 位置 | 問題 | 修復 |
|---|---|---|---|
| A1 | `ykc-smoke` checkSanity | `strings.TrimLeft(arg, "-")` 第二參數是**字元集合**而非前綴，語意錯誤（`--version` 碰巧對，遇 `---x` 即錯） | 改 `strings.TrimPrefix(arg, "--")` |
| A2 | `ykc-guard` hasReceipt（上輪已修，本輪複查確認） | 曾有「子字串比對帳本」誤判偽造收據 | 已確認只認三種真實簽發來源 |

### B. 技術債（4 類）
| # | 位置 | 問題 | 修復 |
|---|---|---|---|
| B1 | `ykc-smoke` checkBehavior | 參數 `dir` 未使用（死參數，用全域 workDir） | 移除死參數 |
| B2 | `ykc-smoke` checkContract | `helpText` 變數 + `_ = helpText` 死代碼 | 刪除 |
| B3 | `ykc-judge` ledger | `Ledger.key` 欄位從未使用（死欄位） | 移除 |
| B4 | `ykc-judge` ledger | `itoa()` 用 `json.Number(...).String()` 繞路 | 直接 FormatUint |
| B5 | `ykc-judge` main | `upper()` 用 `s[0]-32` ASCII 技巧 | 改 `strings.ToUpper` |
| B6 | `ykc-guard` console | `var _ = filepath.Join` 湊 import 死代碼 | 刪除死 import |

### C. 重複代碼（5 處 → 全部收斂為 1 份）
| # | 重複內容 | 原分佈 | 收斂 |
|---|---|---|---|
| C1 | **事實帳本 Fact + hash 演算法 + Ledger** | ykc-judge/ledger.go 與 ykc-guard/ledger.go 兩份手寫 | `internal/ledger`（**消除跨模組 hash drift 風險——兩份演算法若走樣，跨模組驗證即失效**） |
| C2 | `sign` HMAC | smoke / judge 兩份 | `internal/rustutil.Sign` |
| C3 | `run`/`runCapture`/`runIn`（在 dir 執行命令） | smoke / judge / guard 三份 | `internal/rustutil.Run` |
| C4 | `packageName`（讀 Cargo.toml） | smoke / guard 兩份 | `internal/rustutil.PackageName` |
| C5 | `subcommands`（解析 clap help） | smoke / guard 兩份 | `internal/rustutil.Subcommands` |

### D. 缺陷 / 健壯性（6）
| # | 位置 | 問題 | 修復 |
|---|---|---|---|
| D1 | `ykc-smoke` checkBehavior | 硬編碼 example 名 "hello"——無 hello example 的真實專案會**誤報 fail** | 掃描 `examples/*.rs`，無則 **skip** 而非 fail |
| D2 | `ykc-smoke` checkContract | panic probe 硬編碼 `greet` 子命令（demo 專用假設） | 改「未知旗標 + 第一個子命令缺參數」通用 probe |
| D3 | `ykc-judge` / `ykc-guard` | `_ = led.Append(...)` 靜默忽略帳本寫入失敗（磁碟滿時**裁判完整性被靜默破壞**） | `appendFact()` 檢查錯誤並醒目告警 |
| D4 | `ykc-judge` ledger | `Append` 失敗不回滾 seq（會造成序號跳號） | 失敗時回滾 seq |
| D5 | `ykc-guard` hasFunction | `filepath.Walk` 未跳過 `target/` 編譯產物 | 加 `SkipDir` |
| D6 | `ykc-guard` console | markdown 表格未轉義 `|` 與換行（聲明文字可破壞表格） | 加 `mdEscape` |

---

## 二、結構重構（去重的治本方案）

**重構前**：4 個獨立 module（ykc-smoke / ykc-judge / ykc-guard / ykc-lsp + ykc-core），各自 go.mod，跨 module 無法共享 → 重複實作。

**重構後**：單一 module `ykc`，Go 官方標準布局：

```
cmd/          四個命令（ykc-smoke / ykc-judge / ykc-guard / ykc-lsp）
internal/     共享包（唯一實作）
  ledger/     事實帳本（judge+guard 共用 → 消除 drift）
  rustutil/   執行/解析/簽名/雜湊通用工具
core/         五層窄介面 + Executor 骨架
```

同步更新：`Makefile`（health 目標、build 路徑）、`deploy/Dockerfile`（COPY 路徑）、`README.md`（目錄結構）。

---

## 三、第二輪：複檢結果

| 檢查 | 結果 |
|---|---|
| `gofmt -l` | 空（0 違規）✅ |
| `go vet ./...` | 通過 ✅ |
| `go build ./...` | 通過 ✅ |
| 重複代碼複查（sign/run/packageName/subcommands） | 各僅 1 份實作 ✅ |
| **跨模組帳本互操作** | judge 寫 → guard 讀 ✅；guard 寫 → judge verify ✅ |

---

## 四、回歸與邊界測試（全通過）

**8 項回歸**：① 煙測（正確揪出 2 謊報 → FAIL 為預期）② L4 閘門 ③ 帳本完整性 ④ LSP 診斷 ⑤ 護欄誠實 PASS ⑥ 護欄撒謊 TAKEOVER ⑦ MCP 三工具 ⑧ 除錯閉環端到端（機械修復 2 錯誤 + 1 警告，檔案正確修復）。

**5 項邊界**：① 無 examples 專案 → skip 不誤報 ② claims 檔案不存在 → 安全跳過 ③ 空 claims → 報「無任何聲明」 ④ claims 不存在 → 報載入失敗 ⑤ verify 帳本不存在 → 報讀取失敗。

---

## 五、遺留事項（誠實記錄，非本輪範圍）

| # | 事項 | 風險 | 建議時機 |
|---|---|---|---|
| R1 | 帳本仍為 JSONL 檔案（設計上會換 bbolt） | 低（單寫者 + hash 鏈已保證不可竄改；bbolt 換來 MVCC 快照與唯讀檢視） | P3 |
| R2 | `subcommands` 依賴 clap help 文本格式（人類文本解析） | 中（clap 換格式會破） | 改用 clap `CommandFactory` 匯出機器可讀 command tree |
| R3 | `hasFunction` 用 regex 匹配原始碼（非語意層） | 低（會被註解/字串內的 `fn` 誤匹配） | L2 接入 rust-analyzer documentSymbol 後取代 |
| R4 | MCP server 為 newline-delimited 最小實作 | 低（協定已實證） | 換 mark3labs/mcp-go 取得完整 MCP 規格相容 |
| R5 | smoke 的 `demo` panic probe 移除後，對「無子命令」的 CLI 少一個 probe | 極低 | 依真實使用場景再擴 |

---

## 六、結論

兩輪健檢達成用戶四項要求：
- **除錯**：修 2 個真實 bug + 6 處缺陷；
- **減債**：清 6 處死代碼/死參數/死欄位/繞路；
- **去重**：5 處重複收斂為 1 份，並消除最危險的「跨模組 hash 演算法 drift」；
- **補漏**：帳本寫入失敗不再靜默、example 不再硬編碼、markdown 轉義、seq 回滾。

**代碼健康度顯著提昇，且全部行為經回歸與邊界測試驗證未退化。** 重構後的單一 module 布局，也為 P3（L1/L2 併入、全面接管編排器）掃清了「跨模組共享」的障礙。

---

*健檢工具：`go vet`、`gofmt -l`、`go build`、人工逐檔通讀 12 檔案；回歸 8 項、邊界 5 項，全部通過。*
