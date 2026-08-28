// watcher.go：Watcher 把「後端事件流 → 過濾 → 去抖 → 批次」組合成單一抽象，
// 供 ykc serve 消費。這是監看層的唯一對外入口。

package watch

import (
	"fmt"
	"sync"
	"time"
)

// Config 是 Watcher 配置。
type Config struct {
	Roots        []string      // 要監看的專案根（各自遞迴）
	Filter       Filter        // 零值 → DefaultFilter
	Debounce     time.Duration // 零值 → 300ms
	PollInterval time.Duration // 輪詢後端用；零值 → 250ms
	Backend      Backend       // 可注入（測試）；零值 → 平台預設
}

// Watcher 是已啟動的監看器。
type Watcher struct {
	cfg     Config
	backend Backend

	batchMu  sync.Mutex
	batches  chan []Event
	errors   chan error
	done     chan struct{}
	loopDone chan struct{}
	once     sync.Once

	lastBatch []Event // 最近一次輸出批次（觀察用；有界=1）

	errorMu    sync.Mutex
	lastError  string
	errorCount uint64
}

// NewWatcher 建立並啟動監看器（冪等起點：調用即運行）。
func NewWatcher(cfg Config) (*Watcher, error) {
	if len(cfg.Roots) == 0 {
		return nil, fmt.Errorf("watch: at least one root is required")
	}
	if cfg.Debounce <= 0 {
		cfg.Debounce = 300 * time.Millisecond
	}
	if cfg.Filter.Exts == nil && cfg.Filter.ExactNames == nil {
		cfg.Filter = DefaultFilter()
	}
	backend := cfg.Backend
	if backend == nil {
		if b, err := NewBackend(); err == nil {
			backend = b
		} else {
			backend = NewPollBackend(cfg.PollInterval)
		}
	}
	for _, r := range cfg.Roots {
		if err := backend.Add(r); err != nil {
			backend.Close()
			return nil, fmt.Errorf("watch: add root %s: %w", r, err)
		}
	}
	w := &Watcher{
		cfg:      cfg,
		backend:  backend,
		batches:  make(chan []Event, 64),
		errors:   make(chan error, 16),
		done:     make(chan struct{}),
		loopDone: make(chan struct{}),
	}
	if starter, ok := backend.(interface{ Start() }); ok {
		starter.Start()
	}
	go w.loop()
	return w, nil
}

// Batches 回傳去抖後的事件批次流。
func (w *Watcher) Batches() <-chan []Event { return w.batches }

// BackendName 回傳後端名稱（inotify/poll/自訂）。
func (w *Watcher) BackendName() string { return w.backend.Name() }

// LastBatch 回傳最近一次輸出批次（快照拷貝）。
func (w *Watcher) LastBatch() []Event {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	return append([]Event(nil), w.lastBatch...)
}

// Errors exposes watcher-level errors after they have been recorded. The
// backend error channel is consumed internally so an overflow cannot silently
// block the backend; callers may use this stream to trigger a rescan/rebuild.
func (w *Watcher) Errors() <-chan error { return w.errors }

// LastError returns the most recent backend or watcher queue error and count.
func (w *Watcher) LastError() (string, uint64) {
	w.errorMu.Lock()
	defer w.errorMu.Unlock()
	return w.lastError, w.errorCount
}

func (w *Watcher) recordError(err error) {
	if err == nil {
		return
	}
	w.errorMu.Lock()
	w.lastError = err.Error()
	w.errorCount++
	w.errorMu.Unlock()
}

// Close 停止監看（冪等）。
func (w *Watcher) Close() error {
	w.once.Do(func() {
		close(w.done)
		_ = w.backend.Close()
	})
	<-w.loopDone
	return nil
}

func (w *Watcher) loop() {
	defer close(w.loopDone)
	defer close(w.batches)
	defer close(w.errors)
	deb := NewDebouncer(w.cfg.Debounce, nil)
	tick := time.NewTicker(20 * time.Millisecond) // 去抖收割頻率（非輪詢間隔）
	defer tick.Stop()
	backendErrors := w.backend.Errors()
	backendEvents := w.backend.Events()
	for {
		select {
		case <-w.done:
			return
		case err, ok := <-backendErrors:
			if !ok {
				backendErrors = nil
				continue
			}
			if err == nil {
				continue
			}
			w.recordError(err)
			select {
			case w.errors <- err:
			default:
			}
		case e, ok := <-backendEvents:
			if !ok {
				backendEvents = nil
				continue
			}
			if w.cfg.Filter.Allow(e.Path, e.IsDir) {
				deb.Add(e)
			}
		case <-tick.C:
			if deb.Due() {
				batch := deb.Flush()
				if len(batch) == 0 {
					continue
				}
				w.batchMu.Lock()
				w.lastBatch = batch
				w.batchMu.Unlock()
				select {
				case w.batches <- batch:
				default:
					// Keep the event loop non-blocking, but make loss explicit and
					// observable instead of silently dropping a batch.
					err := &watcherError{"watcher batch queue overflow: batch dropped"}
					w.recordError(err)
					select {
					case w.errors <- err:
					default:
					}
				}
			}
		}
	}
}

type watcherError struct{ msg string }

func (e *watcherError) Error() string { return e.msg }
