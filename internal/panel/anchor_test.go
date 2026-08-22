package panel

import (
	"os"
	"path/filepath"
	"testing"

	"ykc/internal/ledger"
)

func TestProjectStateExposesHeadAnchorAndRollback(t *testing.T) {
	anchorRoot := t.TempDir()
	t.Setenv("YKC_ANCHOR_DIR", filepath.Join(anchorRoot, "anchors"))
	t.Setenv("YKC_ANCHOR_KEY_FILE", filepath.Join(anchorRoot, "anchor.key"))
	project := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(filepath.Join(project, ".ykc"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, ".ykc", "ledger.jsonl")
	led, err := ledger.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := led.Append("test", "panel-test", map[string]int{"i": i}); err != nil {
			t.Fatal(err)
		}
	}
	if err := led.Close(); err != nil {
		t.Fatal(err)
	}

	ps := projectState(project)
	if ps.Integrity != "verified" || ps.Anchor.State != "anchored" || ps.Anchor.AnchorSeq != 2 {
		t.Fatalf("anchored project state = %+v", ps)
	}

	// 保留第一行的合法前綴，模擬截斷；裸 hash chain 仍會自洽，面板必須依
	// project 外 anchor 把完整性標紅。
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	firstEnd := 0
	for i, c := range b {
		if c == '\n' {
			firstEnd = i + 1
			break
		}
	}
	if err := os.WriteFile(path, b[:firstEnd], 0o644); err != nil {
		t.Fatal(err)
	}
	ps = projectState(project)
	if ps.Integrity != "tampered" || ps.Anchor.State != "rollback" {
		t.Fatalf("rollback must be surfaced as tampered: %+v", ps)
	}
}
