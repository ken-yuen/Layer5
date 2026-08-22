// poll.go：stat 輪詢後備後端（全平台可用；非 Linux 的預設後端）。
//
// 取捨：輪詢間隔預設 250ms——對「代理改檔 → 護欄反應」的秒級迴路足夠，
// 且零依賴、跨平台。mtime+size 差分；rename 以 remove+create 近似（已知限制，文檔註明）。
// 事件一律攜帶絕對路徑（多 root 場景由上層按前綴歸屬專案）。

package watch

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type pollBackend struct {
	mu       sync.Mutex
	roots    []string
	exclDirs map[string]bool

	interval time.Duration
	events   chan Event
	errors   chan error
	closed   chan struct{}
	once     sync.Once
	started  sync.Once
}

// NewPollBackend 建立輪詢後端（interval <= 0 → 250ms）。
func NewPollBackend(interval time.Duration) Backend {
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	return &pollBackend{
		interval: interval,
		events:   make(chan Event, 256),
		errors:   make(chan error, 16),
		closed:   make(chan struct{}),
		exclDirs: map[string]bool{},
	}
}

func (p *pollBackend) Name() string { return "poll" }

func (p *pollBackend) Add(root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return os.ErrNotExist
	}
	p.mu.Lock()
	p.roots = append(p.roots, abs)
	p.mu.Unlock()
	return nil
}

func (p *pollBackend) Events() <-chan Event { return p.events }
func (p *pollBackend) Errors() <-chan error { return p.errors }

func (p *pollBackend) Close() error {
	p.once.Do(func() { close(p.closed) })
	return nil
}

// Start 啟動輪詢循環（由 NewWatcher 統一調用；冪等）。
func (p *pollBackend) Start() {
	p.started.Do(func() { go p.run() })
}

func (p *pollBackend) run() {
	snap := map[string]fileStat{}
	first := true
	tick := time.NewTicker(p.interval)
	defer tick.Stop()
	for {
		select {
		case <-p.closed:
			return
		case <-tick.C:
		}
		p.mu.Lock()
		roots := append([]string(nil), p.roots...)
		p.mu.Unlock()
		next := map[string]fileStat{}
		for _, root := range roots {
			_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil // 容錯：單點錯誤不中止輪詢
				}
				name := d.Name()
				if d.IsDir() {
					if p.exclDir(name) && path != root {
						return filepath.SkipDir
					}
					return nil
				}
				if !d.Type().IsRegular() {
					return nil
				}
				st, err := d.Info()
				if err != nil {
					return nil
				}
				next[path] = fileStat{size: st.Size(), mtime: st.ModTime().UnixNano()}
				return nil
			})
		}
		if first {
			first = false
			snap = next
			continue
		}
		for path, st := range next {
			old, ok := snap[path]
			switch {
			case !ok:
				p.emit(Event{Op: OpCreate, Path: path})
			case old != st:
				p.emit(Event{Op: OpWrite, Path: path})
			}
		}
		for path := range snap {
			if _, ok := next[path]; !ok {
				p.emit(Event{Op: OpRemove, Path: path})
			}
		}
		snap = next
	}
}

type fileStat struct {
	size  int64
	mtime int64
}

func (p *pollBackend) exclDir(name string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.exclDirs) == 0 {
		for _, d := range DefaultExcludeDirs {
			p.exclDirs[d] = true
		}
	}
	return p.exclDirs[name]
}

func (p *pollBackend) emit(e Event) {
	select {
	case p.events <- e:
	default: // 滿了丟棄（觀察層容錯；錯誤通道提示）
		select {
		case p.errors <- errPollOverflow:
		default:
		}
	}
}

var errPollOverflow = &pollError{"poll backend event queue overflow"}

type pollError struct{ msg string }

func (e *pollError) Error() string { return e.msg }
