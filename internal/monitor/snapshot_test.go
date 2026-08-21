package monitor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotDetectsAtomicFileChanges(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.rs"), []byte("fn main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s1, err := Snapshotter{Root: dir}.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.rs"), []byte("fn main(){println!(\"hi\");}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lib.rs"), []byte("pub fn x(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s2, err := Snapshotter{Root: dir}.Capture()
	if err != nil {
		t.Fatal(err)
	}
	d := DiffSnapshots(s1, s2)
	if !d.HasChanges() {
		t.Fatalf("expected changes")
	}
	if len(d.Modified) != 1 || d.Modified[0] != "main.rs" {
		t.Fatalf("unexpected modified files: %+v", d.Modified)
	}
	if len(d.Added) != 1 || d.Added[0] != "lib.rs" {
		t.Fatalf("unexpected added files: %+v", d.Added)
	}
}
