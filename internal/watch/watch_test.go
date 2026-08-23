package watch

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------- Debouncer ----------

func TestDebouncerCoalesce(t *testing.T) {
	now := time.Unix(1000, 0)
	d := NewDebouncer(300*time.Millisecond, func() time.Time { return now })
	d.Add(Event{Op: OpCreate, Path: "a.rs"})
	now = now.Add(50 * time.Millisecond)
	d.Add(Event{Op: OpWrite, Path: "a.rs"})
	now = now.Add(50 * time.Millisecond)
	d.Add(Event{Op: OpWrite, Path: "b.rs"})
	if d.Pending() != 2 {
		t.Fatalf("pending = %d, want 2", d.Pending())
	}
	if d.Due() {
		t.Fatal("nothing due yet (window=300ms, elapsed=100ms)")
	}
	now = now.Add(201 * time.Millisecond) // a.rs firstSeen+301ms
	if !d.Due() {
		t.Fatal("a.rs should be due")
	}
	got := d.Flush()
	if len(got) != 1 || got[0].Path != "a.rs" || got[0].Op != OpWrite {
		t.Fatalf("expected a.rs write, got %v", got)
	}
	// b.rs 未到期應保留
	if d.Pending() != 1 {
		t.Fatalf("b.rs should remain pending, got %d", d.Pending())
	}
}

func TestDebouncerCreateRemoveCancel(t *testing.T) {
	now := time.Unix(2000, 0)
	d := NewDebouncer(100*time.Millisecond, func() time.Time { return now })
	d.Add(Event{Op: OpCreate, Path: "tmp.rs"})
	d.Add(Event{Op: OpRemove, Path: "tmp.rs"})
	if d.Pending() != 0 {
		t.Fatal("create+remove within window should cancel")
	}
}

func TestDebouncerRemoveThenCreateIsWrite(t *testing.T) {
	now := time.Unix(3000, 0)
	d := NewDebouncer(100*time.Millisecond, func() time.Time { return now })
	d.Add(Event{Op: OpRemove, Path: "x.rs"})
	d.Add(Event{Op: OpCreate, Path: "x.rs"})
	now = now.Add(150 * time.Millisecond)
	got := d.Flush()
	if len(got) != 1 || got[0].Op != OpWrite {
		t.Fatalf("remove+create should coalesce to write, got %v", got)
	}
}

// ---------- Filter ----------

func TestFilterAllow(t *testing.T) {
	f := DefaultFilter()
	cases := []struct {
		path string
		want bool
	}{
		{"src/main.rs", true},
		{"Cargo.toml", true},
		{"Cargo.lock", true},
		{"/abs/path/lib.rs", true},
		{"/abs/.ykc/ledger.jsonl", false}, // YKC 自身寫入——必須排除（防回環）
		{"/abs/target/debug/x.rs", false}, // build 產物
		{"/abs/.git/HEAD", false},         // .git 且副檔名不符
		{"/abs/src/main.py", false},       // 非 Rust 副檔名
		{"/abs/target.rs", true},          // 檔名為 target 的 .rs 不誤傷
	}
	for _, c := range cases {
		if got := f.Allow(c.path, false); got != c.want {
			t.Errorf("Allow(%q) = %v, want %v", c.path, got, c.want)
		}
	}
	if f.Allow("somedir", true) {
		t.Error("directories must never pass filter")
	}
}

// ---------- 後端整合（Linux 沙盒實測 inotify；poll 後端全平台） ----------

func waitBatch(t *testing.T, w *Watcher, timeout time.Duration) []Event {
	t.Helper()
	select {
	case b := <-w.Batches():
		return b
	case <-time.After(timeout):
		t.Fatal("timeout waiting for batch")
		return nil
	}
}

func containsEvent(es []Event, op Op, suffix string) bool {
	for _, e := range es {
		if e.Op == op && filepath.Base(e.Path) == suffix {
			return true
		}
	}
	return false
}

func TestInotifyBackendEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := t.TempDir()
	b, err := NewInotifyBackend()
	if err != nil {
		t.Skipf("inotify unavailable: %v", err)
	}
	w, err := NewWatcher(Config{Roots: []string{dir}, Backend: b, Debounce: 150 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// ① 新建 .rs → create/write
	if err := os.WriteFile(filepath.Join(dir, "main.rs"), []byte("fn main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	batch := waitBatch(t, w, 5*time.Second)
	if !containsEvent(batch, OpCreate, "main.rs") && !containsEvent(batch, OpWrite, "main.rs") {
		t.Fatalf("expected main.rs create/write, got %v", batch)
	}
	// ② 修改 → write
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "main.rs"), []byte("fn main() { let x = 1; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	batch = waitBatch(t, w, 5*time.Second)
	if !containsEvent(batch, OpWrite, "main.rs") {
		t.Fatalf("expected main.rs write, got %v", batch)
	}
	// ③ rename 應在 cookie 配對後保留 rename 語義，而不是因 read(2)
	// 分段被過早拆成 remove/create。
	if err := os.Rename(filepath.Join(dir, "main.rs"), filepath.Join(dir, "renamed.rs")); err != nil {
		t.Fatal(err)
	}
	batch = waitBatch(t, w, 5*time.Second)
	if !containsEvent(batch, OpRename, "renamed.rs") {
		t.Fatalf("expected paired rename, got %v", batch)
	}
	// ④ 子目錄（watch 動態遞迴）
	sub := filepath.Join(dir, "src")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "lib.rs"), []byte("pub fn f() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	batch = waitBatch(t, w, 5*time.Second)
	// create+後續 modify 在窗口內合併為 write（最後操作勝）——兩者皆可接受。
	if !containsEvent(batch, OpCreate, "lib.rs") && !containsEvent(batch, OpWrite, "lib.rs") {
		t.Fatalf("expected src/lib.rs create/write, got %v", batch)
	}
	// ④ .ykc 內寫檔 → 必須被過濾（防回環）
	ykcDir := filepath.Join(dir, ".ykc")
	if err := os.MkdirAll(ykcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ykcDir, "ledger.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 同時改一個正規檔案驅動一個 batch；.ykc 不應出現在其中
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "main.rs"), []byte("fn main() { let y = 2; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	batch = waitBatch(t, w, 5*time.Second)
	for _, e := range batch {
		if filepath.Base(filepath.Dir(e.Path)) == ".ykc" {
			t.Fatalf(".ykc events must never surface: %v", batch)
		}
	}
	if !containsEvent(batch, OpWrite, "main.rs") {
		t.Fatalf("expected main.rs write, got %v", batch)
	}
}

func TestPollBackendEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := t.TempDir()
	w, err := NewWatcher(Config{
		Roots:    []string{dir},
		Backend:  NewPollBackend(40 * time.Millisecond),
		Debounce: 120 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// 等首輪基線掃描完成（2×tick）再寫檔，否則 create 會被基線吞掉。
	time.Sleep(120 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	batch := waitBatch(t, w, 5*time.Second)
	// 輪詢可能在 WriteFile 的 open/close 之間掃到半成品（create 後接 write）——
	// 去抖後最後操作勝，create 與 write 皆可接受。
	if !containsEvent(batch, OpCreate, "Cargo.toml") && !containsEvent(batch, OpWrite, "Cargo.toml") {
		t.Fatalf("expected Cargo.toml create/write, got %v", batch)
	}
	// 修改 → write
	time.Sleep(80 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"y\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	batch = waitBatch(t, w, 5*time.Second)
	if !containsEvent(batch, OpWrite, "Cargo.toml") {
		t.Fatalf("expected Cargo.toml write, got %v", batch)
	}
	// 刪除 → remove
	time.Sleep(80 * time.Millisecond)
	if err := os.Remove(filepath.Join(dir, "Cargo.toml")); err != nil {
		t.Fatal(err)
	}
	batch = waitBatch(t, w, 5*time.Second)
	if !containsEvent(batch, OpRemove, "Cargo.toml") {
		t.Fatalf("expected Cargo.toml remove, got %v", batch)
	}
}

func TestNewWatcherRejectsEmptyRoots(t *testing.T) {
	if _, err := NewWatcher(Config{}); err == nil {
		t.Fatal("expected error for empty roots")
	}
}

type testBackend struct {
	events chan Event
	errors chan error
}

func (b *testBackend) Add(string) error     { return nil }
func (b *testBackend) Events() <-chan Event { return b.events }
func (b *testBackend) Errors() <-chan error { return b.errors }
func (b *testBackend) Name() string         { return "test" }
func (b *testBackend) Close() error         { return nil }

func TestWatcherReportsBackendErrorAndClosesStreams(t *testing.T) {
	backend := &testBackend{events: make(chan Event), errors: make(chan error, 1)}
	w, err := NewWatcher(Config{Roots: []string{"ignored"}, Backend: backend, Debounce: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	backend.errors <- errors.New("queue overflow")
	select {
	case got := <-w.Errors():
		if got == nil || got.Error() != "queue overflow" {
			t.Fatalf("unexpected watcher error: %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("watcher error was not surfaced")
	}
	if msg, count := w.LastError(); msg != "queue overflow" || count != 1 {
		t.Fatalf("last error = %q/%d", msg, count)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case _, ok := <-w.Batches():
		if ok {
			t.Fatal("batches channel must close after watcher Close")
		}
	case <-time.After(time.Second):
		t.Fatal("batches channel did not close")
	}
	select {
	case _, ok := <-w.Errors():
		if ok {
			t.Fatal("errors channel must close after watcher Close")
		}
	case <-time.After(time.Second):
		t.Fatal("errors channel did not close")
	}
}
