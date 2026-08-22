//go:build linux

package watch

import "time"

// NewBackend 回傳平台預設後端（Linux → inotify；失敗不靜默降級，由呼叫端決策）。
func NewBackend() (Backend, error) {
	return NewInotifyBackend()
}

// NewBackendWithFallback 回傳平台後端；inotify 不可用時退回輪詢（回傳第二值為 true）。
func NewBackendWithFallback(pollInterval time.Duration) (Backend, bool, error) {
	b, err := NewInotifyBackend()
	if err == nil {
		return b, false, nil
	}
	return NewPollBackend(pollInterval), true, nil
}
