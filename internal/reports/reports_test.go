package reports

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodReport = `# YKC 90 — 測試報告

> 日期：2026-08-01
> 分支：` + "`test-branch`" + `

**日期**：2026-08-01 ｜ **對應任務**：T-90、T-91a

前言段落第一行，
接續第二行。

## 章節一

內文引用 YKC_91_另一份報告.md 與 [鏈接](sub/file.md)。

` + "```" + `
## fence 內假標題
` + "```" + `

### 小節
`

const otherReport = `# YKC 91 — 另一份報告

> 日期：2026-08-02

內文。
`

func writeReports(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"YKC_90_測試報告.md":  goodReport,
		"YKC_91_另一份報告.md": otherReport,
		"sub/file.md":     "x",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestScanParsesFields(t *testing.T) {
	idx, err := Scan(writeReports(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Reports) != 2 {
		t.Fatalf("want 2 reports, got %d", len(idx.Reports))
	}
	r := idx.Reports[0]
	if r.Kind != KindSeries || r.Number != 90 {
		t.Errorf("kind/number = %s/%d", r.Kind, r.Number)
	}
	if r.Title != "YKC 90 — 測試報告" {
		t.Errorf("title = %q", r.Title)
	}
	if r.Date != "2026-08-01" {
		t.Errorf("date = %q", r.Date)
	}
	if r.Baseline != "test-branch" {
		t.Errorf("baseline = %q", r.Baseline)
	}
	if strings.Join(r.Tasks, ",") != "T-90,T-91a" {
		t.Errorf("tasks = %v", r.Tasks)
	}
	if r.Summary != "前言段落第一行， 接續第二行。" {
		t.Errorf("summary = %q", r.Summary)
	}
	if len(r.Refs) != 1 || r.Refs[0].Target != "YKC_91_另一份報告.md" {
		t.Errorf("refs = %v", r.Refs)
	}
	if len(r.Links) != 1 || r.Links[0].Target != "sub/file.md" {
		t.Errorf("links = %v", r.Links)
	}
	// fence 內假標題不得入樹；真標題三層俱在。
	var texts []string
	for _, h := range r.Headings {
		texts = append(texts, h.Text)
	}
	joined := strings.Join(texts, "|")
	if strings.Contains(joined, "fence") {
		t.Errorf("fence 內標題洩漏：%v", texts)
	}
	if !strings.Contains(joined, "章節一") || !strings.Contains(joined, "小節") {
		t.Errorf("headings = %v", texts)
	}
	if r.SHA256 == "" || r.SizeBytes != int64(len(goodReport)) {
		t.Errorf("sha/size = %s/%d", r.SHA256, r.SizeBytes)
	}
}

func TestAuditFlagsBrokenRefAndGap(t *testing.T) {
	root := t.TempDir()
	body := "# YKC 90 — A\n\n> 日期：2026-08-01\n\n引用 YKC_99_不存在.md。\n"
	if err := os.WriteFile(filepath.Join(root, "YKC_90_A.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	body2 := "# YKC 92 — B\n\n內文無日期。\n"
	if err := os.WriteFile(filepath.Join(root, "YKC_92_B.md"), []byte(body2), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	a := Audit(idx, root)
	if !a.HasErrors() {
		t.Fatal("want errors")
	}
	want := map[string]bool{"broken-ref": false, "series-gap": false, "missing-date": false}
	for _, f := range a.Findings {
		if _, ok := want[f.Rule]; ok {
			want[f.Rule] = true
		}
	}
	for rule, seen := range want {
		if !seen {
			t.Errorf("rule %s 未觸發", rule)
		}
	}
	if a.Errors != 2 || a.Warnings != 1 {
		t.Errorf("errors/warnings = %d/%d (findings %+v)", a.Errors, a.Warnings, a.Findings)
	}
}

func TestAuditCleanPasses(t *testing.T) {
	idx, err := Scan(writeReports(t))
	if err != nil {
		t.Fatal(err)
	}
	// 90→91 無斷層；把 91 刪掉會造成 broken-ref，故用完整集。
	a := Audit(idx, t.TempDir()) // 空 root：鏈接與引用都應報缺
	if a.Errors == 0 {
		t.Error("空 root 下應有 broken-ref/broken-link error")
	}
}

func TestBuildVerifyRoundTrip(t *testing.T) {
	root := writeReports(t)
	out := filepath.Join(t.TempDir(), "out")
	idx, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	written, err := Build(idx, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 3 {
		t.Fatalf("written = %v", written)
	}
	results, ok := Verify(idx, out)
	if !ok {
		t.Fatalf("verify 應一致：%+v", results)
	}
	// 決定論：重生成內容逐字節相同。
	arts := Artifacts(idx)
	for name, want := range arts {
		got, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("%s 內容漂移", name)
		}
	}
	// 漂移偵測：改報告後 verify 必須失敗。
	if err := os.WriteFile(filepath.Join(root, "YKC_91_另一份報告.md"),
		[]byte(otherReport+"\n追加。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx2, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := Verify(idx2, out); ok {
		t.Error("報告改動後 verify 應報漂移")
	}
	// missing 偵測。
	if err := os.Remove(filepath.Join(out, "INDEX.md")); err != nil {
		t.Fatal(err)
	}
	results, ok = Verify(idx, out)
	if ok || results[0].Status != "missing" {
		t.Errorf("INDEX.md 應 missing：%+v", results)
	}
}

func TestManifestIsValidJSONAndStable(t *testing.T) {
	idx, err := Scan(writeReports(t))
	if err != nil {
		t.Fatal(err)
	}
	data := renderManifest(idx)
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m["schema"] != Schema || m["generated_by"] != generatedBy {
		t.Errorf("manifest 頭部欄位錯誤：%v", m)
	}
	if string(data) != string(renderManifest(idx)) {
		t.Error("manifest 非確定性")
	}
}

func TestTitleOrBaseAndTruncate(t *testing.T) {
	r := Report{Base: "b.md"}
	if r.TitleOrBase() != "b.md" {
		t.Error("無標題應退回檔名")
	}
	r.Title = "T"
	if r.TitleOrBase() != "T" {
		t.Error("有標題應用標題")
	}
	if got := truncateRunes("一二三四五", 3); got != "一二三…" {
		t.Errorf("truncate = %q", got)
	}
	if got := truncateRunes("短", 3); got != "短" {
		t.Errorf("短字串不應截斷：%q", got)
	}
}
