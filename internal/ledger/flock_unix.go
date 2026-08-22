//go:build unix

package ledger

import "syscall"

// acquireLock 取得帳本檔案的排他寫鎖（非阻塞）：
// 單一寫者不變式的強制手段——第二個程序開啟同一帳本時立即失敗（ErrLocked），
// 而非交錯寫入造成 hash 鏈交錯損毀。
// 鎖在 fd 關閉（程序退出 / Ledger.Close）時自動釋放。
func acquireLock(f interface{ Fd() uintptr }) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return ErrLocked
	}
	return nil
}
