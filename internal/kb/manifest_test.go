package kb

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestImportManifestRoundTripAndReplay(t *testing.T) {
	const page = `<!doctype html><html><body>
<h1 id="error-code-e0001"><a>Error code E0001</a></h1>
<p>First error.</p><pre><code class="language-rust compile_fail E0001">bad();</code></pre><pre><code class="language-rust">good();</code></pre>
<h1 id="error-code-e0425"><a>Error code E0425</a></h1>
<p>Second error.</p><pre><code class="language-rust compile_fail E0425">missing();</code></pre><pre><code class="language-rust">defined();</code></pre>
</body></html>`
	etag := `"ykc-fixture-v1"`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/error_codes/print.html" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Last-Modified", "Sun, 23 Aug 2026 00:00:00 GMT")
		_, _ = w.Write([]byte(page))
	}))
	defer srv.Close()

	result, err := ImportErrorIndex(context.Background(), ImportOptions{
		RustcVersion:  "1.98.0",
		ErrorIndexURL: srv.URL + "/error_codes/print.html",
		HTTPClient:    srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	blob, err := result.Blob()
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := result.Manifest(blob)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ErrorIndexETag != etag || manifest.ErrorIndexLastModified == "" || manifest.ErrorCount != 2 || manifest.AtomCount != 167 {
		t.Fatalf("manifest = %+v", manifest)
	}

	dir := t.TempDir()
	blobPath := filepath.Join(dir, "kb.ykc")
	manifestPath := filepath.Join(dir, "kb.manifest.json")
	written, err := result.SaveWithManifest(blobPath, manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadImportManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != written {
		t.Fatalf("loaded manifest differs:\nwant=%+v\ngot=%+v", written, loaded)
	}

	replayed, err := ReplayImport(context.Background(), loaded, srv.Client())
	if err != nil {
		t.Fatalf("ReplayImport: %v", err)
	}
	replayBlob, err := replayed.Blob()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(blob, replayBlob) {
		t.Fatal("same manifest/source must rebuild byte-identical blob")
	}

	etag = `"ykc-fixture-v2"`
	if _, err := ReplayImport(context.Background(), loaded, srv.Client()); err == nil {
		t.Fatal("ETag drift must fail manifest replay")
	}
}
