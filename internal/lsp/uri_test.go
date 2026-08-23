package lsp

import (
	"net/url"
	"strings"
	"testing"
)

func TestFileURIQuotesPathAndUsesAbsoluteForm(t *testing.T) {
	path := "/tmp/space % file.rs"
	got := fileURI(path)
	if !strings.HasPrefix(got, "file:///") {
		t.Fatalf("file URI = %q, want absolute file URI", got)
	}
	if strings.Contains(got, " ") || !strings.Contains(got, "%20") || !strings.Contains(got, "%25") {
		t.Fatalf("file URI is not escaped: %q", got)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "file" || u.Path != path {
		t.Fatalf("parsed URI = %#v, want path %q", u, path)
	}
}

func TestFileURIWindowsDriveAndUNC(t *testing.T) {
	if got := fileURI(`C:\work dir\main.rs`); got != "file:///C:/work%20dir/main.rs" {
		t.Fatalf("drive URI = %q", got)
	}
	if got := fileURI(`\\server\share\main.rs`); got != "file://server/share/main.rs" {
		t.Fatalf("UNC URI = %q", got)
	}
}
