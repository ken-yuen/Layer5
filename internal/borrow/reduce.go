package borrow

// ============================================================
// 模板法歸約（P1）：rustc borrow 錯誤碼 → canonical .cl 最小樣例。
//
// 定位：真實 Rust → .cl 的 AST 級歸約是 P2（syn 前端）範圍；
// 本期以「錯誤碼 → 幾何類 → 教科書樣例」給代理展示該類錯誤的
// 典型幾何形狀與修法。樣例內嵌於二進制（決策 B3: 自足, 不讀檔）。
// 每個模板都經 vendored ChordLaw 驗證會產生預期錯誤碼
// （analyzer_test.go golden 鎖定）。
// ============================================================

// Template 是一個 canonical 錯誤樣例。
type Template struct {
	RustcCode string // 觸發它的 rustc 錯誤碼（代表）
	ChordCode string // ChordLaw 判出的錯誤碼
	Title     string
	CL        string // .cl 源
}

// Templates 按 rustc 錯誤碼索引的 canonical 樣例。
// 注意: 一碼可對多幾何類（如 E0502 既可是 E01 也可是 E09）; 取最高頻形態。
var Templates = []Template{
	{
		RustcCode: "E0502", ChordCode: "E01", Title: "共享借用活躍期間建立 mut 借用（紅弧交越）",
		CL: `fn f() {
  let x
  let a = &x
  use a
  let b = &mut x
  use a
  use b
}`,
	},
	{
		RustcCode: "E0499", ChordCode: "E01", Title: "兩個 mut 借用重疊（紅弧交越）",
		CL: `fn f() {
  let x
  let a = &mut x
  let b = &mut x
  use a
  use b
}`,
	},
	{
		RustcCode: "E0506", ChordCode: "E02", Title: "借用活躍期間寫入被借者",
		CL: `fn f() {
  let x
  let a = &x
  set x
  use a
}`,
	},
	{
		RustcCode: "E0503", ChordCode: "E03", Title: "mut 借用活躍期間直接讀取被借者",
		CL: `fn f() {
  let x
  let a = &mut x
  use x
  use a
}`,
	},
	{
		RustcCode: "E0382", ChordCode: "E06", Title: "move 後使用",
		CL: `fn f() {
  let x
  mv x
  use x
}`,
	},
	{
		RustcCode: "E0505", ChordCode: "E07", Title: "借用活躍期間 move 被借者",
		CL: `fn f() {
  let x
  let a = &x
  mv x
  use a
}`,
	},
	{
		// E0597 的幾何本質「被借者活得不夠久」在 .cl 玩具語言中以
		// 「借用活躍期間被借者被銷毀」呈現（E08; 引擎另會報 E05 use-after-drop）。
		RustcCode: "E0597", ChordCode: "E08", Title: "被借者在借用期間被銷毀（活得不夠久）",
		CL: `fn f() {
  let x
  let a = &x
  dp x
  use a
}`,
	},
	{
		RustcCode: "E0106", ChordCode: "E10", Title: "回傳局部變數的引用（引用逸出）",
		CL: `fn f() {
  let x
  let a = &x
  ret a
}`,
	},
}

// TemplateFor 回傳最匹配 rustc 錯誤碼的樣例（無則 nil）。
// E0515/E0716 併入 E0106 幾何類; E0594/E0507 併入 E0382/E0505 類。
func TemplateFor(rustcCode string) *Template {
	alias := map[string]string{
		"E0515": "E0106",
		"E0716": "E0106",
		"E0594": "E0382",
		"E0507": "E0505",
	}
	if a, has := alias[rustcCode]; has {
		rustcCode = a
	}
	for i := range Templates {
		if Templates[i].RustcCode == rustcCode {
			return &Templates[i]
		}
	}
	return nil
}
