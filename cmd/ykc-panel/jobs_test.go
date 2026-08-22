package main

import (
	"os"
	"path/filepath"
	"testing"
)

// makePanelRoot 建一個含兩個 Cargo 專案的面板根。
func makePanelRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"proj-a", "proj-b"} {
		d := filepath.Join(root, name)
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "Cargo.toml"), []byte("[package]\nname = \""+name+"\"\n"), 0o644)
	}
	return root
}

func TestValidateProjectWhitelist(t *testing.T) {
	root := makePanelRoot(t)
	m := newJobManager(filepath.Join(root, "bin"), root)

	// 白名單內：絕對路徑、相對於面板 root 的相對路徑都應過
	absA := filepath.Join(root, "proj-a")
	if got, err := m.validateProject(absA); err != nil || got != absA {
		t.Fatalf("whitelisted project rejected: %v %v", got, err)
	}
	if got, err := m.validateProject("proj-b"); err != nil || got != filepath.Join(root, "proj-b") {
		t.Fatalf("whitelisted relative project rejected: %v %v", got, err)
	}

	// 白名單外：任意路徑必須拒收（S1 核心）
	for _, bad := range []string{"/etc", "/tmp", root + "/nope", "not-a-project"} {
		if _, err := m.validateProject(bad); err == nil {
			t.Fatalf("project %q must be rejected (not in whitelist)", bad)
		}
	}
}

func TestValidateClaimsContainment(t *testing.T) {
	root := makePanelRoot(t)
	m := newJobManager(filepath.Join(root, "bin"), root)
	projA := filepath.Join(root, "proj-a")

	// 專案內的 claims：過
	inProj := filepath.Join(projA, "claims.json")
	os.WriteFile(inProj, []byte("{}"), 0o644)
	if _, err := m.validateClaims(projA, inProj); err != nil {
		t.Fatalf("claims inside project rejected: %v", err)
	}
	// 面板根內（專案外）：過
	inRoot := filepath.Join(root, "shared-claims.json")
	os.WriteFile(inRoot, []byte("{}"), 0o644)
	if _, err := m.validateClaims(projA, inRoot); err != nil {
		t.Fatalf("claims inside panel root rejected: %v", err)
	}
	// 兩處之外：拒收（防任意檔讀取）
	for _, bad := range []string{"/etc/passwd", "/tmp/claims.json"} {
		if _, err := m.validateClaims(projA, bad); err == nil {
			t.Fatalf("claims %q outside project/root must be rejected", bad)
		}
	}
}

func TestInsideHelper(t *testing.T) {
	if !inside("/a/b", "/a/b/c") {
		t.Fatal("inside must match descendant")
	}
	if inside("/a/b", "/a/bc") {
		t.Fatal("prefix trap: /a/bc is NOT inside /a/b")
	}
	if !inside("/a/b", "/a/b") {
		t.Fatal("self must be inside")
	}
}
