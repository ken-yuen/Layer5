# YKC 07 — 新增功能技術債審計與優化報告

## 審計範圍

本次全面檢查以下新增與既有功能交界：

- 原子監控：`internal/atomicfile`、`internal/eventstore`、`internal/monitor`
- 動態護欄：`internal/guardrail`、`internal/enforcement`、`cmd/ykc-atom`
- 沙盒預編譯：`internal/sandbox`、`internal/precompile`、`cmd/ykc-precompile`
- 舊功能回歸：`ykc-smoke`、`ykc-judge`、`ykc-guard`、`ykc-lsp`、`ykc-panel`、`make verify-all`

## 發現並已修復的問題

| 類型 | 問題 | 風險 | 修復 |
|---|---|---:|---|
| 輸出處理 | command runner 原先保留 stdout/stderr 開頭，不是真 tail | 高：大型 cargo 輸出可能丟失最後 rustc error | 改為 tail buffer，保留最後 N bytes，完整 stream SHA-256 |
| 環境污染 | precompile 使用 host `CARGO_HOME` / target | 中高：污染全局 cache、結果不穩定 | 改用 `.ykc/precompile/cargo-home`、`.ykc/precompile/target` |
| 網絡控制 | native / sandbox env 未明確 offline | 中：compile 階段可能意外訪問網絡 | network-none 階段注入 `CARGO_NET_OFFLINE=true` |
| 容器權限 | docker/podman 預設 root 寫 workspace | 中：產生 root-owned target/.ykc | container backend 加 `--user uid:gid` |
| 回歸覆蓋 | `verify-all` 未覆蓋 ykc-precompile | 中：新功能可能 drift | 新增第 ⑩ 預編譯健康檢查 |
| Makefile 分工 | `atom` target 曾連帶 build precompile | 低：命令語義不清 | 拆回 `atom` / `precompile` target |
| 測試覆蓋 | sandbox 無單元測試 | 中 | 新增 tail buffer 與 offline env 測試 |

## 目前未形成高難度技術債的判斷

目前沒有發現會阻塞後續架構的「高難度技術債」。原因：

1. 新功能以 `internal/` package 方式接入，沒有硬改原 `ykc-guard` / `ykc-judge` 主邏輯。
2. `ykc-precompile` 是新增 CLI，不破壞舊 `bin/ykc` 語義。
3. `verify-all` 仍全通過，證明舊功能沒有被新功能破壞。
4. 沙盒 backend 以 interface/config 方式抽象，未來可接 Firecracker/Kata/nsjail，不需重寫 precompile pipeline。
5. 無 sandbox 時 fail-closed，不會暗中執行不可信 cargo。

## 仍需追蹤的中期債務

| 債務 | 風險 | 建議處理 |
|---|---|---|
| `eventstore` 與舊 `ledger` 並存 | 兩套事實來源可能 drift | 下一步做 bridge：eventstore atomic commit 後再 append hash-chain ledger |
| Firecracker / Kata 尚未實作 backend | 生產級 SaaS 隔離未完成 | 先以 gVisor runsc 落地；企業版加入 microVM executor |
| bwrap 不是強多租戶隔離 | 本地 fallback 可用，但不可宣稱 SaaS 安全 | 報告中保持 `trust=moderate`，不升級為 strong |
| Native backend 存在但有明示開關 | 誤用會有風險 | 保持 `-allow-native` 必須明示；panel 顯示紅色 trust none |
| cargo diagnostics 仍可能重複 | 同一 source error 在 bin/test targets 可能重複 | 後續加入 diagnostic fingerprint 去重與 occurrence count |

## 已執行驗證

```bash
go test ./...
go vet ./...
YKC_HOME=/home/user/.cache/ykc-tools make verify-all
bin/ykc-precompile -project ./demo-rust-cli -sandbox native -allow-native -json=false
bin/ykc-precompile -project ./demo-semantic-cli -sandbox native -allow-native -json=true
```

結果：

- `go test ./...` 通過
- `go vet ./...` 通過
- `make verify-all` 通過，並新增第 ⑩ 預編譯 PASS
- `demo-rust-cli` 預編譯 PASS
- `demo-semantic-cli` 預期 FAIL，成功解析 `E0425 cannot find function greet`

## 結論

新增功能沒有引入高難度技術債；已修復數個中高風險穩定性問題。已完成第一版 `eventstore` 與 `ledger` 橋接，令「原子完整性」與「hash-chain 可審計性」合併成單一路徑。下一步最值得做的是在 panel 顯示 ledger chain health，並把 ykc-guard 舊 score path 也切到 bridge。
