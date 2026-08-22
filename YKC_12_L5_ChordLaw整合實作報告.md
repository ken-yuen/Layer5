# YKC_12 — L5 借用幾何解釋：ChordLaw 整合實作報告

**日期**：2026-08-22 ｜ **對應任務**：T-18a ✅ ｜ **對應決策**：D22（L5 雙軌）

---

## 1. 問題與構想

borrow/生命週期錯誤是 LLM 代理最難修的一類（YKC_04 §5：「LLM 睇唔見生命週期圖」）。本輪把 vendored ChordLaw（Datalog 借用檢查器，26/26 rustc 1.98.0 oracle 一致）開發成 YKC 的 L5 可視化層，核心修正一句話：

> **SVG 給人看，幾何給代理讀。**

LLM 代理的「可視化」不是圖片，而是機器可讀的幾何：文字拓撲（token 便宜、直接進 prompt）、區間代數事實（可組合推理）、封閉修法菜單（開放式除錯 → 選擇題）。

## 2. 交付清單

| 交付 | 位置 | 說明 |
|---|---|---|
| vendored 引擎 | `l5/chordlaw/` | ChordLaw v0.3 @ Layer5 `3b93854`；2 個本地補丁（`--no-svg`、JSON `schema` 欄位）；MPL-2.0；見 VENDOR.md |
| L5 接線包 | `internal/borrow/` | types/analyzer/topo/rulecard/reduce ＋ 3 個測試檔（36 測試） |
| 文字拓撲 | `topo.go` | ASCII 區間圖（#=mut =sh X=錯誤點）＋區間代數（OVERLAP/DISJOINT/path_conflict）＋幾何修法推導 |
| 衝突圖 | `topo.go` | 節點=借用、紅邊=違法重疊；JSON＋DOT 雙輸出；**RedEdges()==0 = 幾何收斂判據** |
| 幾何規則卡 | `rulecard.go` | 兩條法則（紅弧孤立/弧在圓內）＋ E01–E10 ↔ rustc 碼 ↔ 修法菜單；純靜態，無 python 也可用 |
| 模板歸約 | `reduce.go` | 8 類 borrow rustc 碼 → 內嵌 canonical `.cl` 樣例（每個經引擎 golden 驗證） |
| judge 掛鉤 | `cmd/ykc-judge/l5.go` | 剩餘錯誤含 borrow 碼 → 打印「📐 L5 借用幾何解釋」節；帳本追加 `borrow.analysis` 事實（含解釋 sha256——解釋可審計）；有界（≤3 碼） |
| MCP 工具 ×2 | `cmd/ykc-guard/mcp.go` | `ykc.borrow_rules`（規則卡）、`ykc.borrow_explain`（.cl→拓撲；輸入 ≤8KB、寫入固定於 `<dir>/.ykc/l5/`） |
| 構建/CI | `Makefile`、`ci.yml` | `make l5-test`（19 回歸）、`make borrow-test`；CI 加 `go test ./...`＋l5-test |
| 文檔 | README、YKC_00 | L5 節；T-18 拆 a/b；決策 +D22；版次 v10.0 |

## 3. 紀律遵守（對齊既有決策）

| 紀律 | 落實 |
|---|---|
| 判定權不轉移（D2/第一性原理） | 一切 L5 輸出標明「解釋非判定, 判定以 rustc 為準」；不觸碰 receipt.Overall |
| 事實帳本為基板 | `borrow.analysis` 事實含 codes/l5_available/explanation_sha256/engine——解釋不可事後竄改 |
| 邊界加固慣例 | cl_source ≤8KB；臨時檔固定路徑 `<dir>/.ykc/l5/input.cl`；子行程 20s 超時；schema 版本校驗（vendor 漂移即拒） |
| T0 零依賴承諾 | L5 是可選能力：`Analyzer.Available()` 如實申報（仿 sandbox.Capabilities）；python3 缺席時 judge 降級為純規則卡、MCP 工具如實報錯、E2E 測試自動 Skip |
| 誠實近似（新, 決策 B2） | 區間代數按線性化近似計算；`Illegal` 只在「重疊∧含mut∧路徑衝突∧**引擎錯誤佐證**」時成立；無佐證降級 `Suspect`（存疑），不誤導代理 |

## 4. 實測記錄（全部通過）

| 驗收 | 結果 |
|---|---|
| vendored 19 項回歸 | OK（含 --no-svg 補丁後） |
| `go vet ./...` / `go build ./...` | 乾淨 |
| `go test ./...` | 全綠（internal/borrow 36 測試：17 範例 golden＋8 模板 golden＋純 Go 單測） |
| judge E2E（自建 E0502 專案, 真 cargo） | 打印規則卡＋ex1 拓撲；帳本出現 `borrow.analysis`；`-verify` 鏈完整 |
| 非 borrow 錯誤（E0425, demo-semantic-cli） | 不觸發 L5（正確） |
| MCP 5 工具 | tools/list 齊；原 3 工具不回歸（test-mcp-client.py 通過）；新 2 工具輸出正確 |

judge 對 E0502 的實際輸出（節選）：

```
--- 📐 L5 借用幾何解釋（ChordLaw; 解釋非判定）---
   [E01] 紅弧交越  (法則①; ~ rustc E0499/E0502)
     幾何: 路徑衝突的兩借用區間重疊, 且至少一者為 mut
     修法:
       1. 縮短先前借用的區間: 把它的最後使用移到新借用誕生之前（NLL 順序化）
       2. 延後新借用: 移到先前借用死亡之後
       3. 拆分路徑（split borrow）: 若觸及不同字段, 借 x.f 與 x.g 而非整體 x
   ...
   借用區間拓撲 (軸=陳述序; #=mut 排他, ==sh 共享, X=錯誤點):
                 s2    s3    s4    s5    s6
   a: &x         ============X======(end)
   b: &mut x                 X############(end)
   區間代數事實:
     a vs b: OVERLAP[s4..s5]  ← 違法
   幾何修法: [E01 @ s4] 縮短 a 的區間: 把 a 的最後使用（現於 s5）移到 s4 之前
```

## 5. P2 增量（2026-08-22 第二輪, v0.0.2）

| 交付 | 位置 | 說明 |
|---|---|---|
| **真實 Rust→.cl 歸約** | `internal/borrow/extract.go` | 行級啟發式翻譯錯誤現場 fn；**驗證式**：歸約產物必須經引擎驗證（錯誤碼落在該 rustc 碼的幾何族 `ChordFamily` 內）才展示，否則回退 canonical 模板——寧缺勿錯；附 `sN ← file:line` 對照把拓撲錨回用家源碼；`E0502/E0506/E0382/E0505` 四類端到端測試通過 |
| **L5 報告落盤** | `internal/borrow/persist.go` | `.ykc/l5/report.json`（judge 寫、panel 讀的唯一實作）；含紅邊數/歸約命中數/衝突圖/解釋 sha256（對賬帳本）；borrow 錯誤全修好時自動清除 |
| **panel L5 視圖** | `cmd/ykc-panel` | 專案卡顯示「L5 紅邊」（0=幾何收斂, 綠；>0 紅）＋錯誤碼＋現場歸約標記；`<details>` 展開完整幾何解釋；`/api/state` 帶 `l5` 節點（機器可讀）；L5 report mtime 入觀察端快取鍵（防陳舊） |
| **MCP 紅邊輸出** | `cmd/ykc-guard/mcp.go` | `ykc.borrow_explain` 回應加「紅邊(違法重疊): N — 修復收斂判據: 紅邊清零」 |
| **帳本欄位擴充** | `borrow.analysis` 事實 | 加 `reduced`（真實歸約命中數）與 `red_edges`——未來量測的基礎數據 |

### 第二輪審計（找錯/減債/去重）

| 類 | 項 | 處置 |
|---|---|---|
| 重 | `internal/borrow.firstLine` 重複 `rustutil.FirstLine` | 刪除, 統一用 rustutil ✅ |
| 重 | `topo.go` 自定義 `max/min` 重複 Go 1.21+ 內建 | 刪除 ✅ |
| 重 | judge `l5Report` 與 panel `L5View` 結構手工同步（漂移隱患） | 抽出 `internal/borrow/persist.go` 唯一實作 ✅ |
| 錯 | panel 觀察端快取鍵不含 L5 report mtime——`RemoveReport` 不寫帳本, 舊紅邊會殘留 | `stateKey` 加 `l5ModTime` ✅ |
| 錯 | 修好 borrow 錯誤後 `.ykc/l5/report.json` 陳舊殘留 | `l5Explain` 無條件調用＋空時 `RemoveReport` ✅（E2E 驗證） |
| 債 | `rustNoise["self"]=false` 無意義項 | 刪除 ✅ |
| 債 | CI release 缺 `ykc-atom`/`ykc-precompile` 二進制與 `l5/chordlaw` 引擎（下載包 L5 幾何啞火） | release 打包補齊 ✅ |
| 驗證 | staticcheck 全倉 | 0 告警 ✅ |
| 驗證 | vendored 19 回歸＋26/26 rustc oracle（本機 rustc 1.98.0） | 全綠 ✅ |

## 6. 已知限制與後續（如實申報）

| 限制 | 出路 |
|---|---|
| Rust→.cl 歸約是行級啟發法（`match`/closure/else 即放棄回退模板），非 AST 級 | P3：syn AST 前端；驗證式框架（ChordFamily 判據）已就緒, 換前端不動出口 |
| 區間代數對迴圈/分支是線性近似（已以 Suspect 降級語氣） | T-18b：-Znll-facts 官方事實路線（控制流精確） |
| panel 衝突圖以 JSON 展示, 未做 SVG 圖形渲染 | 後續：dashboard 內嵌 DOT→SVG 渲染或直接畫弧 |
| 成功率提升未量測 | 下一步：D 表加指標——同一 borrow 錯誤集,「純 rustc 診斷」vs「rustc＋幾何」的代理一次修復率對照；`borrow.analysis.reduced/red_edges` 欄位已為此鋪路 |

## 6. 上游同步約定

vendor 升級流程（VENDOR.md）：上游 diff → 重放 2 補丁 → 19 回歸＋26 oracle＋`go test ./internal/borrow/...`（17 golden）全綠才合入。golden 同時是 T-18b Go 引擎的 differential testing 基準——ChordLaw Python 版未來轉為參照物與教學資產。
