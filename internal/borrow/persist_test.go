package borrow

import (
	"strings"
	"testing"
)

func TestReportRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if got := ReadReport(dir, 0); got != nil {
		t.Fatal("無報告應回 nil")
	}
	rep := AnalysisReport{
		Codes: []string{"E0502"}, Reduced: 1, RedEdges: 2, L5Available: true,
		Graphs:            []ConflictGraph{{Fn: "f"}},
		ExplanationSHA256: "abc", Explanation: "text", UpdatedAt: "2026-08-22T00:00:00Z",
	}
	if err := WriteReport(dir, rep); err != nil {
		t.Fatalf("寫入失敗: %v", err)
	}
	got := ReadReport(dir, 0)
	if got == nil || got.RedEdges != 2 || got.Reduced != 1 || len(got.Graphs) != 1 {
		t.Fatalf("讀回不符: %+v", got)
	}
	RemoveReport(dir)
	if ReadReport(dir, 0) != nil {
		t.Fatal("清除後應回 nil")
	}
}

func TestReadReportTruncation(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("x", 100)
	_ = WriteReport(dir, AnalysisReport{Explanation: long})
	got := ReadReport(dir, 10)
	if got == nil || len(got.Explanation) <= 10 || !strings.Contains(got.Explanation, "截斷") {
		t.Fatalf("截斷邏輯錯誤: %d bytes", len(got.Explanation))
	}
	// maxExplain=0 = 不截斷
	if got := ReadReport(dir, 0); got == nil || got.Explanation != long {
		t.Fatal("maxExplain=0 不應截斷")
	}
}
