# AutoHPBorrowBase（AHPBB）— 自動生命借貸工廠

> 弦律 v0.7 · P1 語意意圖樹 + 生命週期演算法 · P2 syn 子集 / quote 模板 · 工廠多線出貨

**工廠能**：自動生成**正確**的 Rust borrow / lifetime 程式碼，**多線批量**生產，並保存到一個**新開**資料夾。

本環境**沒有 rustc / cargo**。`syn` / `quote` 是 Python 子集，不是 crates。內環 PASS ≠ rustc 放行。

---

## 1. 三層

| 層 | 檔 | 角色 |
|---|---|---|
| **意圖樹 (SIT)** | `intent.py` | `IntentFn` / `Region`；生命週期 = `I(loan) = {Q \| reach(L,Q) ∧ reach(Q,E)}`（弦律 `onregion`） |
| **syn 子集** | `syn_subset.py` | oracle 形 Rust → `.cl`。看不懂就 warning，不發明借用事實 |
| **quote 模板** | `quote_tpl.py` | `.cl` + 意圖樹 → Rust（對齊 `oracle_check.py`：`let _uN = *r` / `*r += 1` / `let _m = x`） |
| **工廠** | `ahpbb.py` | 配方 → 檢查 → 僅 PASS 出貨；ThreadPool；永遠 `factory_out/AHPBB_YYYYMMDD_HHMMSS/` |

## 2. 生命週期怎麼算

不猜 `'a`。流程：

1. 解析（`.cl` 或 syn 降階）→ CFG
2. 弦律定點 → `span_end` / `onregion`
3. `compute_lifetimes` 把每條弧編成 `'a` `'b` …
4. **回傳參考**時，把該弧的 lt 寫進簽名：`fn f<'a>(p: &'a u32) -> &'a u32`

兩條序貫借貸的點集不相交 → 可依序化，不必同時活。

## 3. syn 子集（能進內環的 Rust）

與 `oracle_check.py` 對偶：

```
fn / struct / let mut / & / &mut / &*p
let _uN = *r;     → use r
*r += 1;          → use r（mut）
let _m = x;       → mv x
std::mem::drop(x) → dp x
x = …;            → set x
if true { } / loop { } / 尾運算式回傳
fn main() 略過
```

非子集（`impl` / `mod` / 巨集 / 泛型 / async）→ **直通 rustc**，不假裝已檢。

## 4. 出貨合約

- 只寫 **弦律 PASS** 的產品（`--include-fail` 才留反例）。
- 每個產品三件套：`.cl` · `.rs` · `.json`（具名區域）。
- 批次目錄**永遠新建**，不覆蓋舊批次。
- 幾何自動修（可選）：`shrink_arc` / `raise_circle` / `move_point`。

## 4b. 環境：Python 倉 ≠ Rust crate

| 環境 | 得唔得 | 點用 |
|---|---|---|
| **本倉（純 Python 3 標準庫）** | 得 | `python3 chordlaw.py --factory`。**根目錄沒有、也不該有 `Cargo.toml`** |
| **任何有 Python 3 的機器** | 得 | 複製 `Layer5/` 即可；零 pip、零 cargo |
| **有 `rustc` 的機器** | 終審單檔 | `rustc factory_out/AHPBB_*/products/foo.rs` |
| **有 `cargo` 的機器** | 終審整批 | `cd factory_out/AHPBB_* && cargo check`（Cargo.toml **只在該批次資料夾**） |

工廠寫的 `Cargo.toml` 永遠落在新開的 `AHPBB_*` 裡，`factory_out/` 已 gitignore。唔會改你個 Python 環境。

## 5. 用法

```bash
python3 chordlaw.py --factory                 # 多線批量 → factory_out/AHPBB_*
python3 ahpbb.py --batch -j 4 -o factory_out
python3 ahpbb.py --from-rs path/to/f.rs
python3 ahpbb.py --from-cl examples/ex2_sequential.cl
python3 test_ahpbb.py                         # P1/P2/工廠
```

## 6. 誠實缺口

| 沒做 | 為什麼 |
|---|---|
| rustc 終審 | 環境無 `rustc`；合約 S 仍靠 oracle 語料 + 內環 |
| 全量 syn crate | 無 cargo；子集覆蓋 oracle 26 例判決 |
| 命名 lt 推斷跨 fn / variance | 區域仍是 CFG 點集 |
| 自動 apply 進使用者檔 | 工廠寫**新資料夾**，不改你的 src |
