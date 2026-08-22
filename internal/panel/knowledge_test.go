package panel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPanelKnowledgeSurfaceIsReadOnlyAndMounted(t *testing.T) {
	root := makePanelRoot(t)
	mux := BuildMux(Options{Root: root, BinDir: root}, nil)

	stateReq := httptest.NewRequest(http.MethodGet, "/api/know/state", nil)
	stateRes := httptest.NewRecorder()
	mux.ServeHTTP(stateRes, stateReq)
	if stateRes.Code != http.StatusOK {
		t.Fatalf("knowledge state status=%d body=%s", stateRes.Code, stateRes.Body.String())
	}
	var state map[string]any
	if err := json.NewDecoder(stateRes.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if state["read_only"] != true || state["rustc_version"] == "" {
		t.Fatalf("knowledge state lacks read-only/version metadata: %#v", state)
	}

	searchReq := httptest.NewRequest(http.MethodGet, "/api/know/search?q=E0382&k=1", nil)
	searchRes := httptest.NewRecorder()
	mux.ServeHTTP(searchRes, searchReq)
	if searchRes.Code != http.StatusOK {
		t.Fatalf("knowledge search status=%d body=%s", searchRes.Code, searchRes.Body.String())
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/know/search?q=E0382", nil)
	postRes := httptest.NewRecorder()
	mux.ServeHTTP(postRes, postReq)
	if postRes.Code != http.StatusMethodNotAllowed {
		t.Fatalf("knowledge POST must be rejected, got %d", postRes.Code)
	}
}
