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

	batchMu sync.Mutex
	batches chan []Event
	done    chan struct{}
	once    sync.Once

	lastBatch []Event // 最近一次輸出批次（觀察用；有界=1）
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
		cfg:     cfg,
		backend: backend,
		batches: make(chan []Event, 64),
		done:    make(chan struct{}),
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

// Close 停止監看（冪等）。
func (w *Watcher) Close() error {
	w.once.Do(func() {
		close(w.done)
		w.backend.Close()
	})
	return nil
}

func (w *Watcher) loop() {
	deb := NewDebouncer(w.cfg.Debounce, nil)
	tick := time.NewTicker(20 * time.Millisecond) // 去抖收割頻率（非輪詢間隔）
	defer tick.Stop()
	for {
		select {
		case <-w.done:
			return
		case err := <-w.backend.Errors():
			// 後端錯誤不終止服務；經由批次通道旁路無法表達——記錄於空批次？
			// 設計決策：錯誤以「僅含一條特殊事件」表達會污染事件語意；
			// 保持 Errors 通道只被 select 排空（避免後端阻塞），錯誤可觀測性
			// 由 /api/watch 的 backend 名稱與 pending 數承擔。
			_ = err
		case e := <-w.backend.Events():
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
				default: // 消費端停滯：丟棄最舊策略不可行（channel 無隨機存取）——阻塞會拖垮後端，故丟棄新批並保留 lastBatch 供觀察
				}
			}
		}
	}
}
