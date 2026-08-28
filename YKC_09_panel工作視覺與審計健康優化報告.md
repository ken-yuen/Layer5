# YKC 09 — Panel 工作視覺與審計健康優化報告
> 日期：2026-08-22（依 git 提交時刻 302fdf6 08-21 21:43 UTC = HKT 08-22 換算回填；2026-08-25 審計）

## 目標

人類用戶需要在 YKC 執行任務時立即知道「YKC 正在工作」，不能只靠表格內一行 running。參考 Arena 類工作狀態提示，本輪加入全局動態工作指示器：有任務運行時，面板右上方會有一顆會放大縮小、向外呼吸的亮色光點。

## 已實作

### 1. 全局工作視覺

位置：`cmd/ykc-panel/dashboard.html`

新增：

- `#work-indicator`
- `.work-orb`
- `@keyframes ykcPulse`
- `@keyframes ykcRing`

狀態：

| 狀態 | 顯示 |
|---|---|
| 無任務 | `YKC IDLE`，灰色靜止點 |
| 任務啟動中 | `YKC STARTING`，藍綠動態點 |
| 任務運行中 | `YKC WORKING ×N`，光點會大細大細呼吸 |
| job feed 讀取錯誤 | 紅色錯誤狀態 |

同時更新 browser title：

```text
●(N) YKC 工作中
```

### 2. 控制面板新增動作

新增按鈕：

```text
預編譯 precompile
同步帳本 sync
```

對應後端：

| Button | Binary |
|---|---|
| precompile | `ykc-precompile -project <project> -sandbox native -allow-native -json=false` |
| sync-ledger | `ykc-atom sync-ledger -root <project> -state .ykc` |

### 3. Ledger / EventStore 健康顯示

`ProjectState` 新增：

```json
{
  "event_count": 0,
  "projected_events": 0,
  "missing_projections": 0
}
```

面板 project card 顯示：

```text
事件/投影  N/M
缺投影     K
```

頂部 stat strip 顯示：

```text
事件 EVENTS
鏈缺投影 MISSING
```

### 4. JobManager 穩定性修正

`cmd/ykc-panel/jobs.go`：

- `cmd.Start()` 失敗時不再啟動 `cmd.Wait()` goroutine。
- 失敗 job 仍入 jobs list，讓人類看到錯誤原因。
- 新增 `precompile` / `sync-ledger` action。

## 減債價值

| 問題 | 改善 |
|---|---|
| 人類不知道 YKC 是否正在做事 | 全局 pulse indicator 直接顯示 working / idle |
| 新增 precompile 只能 CLI 用 | 面板可直接觸發 |
| eventstore / ledger drift 不可視 | 面板顯示 event / projection / missing |
| 啟動 job 失敗可能不清楚 | failure job 保留在 UI 中顯示 log |
| 工作中狀態只藏在表格 | header 與 title 也顯示 working 狀態 |

## 驗證

已執行：

```bash
go test ./...
go vet ./...
go build -o bin/ykc-panel ./cmd/ykc-panel
go build -o bin/ykc-precompile ./cmd/ykc-precompile
go build -o bin/ykc-atom ./cmd/ykc-atom
YKC_HOME=/home/user/.cache/ykc-tools make verify-all
```

結果：全部通過。
