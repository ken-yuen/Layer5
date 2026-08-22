package main

import (
	"os"
	"path/filepath"
	"testing"
)

// makeProject 建一個最小 Rust 專案（無 cargo 亦可測 file:/function: 比對）。
func makeProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"t\"\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "src", "main.rs"), []byte("fn greet() {}\nfn main() { greet() }\n"), 0o644)
	return dir
}

func mustVerdict(t *testing.T, p Project, feature string) Verdict {
	t.Helper()
	return p.Verify(Claim{ID: "x", Text: "t", Feature: feature})
}

func TestFileClaimVerifiedInsideProject(t *testing.T) {
	p := Project{Dir: makeProject(t)}
	v := mustVerdict(t, p, "file:src/main.rs")
	if v.Verdict != "verified" {
		t.Fatalf("want verified, got %+v", v)
	}
}

// TestFileClaimTraversalRejected 是 path traversal 邊界測試：
// `file:../../etc/passwd` 類聲明不得被 verified（也不得去讀檔案）。
func TestFileClaimTraversalRejected(t *testing.T) {
	p := Project{Dir: makeProject(t)}
	for _, feat := range []string{
		"file:../../etc/passwd",
		"file:../outside.rs",
		"file:src/../../etc/passwd",
	} {
		v := mustVerdict(t, p, feat)
		if v.Verdict == "verified" {
			t.Fatalf("%s must NOT be verified (traversal), got %+v", feat, v)
		}
		if v.Verdict != "contradicted" {
			t.Fatalf("%s should be contradicted, got %+v", feat, v)
		}
	}
}

func TestFileClaimAbsolutePathRejected(t *testing.T) {
	p := Project{Dir: makeProject(t)}
	for _, feat := range []string{"file:/etc/passwd", "file:C:\\\\windows\\\\system32"} {
		v := mustVerdict(t, p, feat)
		if v.Verdict == "verified" {
			t.Fatalf("%s must NOT be verified (absolute path), got %+v", feat, v)
		}
	}
}

func TestFunctionClaim(t *testing.T) {
	p := Project{Dir: makeProject(t)}
	if v := mustVerdict(t, p, "function:greet"); v.Verdict != "verified" {
		t.Fatalf("greet exists, want verified, got %+v", v)
	}
	if v := mustVerdict(t, p, "function:nope"); v.Verdict != "contradicted" {
		t.Fatalf("nope missing, want contradicted, got %+v", v)
	}
}

func TestRatchetFirstStrikeAndMonotonic(t *testing.T) {
	// 嚴重度 3（欺騙）：任何起點 → T0
	for _, from := range []TrustLevel{T3, T2, T1} {
		to, _ := applyRatchet(from, Verdict{Severity: 3})
		if to != T0 {
			t.Fatalf("sev3 from %s must go T0, got %s", from, to)
		}
	}
	// 只降不升：T0 上任何嚴重度都不得回升
	for _, sev := range []int{1, 2, 3, 4} {
		to, _ := applyRatchet(T0, Verdict{Severity: sev})
		if to != T0 {
			t.Fatalf("T0 must never rise (sev %d), got %s", sev, to)
		}
	}
	// 嚴重度 1（誇大）：T3→T2、T2→T2
	if to, _ := applyRatchet(T3, Verdict{Severity: 1}); to != T2 {
		t.Fatalf("sev1 T3→T2, got %s", to)
	}
	if to, _ := applyRatchet(T2, Verdict{Severity: 1}); to != T2 {
		t.Fatalf("sev1 T2 stays, got %s", to)
	}
	// 嚴重度 4（偽造，首擊棘輪）：T3→T0
	if to, _ := applyRatchet(T3, Verdict{Severity: 4}); to != T0 {
		t.Fatalf("sev4 first strike → T0, got %s", to)
	}
}
