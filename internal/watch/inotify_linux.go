//go:build linux

// inotify_linux.go：以 stdlib syscall 實現的遞迴 inotify 後端。
//
// 為何不用 fsnotify：YKC 裁判核心零依賴紀律；inotify 經 syscall 標準庫
// 即可完整表達（fsnotify 在 Linux 底層同為 inotify）。事件語意對齊：
//
//	IN_MODIFY→write、IN_CREATE→create、IN_DELETE/IN_MOVED_FROM→remove、
//	IN_MOVED_TO→create、cookie 配對→rename。
//
// 遞迴維護：新目錄出現即加 watch 並補掃（消除 watch 建立前的漏報）；
// IN_IGNORED 時清理；IN_Q_OVERFLOW 上報 error 通道。

package watch

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const inotifyMask = syscall.IN_MODIFY | syscall.IN_CREATE | syscall.IN_DELETE |
	syscall.IN_MOVED_FROM | syscall.IN_MOVED_TO | syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF

type inotifyBackend struct {
	mu      sync.Mutex
	fd      int
	dirByWd map[int]string
	wdByDir map[string]int

	events  chan Event
	errors  chan error
	closed  chan struct{}
	once    sync.Once
	started sync.Once
	closing bool
	pending map[uint32]pendingRename // cookie → 待配對 MOVED_FROM
}

type pendingRename struct {
	path  string
	timer *time.Timer
}

const renamePairWindow = 100 * time.Millisecond

// NewInotifyBackend 建立 inotify 後端。
func NewInotifyBackend() (Backend, error) {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC)
	if err != nil {
		return nil, err
	}
	b := &inotifyBackend{
		fd:      fd,
		dirByWd: map[int]string{},
		wdByDir: map[string]int{},
		events:  make(chan Event, 256),
		errors:  make(chan error, 16),
		closed:  make(chan struct{}),
		pending: map[uint32]pendingRename{},
	}
	return b, nil
}

func (b *inotifyBackend) Name() string { return "inotify" }

func (b *inotifyBackend) Events() <-chan Event { return b.events }
func (b *inotifyBackend) Errors() <-chan error { return b.errors }

func (b *inotifyBackend) Add(root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	return b.addTree(abs)
}

// addTree 遞迴加入目錄樹（排除黑名單目錄）。
func (b *inotifyBackend) addTree(dir string) error {
	fi, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return nil
	}
	if err := b.addOne(dir); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if isExcludedWatchDir(e.Name()) {
			continue
		}
		if err := b.addTree(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (b *inotifyBackend) addOne(dir string) error {
	b.mu.Lock()
	if _, ok := b.wdByDir[dir]; ok {
		b.mu.Unlock()
		return nil
	}
	b.mu.Unlock()
	wd, err := syscall.InotifyAddWatch(b.fd, dir, inotifyMask)
	if err != nil {
		if errors.Is(err, syscall.ENOSPC) {
			return errors.New("inotify watch limit reached (ENOSPC); raise fs.inotify.max_user_watches or use poll backend")
		}
		return err
	}
	b.mu.Lock()
	// 舊 wd 重用時先清反向表
	if old, ok := b.dirByWd[wd]; ok {
		delete(b.wdByDir, old)
	}
	b.dirByWd[wd] = dir
	b.wdByDir[dir] = wd
	b.mu.Unlock()
	return nil
}

func (b *inotifyBackend) Start() {
	b.started.Do(func() { go b.readLoop() })
}

func (b *inotifyBackend) Close() error {
	b.mu.Lock()
	b.closing = true
	for cookie, pending := range b.pending {
		if pending.timer != nil {
			pending.timer.Stop()
		}
		delete(b.pending, cookie)
	}
	b.mu.Unlock()
	b.once.Do(func() {
		close(b.closed)
		syscall.Close(b.fd) // 令阻塞中的 read 返回
	})
	return nil
}

// readLoop 是事件讀取主循環。
func (b *inotifyBackend) readLoop() {
	buf := make([]byte, 64*1024)
	for {
		n, err := syscall.Read(b.fd, buf)
		if err != nil {
			b.mu.Lock()
			closing := b.closing
			b.mu.Unlock()
			if closing || errors.Is(err, syscall.EINTR) && b.isClosed() {
				return
			}
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			select {
			case b.errors <- err:
			default:
			}
			return
		}
		if n == 0 {
			return
		}
		b.consume(buf[:n])
	}
}

func (b *inotifyBackend) isClosed() bool {
	select {
	case <-b.closed:
		return true
	default:
		return false
	}
}

// consume 解析一個 read 區塊內的全部 inotify 事件。
func (b *inotifyBackend) consume(raw []byte) {
	const hdr = syscall.SizeofInotifyEvent
	for off := 0; off+hdr <= len(raw); {
		rawEvt := (*syscall.InotifyEvent)(unsafe.Pointer(&raw[off]))
		nameLen := int(rawEvt.Len)
		if off+hdr+nameLen > len(raw) {
			return // 截斷事件：丟棄尾部
		}
		name := ""
		if nameLen > 0 {
			nb := raw[off+hdr : off+hdr+nameLen]
			if i := bytes.IndexByte(nb, 0); i >= 0 {
				nb = nb[:i]
			}
			name = string(nb)
		}
		b.handle(int(rawEvt.Wd), rawEvt.Mask, rawEvt.Cookie, name)
		off += hdr + nameLen
	}
}

func (b *inotifyBackend) handle(wd int, mask uint32, cookie uint32, name string) {
	if mask&syscall.IN_Q_OVERFLOW != 0 {
		select {
		case b.errors <- errors.New("inotify queue overflow: events dropped; rescan advised"):
		default:
		}
		return
	}
	b.mu.Lock()
	dir, ok := b.dirByWd[wd]
	b.mu.Unlock()
	if !ok {
		return
	}
	full := dir
	if name != "" {
		full = filepath.Join(dir, name)
	}
	isDir := mask&syscall.IN_ISDIR != 0

	switch {
	case mask&syscall.IN_IGNORED != 0:
		b.mu.Lock()
		delete(b.wdByDir, dir)
		delete(b.dirByWd, wd)
		b.mu.Unlock()
		return
	case mask&syscall.IN_MOVED_FROM != 0:
		if cookie == 0 {
			b.emit(Event{Op: OpRemove, Path: full, IsDir: isDir})
			return
		}
		b.mu.Lock()
		// A cookie should be unique, but stop/replace a stale entry rather
		// than leaking a timer if the kernel reuses one unexpectedly.
		if old, ok := b.pending[cookie]; ok && old.timer != nil {
			old.timer.Stop()
		}
		b.pending[cookie] = pendingRename{path: full}
		b.mu.Unlock()
		timer := time.AfterFunc(renamePairWindow, func() { b.expireRename(cookie, full) })
		b.mu.Lock()
		if pending, ok := b.pending[cookie]; ok && pending.path == full {
			pending.timer = timer
			b.pending[cookie] = pending
		} else {
			// MOVED_TO paired before the timer was installed.
			timer.Stop()
		}
		b.mu.Unlock()
		return
	case mask&syscall.IN_MOVED_TO != 0:
		b.mu.Lock()
		pending, had := b.pending[cookie]
		if had {
			delete(b.pending, cookie)
		}
		b.mu.Unlock()
		if had {
			if pending.timer != nil {
				pending.timer.Stop()
			}
			b.emit(Event{Op: OpRename, Path: full, IsDir: isDir})
		} else {
			b.emit(Event{Op: OpCreate, Path: full, IsDir: isDir})
		}
		if isDir {
			if err := b.addTree(full); err != nil {
				b.reportError(fmt.Errorf("inotify add moved directory %s: %w", full, err))
			}
			b.rescanNewDir(full)
		}
		return
	case mask&syscall.IN_DELETE != 0:
		b.emit(Event{Op: OpRemove, Path: full, IsDir: isDir})
		return
	case mask&syscall.IN_CREATE != 0:
		b.emit(Event{Op: OpCreate, Path: full, IsDir: isDir})
		if isDir {
			if err := b.addTree(full); err != nil {
				b.reportError(fmt.Errorf("inotify add created directory %s: %w", full, err))
			}
			b.rescanNewDir(full) // watch 建立前已生成的檔案補報
		}
		return
	case mask&syscall.IN_MODIFY != 0:
		b.emit(Event{Op: OpWrite, Path: full})
		return
	case mask&(syscall.IN_DELETE_SELF|syscall.IN_MOVE_SELF) != 0:
		return // 由 IN_IGNORED 統一清理
	}
}

// rescanNewDir 補掃新目錄（競態補報：目錄建立與 watch 之間寫入的檔案）。
func (b *inotifyBackend) rescanNewDir(dir string) {
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != dir && isExcludedWatchDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() {
			b.emit(Event{Op: OpCreate, Path: path})
		}
		return nil
	})
	if err != nil {
		b.reportError(fmt.Errorf("inotify rescan %s: %w", dir, err))
	}
}

func isExcludedWatchDir(name string) bool {
	for _, excluded := range DefaultExcludeDirs {
		if name == excluded {
			return true
		}
	}
	return false
}

func (b *inotifyBackend) reportError(err error) {
	if err == nil {
		return
	}
	select {
	case b.errors <- err:
	default:
	}
}

// expireRename 把逾時未配對的 MOVED_FROM 轉為 remove。延遲配對讓
// MOVED_FROM 與 MOVED_TO 分跨兩次 read(2) 時仍能產生 rename，而不會在
// 每個 read 區塊結尾過早把所有移動都拆成 remove/create。
func (b *inotifyBackend) expireRename(cookie uint32, path string) {
	b.mu.Lock()
	pending, ok := b.pending[cookie]
	if ok && pending.path == path {
		delete(b.pending, cookie)
	}
	closing := b.closing
	b.mu.Unlock()
	if ok && pending.path == path && !closing {
		b.emit(Event{Op: OpRemove, Path: path})
	}
}

func (b *inotifyBackend) emit(e Event) {
	select {
	case b.events <- e:
	default:
		select {
		case b.errors <- errors.New("inotify event queue overflow: events dropped"):
		default:
		}
	}
}
