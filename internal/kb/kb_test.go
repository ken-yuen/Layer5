package kb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCounts(t *testing.T) {
	s, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := len(s.AtomsByKind(KindError)); got != 518 {
		t.Errorf("error atoms = %d, want 518", got)
	}
	if got := len(s.AtomsByKind(KindRule)); got != 54 {
		t.Errorf("rule atoms = %d, want 54", got)
	}
	if got := len(s.AtomsByKind(KindBook)); got != 91 {
		t.Errorf("book atoms = %d, want 91", got)
	}
	if got := len(s.AtomsByKind(KindPart)); got != 19 {
		t.Errorf("part atoms = %d, want 19", got)
	}
	if got := len(s.AtomsByKind(KindTOC)); got != 1 {
		t.Errorf("toc atoms = %d, want 1", got)
	}
	if s.Count() != 518+54+91+19+1 {
		t.Errorf("total = %d", s.Count())
	}
}

func TestVersionDeterministic(t *testing.T) {
	a, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if a.Version() != b.Version() {
		t.Fatalf("version differs: %s vs %s", a.Version(), b.Version())
	}
	if len(a.Version()) != 16 {
		t.Fatalf("version should be 16 hex chars, got %q", a.Version())
	}
}

func TestByCodeCaseInsensitive(t *testing.T) {
	s, _ := Open()
	if _, ok := s.ByCode("E0382"); !ok {
		t.Fatal("E0382 not found")
	}
	if _, ok := s.ByCode("e0382"); !ok {
		t.Fatal("lowercase e0382 not found")
	}
	if a, ok := s.ByCode("OWN-01"); !ok || a.Kind != KindRule {
		t.Fatal("OWN-01 rule not found")
	}
	if a, ok := s.ByCode("what-is-ownership-1"); !ok || a.Kind != KindBook {
		t.Fatal("book chapter not found")
	}
}

func TestSearchExactCode(t *testing.T) {
	s, _ := Open()
	hits := s.Search("E0382", 3)
	if len(hits) == 0 || hits[0].Atom.Code != "E0382" {
		t.Fatalf("exact code search top hit = %+v", hits)
	}
}

func TestSearchSemantic(t *testing.T) {
	s, _ := Open()

	topCode := func(q string, k int) []string {
		var out []string
		for _, h := range s.Search(q, k) {
			out = append(out, h.Atom.Code)
		}
		return out
	}

	// "trait not implemented" 應命中 E0277 或 TRT-* 規則。
	got := strings.Join(topCode("trait not implemented", 5), ",")
	if !containsAny(got, "E0277", "TRT-01", "TRT-02") {
		t.Errorf("trait query top5 = %s", got)
	}
	// "used after move" 應命中 E0382。
	got = strings.Join(topCode("variable used after move", 5), ",")
	if !strings.Contains(got, "E0382") {
		t.Errorf("move query top5 = %s", got)
	}
	// 拼寫誤差（borow checkr）→ shingle 模糊回退仍應命中 borrow 領域知識。
	typo := s.Search("borow checkr", 10)
	if len(typo) == 0 {
		t.Fatal("typo query returned nothing (shingle fallback broken)")
	}
	borrowFamily := map[string]bool{
		"E0521": true, "E0525": true, "E0502": true, "E0596": true, "E0499": true,
		"E0597": true, "E0106": true, "E0515": true,
	}
	found := false
	for _, h := range typo {
		if borrowFamily[h.Atom.Code] {
			found = true
			break
		}
		if h.Atom.Kind == KindRule {
			switch strings.ToLower(h.Atom.Domain) {
			case "borrowing", "lifetime":
				found = true
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Errorf("typo query did not surface borrowing knowledge: %s", got)
	}
}

func TestSearchDeterministic(t *testing.T) {
	s, _ := Open()
	a := s.Search("cannot borrow as mutable", 10)
	b := s.Search("cannot borrow as mutable", 10)
	if len(a) != len(b) {
		t.Fatalf("result length differs")
	}
	for i := range a {
		if a[i].Atom.ID != b[i].Atom.ID || a[i].Score != b[i].Score {
			t.Fatalf("result %d differs", i)
		}
	}
}

func TestGraphExpand(t *testing.T) {
	s, _ := Open()
	a, _ := s.ByCode("E0382")
	atoms := s.ExpandContext([]string{a.ID}, 1)
	codes := map[string]bool{}
	for _, n := range atoms {
		codes[n.Code] = true
	}
	if !codes["E0382"] {
		t.Fatal("root E0382 missing from expansion")
	}
	// E0382 被 OWN-* 規則引用（backrefs），展開應帶出至少一條 ownership 規則。
	hasOwn := false
	for _, n := range atoms {
		if n.Kind == KindRule && strings.EqualFold(n.Domain, "ownership") {
			hasOwn = true
		}
	}
	if !hasOwn {
		t.Errorf("expansion did not include ownership rules: %v", codes)
	}
	// 章節應指向其部（chapter → part）。
	ch, _ := s.ByCode("what-is-ownership-1")
	deps := s.Deps(ch.ID)
	if len(deps) == 0 || deps[0].Kind != KindPart {
		t.Errorf("chapter deps = %v, want its part", deps)
	}
}

func TestContextCacheAndBudget(t *testing.T) {
	s, _ := Open()
	b1 := s.Retrieve("E0382", SearchOpts{K: 3, ExpandDepth: 2})
	if b1.CacheHit {
		t.Fatal("first retrieve should be a miss")
	}
	b2 := s.Retrieve("E0382", SearchOpts{K: 3, ExpandDepth: 2})
	if !b2.CacheHit {
		t.Fatal("second retrieve should be a cache hit")
	}
	cs := s.CacheStats()
	if cs.Hits < 1 {
		t.Errorf("cache hits = %d", cs.Hits)
	}
	// 極小預算 → 截斷。
	bt := s.Retrieve("ownership borrowing lifetime", SearchOpts{K: 10, ExpandDepth: 3, BudgetBytes: 300})
	if !bt.Truncated {
		t.Errorf("expected truncation with tiny budget (total=%d)", bt.TotalBytes)
	}
}

func TestRenderMarkdown(t *testing.T) {
	s, _ := Open()
	b := s.Retrieve("E0382", SearchOpts{K: 1, ExpandDepth: 1})
	md := RenderMarkdown(b)
	if !strings.Contains(md, "E0382") || !strings.Contains(md, "```rust") {
		t.Errorf("markdown missing expected parts:\n%s", md[:200])
	}
}

func TestBlobRoundTripAndTamper(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kb.ykc")
	if err := Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	emb, _ := Open()
	if s.Version() != emb.Version() {
		t.Fatalf("blob version %s != embedded %s", s.Version(), emb.Version())
	}
	if s.Count() != emb.Count() {
		t.Fatalf("blob count %d != embedded %d", s.Count(), emb.Count())
	}
	// 竄改 → 開檔必須失敗。
	b, _ := os.ReadFile(path)
	b[len(b)-10] ^= 0xff
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFile(path); err == nil {
		t.Fatal("tampered blob must fail checksum")
	}
}

func TestGraphExpandPreservesRootOrder(t *testing.T) {
	s, _ := Open()
	a, _ := s.ByCode("E0382")
	b, _ := s.ByCode("E0502")
	order := s.graph.ExpandContext([]string{a.ID, b.ID}, 0)
	if len(order) != 2 || order[0] != a.ID || order[1] != b.ID {
		t.Fatalf("root order not preserved: %v", order)
	}
	// 決定論：同一輸入重複展開結果一致。
	order2 := s.graph.ExpandContext([]string{a.ID, b.ID}, 0)
	for i := range order {
		if order[i] != order2[i] {
			t.Fatalf("expansion not deterministic at %d", i)
		}
	}
}

func TestContentAddressing(t *testing.T) {
	a := &Atom{Kind: KindError, Code: "E9999", Title: "x", Body: "y"}
	b := &Atom{Kind: KindError, Code: "E9999", Title: "x", Body: "y"}
	if a.contentHash() != b.contentHash() {
		t.Fatal("same content must hash identically")
	}
	c := &Atom{Kind: KindError, Code: "E9999", Title: "x", Body: "z"}
	if a.contentHash() == c.contentHash() {
		t.Fatal("different content must hash differently")
	}
}

func containsAny(haystack string, substrs ...string) bool {
	for _, s := range substrs {
		if strings.Contains(haystack, s) {
			return true
		}
	}
	return false
}
