package kb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportErrorIndexWritesVersionedBlob(t *testing.T) {
	const page = `<!doctype html><html><body>
<h1 id="error-code-e0001"><a>Error code E0001</a></h1>
<p>A value was moved.</p>
<p>Erroneous code example:</p>
<pre><code class="language-rust compile_fail E0001">let moved = value &amp;&amp; other;</code></pre>
<p>Use a borrow instead.</p>
<pre><code class="language-rust">let borrowed = &amp;value;</code></pre>
<div style="break-before: page"></div>
<h1 id="error-code-e0425"><a>Error code E0425</a></h1>
<p>An unresolved name was used.</p>
<pre><code class="language-rust compile_fail E0425">missing();</code></pre>
<pre><code class="language-rust">defined();</code></pre>
</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/error_codes/print.html" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(page))
	}))
	defer srv.Close()

	result, err := ImportErrorIndex(context.Background(), ImportOptions{
		RustcVersion:  "rustc 1.98.0 (test-hash 2026-08-18)",
		ErrorIndexURL: srv.URL + "/error_codes/print.html",
	})
	if err != nil {
		t.Fatalf("ImportErrorIndex: %v", err)
	}
	if result.ErrorCount() != 2 {
		t.Fatalf("ErrorCount = %d, want 2", result.ErrorCount())
	}
	if result.Count() != 2+54+91+19+1 {
		t.Fatalf("Count = %d", result.Count())
	}
	meta := result.Metadata()
	if meta.RustcVersion != "1.98.0" || meta.ErrorIndexURL != srv.URL+"/error_codes/print.html" {
		t.Fatalf("metadata = %+v", meta)
	}
	if len(meta.ErrorIndexSHA256) != 64 {
		t.Fatalf("source checksum = %q", meta.ErrorIndexSHA256)
	}

	path := filepath.Join(t.TempDir(), "kb-1.98.0.ykc")
	if err := result.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	st, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if st.RustcVersion() != "1.98.0" || st.ErrorIndexSHA256() != meta.ErrorIndexSHA256 {
		t.Fatalf("opened metadata = %+v", st.Metadata())
	}
	a, ok := st.ByCode("E0001")
	if !ok {
		t.Fatal("imported E0001 missing")
	}
	if !strings.Contains(a.Err, "&&") || !strings.Contains(a.Fix, "&value") {
		t.Fatalf("examples were not extracted: err=%q fix=%q", a.Err, a.Fix)
	}
	if want := srv.URL + "/error_codes/E0001.html"; a.Source != want {
		t.Fatalf("source = %q, want %q", a.Source, want)
	}
}

func TestImportRejectsInvalidVersionAndURL(t *testing.T) {
	if _, err := normalizeRustcVersion("1.98"); err == nil {
		t.Fatal("short version should fail")
	}
	if _, err := normalizeRustcVersion("1.98.0/../../etc"); err == nil {
		t.Fatal("path-like version should fail")
	}
	if err := validateIndexURL("file:///tmp/error_codes/print.html"); err == nil {
		t.Fatal("file URL should fail")
	}
	if err := validateIndexURL("https://example.test/index.html"); err == nil {
		t.Fatal("non-print index URL should fail")
	}
}

func TestImportedBlobProtectsMetadata(t *testing.T) {
	b, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	// v2 metadata starts immediately after magic(6), version(4), and length(4).
	if len(b) < 15 {
		t.Fatal("blob unexpectedly short")
	}
	b[14] ^= 0x01
	if _, _, err := decodeBlob(b); err == nil {
		t.Fatal("metadata tamper must invalidate v2 checksum")
	}
}
