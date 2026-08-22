package toolchain

import (
	"context"
	"strings"
	"testing"
)

// 零工具鏈測試（core-lane）：全部走 replay / unavailable，不需要 cargo。

func TestReplayCheckParsesE0502(t *testing.T) {
	r := NewReplay("testdata/e0502")
	res, err := r.Check(context.Background(), "ignored")
	if err != nil {
		t.Fatalf("replay check: %v", err)
	}
	// 錄音帶內：1 條 E0502 錯誤 + 1 條無碼 aborting 錯誤 + 1 條 warning
	if len(res.Errors) != 2 {
		t.Fatalf("errors = %d, want 2", len(res.Errors))
	}
	if res.Warnings != 1 {
		t.Fatalf("warnings = %d, want 1", res.Warnings)
	}
	e := res.Errors[0]
	if e.Code != "E0502" {
		t.Fatalf("code = %q, want E0502", e.Code)
	}
	if e.File != "src/main.rs" || e.Line != 4 || e.Col != 5 {
		t.Fatalf("primary span 錯位: %+v", e)
	}
	if e.Label != "mutable borrow occurs here" {
		t.Fatalf("label = %q", e.Label)
	}
	if fp := e.Fingerprint(); fp != "E0502@src/main.rs:4" {
		t.Fatalf("fingerprint = %q", fp)
	}
}

func TestReplayCleanProject(t *testing.T) {
	r := NewReplay("testdata/clean")
	res, err := r.Check(context.Background(), "ignored")
	if err != nil {
		t.Fatalf("replay check: %v", err)
	}
	if len(res.Errors) != 0 || res.Warnings != 0 {
		t.Fatalf("clean fixture 應零錯誤零警告: %+v", res)
	}
	if ok, _ := r.QuickCheck(context.Background(), ""); !ok {
		t.Fatal("clean quickcheck 應通過")
	}
	if ok, _ := r.Test(context.Background(), ""); !ok {
		t.Fatal("clean test 應通過")
	}
}

func TestReplayQuickCheckAndTestEvidence(t *testing.T) {
	r := NewReplay("testdata/e0502")
	ok, ev := r.QuickCheck(context.Background(), "")
	if ok {
		t.Fatal("e0502 quickcheck 應失敗")
	}
	if !strings.Contains(ev, "E0502") {
		t.Fatalf("證據應含 E0502: %q", ev)
	}
	if ok, _ := r.Test(context.Background(), ""); ok {
		t.Fatal("e0502 test 應失敗")
	}
}

func TestReplayExplainAndVersion(t *testing.T) {
	r := NewReplay("testdata/e0502")
	if ex := r.Explain(context.Background(), "", "E0502"); !strings.Contains(ex, "borrowed as immutable") {
		t.Fatalf("explain 內容不符: %q", ex)
	}
	if ex := r.Explain(context.Background(), "", "E9999"); ex != "" {
		t.Fatalf("未知碼應回空: %q", ex)
	}
	info, err := r.Version(context.Background())
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if !strings.Contains(info.Rustc, "1.98.0") {
		t.Fatalf("version.json 未生效: %+v", info)
	}
}

func TestParseIgnoresNonCompilerMessages(t *testing.T) {
	stream := `{"reason":"compiler-artifact","target":{"name":"x"}}
not-json-line
{"reason":"build-finished","success":true}`
	res := ParseCargoCheckJSON(stream)
	if len(res.Errors) != 0 || res.Warnings != 0 {
		t.Fatalf("非 compiler-message 行應全部忽略: %+v", res)
	}
}

func TestUnavailableDegradesHonestly(t *testing.T) {
	u := NewUnavailable("")
	ok, why := u.Available()
	if ok || why == "" {
		t.Fatal("Unavailable 必須如實申報缺席並附原因")
	}
	if _, err := u.Check(context.Background(), "."); err == nil {
		t.Fatal("Check 應回錯誤供呼叫方降級")
	}
	if ok, ev := u.QuickCheck(context.Background(), "."); ok || ev == "" {
		t.Fatal("QuickCheck 應回不可用證據")
	}
	if u.Explain(context.Background(), ".", "E0502") != "" {
		t.Fatal("Explain 缺席時應回空")
	}
}

func TestNativeAvailableHonest(t *testing.T) {
	// 不論本機有無 cargo，Available 都不得 panic，且不可用時必須有原因。
	n := NewNative()
	ok, why := n.Available()
	if !ok && why == "" {
		t.Fatal("不可用時必須申報原因")
	}
	if ok {
		t.Log("本機有 cargo：native 契約由 contract_test.go 覆蓋")
	}
}

func TestFingerprints(t *testing.T) {
	r := NewReplay("testdata/e0502")
	res, _ := r.Check(context.Background(), "")
	fps := Fingerprints(res.Errors)
	if len(fps) != 2 || fps[0] != "E0502@src/main.rs:4" {
		t.Fatalf("fingerprints = %v", fps)
	}
}
