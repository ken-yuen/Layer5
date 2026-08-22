# Vendor 記錄 — 弦律 ChordLaw

| 項目 | 內容 |
|---|---|
| 上游 | https://github.com/ken-yuen/Layer5 |
| 併入 commit | `3b93854`（ChordLaw v0.3, 2026-08-22） |
| 併入日期 | 2026-08-22 |
| 授權 | MPL-2.0（與 YKC 開源線一致；同一版權擁有人） |
| 角色 | YKC L5 解釋層引擎（判定權屬 rustc，本引擎僅產「幾何解釋」）|

## 本地補丁（相對上游）

1. `chordlaw.py --no-svg`：抑制 SVG 副作用（`run_one(no_svg=)` ＋ outdir 條件建立）。
   YKC 的 Go 接線（`internal/borrow`）一律帶 `--json --no-svg` 調用。
2. `--json` 輸出加 `"schema": "chordlaw.report/v1"` 欄位（Go 端向前相容鉤子）。
3. 不帶上游 `examples/out/`（SVG 為運行產物；見根 `.gitignore`）。

## 驗證

```bash
python3 l5/chordlaw/test_chordlaw.py    # 19 項回歸（上游測試，原樣保留）
python3 l5/chordlaw/oracle_check.py     # 26 例 rustc 差異測試（需 rustc）
make l5-test                            # 由倉庫根一鍵執行
```

上游升級流程：對照上游 diff → 重放上述補丁 → 跑 19 回歸＋26 oracle ＋ `go test ./internal/borrow/...`（golden 快照）全綠才合入。
