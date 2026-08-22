// Package kb 是 YKC 的「嵌入式唯讀知識庫 + 代理上下文引擎」。
//
// 它把 Rust 官方教學文檔、rustc 全部編譯錯誤碼（518 條，含錯誤範例與正解）、
// 以及「Rust 規則抽象」（ownership/borrowing/lifetime/… 54 條規則）組織成
// 內容定址（content-addressed）的「上下文原子」（Atom），並在其上提供：
//
//   - 精準檢索（inverted index + BM25 + char-shingle 模糊相似 + 精確碼查詢）
//   - 代理上下文緩存（LRU，keyed by 查詢指紋 + 資料版本）
//   - 依賴項圖（原子間 Refs 邊 + 反向索引；BFS 上下文展開；環偵測）
//   - 上下文原子化（每條知識 = 一個自足原子；預算截斷組裝）
//
// 全部零外部依賴（僅 Go 標準庫），與 YKC「零依賴 / 靜態二進制」紀律一致。
// 資料以 go:embed 編入二進制（嵌入式），載入後唯讀（無任何寫入 API）；
// 亦支援 Build/Save 產出單一不可變 blob（磁碟上的唯讀資料庫），Open 時
// 以 sha256 校驗防竄改——與 YKC 的 hash 串鏈紀律同源。
package kb

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Kind 是上下文原子的種類。
type Kind string

const (
	KindError Kind = "error"     // rustc 編譯錯誤碼卡（含錯誤範例 + 正解）
	KindRule  Kind = "rule"      // Rust 規則抽象（領域規則）
	KindBook  Kind = "book"      // Rust 官方教學文檔章節
	KindPart  Kind = "book-part" // 教學文檔「部」（如 Understanding Ownership）
	KindTOC   Kind = "toc"       // 教學文檔目錄
)

// Section 是書本章節下的小節標題（原子化後僅保留結構）。
type Section struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Level int    `json:"level"`
}

// Atom 是一條自足的最小知識單元（上下文原子）。Refs 是「依賴項」——
// 指本原子所引用/依賴的其他原子 ID（構成依賴項圖）。
type Atom struct {
	ID       string    `json:"id"`
	Kind     Kind      `json:"kind"`
	Code     string    `json:"code,omitempty"`   // E0382 / BRW-01 / 章節 id
	Domain   string    `json:"domain,omitempty"` // 規則領域（ownership/borrowing/…）
	Title    string    `json:"title"`
	ZH       string    `json:"zh,omitempty"` // 規則的中文陳述
	Body     string    `json:"body,omitempty"`
	Why      string    `json:"why,omitempty"`      // 規則的「為什麼」
	Err      string    `json:"err,omitempty"`      // 錯誤範例（會編譯失敗的程式）
	Fix      string    `json:"fix,omitempty"`      // 正解（修正後的程式）
	Fixes    []string  `json:"fixes,omitempty"`    // 規則修法菜單
	Tags     []string  `json:"tags,omitempty"`     // 檢索標籤
	Refs     []string  `json:"refs,omitempty"`     // 依賴的其他原子 ID
	Source   string    `json:"source,omitempty"`   // 出處 URL
	Sections []Section `json:"sections,omitempty"` // 章節小節結構
}

// contentHash 對原子的「內容」（不含 ID 與 Refs）做內容定址。
// 結構體欄位以宣告順序序列化，json.Marshal 具確定性 → 同一內容永遠同一 ID。
func (a *Atom) contentHash() string {
	type c struct {
		Kind     Kind      `json:"kind"`
		Code     string    `json:"code,omitempty"`
		Domain   string    `json:"domain,omitempty"`
		Title    string    `json:"title"`
		ZH       string    `json:"zh,omitempty"`
		Body     string    `json:"body,omitempty"`
		Why      string    `json:"why,omitempty"`
		Err      string    `json:"err,omitempty"`
		Fix      string    `json:"fix,omitempty"`
		Fixes    []string  `json:"fixes,omitempty"`
		Tags     []string  `json:"tags,omitempty"`
		Source   string    `json:"source,omitempty"`
		Sections []Section `json:"sections,omitempty"`
	}
	b, _ := json.Marshal(c{
		Kind: a.Kind, Code: a.Code, Domain: a.Domain, Title: a.Title, ZH: a.ZH,
		Body: a.Body, Why: a.Why, Err: a.Err, Fix: a.Fix, Fixes: a.Fixes,
		Tags: a.Tags, Source: a.Source, Sections: a.Sections,
	})
	sum := sha256.Sum256(b)
	return "kb-" + hex.EncodeToString(sum[:8])
}

// Size 回傳原子的「渲染大小」估值（bytes），用於上下文預算截斷。
func (a *Atom) Size() int {
	n := len(a.ID) + len(a.Title) + len(a.Body) + len(a.ZH) + len(a.Why) +
		len(a.Err) + len(a.Fix) + len(a.Source) + 8
	for _, f := range a.Fixes {
		n += len(f) + 2
	}
	for _, t := range a.Tags {
		n += len(t) + 1
	}
	for _, s := range a.Sections {
		n += len(s.ID) + len(s.Title) + 4
	}
	return n
}
