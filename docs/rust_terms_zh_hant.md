# YKC Rust 術語表（繁體中文）

> 版本：`zh-Hant-tier1-60@2026-08-23`  
> 範圍：YKC tier-1 error card 的繁中摘要。此表是**翻譯一致性規約**，不是 Rust
> 官方英文文件的替代品；錯誤碼卡仍保留英文原文與官方 URL。

| English | YKC 繁中 | 使用原則 |
|---|---|---|
| ownership | 所有權 | 不譯為「擁有權」以貼近 Rust 社群慣用語。 |
| move | move／移動 | 程式碼與診斷常保留 `move`；敘述可寫「值已被 move」。 |
| borrow | 借用 | 對應 `&T` 或 `&mut T` 的借用語意。 |
| mutable borrow | 可變借用 | 對應 `&mut T`；避免誤譯為「可變引用」。 |
| reference | 參考 | 指 Rust `&T`／`&mut T`；中文摘要不把它混同於一般指標。 |
| lifetime | 生命週期 | 泛型參數 `'a` 保留原樣。 |
| trait | 特徵 | Rust 專有抽象；程式碼 `Trait` 不翻譯。 |
| implementation / impl | 實作／`impl` | 關鍵字 `impl` 保留；一般說明用「實作」。 |
| associated type | 關聯型別 | 不使用「附屬型別」。 |
| orphan rule | 孤兒規則 | Rust coherence 的既有常用譯名。 |
| trait object | trait object／特徵物件 | `dyn Trait` 保留；首次可寫「trait object（特徵物件）」。 |
| object safety | object safety | 以英文術語搭配「可作為 trait object 的安全條件」。 |
| type inference | 型別推斷 | `type annotation` 譯為「型別註記」。 |
| generic argument | 泛型參數 | 包括型別、常數與生命週期參數，按上下文補充。 |
| pattern | 模式 | `match` 的 pattern；`pattern guard` 譯為「模式守衛」。 |
| irrefutable pattern | 不可反駁模式 | 必定匹配成功的模式。 |
| scope | 作用域 | `visibility` 則譯為「可見性」，不可混用。 |
| private item | 私有項目 | struct field、函式、模組等統稱 item。 |
| crate | crate | 保留 Rust 生態常用術語；必要時寫「crate（套件編譯單位）」。 |
| module | 模組 | 對應 `mod`／module path。 |
| closure | closure | 首次可寫「closure（閉包）」；程式碼保留 `closure`。 |
| unsafe | unsafe | 關鍵字和概念均保留英文。 |
| async / await | async / await | Rust 關鍵字不翻譯。 |
| temporary value | 暫時值 | 指 expression 產生、壽命有限的值。 |
| indirection | 間接層 | 如 `Box`、reference 等讓遞迴型別有有限大小的層。 |
| conflicting implementations | 衝突的特徵實作 | 避免簡寫成「實作衝突」而丟失 trait 語意。 |

## 翻譯品質紀律

1. **不覆寫原文**：`Atom.ZH` 是獨立欄位，英文 `Title`、`Body`、範例與 source URL 不變。
2. **不翻譯識別字**：error code、型別／函式／欄位名稱、`&mut`、`dyn`、`impl` 等保持可搜尋。
3. **精簡可操作**：繁中摘要先描述 Rust 編譯器拒絕了甚麼；詳細原因和修法仍連回官方英文卡與規則原子。
4. **版本綁定**：每批翻譯隨 `translation_version` 寫入 blob metadata／manifest，並由 coverage test 鎖定。
