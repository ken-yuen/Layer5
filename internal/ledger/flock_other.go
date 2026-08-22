//go:build !unix

package ledger

// acquireLock：非 unix 平台（Windows）無 flock——降級為不鎖（單寫者靠
// 紀律維持）。YKC 官方支援平台為 Linux/macOS；此處保持可編譯。
func acquireLock(f interface{ Fd() uintptr }) error { return nil }
