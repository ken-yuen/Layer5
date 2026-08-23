# YKC_14 — 常駐進程（ykc serve）與宣告式護欄（Datalog）報告

**日期**：2026-08-22 ｜ **對應任務**：T-25（serve）、T-26（datalog 護欄）
**一句話**：把 atom+judge+guard+panel 的**運行時監督面**合併為單一 `ykc serve` 常駐進程（inotify 事件流 + 去抖 + 帳本序列化），並把護欄的違規判定從硬編碼 switch **遷移為 Datalog 規則（規則即數據）**——零外部依賴不變。

---

## 1. 交付清單

| 新增 | 內容 | 行數約 |
|---|---|---|
| `internal/datalog/` | 迷你 Datalog 引擎：分層否定、neq 內建、dotted 謂詞（claim.kind 風格）、跨行規則、安全檢查（頭部/否定變數必綁定）、非分層程式顯式報錯、確定性輸出（規則宣告序）、求值保險絲（MaxDerivedFacts/MaxRounds） | ~700 |
| `internal/guardrail/rules.go` | 預設規則文本 `defaultRulesDL`（7 種違規碼逐條對應舊 switch）＋ `LoadRules`（.dl 檔/目錄附加集） | ~130 |
| `internal/guardrail/dl.go` | 事實抽取器（時間/epoch/新鮮度→ground facts）＋ `EvaluateClaimWithRules`（附加規則入口）＋ violation/5→Violation 映射（規則宣告序穩定排序） | ~180 |
| `internal/watch/` | 監看層：Linux inotify（stdlib syscall，零依賴）/ 他平台 stat 輪詢；遞迴＋新目錄動態加 watch＋競態補掃；去抖器（最後操作勝、create+remove 抵銷、remove+create→write）；過濾器（.rs/.toml/.lock；**必排 .ykc 防回環**） | ~700 |
| `internal/panel/` | panel 由 cmd/ykc-atom 同級之 cmd/ykc-panel **提升為 internal 套件**（ykc-panel 與 ykc-serve 共用唯一實作；BuildMux/BuildMuxWith/Options/Authed/WriteJSON/EnsureToolchainPath 匯出；JobManager 匯出 ValidateProject/ValidateClaims） | 遷移+改造 |
| `internal/serve/` | 常駐核心：專案發現→bridges→watcher→panel mux＋`/api/claims`、`/api/watch`、`/api/rules`；事件循環（批次→按專案分組→路徑排序→`file.change` 入帳本）；主動 Rust 預譯（啟動全掃、變更重跑、single-flight/coalescing、report/ledger 投影）；ErrLocked 指數退避重試（與 judge 子行程共用帳本）；`-auto-judge` 單飛觸發；SIGTERM 優雅退出 | ~800 |
| `cmd/ykc-serve/` | 薄殼（旗標解析＋signal.NotifyContext） | ~100 |
| `domain` | 新事件種類 `file.change`（FileChangeBatch payload；附加式，不破壞既有事件流） | +20 |

**修改**：`guardrail/policy.go` 的 `EvaluateClaim` 改為 `EvaluateClaimWithRules(..., nil)` 薄包裝（**違規判定唯一實作＝規則文本**，無第二套邏輯）；Makefile（serve 目標、binaries、verify-all ⑪ serve 冒煙）；ci.yml release 增 ykc-serve；README/YKC_00 進度表。

## 2. 設計決策（記錄理由）

| # | 決策 | 為何 |
|---|---|---|
| D1 | **合併「運行時」，不合併「功能」**：四個 CLI 全部保留 | git 閘門（pre-commit）、MCP（stdio）、一次性除錯的場景本來就是短命進程；serve 常駐的是「監督面」。ykc-guard 的 MCP 維持獨立進程——stdio 協定不屬於 HTTP 常駐 |
| D2 | **inotify 用 stdlib syscall，不引入 fsnotify** | 裁判核心零依賴紀律（go.mod 仍零外部依賴）；fsyntax 底層同為 inotify；他平台退 stat 輪詢（mtime+size 差分，250ms） |
| D3 | **去抖語義「最後操作勝」**；create+remove 抵銷；remove+create→write | os.WriteFile 即產生 CREATE+MODIFY——「最後操作」才對帳本有意義；語義經測試鎖定 |
| D4 | **.ykc/target/.git 必排** | 防自身寫帳本→監看→再寫帳本的**事件回環**；target 防編譯產物噪音 |
| D5 | **時間/epoch 留 Go，違規判定進規則** | datalog 不做時間運算（業界慣例）；「新鮮度、同 epoch、變更先後」是事實計算，`violation(...)` 是政策——政策變更（用家 -rules）不再需要改 Go 代碼 |
| D6 | **附加集而非取代集**（-rules 只能增補） | 護欄是裁判底線：用家規則可加嚴/加新違規碼，不可削弱預設規則（防「代理說服用家放行」） |
| D7 | **規則求值失敗＝fail-closed 接管** | 裁判故障與解碼故障同等對待：BlockWrites+RunSmoke（`evaluation_failed_fail_closed`） |
| D8 | **批次按路徑排序後入帳本** | 帳本事實與事件到達順序無關——可重放對賬（deterministic receipt） |
| D9 | **ErrLocked 指數退避（6 次，40ms→640ms）** | serve 與 judge 子行程共用 ledger.jsonl（flock）；短暫爭鎖是常態不是錯誤 |
| D10 | **`run_smoke` 須顯式請求** | 聲明被駁回時自動跑 cargo 會令「控制權在人」失效；預設只落盤接管狀態，人（或上位编排）決定何時執行煙測 |

## 3. 護欄遷移：語義等價驗證

- **原 policy_test 4 條測試未改一行，在 datalog 引擎下全綠**（fake_test_claim 接管／新鮮證據放行／no_errors 阻斷駁回／後到 smoke 成功覆蓋前次 test 失敗）——這是遷移正確性的主證據。
- 新增 7 條遷移專屬測試：diagnostics_regression（證據前後序）、未知種類→high（不接管）、附加規則注入、LoadRules 檔案/目錄、非分層附加規則→fail-closed、過期證據（!evidence.latest 而非 latest_failed 變體）。
- 違規輸出順序＝規則宣告序（claim 類→regression），與舊 switch 順序一致。

## 4. 驗收（全部可複現）

```
gofmt -l cmd internal core          # 空
go vet ./...                         # 通過
staticcheck ./...                    # 0 告警（清除遷移死碼 4 處）
go test ./... -count=1               # 15 包全綠（datalog 11、watch 8、guardrail 11、serve 7、panel 3…）
make verify-all（CI 同款）⑪          # serve 冒煙：healthz=ok；無證據 tests_passed → fake_test_claim 駁回；/api/watch 含 backend；SIGTERM 優雅退出
```

**真實 daemon 實測**（2026-08-22，本沙盒 Linux/inotify）：
- 3 專案自動發現，backend=inotify，debounce=300ms；
- `POST /api/claims`（無證據 tests_passed）→ `fake_test_claim`、`evidence_only_smoke_takeover`、`AGENT_WRITES_BLOCKED` 落盤；帳本鏈完整（claim+decision 兩事實）；
- 即時監看：兩個 .rs 寫入 → 去抖合併 → 帳本 `event.file.change [('src/main.rs','write'), ('src/serve_probe.rs','write')]`（路徑排序、create+modify 正確歸併 write）；
- `kill -TERM` → 優雅退出（進程消失，無殭屍）。

## 5. 已知限制（誠實清單）

1. **rename 在輪詢後端**以 remove+create 近似（inotify 後端有真 rename，含 cookie 配對）。
2. **事件通道背壓**：消費端停滯時新批次丟棄（lastBatch 仍可觀察）；不重排不阻塞後端——監看是監督不是審計（審計在帳本）。
3. **inotify watch 上限**（ENOSPC）顯式報錯並提示調 `fs.inotify.max_user_watches` 或改用輪詢。
4. **附加規則可寫出永不出發的規則**（引用不存在的謂詞）——求值安全但不提醒；後續可在 /api/rules 加 dry-run 觸發率統計。
5. **watch.DefaultExcludeDirs 與 monitor.DefaultExcludeDirs 是兩份宣告**（避免包循環的取捨）；兩處修改須同步（本報告即為變更記錄錨點）。

## 6. 對 P3 路線的影響

- serve 已是「L1 依賴預警」與「L2 結構統計」的天然宿主：兩者的產出（cargo-audit/deny 結果、AST 統計）可直接作為 datalog 事實注入同一規則引擎（如 `dep.unaudited(P)` → violation）。
- 下一步（建議序）：① 附加規則目錄隨倉庫提供示例（examples/rules/）；② watch 增量快照 digest（與 monitor.Snapshot 對賬）；③ `-auto-gate`（變更後跑 gate 而非 judge）。
