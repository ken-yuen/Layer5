//go:build !linux

package watch

import "time"

// NewBackend 回傳平台預設後端（非 Linux → stat 輪詢）。
func NewBackend() (Backend, error) {
	return NewPollBackend(0), nil
}

// NewBackendWithFallback：非 Linux 一律輪詢（fallback=true 語義成立）。
func NewBackendWithFallback(pollInterval time.Duration) (Backend, bool, error) {
	return NewPollBackend(pollInterval), true, nil
}
