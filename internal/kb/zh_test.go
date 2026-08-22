package kb

import (
	"strings"
	"testing"
)

func TestTier1ErrorCardsHaveTraditionalChineseSummaries(t *testing.T) {
	_, _, _, boost, err := loadSeed()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ErrorTranslationCount(), len(boost.Tier1); got != want {
		t.Fatalf("translation coverage = %d, tier-1 = %d", got, want)
	}
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range boost.Tier1 {
		atom, ok := st.ByCode(code)
		if !ok || atom.Kind != KindError || strings.TrimSpace(atom.ZH) == "" {
			t.Fatalf("tier-1 %s lacks zh-Hant card: %+v", code, atom)
		}
	}
}

func TestErrorMarkdownRendersTraditionalChineseSeparately(t *testing.T) {
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	bundle := st.Retrieve("E0382", SearchOpts{K: 1, ExpandDepth: 0, BudgetBytes: 4096})
	markdown := RenderMarkdown(bundle)
	if !strings.Contains(markdown, "**繁中摘要**") || !strings.Contains(markdown, "值已被 move") {
		t.Fatalf("error card did not render zh summary:\n%s", markdown)
	}
	if !strings.Contains(markdown, "A variable was used after") {
		t.Fatal("English official source must remain alongside translation")
	}
}
