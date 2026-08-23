package lsp

import (
	"net/url"
	"strings"
)

// fileURI converts an OS path to an RFC 8089-style file URI for LSP. Building
// the URI by concatenating "file://" with a path is wrong for spaces, percent
// signs, Windows drive letters, and UNC paths; both initialize and didOpen
// must use the same canonical representation.
func fileURI(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if strings.HasPrefix(path, "//") {
		// UNC path: //server/share/file -> file://server/share/file.
		trimmed := strings.TrimPrefix(path, "//")
		if i := strings.IndexByte(trimmed, '/'); i > 0 {
			return (&url.URL{Scheme: "file", Host: trimmed[:i], Path: trimmed[i:]}).String()
		}
		return (&url.URL{Scheme: "file", Host: trimmed}).String()
	}
	if len(path) >= 2 && path[1] == ':' {
		// Windows drive path. URL.Path needs a leading slash so C:/x becomes
		// file:///C:/x rather than the non-standard file://C:/x.
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}
