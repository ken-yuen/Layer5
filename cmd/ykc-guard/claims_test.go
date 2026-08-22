package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExtractClaimsSnapshot 是「自由文本→聲明抽取」的鎖定測試（RK4）：
// 啟發式 regex 的任何調整都會改變代理被比對的面——必須顯式測試。
func TestExtractClaimsSnapshot(t *testing.T) {
	// 注意：fixture 措辭必須是 regex 現行能匹配的（鎖定「現行為」，非理想行為）。
	// 例如「已成功編譯」不匹配（regex 要求「編譯成功/通過」序）——改措辭即改抽取面，需審視。
	msgs := []string{
		"我已加上 --json 與 --verbose 旗標，支援輸出格式控制。",
		"專案編譯成功，cargo build 通過。",
		"所有測試全部通過，可放心合併。",
		"今天天氣很好（無任何可驗證聲明）。",
	}
	got := extractClaims(msgs)
	type want struct{ feature string }
	wantList := []want{
		{feature: "flag:--json"},
		{feature: "flag:--verbose"},
		{feature: "compiles"},
		{feature: "tests_pass"},
	}
	if len(got) != len(wantList) {
		t.Fatalf("extract snapshot drifted: got %d claims %v, want %d", len(got), features(got), len(wantList))
	}
	for i, w := range wantList {
		if got[i].Feature != w.feature {
			t.Fatalf("extract snapshot drifted at %d: got %q want %q (all: %v)", i, got[i].Feature, w.feature, features(got))
		}
	}
}

func features(cs []Claim) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Feature
	}
	return out
}

func TestLoadClaimsRequiresAgentID(t *testing.T) {
	p := filepath.Join(t.TempDir(), "claims.json")
	os.WriteFile(p, []byte(`{"claims":[{"id":"c1","text":"x","feature":"compiles"}]}`), 0o644)
	if _, err := loadClaims(p); err == nil {
		t.Fatal("claims without agent_id must be rejected (anti-anonymous trust)")
	}
}

func TestLoadClaimsRejectsOversizedFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "claims.json")
	big := make([]byte, MaxClaimsFileSize+1)
	os.WriteFile(p, big, 0o644)
	if _, err := loadClaims(p); err == nil {
		t.Fatal("oversized claims file must be rejected")
	}
}

func TestContradictedSeverityMatrix(t *testing.T) {
	cases := map[string]int{
		"flag:--x":       1,
		"subcommand:x":   1,
		"file:x":         2,
		"function:x":     2,
		"compiles":       3,
		"tests_pass":     3,
		"receipt:abc123": 4,
	}
	for f, want := range cases {
		if got := contradictedSeverity(f); got != want {
			t.Fatalf("severity(%s) = %d, want %d", f, got, want)
		}
	}
}
