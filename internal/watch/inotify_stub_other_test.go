//go:build !linux

package watch

import "errors"

// Keep the platform-neutral test suite buildable on non-Linux targets; the
// real inotify integration test is skipped at runtime on those platforms.
func NewInotifyBackend() (Backend, error) {
	return nil, errors.New("inotify unavailable on this platform")
}
