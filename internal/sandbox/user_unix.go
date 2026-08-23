//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package sandbox

import (
	"fmt"
	"os"
)

func containerUser() string {
	return fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
}
