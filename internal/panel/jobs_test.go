package panel

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
	m := NewJobManager(filepath.Join(root, "bin"), root)

	// 白名單內：絕對路徑、相對於面板 root 的相對路徑都應過
	absA := filepath.Join(root, "proj-a")
	if got, err := m.ValidateProject(absA); err != nil || got != absA {
		t.Fatalf("whitelisted project rejected: %v %v", got, err)
	}
	if got, err := m.ValidateProject("proj-b"); err != nil || got != filepath.Join(root, "proj-b") {
		t.Fatalf("whitelisted relative project rejected: %v %v", got, err)
	}

	// 白名單外：任意路徑必須拒收（S1 核心）
	for _, bad := range []string{"/etc", "/tmp", root + "/nope", "not-a-project"} {
		if _, err := m.ValidateProject(bad); err == nil {
			t.Fatalf("project %q must be rejected (not in whitelist)", bad)
		}
	}
}

func TestValidateClaimsContainment(t *testing.T) {
	root := makePanelRoot(t)
	m := NewJobManager(filepath.Join(root, "bin"), root)
	projA := filepath.Join(root, "proj-a")

	// 專案內的 claims：過
	inProj := filepath.Join(projA, "claims.json")
	os.WriteFile(inProj, []byte("{}"), 0o644)
	if _, err := m.ValidateClaims(projA, inProj); err != nil {
		t.Fatalf("claims inside project rejected: %v", err)
	}
	// 面板根內（專案外）：過
	inRoot := filepath.Join(root, "shared-claims.json")
	os.WriteFile(inRoot, []byte("{}"), 0o644)
	if _, err := m.ValidateClaims(projA, inRoot); err != nil {
		t.Fatalf("claims inside panel root rejected: %v", err)
	}
	// 兩處之外：拒收（防任意檔讀取）
	for _, bad := range []string{"/etc/passwd", "/tmp/claims.json"} {
		if _, err := m.ValidateClaims(projA, bad); err == nil {
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

func TestValidateProjectUsesDiscoveryDepthAndExtraDirs(t *testing.T) {
	root := makePanelRoot(t)
	nested := filepath.Join(root, "one", "two")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "Cargo.toml"), []byte("[package]\nname=\"nested\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	extraRoot := t.TempDir()
	extra := filepath.Join(extraRoot, "extra")
	if err := os.MkdirAll(extra, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extra, "Cargo.toml"), []byte("[package]\nname=\"extra\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewJobManagerWithDiscovery(filepath.Join(root, "bin"), root, []string{extra}, 2)
	for _, want := range []string{nested, extra} {
		if got, err := m.ValidateProject(want); err != nil || got != want {
			t.Fatalf("project %q was not admitted: got=%q err=%v", want, got, err)
		}
	}
}

func TestValidateClaimsRejectsSymlinkEscape(t *testing.T) {
	root := makePanelRoot(t)
	project := filepath.Join(root, "proj-a")
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(project, "claims-link.json")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable on this platform: %v", err)
	}
	m := NewJobManager(filepath.Join(root, "bin"), root)
	if _, err := m.ValidateClaims(project, link); err == nil {
		t.Fatal("claims symlink escaping project/root must be rejected")
	}
}

func TestValidateClaimsResolvesRelativeToRoot(t *testing.T) {
	root := makePanelRoot(t)
	project := filepath.Join(root, "proj-a")
	claims := filepath.Join(root, "shared.json")
	if err := os.WriteFile(claims, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewJobManagerWithDiscovery(filepath.Join(root, "bin"), root, nil, 1)
	got, err := m.ValidateClaims(project, "shared.json")
	if err != nil || got != claims {
		t.Fatalf("relative claims should resolve under root: got=%q err=%v", got, err)
	}
}
