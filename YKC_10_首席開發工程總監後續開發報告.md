# YKC 10 — 首席開發工程總監：環境還原、全面代碼審視與後續開發報告
> 日期：2026-08-22（依 git 提交時刻 e97364e 08-21 23:44 UTC = HKT 08-22；副標題的 08-21 為受命日——2026-08-25 審計回填）

### （2026-08-21 ｜ 受命於「git 拉取全部檔案 → 安裝依賴 → 代碼全面理解 → 提交後續開發報告」之職責）

> 本報告為**獨立第三輪審視**（繼 YKC_03 兩輪健檢、YKC_07 技術債審計之後）。
> 審視方式：**全部 30 個 Go 源檔逐檔通讀（5,619 行）＋ 18 份設計文件全讀 ＋ 實際安裝工具鏈並運行驗證**，非僅文獻審查。

---

## 0. 執行摘要

| 維度 | 結論 |
|---|---|
| 專案狀態 | P0.5/P1/P2 三里程碑已落地（煙測、除錯閉環、反欺騙護欄 + 控制台），與 YKC_00 D 表一致，**代碼與文檔高度自洽** |
| 工具鏈安裝 | Go 1.27.0 已於本沙盒成功還原（見 §1 之 bootstrap 鏈）；**Rust 工具鏈在本沙盒不可獲得**（網絡白名單無 static.rust-lang.org / crates.io），凡 cargo 相關驗證步驟標記為「環境受限、命令已備妥」 |
| 代碼品質 | 整體**高於同規模專案平均水準**：hash 鏈帳本、fail-closed 沙盒、確定性比對、原子寫入等核心安全屬性實作嚴謹；發現 **6 處 S 級新問題**（面板網路暴露＋任意專案路徑、雙套煙測引擎 drift、diagnostic 事件 schema 斷層、帳本開鏈不校驗、CI 工具鏈浮動、release 漏 2 個二進制）與 10 處 D 級債務（詳 §4） |
| 最大結構性缺口 | `core/interfaces.go` 五層窄介面是**未被任何命令使用的骨架**——實際命令直接呼叫 internal 包。模組化單體（YKC_02）目前停在文件層面（§3.4） |
| 最高價值下一步 | ① 面板安全加固（1 天）② 煙測引擎合一（2–3 天）③ 常駐監聽器（atom → daemon）④ L1 依賴對齊（cargo-audit/deny 包裝，2–4 天）⑤ 可證偽指標基線（Rust-SWE-bench 對照）|

---

## 1. 環境還原與依賴安裝記錄

### 1.1 沙盒網絡現狀（實測）

| 通道 | 狀態 |
|---|---|
| github.com / api.github.com / codeload.github.com | ✅ 可用 |
| registry.npmjs.org、pypi.org、files.pythonhosted.org | ✅ 可用 |
| go.dev、dl.google.com、golang.org、proxy.golang.org | ❌ 封鎖 |
| static.rust-lang.org、sh.rustup.rs、crates.io、static.crates.io | ❌ 封鎖 |
| 全部容器 registry（Docker Hub/ghcr/quay/ecr/gcr）、conda、Debian apt 鏡像 | ❌ 封鎖 |
| GitHub release 資產 CDN（release-assets / objects / media） | ❌ 封鎖（golang/go 與 rust-lang/rust 在此世界亦無 release 資產） |

### 1.2 Go 工具鏈：六級 bootstrap 鏈（本沙盒原創還原法）

`dev-setup.sh` 預設從 go.dev 下載——本沙盒封鎖。鑑於 **golang/go 各 release 的 `src/make.bash` 明文標註其最低 bootstrap 版本**，且 go1.5–go1.19 的 floor 皆為「≥ Go 1.4」、而 **Go 1.4 可用純 C（gcc）從源碼編譯**，故設計最短路徑：

```
gcc ─► go1.4.3 ─► go1.12.17 ─► go1.17.13 ─► go1.20.14 ─► go1.22.12 ─► go1.24.6 ─► go1.27.0
     (C 編譯)   (floor ≥1.4)  (floor ≥1.4)  (floor ≥1.17.13) (floor ≥1.20.6) (floor ≥1.22.6) (floor ≥1.24.6)
```

- 每一級源碼自 codeload 取得（tag tarball），**就地編譯**（`GOROOT_BOOTSTRAP=上一級`），編譯完即驗證 `go version` 再進入下一級；
- 安裝到 `~/.ykc/go-<ver>`，最終 `~/.ykc/go → go-1.27.0`，與 `dev-setup.sh` / Makefile 的預期位置一致；
- 全程零 sudo、CGO_ENABLED=0。

> **建議**：把這條鏈沉澱為 `bootstrap-go.sh` 收進 repo（附 `make bootstrap-go` target），作為 go.dev 不可達環境（內網/受限 CI）的官方備援安裝路徑——這是本輪實測的附帶貢獻，對「任何裝置都能運行」的 Tier-0 主張是直接強化。

### 1.3 Rust 工具鏈：不可獲得（誠實記錄）

- rustup（sh.rustup.rs）、static.rust-lang.org、crates.io 全數封鎖；rust-lang/rust 的 GitHub release 無資產；
- 源碼自編 rustc 需從 crates.io 拉 bootstrap 依賴——同被封鎖；
- 結論：**本沙盒內 `make verify-all` 的 ③④⑨⑩ 與 ① 的 cargo 段無法實跑**；命令與預期輸出已全部核對文檔備妥（§3.3），待有完整網絡的機器一鍵複驗。
- 注意：`deploy/rust-toolchain.toml` 鎖 1.98.0、沙盒鏡像 `rust:1.98`、demo-rust-cli 之 Cargo.lock（clap 鎖版）均在位；**鎖版缺口只在 CI 的浮動 stable/rust-analyzer latest（S5）**。

---

## 2. 專案全盤理解（代碼層）

### 2.1 一句話定位

YKC 是一個**以 Go 編排的「可信裁判」系統**：把「AI 代理說它修好了」變成「環境證據說它修好了」——所有判決只由確定性機械比對（二進制 `--help`、cargo check/test 退出碼、檔案 hash、hash 鏈帳本）產生，零 LLM 參與裁決。

### 2.2 模組地圖（30 Go 檔、單 module `ykc`、零外部依賴）

| 命令 / 包 | 職責 | 狀態評估 |
|---|---|---|
| `cmd/ykc-smoke`（345 行） | T0–T3 分層煙測 + 反欺騙比對 + HMAC 簽名收據 | 核心資產，實證完整 |
| `cmd/ykc-judge`（judge/gate/verify） | L4 除錯閉環：cargo check→fix→check→收據+帳本；gate 供 pre-commit | 閉環完整；LLM 修復段留待 P2/P3（D12） |
| `cmd/ykc-guard`（verify/score/console/reset/mcp） | 聲明抽取→六類確定性比對→信任棘輪（T0–T3 只降不升）→人類 reset 唯一回升入口；最小 MCP server | 反欺騙核心，邏輯嚴謹（§3.2） |
| `cmd/ykc-atom`（snapshot/claim/smoke/replay/sync-ledger） | 原子監控 CLI：workspace 快照→事件庫→護欄判定→enforcement 落盤 | 功能齊備，但**是拉動式 CLI，非常駐監聽器**（§3.4） |
| `cmd/ykc-precompile` | cargo metadata/fetch/check/test-no-run/clippy 五階段沙盒預編譯 + 診斷解析 | 工程完成度高；fail-closed |
| `cmd/ykc-lsp` | Go↔rust-analyzer stdio（Content-Length 框定）實證 | T-09 命脈已通 |
| `cmd/ykc-panel` | Trust Console：唯讀觀察（/api/state、/api/raw）+ 人類控制（/api/jobs） | 雙表面職責分明；**安全缺口見 §4-S1** |
| `internal/ledger` | append-only JSONL + hash 串鏈（judge/guard/panel 共用，已消除 drift） | 核心基板，實作乾淨 |
| `internal/eventstore` | 每事件一個 JSON 檔 + 原子 rename（immutable outbox） | 正確 |
| `internal/eventledger` | eventstore→ledger 橋接 + 缺投影補償（SyncMissing） | 設計良好；O(n²) 補償見 §4-D3 |
| `internal/guardrail` | 行為驅動護欄策略（freshness 15min、fail-closed、critical→smoke takeover+block writes） | 邏輯嚴謹，測試覆蓋 |
| `internal/enforcement` | 判定落盤（state.json + AGENT_WRITES_BLOCKED 哨兵檔） | 簡單可審計 |
| `internal/monitor` | workspace 快照（sha256 per file）+ diff | 正確 |
| `internal/sandbox` | native/bwrap/docker(-runsc)/podman(-runsc) 抽象 + 能力探測 + fail-closed | 分層信任（strong/moderate/weak/none）表態誠實 |
| `internal/precompile` | 預編譯 pipeline + cargo JSON 診斷解析 | 完成度高 |
| `internal/smoke` | 可重用 smoke runner（atom/panel 用） | **與 cmd/ykc-smoke 並存的第二套引擎（§4-S2）** |
| `internal/rustutil` | Run/Sign/SHA256/Subcommands/PackageName 收斂 | 好 |
| `internal/atomicfile` | tmp+fsync+rename+dirsync 原子寫 | 好 |
| `internal/domain` | 事件信封/claim/diagnostic 型別 | 注意與 precompile 的 DiagnosticSummary 重名斷層（§4-S3） |
| `core/interfaces.go` | 五層窄介面 + Core + Executor 骨架 | **無實作、無使用者（§3.4）** |

### 2.3 關鍵資料流（已實證理解）

```
代理聲明(claims.json / MCP) ──► 確定性比對器(verify.go)
        │                          │ 三態 verdict + 嚴重度(1誇大 2隱瞞 3欺騙 4偽造)
        ▼                          ▼
   信任棘輪(ratchet.go) ◄── 行為分=嚴重度×意圖；只降不升；偽造首擊即 T0
        │                          │
        ▼                          ▼
 .ykc/ledger.jsonl（hash 鏈）  ←  收據(receipt.json, HMAC) ← judge/smoke
        │
        ▼
 ykc-panel /api/state（人類紅黃綠 + AI 機器可讀）
```

**雙事實來源現狀**：舊 `ledger.jsonl`（judge/guard 寫）與新 `eventstore/events`（atom/precompile 寫，經 bridge 投影回 ledger）。YKC_08 已建橋、YKC_09 已可視（事件/投影/缺投影）；**但 guard 的 score 路徑仍直寫舊 ledger，未走 bridge**（YKC_07 結論中自認的下一步，至今未做，見 §4-D4）。

### 2.4 我對架構的判斷（獨立於文檔）

- **正確的核心決策**：
  1. 「真相由環境產生」落到工程 = 裁決路徑零 LLM、全部退出碼/hash/檔案系統事實——這是專案真正的護城河，且已實作而非口號；
  2. 帳本 hash 算法集中於 `internal/ledger`（消除跨模組 drift）——健檢報告 YKC_03 的 C1 修對了一個真正危險的債；
  3. 沙盒 fail-closed + 信任分級表態誠實（native=trust none 必須明示開關）——沒有過度宣稱；
  4. 零外部 Go 依賴 → 單 module、CGO_ENABLED=0 靜態二進制、任何裝置可跑——Tier-0 主張成立。
- **與文檔的落差**（§3.4 詳述）：五層窄介面（core）、進程內事件匯流排（Bus）、Executor 統一外殼——**都是文件與骨架，未接線**。目前實作是「六個 CLI 共享 internal 包」的聯合體，不是 YKC_02 所畫的模組化單體。這不算缺陷（CLI 聯合體更簡單可靠），但**文檔應如實降級表述**，否則 P3 接線時會產生預期落差。

---

## 3. 運行驗證結果

### 3.1 Go 側（本沙盒實跑，go1.27.0 bootstrap 鏈產物）

```
$ go version
go1.27.0 linux/amd64
$ go vet ./...        → 通過（0 發現）
$ go test ./...       → 6/6 測試包全過
ok  ykc/internal/eventledger   0.011s
ok  ykc/internal/eventstore    0.005s
ok  ykc/internal/guardrail     0.017s
ok  ykc/internal/monitor       0.002s
ok  ykc/internal/precompile    0.004s
ok  ykc/internal/sandbox       0.002s
$ make binaries         → 7/7 二進制建立（ykc 3.3MB … ykc-panel 10.1MB，靜態）
```

**無 cargo 环境下仍可實跑的閉環（全部實測通過）：**

| 驗證 | 結果 |
|---|---|
| `ykc-atom snapshot`（demo-rust-cli） | ✅ 快照 + 事件 + 帳本投影落地 |
| `ykc-atom claim -kind tests_passed`（撒謊代理、零證據）| ✅ `mode=evidence_only_smoke_takeover`、`block_writes=true`、violation=`fake_test_claim critical`、三動作（smoke_takeover / block_agent_writes / require_evidence）|
| smoke takeover 後 | ✅ `smoke_failed_blocked` + `AGENT_WRITES_BLOCKED` 哨兵落盤（fail-closed 如設計）|
| `ykc-judge -verify` | ✅ 帳本完整（4 條事實，hash 鏈無斷裂）|
| **獨立 Python 重算全鏈 hash**（跨實作交叉驗證 Go 簽發值）| ✅ 4/4 一致 |
| `ykc-guard verify`（file:/function: 聲明）| ✅ 2 verified / 2 contradicted（severity 2 隱瞞），判定與檔案系統事實完全一致 |
| `ykc-guard mcp`（initialize / tools/list / tools/call）| ✅ 三工具（ykc.check / verify_claims / trust_status）全通；專案附帶 `test-mcp-client.py` 亦通過（cargo 缺失時誠實報錯不崩潰）|
| `ykc-precompile` 無沙盒 | ✅ fail-closed：`overall=unsupported`（sandbox_required + native_not_isolated）|
| `ykc-precompile -allow-native` | ✅ 如實執行並報告 `cargo not found`（環境限制，非缺陷）|
| `ykc-panel` 觀察面（/healthz /api/state /api/projects /api/raw）| ✅ integrity=verified、facts=4、events=4、projections=4、missing=0、tampered=0 |
| `ykc-panel` 控制面（POST /api/jobs → 實時日誌）| ✅ verify 任務 done exit=0，日誌「帳本完整」|
| **S1 實證** | ✅ `POST /api/jobs {"project":"/etc"}` 被接受並執行——任意路徑向量成立（§4-S1）|

**Rust 依賴步驟（本沙盒無法實跑，命令已備妥 §3.3）：** 煙測 ①②、judge 除錯閉環 ③④、LSP ⑨、預編譯 ⑩ 之 cargo 段。

### 3.2 沙盒限制下的替代驗證

- 帳本 hash 鏈：以 Go 重算 `factHash` 定義與 `VerifyChain` 邏輯做交叉複核（§4-L1/L2 記錄了兩處需補強的邊界）；
- 棘輪邏輯：對 `applyRatchet` 全路徑（sev 1–4 × from T0–T3）做紙上窮舉，無越權回升路徑；
- 沙盒命令生成：bwrap/docker 參數矩陣逐項複核（`--user uid:gid`、`--network=none`、CARGO_NET_OFFLINE 注入皆在位）。

### 3.3 待有網絡機器一鍵複驗的命令（已核對 Makefile）

```bash
make verify-all   # 10 項；②⑦ 的 FAIL/TAKEOVER 為反欺騙預期行為
make panel        # 面板實測
```

---

## 4. 獨立代碼審查發現（新於 YKC_03 / YKC_07）

### S 級（安全/正確性，建議本週處理）

| # | 位置 | 問題 | 風險 | 建議修復 |
|---|---|---|---|---|
| **S1** | `cmd/ykc-panel` | **預設綁 `:8080`（全部介面）且無任何認證**；`POST /api/jobs` 的 `project` 參數**未對照 `discoverCargoProjects` 白名單**，任意路徑直接成為 `cmd.Dir` 與 ykc 二進制的 `-dir` 目標。LAN 內任何主機可觸發 ykc 在攻擊者目錄執行 cargo（build.rs → 任意代碼執行）、讀任意 claims 檔。README 僅宣稱「本機啟動器綁 127.0.0.1」，但 `make panel` 預設是全介面。 | 中高 | ① `project` 必須 ∈ discoverCargoProjects（base 或 abs 比對，rawHandler 已有此模式，jobs 沒有）；② 預設綁 127.0.0.1，`-addr` 顯式才全介面；③ 中長期加 token 認證（env 注入）|
| **S2** | `cmd/ykc-smoke` vs `internal/smoke` | **兩套煙測引擎並存**：ykc-smoke 自含 345 行（T0–T3 + claims + 收據）；internal/smoke 是另一套 CommandSpec runner（atom/panel 用）。兩者對「成功」的定義、超時、輸出處理各寫一份，必然 drift。YKC_07 已列為中期債但未合。 | 中 | ykc-smoke 的檢查層改組為 internal/smoke 的 CommandSpec 實例；收據/claims 留 CLI 層。2–3 天 |
| **S3** | `internal/domain` vs `internal/precompile` | **兩個同名異構 `DiagnosticSummary`**：domain 版（ErrorCount/WarningCount/BuildBlockingCount）是 guardrail 解碼 `diagnostic.summary` 事件所用的型別；precompile 版（Errors/Warnings/Messages）是實際產出診斷的型別。目前**尚無任何代碼寫出 `diagnostic.summary` 事件**，故未爆；一旦把 precompile 診斷投影進事件流，field 名對不上 → `HasBlockingErrors()` 恆 false → **no_errors 護欄靜默失效（fail-open）**。 | 中高（潛伏） | 立即統一：precompile 改輸出 domain.DiagnosticSummary（或加轉換器），並補一個「precompile 診斷→事件→guardrail 判定」的集成測試守住 |
| **S4** | `internal/ledger.Open` | 開啟帳本時**只信任最後一行的 hash 作為鏈頭，不驗證全鏈**。攻擊者（或磁碟損毀）截斷+重簽最後一條即可讓新寫入接續偽鏈；`VerifyChain` 存在但只被 `-verify`/panel 調用，append 路徑從不調。 | 中 | `Open` 增加 `OpenVerified`（或 Open 時校驗末 N 條 + 提供 strict 選項）；judge/guard 的開帳本路徑改走校驗版 |
| **S5** | `.github/workflows/ci.yml` | CI 的 Rust 用 `dtolnay/rust-toolchain@stable`（**浮動**）、rust-analyzer 用 `latest` 下載（**浮動**）——與 `deploy/rust-toolchain.toml` 鎖 1.98.0、「位元組一致」主張直接矛盾；stable 一升級，CI 結果可能與鎖版鏡像分岔。 | 中 | 釘 `toolchain: "1.98.0"`、rust-analyzer 釘具體版本號 |
| **S6** | `.github/workflows/ci.yml` release job | release 資產只建 5 個二進制（ykc/judge/guard/lsp/panel），**漏 `ykc-atom` 與 `ykc-precompile`**——下載 release 包的用家，面板「預編譯/同步帳本」兩鍵必然報「找不到可執行檔」。`make binaries` 建 7 個，release 路徑與之不一致。 | 中 | release job 改用 `make binaries` 後打包 `dist/ykc/`（單一來源）|

### D 級（債務/健壯性）

| # | 位置 | 問題 | 建議 |
|---|---|---|---|
| D1 | `internal/ledger` | 無檔案鎖（flock）：judge 與 guard 同時對同一專案 score 會交錯寫。目前是「單一寫者」設計假設，但未強制。 | 以 flock/lockfile 強制單寫者；違反時 fail 而非交錯 |
| D2 | `internal/ledger` | `bufio.Scanner` 1MB 上限：payload>1MB（如完整 cargo JSON）時 Open/Verify 靜默截斷 | 換 bufio.Reader 逐行讀，或提高上限並顯式報錯 |
| D3 | `internal/eventledger.projectIfMissing` | 每個事件都 `ReadAll` 整個 ledger（O(n²) 補償）；且 TOCTOU 下並發可重複投影 | SyncMissing 一次讀全表建索引；投影前用 ledger 單寫者鎖 |
| D4 | `cmd/ykc-guard` score | 直寫舊 ledger，未走 eventledger bridge（YKC_07 自認的下一步未做）→ 雙來源 drift 持續 | guard 的 appendFact 切到 bridge（或至少事件化） |
| D5 | `internal/smoke` + `internal/sandbox` | tailBuffer 完整重複兩份（hash+截斷邏輯） | 收斂到 internal 公用包（如 rustutil 或新 internal/tail） |
| D6 | `cmd/ykc-panel/main.go` | `itoa()` 用 json.Marshal 繞路 | strconv.Itoa（一行） |
| D7 | `cmd/ykc-panel/state.go` | `/api/state` 每 poll 全量 ReadAll+VerifyChain 每個專案 | 大專案會 O(n)×頻度；加 ETag/seq 增量（panel 已有 fact seq，可行）|
| D8 | `cmd/ykc-panel/state.go` | `discoverProjects` 只掃一層子目錄 | 文檔化限制或可配置深度 |
| D9 | `cmd/ykc-lsp` | pipe 錯誤未檢查（`stdin, _ :=`）；readMessage 死等 | 低風險展示用 CLI，補 err 檢查即可 |
| D10 | demo-semantic-cli / demo-broken-cli | 無外部依賴故無 Cargo.lock（無傷大雅）；**但 CI 的 rust 浮動（S5）才是鎖版缺口**。複核更正：demo-rust-cli 的 Cargo.lock 已入庫（clap 鎖版在位），原「無 lock」判斷不成立 | 僅隨 S5 處理 |

### 已知債務複核（YKC_03 遺留 R1–R5 現狀確認）

- R2（subcommands 解析 clap help 人類文本）：**仍為最高優先級功能債**——這是反欺騙比對的输入端，clap 改版即破。建議 P3 首項：改讀 `--format=json`（clap 4 的 `command::from_cargo`/derive 無原生 JSON，可自加 `--ykc-manifest` 副命令，或在 L2 rust-analyzer 就位前以快照回歸測試鎖定現格式）。
- R3（hasFunction regex）、R4（MCP 最小實作）、R5（probe 覆蓋）：維持觀察。
- R1（ledger→bbolt）：P3 合理。

---

## 5. 風險登記（按 機率×影響 排序）

| # | 風險 | 機率 | 影響 | 緩解 |
|---|---|---|---|---|
| RK1 | 面板 LAN 暴露（S1） | 中 | 高 | 本週修（1 天）|
| RK2 | 雙煙測引擎 drift（S2） | 高（時間函數） | 中 | P3 第一項 |
| RK3 | diagnostic 事件斷層導致護欄 fail-open（S3） | 中（接線時必遇） | 高 | 接線前先統一 schema + 集成測試 |
| RK4 | clap help 格式變化破 subcommand 比對（R2） | 低中 | 高 | 格式快照回歸測試（現在就做，成本半天）|
| RK5 | Rust 工具鏈鎖版缺失（無 Cargo.lock、CI rust/rust-analyzer 浮動——S5） | 高 | 中 | 補 lock + CI 釘 1.98.0 + rust-analyzer 釘版 |
| RK6 | LLM 修復段缺席使「除錯閉環」停在機械層 | 確定 | 中 | P3 排程（先模板庫、後 LLM，成本序）|
| RK7 | 單寫者假設（D1）在多 CLI 併發下破 | 低中 | 中 | flock |
| RK8 | 文檔超前實作（core 骨架未接線）造成團隊預期錯位 | 高 | 低 | 更新 YKC_00 A.3/YKC_02 標記「骨架，未接線」|
| RK9 | release 包缺 atom/precompile → 面板功能半殘（S6） | 高（每次發版） | 低中 | 本週隨 H6 修 |

---

## 6. 運行驗證摘要

完整輸出已併入 §3.1。一句話：**Go 側全綠（vet/test/build/7 二進制 + 反欺騙閉環 + 帳本跨實作驗證 + 面板雙表面 + MCP + fail-closed 預編譯）；Rust 側因沙盒網絡限制標記待複驗，且已證實限制範圍恰為 cargo 可執行性，YKC 各命令在 cargo 缺失時皆誠實降級、不崩潰、不偽造結果。**

---

## 7. 後續開發路線圖（P3 細化，含工時估算）

> 原則維持 YKC_00-E：trunk-based、dogfooding、合入附自簽 receipt、不可信代碼必走沙盒。

### 7.1 第 1 週：加固週（把已實作部分焊死）

| 項 | 內容 | 工時 |
|---|---|---|
| H1 | S1 面板加固（白名單 project、預設 127.0.0.1、可選 token）| 0.5–1 天 |
| H2 | S4 ledger.Open 校驗 + D1 flock + D2 行長 | 1 天 |
| H3 | S3 schema 統一 + 集成測試（precompile→event→guardrail 全鏈）| 0.5–1 天 |
| H4 | R2 格式快照回歸測試（subcommands/claims 抽取）+ D10 Cargo.lock | 0.5 天 |
| H5 | `bootstrap-go.sh` 收進 repo（§1.2 附帶貢獻）+ Makefile target | 0.5 天 |
| H6 | CI 修補（既有 `.github/workflows/ci.yml` 已具 gofmt/vet/build/verify-all/release 骨架，品質合格；修兩處）：① `dtolnay/rust-toolchain@stable` 改釘 `1.98.0`、rust-analyzer 下載改釘版本（否則 CI 與「鎖版 1.98」主張矛盾，見 S5）；② release job 補 `ykc-atom`、`ykc-precompile` 兩個二進制（見 S6）| 0.5 天 |

### 7.2 第 2–3 週：合一週（消 debt、補常駐）

| 項 | 內容 | 工時 |
|---|---|---|
| M1 | S2 煙測引擎合一（internal/smoke 為唯一執行核，ykc-smoke 收據層保留）| 2–3 天 |
| M2 | atom 常駐化：`ykc-atom daemon`（fsnotify + 快照週期 + 護欄判定 + enforcement），取代拉動式 CLI 作為 L3 主體；CLI 保留為單發工具 | 3–4 天 |
| M3 | D4 guard 走 bridge；D3 O(n²) 修；D5 tailBuffer 收斂 | 1–2 天 |
| M4 | L1 依賴對齊 MVP：包 cargo-audit + cargo-deny（子行程 + JSON 解析 + block/warn/suggest 三級），接事件流 | 2–4 天 |

### 7.3 第 4 週起：價值週（P3 主軸 + 指標）

| 項 | 內容 |
|---|---|
| V1 | T0 全面接管編排器：trust=T0 時代理寫入經 YKC 代理（patch 審核流程），而非只 block 哨兵檔 |
| V2 | 可證偽指標基線（深度分析報告 §6.1 三層 SLA）：建 rustlings + 自建 broken→fixed 基準集，出「有/無 YKC compile-pass 對照」與錯誤消除曲線——這是對外說服力的根 |
| V3 | L2 結構統計 MVP（tree-sitter 語法層先行；SCIP/bbolt 後置）|
| V4 | MCP 換 mark3labs/mcp-go（R4）+ tools 擴充（smoke/judge/guard 全暴露）|
| V5 | L5 解釋器 PoC（-Znll-facts→節點圖，先解釋不判定，YKC_04 路線）|

### 7.4 驗收標準（每週 dogfooding 公開）

- YKC 自身 `make verify-all` 全綠 + 自簽 receipt；
- 新合入必附：相關單元測試 + 帳本/事件流斷言；
- 指標儀表：compile-pass 率、機械修復輪次、謊報攔截率（用 demo-agent-lying 回歸集）。

---

## 8. 給技術負責人的一句話

> **YKC 的價值心臟（確定性反欺騙）已經跳動，且跳得乾淨。** 當前最大的風險不在技術深處，而在「已實作的邊界」（面板暴露、雙引擎 drift、schema 斷層）與「文檔超前實作的落差」。把 §7.1 的加固週做完，專案即具備對外演示與 dogfooding 的資格；其後依 7.2→7.3 推進，P3（接管+L1/L2）可在 4 週內落地。

---

## 附錄 A：本輪環境操作記錄

- `git status`/`git log`：branch `arena/01a02698-kyc`，基線 `302fdf6`（feat: add guarded precompile and working status panel），起始工作樹乾淨；
- Go 1.27.0：bootstrap 鏈六級自編（§1.2），**全鏈 18 分鐘**（23:23:37→23:41:45：1.4.3=31s、1.12.17=1m49s、1.17.13=2m06s、1.20.14=2m32s、1.22.12=3m10s、1.24.6=3m45s、1.27.0=4m15s，2C/3GB 機），安裝於 `~/.ykc/go-1.27.0`（`~/.ykc/go` 符號連結）；
- Rust：不可獲得（§1.3），未做任何偽裝；
- 全部工具鏈/暫存置於 `~/.ykc` 與 `/tmp/goboot`，**未污染 repo 工作樹**（運行產物 `**/.ykc/`、`bin/` 均已被 .gitignore 覆蓋，實測 `git check-ignore` 確認）；
- 本輪唯一入庫新檔：本報告。

## 附錄 B：代碼審視覆蓋清單

30/30 Go 檔逐檔通讀：cmd 7 命令（smoke 345、judge 206+145、guard 178+194+118+127+137+~180、lsp 174、atom 268、precompile ~90、panel 202+290+196）、core 141、internal 12 包（ledger 166、rustutil 93、eventstore 109、eventledger 211、monitor 208、guardrail 337、enforcement 80、precompile 438、sandbox 419、smoke 147、atomicfile 72、domain 122）＋ 6 個 _test.go；18 份設計文件全讀；Makefile/QUICKSTART/dev-setup.sh/.github/workflows/ci.yml/deploy 全讀。
