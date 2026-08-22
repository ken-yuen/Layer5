package kb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPStateReportsVersionedDatasetMetadata(t *testing.T) {
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/kb/state", nil)
	w := httptest.NewRecorder()
	Handler(st).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["rustc_version"] != EmbeddedRustcVersion() {
		t.Fatalf("rustc_version = %#v", got["rustc_version"])
	}
	if got["error_index_url"] != EmbeddedMetadata().ErrorIndexURL {
		t.Fatalf("error_index_url = %#v", got["error_index_url"])
	}
	if got["read_only"] != true {
		t.Fatalf("read_only = %#v", got["read_only"])
	}
}
