// Package watch 是 YKC 的常駐監看層（ykc serve 的事件源）。
//
// 設計（對應 YKC_04 §3「原子監聽」與 YKC_14）：
//   - 事件驅動優先：Linux 以 stdlib syscall inotify（零外部依賴、CGO_ENABLED=0
//     相容）；其他平台退回 stat 輪詢後備（mtime+size 差分）。
//   - 遞迴監看：新目錄出現即動態加 watch，並補掃一次（消除 add 前的競態漏報）。
//   - 去抖（debounce）：同一路徑在窗口內多次變更合併為一條「最後操作」；
//     create+remove 互相抵銷；輸出按路徑排序——同一輸入永遠同一輸出。
//   - 過濾：只上拋 Rust 相關檔案（.rs/.toml/.lock）；**必排 .ykc**（YKC 自身
//     寫帳本/收據絕不可觸發自己的監看——否則事件回環）與 .git/target 等。
//
// 事件模型刻意最小：Op + Path。雜湊/快照由上層（monitor/serve）按需補，
// 監看層不重活（YKC「唯一實作」紀律：快照屬 internal/monitor）。
package watch

import (
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Op 是檔案操作分類。
type Op string

const (
	OpCreate Op = "create"
	OpWrite  Op = "write"
	OpRemove Op = "remove"
	OpRename Op = "rename"
)

// Event 是一次（已過濾、未去抖的）原始檔案事件。
type Event struct {
	Op    Op     `json:"op"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir,omitempty"`
}

// Backend 是平台監看後端（inotify / poll）。
type Backend interface {
	// Add 遞迴地把 root（及其全部子目錄）納入監看。
	Add(root string) error
	// Events 回傳原始事件流（未過濾、未去抖；含目錄事件）。
	Events() <-chan Event
	// Errors 回傳後端錯誤（溢出、不可恢復錯誤）。
	Errors() <-chan error
	// Name 回傳後端名（"inotify" / "poll"）。
	Name() string
	Close() error
}

// DefaultExcludeDirs 與 monitor.DefaultExcludeDirs 保持一致語義
// （獨立宣告避免包循環；兩處修改須同步——見 YKC_14 紀律註記）。
var DefaultExcludeDirs = []string{
	".git", ".ykc", "target", "node_modules", "dist", "build", "coverage",
	".next", ".turbo", ".cache", "__pycache__",
}

// DefaultExts 是預設上拋的副檔名（Rust 工作區相關）。
var DefaultExts = []string{".rs", ".toml", ".lock"}

// Filter 決定哪些路徑值得上拋。
type Filter struct {
	Exts        []string // 副檔名白名單（含點；預設 DefaultExts）
	ExcludeDirs []string // 目錄名黑名單（路徑任一段命中即排除；預設 DefaultExcludeDirs）
	ExactNames  []string // 精確檔名白名單（可選；副名單外補充）
}

// DefaultFilter 回傳預設過濾器。
func DefaultFilter() Filter {
	return Filter{Exts: append([]string(nil), DefaultExts...), ExcludeDirs: append([]string(nil), DefaultExcludeDirs...)}
}

// Allow 判斷路徑是否上拋（目錄一律不上拋——後端內部自用）。
func (f Filter) Allow(path string, isDir bool) bool {
	if isDir {
		return false
	}
	excl := f.ExcludeDirs
	if len(excl) == 0 {
		excl = DefaultExcludeDirs
	}
	for _, seg := range strings.Split(filepath.ToSlash(path), "/") {
		for _, d := range excl {
			if seg == d {
				return false
			}
		}
	}
	ext := strings.ToLower(filepath.Ext(path))
	exts := f.Exts
	if len(exts) == 0 {
		exts = DefaultExts
	}
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	base := filepath.Base(path)
	for _, n := range f.ExactNames {
		if base == n {
			return true
		}
	}
	return false
}

// ---------- 去抖器 ----------

// debItem 是去抖窗口內單一路徑的聚合狀態。
type debItem struct {
	firstSeen time.Time
	lastSeen  time.Time
	op        Op
}

// Debouncer 把事件流按「窗口內最後操作勝」合併；時鐘可注入（測試確定性）。
type Debouncer struct {
	Window time.Duration
	now    func() time.Time

	mu    sync.Mutex
	items map[string]*debItem
}

// NewDebouncer 建立去抖器；window <= 0 時取 300ms。
func NewDebouncer(window time.Duration, now func() time.Time) *Debouncer {
	if window <= 0 {
		window = 300 * time.Millisecond
	}
	if now == nil {
		now = time.Now
	}
	return &Debouncer{Window: window, now: now, items: map[string]*debItem{}}
}

// Add 記錄一個事件（合併規則：後到操作勝；remove 後 create 視為 write；
// 窗口內 create→remove 抵銷）。
func (d *Debouncer) Add(e Event) {
	d.mu.Lock()
	defer d.mu.Unlock()
	t := d.now()
	it, ok := d.items[e.Path]
	if !ok {
		// 窗口外的新起點
		d.items[e.Path] = &debItem{firstSeen: t, lastSeen: t, op: e.Op}
		return
	}
	it.lastSeen = t
	switch {
	case e.Op == OpRemove && it.op == OpCreate:
		delete(d.items, e.Path) // 建了又刪 → 抵銷
	case e.Op == OpCreate && it.op == OpRemove:
		it.op = OpWrite // 刪了又建 → 內容等效改寫
	default:
		it.op = e.Op
	}
}

// Due 回傳是否有路徑已到窗口邊界。
func (d *Debouncer) Due() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	t := d.now()
	for _, it := range d.items {
		if t.Sub(it.firstSeen) >= d.Window {
			return true
		}
	}
	return false
}

// Flush 收割全部已到期項（按路徑排序輸出）；未到期項保留。
func (d *Debouncer) Flush() []Event {
	d.mu.Lock()
	defer d.mu.Unlock()
	t := d.now()
	var out []Event
	var done []string
	for p, it := range d.items {
		if t.Sub(it.firstSeen) >= d.Window {
			out = append(out, Event{Op: it.op, Path: p})
			done = append(done, p)
		}
	}
	for _, p := range done {
		delete(d.items, p)
	}
	sortEvents(out)
	return out
}

// Pending 回傳等待中路徑數（狀態觀察用）。
func (d *Debouncer) Pending() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.items)
}

func sortEvents(es []Event) {
	for i := 1; i < len(es); i++ {
		for j := i; j > 0 && es[j].Path < es[j-1].Path; j-- {
			es[j], es[j-1] = es[j-1], es[j]
		}
	}
}
