// Package borrow 是 YKC 的 L5 接線層：借用/生命週期幾何解釋。
//
// 職能定位（決策 D22）：
//   - 引擎 = vendored ChordLaw（l5/chordlaw，Datalog 借用檢查器，26/26 rustc oracle 一致）；
//   - 本包把引擎的 --json 報告轉為「代理原生消費品」：
//     文字拓撲（ASCII 區間圖）＋ 區間代數事實 ＋ 幾何規則卡 ＋ 修法菜單；
//   - 判定權不轉移：一切輸出都是 explanation，判定以 rustc 為準；
//   - L5 是可選能力：python3 缺席時如實降級（規則卡仍可用——純靜態）。
package borrow

// SchemaV1 是本包支援的 ChordLaw JSON schema 版本。
const SchemaV1 = "chordlaw.report/v1"

// Report 對應 chordlaw.py --json 的單檔輸出。
type Report struct {
	Schema   string   `json:"schema"`
	File     string   `json:"file"`
	Liveness string   `json:"liveness"`
	Verdict  string   `json:"verdict"` // PASS | FAIL | FAIL (前端)
	Errors   []Error  `json:"errors"`
	Regions  []Region `json:"regions"`
}

// Error 是引擎判出的一條違規（E00 前端錯誤時 Fn/Proof 可能為空）。
type Error struct {
	Code     string   `json:"code"` // E00–E10
	Stmt     string   `json:"stmt"` // s1…
	Fn       string   `json:"fn,omitempty"`
	StmtText string   `json:"stmt_text"`
	Message  string   `json:"message"`
	Proof    []string `json:"proof,omitempty"` // 證明樹（已渲染為行）
}

// Region 是一條借貸的活躍區間 [Start, End]（陳述 ID）。
type Region struct {
	Fn       string `json:"fn"`
	Ref      string `json:"ref"`      // 參考名（a）
	Referent string `json:"referent"` // 被借者路徑（x / x.f）
	Kind     string `json:"kind"`     // sh | mut
	Start    string `json:"start"`    // sN（出借點）
	End      string `json:"end"`      // sM（終端使用點）
}
