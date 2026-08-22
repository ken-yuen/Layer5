package kb

import (
	"path/filepath"
	"strings"
	"testing"
)

func diffAtom(kind Kind, code, title, body string, refs ...string) *Atom {
	a := &Atom{Kind: kind, Code: code, Title: title, Body: body, Refs: refs}
	a.ID = a.contentHash()
	return a
}

func diffStore(t *testing.T, atoms []*Atom, source string, meta DatasetMeta) *Store {
	t.Helper()
	st, err := newStore(atoms, source, meta)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestDiffStoresIncludesContentAndReferenceChanges(t *testing.T) {
	baseAtoms := []*Atom{
		diffAtom(KindError, "E0001", "same code, old content", "old", "dep-a"),
		diffAtom(KindRule, "R-OLD", "removed", "old"),
		diffAtom(KindRule, "R-REF", "reference only", "same", "dep-a"),
		diffAtom(KindPart, "part", "unchanged", "same"),
	}
	targetAtoms := []*Atom{
		diffAtom(KindError, "E0001", "same code, new content", "new", "dep-a"),
		diffAtom(KindRule, "R-REF", "reference only", "same", "dep-b"),
		diffAtom(KindRule, "R-NEW", "added", "new"),
		diffAtom(KindPart, "part", "unchanged", "same"),
	}
	base := diffStore(t, baseAtoms, "base.ykc", DatasetMeta{RustcVersion: "1.98.0"})
	target := diffStore(t, targetAtoms, "target.ykc", DatasetMeta{RustcVersion: "1.99.0"})
	report := DiffStores(base, target)
	if !report.MetadataChanged || len(report.Added) != 1 || len(report.Removed) != 1 || len(report.Changed) != 2 || report.Unchanged != 1 {
		t.Fatalf("unexpected diff: %+v", report)
	}
	if report.Added[0].Code != "R-NEW" || report.Removed[0].Code != "R-OLD" {
		t.Fatalf("add/remove ordering wrong: %+v / %+v", report.Added, report.Removed)
	}
	changes := map[string]AtomChange{}
	for _, change := range report.Changed {
		changes[change.Code] = change
	}
	if got := changes["E0001"]; !got.ContentChanged || got.ReferencesChanged || got.FromID == got.ToID {
		t.Fatalf("content change not classified: %+v", got)
	}
	if got := changes["R-REF"]; got.ContentChanged || !got.ReferencesChanged || got.FromID != got.ToID {
		t.Fatalf("ref-only change not classified: %+v", got)
	}
	markdown := RenderDiffMarkdown(report, 200)
	for _, want := range []string{"YKC 知識庫版本差異", "R-NEW", "R-OLD", "E0001", "R-REF"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown missing %q:\n%s", want, markdown)
		}
	}
}

func TestDiffFilesRoundTrip(t *testing.T) {
	baseAtoms := []*Atom{diffAtom(KindError, "E0001", "before", "old")}
	targetAtoms := []*Atom{diffAtom(KindError, "E0001", "after", "new")}
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.ykc")
	targetPath := filepath.Join(dir, "target.ykc")
	baseBlob, err := encodeBlob(baseAtoms, DatasetMeta{RustcVersion: "1.98.0"})
	if err != nil {
		t.Fatal(err)
	}
	targetBlob, err := encodeBlob(targetAtoms, DatasetMeta{RustcVersion: "1.98.0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := saveBlob(basePath, baseBlob); err != nil {
		t.Fatal(err)
	}
	if err := saveBlob(targetPath, targetBlob); err != nil {
		t.Fatal(err)
	}
	report, err := DiffFiles(basePath, targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Changed) != 1 || report.Changed[0].Code != "E0001" {
		t.Fatalf("file diff = %+v", report)
	}
}
