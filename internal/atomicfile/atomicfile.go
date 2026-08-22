// Package atomicfile 提供跨程序的原子寫入 primitives（暫存檔 + fsync + rename + 目錄 fsync）。
package atomicfile

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileSync writes data to path using a same-directory temporary file,
// fsyncs it, atomically renames it into place, and fsyncs the parent directory.
// The function is intentionally small and boring: every monitor checkpoint and
// event payload uses this one path so the atomicity contract is auditable.
func WriteFileSync(path string, data []byte, perm os.FileMode) error {
	if path == "" {
		return errors.New("path is required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, fmt.Sprintf(".%s.tmp.%s", filepath.Base(path), randomSuffix()))
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = f.Close()
		}
		_ = os.Remove(tmp)
	}()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		closed = true
		return err
	}
	closed = true
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return SyncDir(dir)
}

func SyncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	// Some platforms/filesystems do not support directory fsync. Treat it as a
	// best-effort durability enhancement, not as a semantic failure.
	_ = f.Sync()
	return nil
}

func randomSuffix() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "no-rand"
	}
	return hex.EncodeToString(b[:])
}
