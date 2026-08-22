# Rust 規則抽象（54 條 / 17 領域）

> YKC 知識庫的結構化規則層：每條規則 = 中文陳述 + 為什麼 + 修法菜單 + 對應錯誤碼 + 對應章節 + 規則互引。
> 程式內以 `ykc-know rule <ID>` / `ykc-know rules -domain <領域>` / `ykc-know search ...` 查詢。


## ownership（所有權）


### OWN-01 — A value has exactly one owner; assignment moves ownership (non-Copy types)

**規則**：每個值只有一個擁有者；對非 Copy 型別賦值會「移動」擁有權，原變數即失效。

**為什麼**：move 是 Rust 所有權系統的根基：同一時刻一個值只能被一個變數擁有，避免 double-free 與 use-after-free。

**對應錯誤碼**：E0382, E0507

**對應章節**：what-is-ownership-1

**相關規則**：BRW-01, OWN-02

**修法**

1. 改用借用 &x / &mut x 而非 move

2. 呼叫 .clone() 得到副本

3. 對型別 #[derive(Clone)] 或 #[derive(Copy, Clone)]

4. move 後、再次使用前重新初始化變數


### OWN-02 — Copy vs Clone: Copy is implicit and bitwise; Clone is explicit and may allocate

**規則**：Copy 是隱式的按位複製（僅標量與純 Copy 結構）；Clone 是顯式的、可含堆配置的深複製。

**為什麼**：Copy 型別賦值後仍可用；非 Copy 型別賦值即 move。要讓自訂型別可隱式複製須滿足所有欄位皆 Copy。

**對應錯誤碼**：E0382, E0507

**對應章節**：what-is-ownership-1

**相關規則**：OWN-01

**修法**

1. 為型別加上 #[derive(Copy, Clone)]（僅當欄位全為 Copy）

2. 改用 #[derive(Clone)] 並顯式 .clone()

3. 持有堆資料（String/Vec）的型別不可 Copy，只能 Clone


### OWN-03 — Partial move: moving out one field moves that field only; the rest of the struct stays usable

**規則**：部分移動：把結構體某個欄位 move 出去後，該欄位失效，其餘欄位仍可使用（但整體不能再被使用）。

**為什麼**：Rust 以欄位為粒度追蹤移動；理解部分移動才能正確地在 move 部分欄位後繼續使用其他欄位。

**對應錯誤碼**：E0382, E0594

**對應章節**：what-is-ownership-1

**相關規則**：OWN-01

**修法**

1. 先解構把要用的欄位分別取出

2. 以借用的方式讀取欄位而非 move

3. 用 std::mem::take / replace 取走欄位並留下預設值


## borrowing（借用）


### BRW-01 — At any time: one &mut XOR many &, never both

**規則**：同一時刻對一個值：要嘛一個 &mut，要嘛任意多個 &，兩者不可並存。

**為什麼**：這是借用檢查器防止資料競爭（data race）與別名+可變（aliasing XOR mutability）的核心不變量。

**對應錯誤碼**：E0499, E0502, E0596

**對應章節**：references-and-borrowing-1

**相關規則**：BRW-03, LFT-01, OWN-01

**修法**

1. 縮短先前借用的存活區間（把它最後一次使用提前）

2. 延後新借用，讓兩個借用不重疊

3. 拆分路徑：分開借不同欄位 x.f 與 x.g

4. 需要內部可變性時用 RefCell / Cell（見 BRW-08）


### BRW-02 — NLL: a borrow lives only until its last use, not to the end of scope

**規則**：非詞法生命週期（NLL）：借用只活到最後一次使用為止，不必等到作用域結束。

**為什麼**：NLL 讓「看起來重疊、實際不重疊」的借用合法；除錯時把變數的使用順序排好即可消除多數借用錯誤。

**對應錯誤碼**：E0499, E0502, E0503, E0505, E0506

**對應章節**：references-and-borrowing-1

**相關規則**：BRW-01, BRW-04

**修法**

1. 把變數的最後一次使用移到新借用之前

2. 把結果先存進局部變數再釋放借用

3. 縮小作用域：用大括號 { } 把借用圈在更小區塊


### BRW-03 — Cannot mutably borrow what is already immutably borrowed (and vice versa)

**規則**：不能對已被不可變借用的值做可變借用（反之亦然）。

**為什麼**：不可變借用期間值被凍結；要寫入必須等所有 & 結束，或改用內部可變性。

**對應錯誤碼**：E0499, E0502, E0596

**對應章節**：references-and-borrowing-1

**相關規則**：BRW-01, BRW-08

**修法**

1. 把寫入移到所有 & 的最後一次使用之後

2. 經由已持有的 &mut 寫入（若你本來就有可變借用）

3. 用 RefCell 把檢查延後到執行期（見 BRW-08）


### BRW-04 — Cannot assign to a variable while it is borrowed

**規則**：變數被借用期間不能對它賦值。

**為什麼**：賦值 = 對被借者寫入，等於入侵活躍借用的弧跨（幾何法則①）。

**對應錯誤碼**：E0506

**對應章節**：references-and-borrowing-1

**相關規則**：BRW-01, BRW-02

**修法**

1. 把賦值移到借用區間結束之後

2. 先讓引用用完（把最後使用提前）再賦值

3. 若改的是不同欄位，拆分路徑借用


### BRW-05 — Cannot move out of a value that is borrowed (or out of a dereference)

**規則**：不能把「正被借用」的值 move 出去，也不能經由解參考把值移出。

**為什麼**：move 會抽走被借者的擁有權，使借用失去錨點；這是 E0505/E0507 的幾何本質。

**對應錯誤碼**：E0505, E0507

**對應章節**：references-and-borrowing-1

**相關規則**：BRW-01, OWN-01

**修法**

1. 改為借用而非 move

2. 先 clone 再 move

3. 把 move 延後到借用結束之後


### BRW-06 — Split borrows: disjoint fields can be borrowed independently

**規則**：拆分借用：不同欄位可以各自獨立借用，互不衝突。

**為什麼**：借用檢查器以欄位/路徑為粒度判斷衝突；整體借用才互斥，欄位級可並行。

**對應錯誤碼**：E0499, E0502

**對應章節**：defining-and-instantiating-structs-1

**相關規則**：BRW-01

**修法**

1. 把整體借用 x 拆成 x.f 與 x.g 兩個借用

2. 用解構 let (a, b) = (&mut x.f, &mut x.g) 同時取得

3. 避免把整個結構體塞進一個函數參數，改成傳欄位


### BRW-07 — Dereference gives a place, not a copy: use *r carefully to avoid move-out

**規則**：解參考 *r 得到的是「位置」而非副本：要寫入或 move 時需注意別把值抽走。

**為什麼**：&T 的 *r 只能讀；&mut T 的 *r 可寫可移出（移出需留下有效值）。

**對應錯誤碼**：E0382, E0507

**對應章節**：references-and-borrowing-1

**相關規則**：BRW-05, OWN-01

**修法**

1. 讀取用 *r 的副本（如 let v = *r; 對 Copy 型別）

2. 移出 &mut 時用 std::mem::replace / take 留下有效值

3. 改用 .clone() 得到擁有值


### BRW-08 — Interior mutability (Cell/RefCell) moves borrow checks to runtime

**規則**：內部可變性（Cell/RefCell）把借用規則檢查延後到執行期，以換取「看似不可變借用的可變性」。

**為什麼**：當靜態檢查過嚴但設計正確時（如快取、觀察者模式），RefCell 是正當出口；代價是執行期 panic 風險。

**對應錯誤碼**：E0502, E0596

**對應章節**：refcellt-and-the-interior-mutability-pattern-1

**相關規則**：BRW-01, BRW-03

**修法**

1. 把欄位包成 RefCell<T> 並用 borrow_mut()

2. 優先考慮重構讓借用靜態可證，RefCell 是最後手段

3. 搭配 Rc<RefCell<T>> 做共享可變（見 SPT-02）


## lifetime（生命週期）


### LFT-01 — Lifetime elision: 3 rules infer lifetimes on fn signatures automatically

**規則**：生命週期省略（elision）三規則：每個引用參數各有獨自生命週期；只有一個輸入時輸出沿用它；&self/&mut self 的方法輸出沿用 self。

**為什麼**：多數函數不需手寫生命週期；只有當多個輸入且輸出引用時才需要顯式標注。

**對應錯誤碼**：E0106

**對應章節**：validating-references-with-lifetimes-1

**相關規則**：LFT-02

**修法**

1. 依 elision 規則補上 <'a> 並把輸出與正確輸入綁定

2. 只有一個引用輸入時通常可直接省略

3. 方法回傳引用時多半綁定 &self，可省略


### LFT-02 — Missing lifetime specifier: multiple inputs + reference output needs explicit 'a

**規則**：當函數有多個引用輸入且回傳引用時，編譯器無法推斷輸出生命週期，需顯式標注。

**為什麼**：編譯器只在不歧義時省略；多輸入即歧義，必須由你指定輸出與哪個輸入同壽命。

**對應錯誤碼**：E0106

**對應章節**：validating-references-with-lifetimes-1

**相關規則**：LFT-01

**修法**

1. 在 fn 上標 <'a>，輸出 &'a 綁定到正確輸入

2. 若輸出只依賴一個輸入，把其他輸入的生命週期獨立開（<'a,'b>）

3. 改用擁有型別回傳（String 而非 &str）


### LFT-03 — Borrowed value does not live long enough: the referent dies before the reference's last use

**規則**：被借的值活得比引用最後一次使用短——引用指向已死的值。

**為什麼**：引用不得越過被借者的作用域圓（幾何法則②）；把被借者外提或把使用內縮即可。

**對應錯誤碼**：E0597

**對應章節**：validating-references-with-lifetimes-1

**相關規則**：BRW-02, LFT-01

**修法**

1. 把被借變數宣告移到外層作用域

2. 把使用移到被借變數的作用域內

3. 需要讓資料活更久就改用擁有型別（String/Box/Rc）


### LFT-04 — Cannot return a reference to a local variable

**規則**：不能回傳指向局部變數的引用——函數結束局部變數即被釋放。

**為什麼**：回傳引用的被借者必須活得比呼叫者久；局部變數做不到。

**對應錯誤碼**：E0106, E0515

**對應章節**：validating-references-with-lifetimes-1

**相關規則**：LFT-03

**修法**

1. 改為回傳擁有值（move 出去）

2. 被借者改由參數傳入（&'a 綁定參數）

3. 提升為 'static（僅限常數或 leaked 資料）


### LFT-05 — 'static: the referent lives for the entire program

**規則**：'static 表示資料存活於整個程式期間（字串字面量、常數、leaked 值）。

**為什麼**：&'static 是最長生命週期；任何引用都可被協變縮短為 'static 的借用，反過來不行。

**對應錯誤碼**：E0515, E0597

**對應章節**：validating-references-with-lifetimes-1

**相關規則**：LFT-03, LFT-04

**修法**

1. 字串字面量天然是 &'static str

2. Box::leak 可取得 'static（慎用，會洩漏）

3. 不要企圖把局部資料標成 'static


### LFT-06 — Outlives bounds: 'a: 'b means 'a lives at least as long as 'b

**規則**：'a: 'b 表示 'a 比 'b 活得久（outlives 關係）；泛型約束與結構體欄位常需它。

**為什麼**：當一個型別包含引用欄位時，必須聲明該欄位生命週期與型別本身的關係。

**對應錯誤碼**：E0310, E0495

**對應章節**：validating-references-with-lifetimes-1

**相關規則**：LFT-01

**修法**

1. 在 struct 定義補上生命週期參數 <'a> 並標欄位 &'a

2. 用 where 'a: 'b 表達 outlives 約束

3. 讓多個欄位共用同一個 'a


## types（型別）


### TYP-01 — Type mismatch: expected vs found — read the full expected/found pair

**規則**：型別不匹配：rustc 會印出 expected 與 found 兩邊；關鍵是看「完整」型別而非只看名字。

**為什麼**：Rust 型別系統嚴格；&str vs String、i32 vs u32、&T vs T 是最常見的混淆。

**對應錯誤碼**：E0308

**對應章節**：data-types-1

**相關規則**：COL-01

**修法**

1. 按 expected/found 對齊：補 & 或 .to_string()/.as_str()

2. 數字用字面量類型標注（42u32）或 as 轉型

3. 檢查是否把引用與擁有值搞混（見 COL-01）


### TYP-02 — Cannot find type/struct in this scope: it is not in scope or not defined

**規則**：找不到型別：多半是沒 import、拼錯、或定義在別的模組沒加路徑。

**為什麼**：路徑解析是模組系統的一部分；找不到型別先查 use 與模組路徑。

**對應錯誤碼**：E0412, E0422

**對應章節**：paths-for-referring-to-an-item-in-the-module-tree-1

**相關規則**：MOD-02

**修法**

1. 補上 use 或完整路徑（crate::module::Type）

2. 檢查拼寫與大小寫（型別慣例大寫開頭）

3. 確認模組是 pub 且被正確引用


### TYP-03 — if/else and match arms must all yield the same type

**規則**：if/else 各分支與 match 各 arm 的結果型別必須一致。

**為什麼**：運算式型別由所有分支決定；不一致時 rustc 在分支出錯。

**對應錯誤碼**：E0308

**對應章節**：control-flow-1

**相關規則**：TYP-01

**修法**

1. 讓各分支回傳同一型別

2. 用 Ok(()) / Err(e) 統一回傳型別

3. 數字分支標注同型別（如 0u32）


### TYP-04 — Method not found: check the receiver type and trait imports

**規則**：找不到方法：多半是接收者型別不對、trait 沒 use、或方法其實是自由函數。

**為什麼**：方法解析先看固有 impl 再看 in-scope traits；trait 不在 scope 方法就不可見。

**對應錯誤碼**：E0599

**對應章節**：defining-shared-behavior-with-traits-1

**相關規則**：MOD-02, TRT-01

**修法**

1. use 該 trait（如 use std::io::Write;）

2. 確認接收者是 &T 或 T 與簽名一致

3. 方法可能屬另一型別，檢查 .await/.as_ref() 等轉換


## trait（Trait）


### TRT-01 — Trait not implemented: the type does not satisfy the required trait bound

**規則**：型別未實作所需 trait：泛型約束、? 運算子、格式化、迭代等都要求特定 trait。

**為什麼**：trait 是 Rust 的介面；編譯器在泛型邊界強制約束，滿足方式只有實作。

**對應錯誤碼**：E0277

**對應章節**：defining-shared-behavior-with-traits-1

**相關規則**：GEN-01, TRT-02

**修法**

1. 為型別實作該 trait（impl Trait for Type）

2. 加上 #[derive(...)] 取得常用 trait

3. 調整泛型約束或改用已實作的型別


### TRT-02 — Trait bound not satisfied for generic type parameter

**規則**：泛型參數的 trait bound 未滿足：要麼加 bound，要麼改用具體型別。

**為什麼**：泛型函數只能用 bound 內允許的操作；呼叫端必須傳入滿足 bound 的型別。

**對應錯誤碼**：E0277

**對應章節**：generic-data-types-1

**相關規則**：TRT-01

**修法**

1. 在泛型參數加 bound：<T: Display>

2. 呼叫端改用滿足 bound 的型別

3. 用 where 子句把 bound 寫清楚


### TRT-03 — Orphan rule: you can impl a foreign trait for a local type (or local trait for foreign type), never both foreign

**規則**：孤兒規則（coherence）：外來 trait + 外來型別不可實作；至少要有一方是本地定義。

**為什麼**：coherence 保證全域唯一實作；繞過方式只有 newtype 包裝。

**對應錯誤碼**：E0117, E0210

**對應章節**：advanced-traits-1

**相關規則**：TRT-01

**修法**

1. 用 newtype 包裝外來型別再實作外來 trait

2. 把 trait 定義為本地 trait

3. 改用擴充方法模式（本地 trait + 外來型別）


### TRT-04 — Object safety: a trait is dyn-compatible only if its methods have no generics and Self only in return position as receiver

**規則**：物件安全（dyn 相容）：含泛型方法、或 Self 出現在參數/回傳（非接收者）的 trait 不能做 dyn Trait。

**為什麼**：trait object 靠 vtable 動態分派；需要靜態型別資訊的特性使其不 dyn 相容。

**對應錯誤碼**：E0038

**對應章節**：using-trait-objects-to-abstract-over-shared-behavior-1

**相關規則**：TRT-01

**修法**

1. 移除泛型方法或改為 where Self: Sized

2. 把 Self 參數改為接收者形式

3. 改用 enum 取代 trait object 做分派


### TRT-05 — Derivable traits: Debug/Clone/Copy/PartialEq/Default/Hash can be #[derive]d

**規則**：可推導 trait：Debug、Clone、Copy、PartialEq/Eq、PartialOrd/Ord、Default、Hash 可用 #[derive] 自動實作。

**為什麼**：derive 省去手寫樣板；欄位型別需同樣實作該 trait 才能 derive。

**對應錯誤碼**：E0277

**對應章節**：c---derivable-traits

**相關規則**：TRT-01

**修法**

1. 加上 #[derive(Debug, Clone, PartialEq)] 等

2. 欄位型別也要滿足（先給欄位型別 derive）

3. 無法 derive 時手寫 impl


## generics（泛型）


### GEN-01 — Wrong number of generic/lifetime arguments

**規則**：泛型/生命週期參數個數不符：struct 定義了幾個參數，使用時就要給幾個。

**為什麼**：泛型參數在定義與使用處必須一一對應。

**對應錯誤碼**：E0107, E0243, E0244

**對應章節**：generic-data-types-1

**相關規則**：LFT-01

**修法**

1. 對齊參數個數（如 Foo<T, U> 給兩個）

2. 補上生命週期參數（struct Foo<'a>）

3. 使用 turbofish ::<T> 顯式指定


### GEN-02 — Monomorphization: generics compile to concrete code per instantiation; no runtime cost

**規則**：單態化：泛型在編譯期為每個具體型別生成專用代碼，執行期零成本。

**為什麼**：理解單態化有助於判斷何時該用泛型 vs trait object（後者有動態分派成本）。

**對應章節**：generic-data-types-1

**相關規則**：TRT-04

**修法**

1. 熱點路徑用泛型（零抽象成本）

2. 需要異質集合時用 trait object 或 enum


## patterns（模式匹配）


### PAT-01 — Non-exhaustive match: all possible values must be covered (or a wildcard)

**規則**：match 必須窮盡所有可能值，否則要加萬用分支 _。

**為什麼**：match 是窮盡運算式；遺漏分支會產生未定義行為空間，編譯器直接拒絕。

**對應錯誤碼**：E0004

**對應章節**：the-match-control-flow-construct-1

**相關規則**：PAT-03

**修法**

1. 補上漏掉的 arm（含 _ 或 _ => unreachable!()）

2. enum 新增變體時全專案檢查 match

3. 用 if let 取代只關心單一情況的 match


### PAT-02 — Unreachable pattern: an earlier arm already matches this one

**規則**：不可達模式：前面的 arm 已涵蓋後面這個模式。

**為什麼**：模式按順序匹配；後面的更寬鬆模式永不可達即為邏輯錯誤。

**對應錯誤碼**：E0001

**對應章節**：the-match-control-flow-construct-1

**相關規則**：PAT-01

**修法**

1. 把更具體的模式放前面、萬用放最後

2. 刪除被遮蔽的冗餘 arm


### PAT-03 — Refutable pattern in let: let requires an irrefutable pattern

**規則**：let 綁定需要不可反駁模式（必定匹配）；if let/while let 才允許可反駁模式。

**為什麼**：let Some(x) = opt 可能失敗，編譯器拒絕；改用 if let 顯式處理失敗分支。

**對應錯誤碼**：E0005, E0007

**對應章節**：refutability-whether-a-pattern-might-fail-to-match-1

**相關規則**：PAT-01

**修法**

1. 改用 if let / else 或 match 處理

2. 用 expect()/unwrap_or 明確處理 None

3. 若確定必有值，用 unwrap()（會 panic，慎用）


## modules（模組/隱私）


### MOD-01 — Item is private: fields/methods/fns are private by default, pub to expose

**規則**：預設私有：struct 欄位、方法、模組內容預設 private，需 pub 才可跨模組存取。

**為什麼**：Rust 封裝以模組為界；私有不只是約定而是編譯器強制。

**對應錯誤碼**：E0603, E0616, E0624

**對應章節**：control-scope-and-privacy-with-modules-1

**相關規則**：MOD-02

**修法**

1. 把欄位/方法標為 pub

2. 提供公開的建構子與 getter 而非公開欄位

3. 確認模組宣告 pub mod


### MOD-02 — Unresolved import / use: check path, crate root, and feature gating

**規則**：無法解析的 use/import：路徑錯、忘了 crate 前綴、或功能未啟用。

**為什麼**：2018+ edition 使用絕對路徑需以 crate:: 開頭；外部 crate 在 Cargo.toml 聲明。

**對應錯誤碼**：E0432, E0433

**對應章節**：bringing-paths-into-scope-with-the-use-keyword-1

**相關規則**：MOD-01, TYP-02

**修法**

1. 以 crate:: 前綴寫絕對路徑

2. 確認 Cargo.toml 已聲明依賴並拼對名稱

3. 用 use 縮短路徑，並檢查 feature gate


## closures（閉包）


### CLS-01 — Closures capture by reference by default; use move to take ownership

**規則**：閉包預設以引用捕獲環境；需要取得擁有權（跨執行緒/離開作用域）時用 move。

**為什麼**：捕獲方式是閉包最常見的意外來源：move 會把用到的變數整個移入閉包。

**對應錯誤碼**：E0373

**對應章節**：closures-1

**相關規則**：BRW-01, CON-02

**修法**

1. 用 move 把變數移入閉包

2. 只捕獲需要的變數（縮小閉包體）

3. 跨執行緒 spawn 必須 move（見 CON-02）


### CLS-02 — Fn/FnMut/FnOnce: the closure's trait depends on how it captures

**規則**：閉包依捕獲方式自動實作 Fn（共享借用）/FnMut（可變借用）/FnOnce（消耗/移出）。

**為什麼**：把閉包傳給函數時，接收端期待的 Fn 系列 trait 決定了閉包能做什麼。

**對應錯誤碼**：E0525

**對應章節**：closures-1

**相關規則**：CLS-01

**修法**

1. 若閉包需 Fn 但內部改值，改用 RefCell 或讓其 FnMut

2. 降低接收端要求（改泛型 F: FnMut）

3. 不要移出捕獲值就能維持 Fn


## errors（錯誤處理）


### ERR-01 — Result must be used: #[must_use] warnings flag ignored Results

**規則**：Result 是 #[must_use]：忽略它會警告，可能吞掉錯誤。

**為什麼**：Rust 用型別表達可失敗性；忽略 Result 等於忽略錯誤。

**對應章節**：recoverable-errors-with-result-1

**相關規則**：ERR-02

**修法**

1. 用 ? 向上傳播錯誤

2. match / if let 顯式處理 Err

3. 確實要忽略時寫 let _ = ...（表達意圖）


### ERR-02 — The ? operator: unwraps Ok or returns Err early (requires compatible error types)

**規則**：? 運算子：Ok 解包、Err 提前回傳；要求函數回傳型別相容（或用 From 轉換）。

**為什麼**：? 是錯誤傳播的主要手段；型別不相容時需 map_err 或實作 From。

**對應錯誤碼**：E0277, E0308

**對應章節**：recoverable-errors-with-result-1

**相關規則**：ERR-01, TRT-01

**修法**

1. 用 .map_err(|e| ...) 轉換錯誤型別

2. 實作 From<InnerErr> for MyErr 讓 ? 自動轉換

3. 用 anyhow/thiserror（第三方）統一錯誤


### ERR-03 — panic! vs Result: unrecoverable vs recoverable errors

**規則**：panic! 用於不可恢復的錯誤（bug、不變量破壞）；可恢復錯誤用 Result 回傳。

**為什麼**：選擇正確的錯誤策略決定 API 品質；庫代碼盡量回傳 Result 而非 panic。

**對應章節**：to-panic-or-not-to-panic-1

**相關規則**：ERR-01

**修法**

1. 庫代碼回傳 Result

2. 原型與測試可用 unwrap/expect

3. 不變量檢查用 assert!/debug_assert!


## collections（集合）


### COL-01 — String vs &str: owned growable UTF-8 buffer vs borrowed slice

**規則**：String 是擁有的可增長 UTF-8 緩衝；&str 是借用的字串切片。

**為什麼**：兩者是最常混淆的型別對；函數參數慣例用 &str，回傳用 String。

**對應錯誤碼**：E0308, E0369

**對應章節**：storing-utf-8-encoded-text-with-strings-1

**相關規則**：TYP-01

**修法**

1. 參數用 &str，傳 String 時用 &s 或 s.as_str()

2. 回傳新字串用 String::from / to_string()

3. 索引 String 不能直接用 s[i]，用 .chars() 或切片（見 COL-03）


### COL-02 — Indexing a collection requires the right index type and Index trait

**規則**：索引集合需要正確的索引型別並實作 Index trait。

**為什麼**：Vec 用 usize 索引；HashMap 用鍵型別索引；用錯型別即編譯錯誤。

**對應錯誤碼**：E0277, E0308

**對應章節**：storing-lists-of-values-with-vectors-1

**相關規則**：TYP-01

**修法**

1. 用 usize 索引 Vec（usize::try_from 轉換）

2. HashMap 用對應鍵型別索引，或 .get(&key)

3. 改用 .get() 回傳 Option 更安全


### COL-03 — String indexing/bytes vs chars: Rust strings are UTF-8, not arrays of chars

**規則**：Rust 字串是 UTF-8 位元組序列，不是 char 陣列；不能直接 s[i]。

**為什麼**：UTF-8 是變長編碼；直接位元組索引可能切斷字元，所以被禁止。

**對應錯誤碼**：E0277

**對應章節**：storing-utf-8-encoded-text-with-strings-1

**相關規則**：COL-01

**修法**

1. 迭代字元用 s.chars()

2. 取位元組用 s.bytes() 或 s.as_bytes()

3. 切片必須落在字元邊界（用 char_indices 找邊界）


## smart-pointers（智慧指標）


### SPT-01 — Box<T>: heap allocation for sized indirection, recursion, and trait objects

**規則**：Box<T> 提供堆配置的間接層：遞迴型別、trait object、縮小 enum 大小。

**為什麼**：Box 是最基本的智慧指標；需要擁有的堆資料時用它。

**對應錯誤碼**：E0072

**對應章節**：using-boxt-to-point-to-data-on-the-heap-1

**相關規則**：SPT-02

**修法**

1. 遞迴型別用 Box 打斷無限大小

2. dyn Trait 用 Box<dyn Trait> 存放

3. 大值移入堆：Box::new(v)


### SPT-02 — Rc<T> (shared ownership) is not Send/Sync; RefCell adds runtime borrow checks

**規則**：Rc<T> 提供共享擁有權但非執行緒安全；Rc<RefCell<T>> 是常見的共享可變組合。

**為什麼**：單執行緒共享用 Rc；跨執行緒需 Arc + Mutex/RwLock（見 CON-03）。

**對應錯誤碼**：E0277

**對應章節**：rct-the-reference-counted-smart-pointer-1

**相關規則**：BRW-08, CON-01, SPT-01

**修法**

1. 單執行緒共享可變：Rc<RefCell<T>>

2. 跨執行緒：Arc<Mutex<T>>

3. 避免 Rc 循環造成洩漏（用 Weak 打破）


## concurrency（並行）


### CON-01 — Send + Sync: marker traits that govern safe cross-thread transfer and sharing

**規則**：Send：值可跨執行緒轉移；Sync：&T 可跨執行緒共享。自動推導，違反才報錯。

**為什麼**：這兩個 marker trait 是無畏並行的基石：不滿足的型別（Rc、RefCell、*mut）會被編譯器擋下。

**對應錯誤碼**：E0277

**對應章節**：extensible-concurrency-with-send-and-sync-1

**相關規則**：CON-02, SPT-02

**修法**

1. 跨執行緒共享用 Arc 而非 Rc

2. 用 Mutex/RwLock 包住共享可變狀態

3. 把非 Send 資料留在原執行緒


### CON-02 — spawn requires 'static + Send: move captured values into the thread

**規則**：執行緒 spawn 的閉包需 'static 且捕獲值需 Send；用 move 取得擁有權。

**為什麼**：執行緒可能比父作用域活得久，所以不能借用父堆疊。

**對應錯誤碼**：E0277, E0373, E0382

**對應章節**：using-threads-to-run-code-simultaneously-1

**相關規則**：CLS-01, CON-01

**修法**

1. 閉包加 move

2. 借用的資料改用 Arc 或 clone 後移入

3. 用 std::thread::scope 做限定生命週期的借用


### CON-03 — Shared mutable state: Arc<Mutex<T>>; lock() returns a Result<MutexGuard>

**規則**：共享可變狀態用 Arc<Mutex<T>>；lock() 回傳 Result，需處理毒化（poisoning）。

**為什麼**：Mutex 提供互斥；guard 離開作用域自動解鎖（RAII）。

**對應章節**：shared-state-concurrency-1

**相關規則**：CON-01, SPT-02

**修法**

1. let guard = mtx.lock().unwrap(); 離開作用域自動釋放

2. 毒化時用 lock().unwrap_or_else(|e| e.into_inner())

3. 避免長時間持鎖（把運算移出臨界區）


## unsafe（Unsafe）


### UNS-01 — Unsafe operations require an unsafe block; keep them small and documented

**規則**：解參考裸指標、呼叫 unsafe fn、存取可變 static、union 欄位等需包在 unsafe 區塊內。

**為什麼**：unsafe 不關閉檢查器，只是把安全義務移交給人；封裝成安全抽象是慣例。

**對應錯誤碼**：E0133

**對應章節**：unsafe-rust-1

**相關規則**：UNS-02

**修法**

1. 用 unsafe { ... } 包住不安全的操作

2. 把 unsafe 封進安全函數並寫 # Safety 契約

3. 優先尋找安全替代（標準庫大多已封裝）


### UNS-02 — Raw pointers (*const/*mut) are unsafe to dereference; prefer references

**規則**：裸指標解參考是 unsafe；能用引用與智慧指標就別用裸指標。

**為什麼**：裸指標不參與借用檢查、無生命週期；只用於 FFI 與極端效能場景。

**對應錯誤碼**：E0133

**對應章節**：unsafe-rust-1

**相關規則**：UNS-01

**修法**

1. 用引用/Box/Rc 取代裸指標

2. FFI 邊界才用裸指標並隔離

3. 解參考放在最小 unsafe 區塊並註明不變量


## macros（巨集）


### MAC-01 — Macro not found: check use/import, #[macro_use], and edition paths

**規則**：找不到巨集：巨集需在作用域內（use、#[macro_use]、或路徑呼叫）。

**為什麼**：巨集展開早於名稱解析；作用域規則與函數略有不同。

**對應錯誤碼**：E0433

**對應章節**：macros-1

**相關規則**：MOD-02

**修法**

1. use 該巨集或用完整路徑呼叫（crate::mac!）

2. 2018+ 用 use crate::mac 引入

3. 檢查 #[macro_export] 與 crate 名稱


### MAC-02 — Declarative vs procedural macros: macro_rules! is pattern-based; proc-macros are Rust programs

**規則**：宣告式巨集 macro_rules! 以模式匹配代碼；程序巨集（derive/attribute/function-like）是編譯期的 Rust 程式。

**為什麼**：選對巨集種類：簡單語法糖用 macro_rules!，需要型別資訊或複雜轉換用 proc-macro。

**對應章節**：macros-1

**相關規則**：MAC-01

**修法**

1. 重複模式用 macro_rules! 的 $(...)*

2. 需要 AST/型別資訊改用 derive 巨集

3. proc-macro 需獨立 crate（proc-macro = true）


## async（非同步）


### ASY-01 — Futures must be polled; .await yields until ready (non-blocking)

**規則**：Future 惰性：不 .await 就不執行；.await 在未就緒時讓出（非阻塞）。

**為什麼**：async 是協作式排程；理解惰性避免「忘了 await 導致不執行」。

**對應章節**：futures-and-the-async-syntax-1

**相關規則**：ASY-02

**修法**

1. 記得 .await 每個 Future

2. 用 join!/select! 並行等待

3. 忘 await 會有 #[warn(unused_must_use)]


### ASY-02 — Borrowing across .await: held guards/refs must not cross await points (Send + no borrowed locals)

**規則**：跨 .await 持有非 Send 的值（如 MutexGuard、借用局部變數）會使 Future 非 Send 或觸發生命週期錯誤。

**為什麼**：await 點會讓執行緒切換；跨點資料必須可轉移。

**對應錯誤碼**：E0277, E0373

**對應章節**：futures-and-the-async-syntax-1

**相關規則**：ASY-01, CON-01

**修法**

1. 把 lock guard 收進小作用域再 await

2. 跨 await 的資料用擁有型別（clone/Arc）

3. 重構讓 await 不跨越持鎖/借用區間


## misc（其他）


### MSC-01 — Trait objects need dyn: dyn Trait makes dynamic dispatch explicit

**規則**：trait object 需寫 dyn Trait；dyn 使動態分派顯式化。

**為什麼**：省略 dyn 是語法錯誤；dyn 提醒你這是一次 vtable 分派。

**對應錯誤碼**：E0782

**對應章節**：using-trait-objects-to-abstract-over-shared-behavior-1

**相關規則**：TRT-04

**修法**

1. Box<dyn Trait> / &dyn Trait

2. 需要靜態分派時改泛型


### MSC-02 — Missing main/entry point or wrong crate type

**規則**：缺少 main 函數、或 crate 型別（lib/bin）與入口不匹配。

**為什麼**：二進制 crate 需要 main；程式庫 crate 不需要。

**對應錯誤碼**：E0601

**對應章節**：hello-world-1

**修法**

1. 補上 fn main() {}

2. lib crate 移除 main 或改 bin

3. 檢查 Cargo.toml 的 [[bin]]/src 佈局
