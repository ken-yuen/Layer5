//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package sandbox

// containerUser returns an empty value on platforms where numeric Unix user
// IDs are not available. Container engines then use their configured default
// user instead of making the package fail to cross-compile.
func containerUser() string { return "" }
