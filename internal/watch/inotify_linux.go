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
	"os"
	"path/filepath"
	"sync"
	"syscall"
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
	path string
}

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
		name := e.Name()
		excluded := false
		for _, d := range DefaultExcludeDirs {
			if name == d {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		if err := b.addTree(filepath.Join(dir, name)); err != nil {
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
		b.flushPendingRenames()
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
	case mask&syscall.IN_Q_OVERFLOW != 0:
		select {
		case b.errors <- errors.New("inotify queue overflow: events dropped; rescan advised"):
		default:
		}
		return
	case mask&syscall.IN_IGNORED != 0:
		b.mu.Lock()
		delete(b.wdByDir, dir)
		delete(b.dirByWd, wd)
		b.mu.Unlock()
		return
	case mask&syscall.IN_MOVED_FROM != 0:
		b.mu.Lock()
		b.pending[cookie] = pendingRename{path: full}
		b.mu.Unlock()
		return
	case mask&syscall.IN_MOVED_TO != 0:
		b.mu.Lock()
		_, had := b.pending[cookie]
		delete(b.pending, cookie)
		b.mu.Unlock()
		if had {
			b.emit(Event{Op: OpRename, Path: full, IsDir: isDir})
		} else {
			b.emit(Event{Op: OpCreate, Path: full, IsDir: isDir})
		}
		if isDir {
			_ = b.addTree(full) // 新遷入的子樹
		}
		return
	case mask&syscall.IN_DELETE != 0:
		b.emit(Event{Op: OpRemove, Path: full, IsDir: isDir})
		return
	case mask&syscall.IN_CREATE != 0:
		b.emit(Event{Op: OpCreate, Path: full, IsDir: isDir})
		if isDir {
			_ = b.addTree(full)
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
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b.emit(Event{Op: OpCreate, Path: filepath.Join(dir, e.Name())})
	}
}

// flushPendingRenames 把未配對的 MOVED_FROM 轉為 remove。
func (b *inotifyBackend) flushPendingRenames() {
	b.mu.Lock()
	left := b.pending
	b.pending = map[uint32]pendingRename{}
	b.mu.Unlock()
	for _, p := range left {
		b.emit(Event{Op: OpRemove, Path: p.path})
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
