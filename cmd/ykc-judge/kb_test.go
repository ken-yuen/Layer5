package main

import (
	"strings"
	"testing"

	"ykc/internal/kb"
)

func TestBuildJudgeKnowledgeRecordsExactKBProvenance(t *testing.T) {
	st, err := kb.Open()
	if err != nil {
		t.Fatal(err)
	}
	knowledge := buildJudgeKnowledge(st, []Error{
		{Code: "E0382"},
		{Code: "E0425"},
		{Code: "e0382"}, // duplicate must not make a second ledger usage.
		{Code: "E9999"}, // unknown code is not misrepresented as KB evidence.
		{},
	})
	if got, want := len(knowledge.analysis.Usages), 2; got != want {
		t.Fatalf("usage count = %d, want %d: %+v", got, want, knowledge.analysis.Usages)
	}
	if knowledge.analysis.KBVersion != st.Version() {
		t.Fatalf("KB version = %s, want %s", knowledge.analysis.KBVersion, st.Version())
	}
	if knowledge.analysis.RustcVersion != st.RustcVersion() {
		t.Fatalf("rustc version = %s, want %s", knowledge.analysis.RustcVersion, st.RustcVersion())
	}
	usage, ok := knowledge.usageFor("E0382")
	if !ok {
		t.Fatal("E0382 usage missing")
	}
	root, _ := st.ByCode("E0382")
	if usage.RootAtomID != root.ID || len(usage.AtomIDs) == 0 || usage.ContextSHA256 == "" {
		t.Fatalf("invalid usage: %+v", usage)
	}
	if summary := knowledge.summaryFor("E0382"); !strings.Contains(summary, "dataset="+st.Version()) {
		t.Fatalf("summary = %q", summary)
	}
	if _, ok := knowledge.usageFor("E9999"); ok {
		t.Fatal("unknown error code must not create KB usage")
	}
}

func TestOpenJudgeKnowledgeUsesValidatedBlob(t *testing.T) {
	path := t.TempDir() + "/kb.ykc"
	if err := kb.Save(path); err != nil {
		t.Fatal(err)
	}
	st, err := openJudgeKnowledge(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Source() != path || st.RustcVersion() != kb.EmbeddedRustcVersion() {
		t.Fatalf("opened KB = source %q metadata %+v", st.Source(), st.Metadata())
	}
}
