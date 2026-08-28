# 弦律 MCP × 持久化狀態（P3a）

> 版本 v0.6 · 2026-08-27
> 邏輯引擎已完成階段性開發。本檔把引擎接到 **MCP + CLI**，並加上 **專案檢查 → 建議 → 評分 → 報告** 與 **會話持久化**。
>
> 一句話：代理不重跑 fixpoint，只呼叫工具；狀態在 `.chordlaw/state.json`，修法是縮弧／移點／升圓。

配套：[`helper.py`](helper.py)（檢查／評分／狀態）· [`mcp_server.py`](mcp_server.py)（stdio MCP）· [`chordlaw.py`](chordlaw.py)（引擎＋CLI）

---

## 0. 為何現在做

PLAN §5 **P3 代理工具鏈**：`chordlaw_check` / `diagram` / `explain` / `rules`。v0.5 已有 `--json` / `--explain` / 幾何違反塊，但：

| 缺口 | 後果 |
|---|---|
| 無 MCP | 代理要自己拼 CLI，stdout 與證明樹混在一起 |
| 無會話記憶 | 同一錯誤每輪重解釋；無法量 M2（中位 ≤2 次迭代） |
| 無專案級分數 | 只有單檔 PASS/FAIL，不能當「Rust 小幫手」的儀表板 |
| 無 `.rs` 邊界 | 代理會以為能檢查全量 Rust（合約 S 風險） |

P3a（本刀）只補這四條。P2 `syn` 前端、P3b 增量引擎、P3c 自動改碼 **不在本刀**。

---

## 1. 架構

```
                 ┌──────────── CLI ────────────┐
  .cl / 目錄 ──► │ chordlaw.py --check/--report│
                 │ --mcp / --json / --explain  │
                 └────────────┬────────────────┘
                              │ 同一 API
                 ┌────────────▼────────────────┐
                 │ helper.py                   │
                 │  collect → check → advise   │
                 │  score  → persist → report  │
                 └──────┬───────────┬──────────┘
                        │           │
              ┌─────────▼──┐   ┌────▼─────────┐
              │ chordlaw.py │   │ .chordlaw/   │
              │ 引擎+幾何   │   │  state.json  │
              └────────────┘   └──────────────┘
                        ▲
                 ┌──────┴──────┐
                 │ mcp_server  │  stdio JSON-RPC
                 │ tools +     │  stdout 只准 MCP 訊息
                 │ resources   │
                 └─────────────┘
```

**法則 0 延伸**：MCP 工具不立法。判定、✕、建議一律來自 `check()` / `geometry_report()`。

---

## 2. 持久化狀態

路徑：`<project>/.chordlaw/state.json`（gitignore；不含密鑰、不含源碼全文）。

```json
{
  "version": 1,
  "project": "/abs/path",
  "updated": "2026-08-27T12:00:00+08:00",
  "sessions": [
    {
      "id": "s_20260827_120000_ab12",
      "ts": "...",
      "liveness": "nll",
      "score": { "overall": 72, "band": "needs_work", "dimensions": {} },
      "files": [{ "path": "examples/r01_eclash.cl", "verdict": "FAIL", "codes": ["E01"] }],
      "advice": [{ "priority": "P0", "op": "shrink_arc", "file": "...", "code": "E01", "detail": "..." }],
      "skipped_rs": [{ "path": "src/lib.rs", "reason": "no_syn_frontend" }]
    }
  ]
}
```

- 最多保留 **20** 個 session（FIFO）。
- 兩次 session 可比對：新出現／消失的 (file, code, stmt) → 量 M2。
- **不**把 `.cl` 原文寫進 state（隱私＋體積）；只存路徑、判決、錯誤碼、建議。

---

## 3. MCP 工具（stdio，協議 2024-11-05／2025-03-26 相容）

| 工具 | 參數 | 回傳 | 對齊 PLAN |
|---|---|---|---|
| `chordlaw_check` | `source` 或 `path`；`liveness` | verdict / errors+proof / regions / advice | check |
| `chordlaw_explain` | 同上 | 規則原文 + 證明 + 幾何違反 + 修法 | explain |
| `chordlaw_diagram` | 同上；可選 `out` | SVG 字串或寫檔路徑 | diagram |
| `chordlaw_rules` | 可選 `code` | 規格速覽或單則 | rules |
| `chordlaw_report` | `root`；`liveness` | 專案分數 + 建議 + 寫入 state | **本刀新增** |
| `chordlaw_history` | `root`；可選 `n` | 最近 session 摘要與 diff | **本刀新增** |

Resources：`chordlaw://rules`、`chordlaw://session/latest`、`chordlaw://report/latest`。

約束：

- stdout **只**出 newline-delimited JSON-RPC（禁止 `print` 雜訊）。
- 日誌走 stderr。
- `.rs` 不假裝已檢查：標 `skipped`，建議走 rustc 外環。

Claude Desktop / Cursor 掛載：

```json
{
  "mcpServers": {
    "chordlaw": {
      "command": "python3",
      "args": ["/abs/path/Layer5/chordlaw.py", "--mcp"]
    }
  }
}
```

---

## 4. 評分（Rust 小幫手儀表）

兩層分數，不可混為一談。

### 4.1 專案檢查分（對被檢目錄）

| 維度 | 公式 | 權重 |
|---|---|---|
| **correctness** | 100 × PASS 檔 / 已檢檔 | 0.40 |
| **severity** | 100 − min(100, Σ 權重)；E01/E06/E10=12，其餘 6–10 | 0.30 |
| **explainability** | 有幾何+修法的錯誤比例 | 0.15 |
| **coverage** | 已檢 .cl / (.cl + 含借用跡象的 .rs) | 0.15 |

`overall` = 加權和。分檔：

| 帶 | 分 | 含義 |
|---|---|---|
| `ready` | ≥ 85 | 內環可當預檢 |
| `needs_work` | 50–84 | 有可修的弧／點 |
| `blocked` | < 50 | 高頻衝突或覆蓋不足 |

建議操作只許三種幾何動詞（P-G4）：**縮弧 / 移點 / 升圓**（外加 clone／改權限的文字模板）。優先級：P0 = E01 E06 E07 E10；P1 = E02 E03 E09 E18 E24；P2 = 其餘。

### 4.2 產品就緒分（本倉庫當「Rust 小幫手」）

靜態量表，寫在報告開頭，不進 Datalog：

| 維度 | v0.5 | v0.6 目標 | 出口 |
|---|---|---|---|
| 引擎／32 則 | 88 | 88 | 已達 |
| 健全性合約 S | 90 | 90 | oracle 26/26；P2 前維持 |
| 代理接口 | 55 | 80 | MCP 四工具 + report/history |
| 持久化 | 10 | 75 | state.json + session diff |
| Rust 覆蓋 | 25 | 25 | **誠實缺口**：無 syn |
| 修復閉環 | 60 | 70 | 建議可執行；尚未自動改碼 |

---

## 5. CLI

```bash
python3 chordlaw.py --check [DIR]              # 掃 .cl，略過 .rs
python3 chordlaw.py --report [DIR] [-o FILE]   # 評分+建議+寫 state
python3 chordlaw.py --history [DIR]
python3 chordlaw.py --mcp                      # stdio MCP（stdout 純協議）
```

`--check` / `--report` 與 MCP 走 `helper.py`，禁止兩套計分。

---

## 6. 路線（訂立，不只寫下）

| 刀 | 內容 | 出口 | 狀態 |
|---|---|---|---|
| **P3a** | helper + state + MCP stdio + CLI + 本倉庫報告 | 測試綠；`--report examples` 可重跑 | **本刀** |
| **P3b** | 增量：檔 hash → 只重算變更 fn | p95 < 500ms（M3） | 未做 |
| **P3c** | `chordlaw_apply`：按建議改 .cl（縮弧） | 代理中位 ≤2 次（M2 demo） | 未做 |
| **P2** | `syn` → IR；`.rs` 不再 skipped | 200 snippet 保守一致 ≥99% | 未做 |
| **P5** | 有／無弦律 A/B | M1–M5 報告 | 未做 |

---

## 7. 風險

| 風險 | 對策 |
|---|---|
| MCP stdout 被 `print` 污染 | `--mcp` 不走 `main()` 的 print 路徑；測試斷言每行可 `json.loads` |
| 把 .rs 當 .cl 查 | 副檔名閘門；coverage 維度扣分而不是假 PASS |
| state 膨脹／洩源碼 | 只存路徑與碼；上限 20 session |
| 分數被當成 rustc 品質 | 報告抬頭寫「內環預檢，rustc 終審」 |
