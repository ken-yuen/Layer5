package panel

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestReadOnlyEndpointsRejectMutatingMethods(t *testing.T) {
	root := makePanelRoot(t)
	h := BuildMux(Options{Root: root, BinDir: root + "/bin", Depth: 1}, nil)
	for _, path := range []string{"/api/state", "/api/projects"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		resp := httptest.NewRecorder()
		h.ServeHTTP(resp, req)
		if resp.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s status=%d, want 405", path, resp.Code)
		}
	}
	// raw validates the method before resolving a project, so a mutating
	// request cannot turn the observer endpoint into a write surface.
	req := httptest.NewRequest(http.MethodPost, "/api/raw?project=proj-a", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/raw status=%d, want 405", resp.Code)
	}
}

func TestProjectsAndRawUseRelativeRootResolution(t *testing.T) {
	root := makePanelRoot(t)
	// Create a marker so the observer discovery surface contains the project.
	proj := filepath.Join(root, "proj-a", ".ykc")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "ledger.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	h := BuildMux(Options{Root: root, BinDir: root + "/bin", Depth: 1}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/raw?project=./proj-a", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("relative raw project status=%d body=%s", resp.Code, resp.Body.String())
	}
}
