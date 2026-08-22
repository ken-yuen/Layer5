package ledger

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustAppend(t *testing.T, l *Ledger, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		if _, err := l.Append("test.fact", "tester", map[string]int{"i": i}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
}

func TestAppendVerifyRoundtrip(t *testing.T) {
	l, err := Open(filepath.Join(t.TempDir(), "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	mustAppend(t, l, 3)
	if head := l.Head(); head == Genesis {
		t.Fatal("head not advanced")
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	ok, head, err := VerifyChain(l.Path())
	if err != nil || !ok || head == "" {
		t.Fatalf("verify ok=%v err=%v", ok, err)
	}
}

func TestTamperedMiddleLineDetected(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ledger.jsonl")
	l, _ := Open(p)
	mustAppend(t, l, 4)
	l.Close()

	// 改中間一條的 payload（不改 hash）→ 必須偵測
	lines := readLines(t, p)
	bad := string(lines[1])
	// 找到 "i": 2 改為 "i": 99
	mod := strings.Replace(bad, `"i":2`, `"i":99`, 1)
	if mod == bad {
		t.Fatal("test setup: could not patch line 2")
	}
	lines[1] = []byte(mod)
	writeLines(t, p, lines)

	ok, _, err := VerifyChain(p)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("tampered ledger must not verify")
	}
	if _, err := OpenVerified(p); err == nil {
		t.Fatal("OpenVerified must reject tampered ledger")
	}
}

func TestResignedTamperStillDetectedForMiddle(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ledger.jsonl")
	l, _ := Open(p)
	mustAppend(t, l, 4)
	l.Close()

	// 攻擊：改第 2 條 payload 並「重簽」該條 hash（但後續 PrevHash 斷裂）
	facts := make([]Fact, 0, 4)
	for _, ln := range readLines(t, p) {
		var f Fact
		if err := json.Unmarshal(ln, &f); err != nil {
			t.Fatal(err)
		}
		facts = append(facts, f)
	}
	facts[1].Payload = []byte(`{"i":99}`)
	facts[1].Hash = factHash(facts[1].PrevHash, facts[1].Type, facts[1].Actor, facts[1].Seq, facts[1].Payload)
	// 攻擊者不知道後續行——只重簽第 2 條
	writeFacts(t, p, facts)

	if ok, _, err := VerifyChain(p); err != nil || ok {
		t.Fatalf("resigned middle tamper must be detected, got ok=%v err=%v", ok, err)
	}
}

func TestOversizedLineRejected(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ledger.jsonl")
	l, _ := Open(p)
	mustAppend(t, l, 1)
	l.Close()

	// 手工寫入超過 MaxLineBytes 的一行
	huge := make([]byte, MaxLineBytes+1)
	for i := range huge {
		huge[i] = 'a'
	}
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	f.Write(append(huge, '\n'))
	f.Close()

	if _, _, err := VerifyChain(p); err == nil {
		t.Fatal("oversized line must be an explicit error")
	}
}

func TestNoTrailingNewlineStillReadable(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ledger.jsonl")
	l, _ := Open(p)
	mustAppend(t, l, 2)
	l.Close()

	b, _ := os.ReadFile(p)
	// 去掉結尾換行
	b = b[:len(b)-1]
	os.WriteFile(p, b, 0o644)

	if facts := ReadAll(p); len(facts) != 2 {
		t.Fatalf("want 2 facts, got %d", len(facts))
	}
	ok, _, err := VerifyChain(p)
	if err != nil || !ok {
		t.Fatalf("verify ok=%v err=%v", ok, err)
	}
}

func TestFlockSecondWriterBlocked(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ledger.jsonl")
	l1, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer l1.Close()
	mustAppend(t, l1, 1)

	l2, err := Open(p)
	if err == nil {
		l2.Close()
		t.Fatal("second concurrent writer must be blocked (single-writer invariant)")
	}
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("want ErrLocked, got %v", err)
	}
}

func TestOpenVerifiedAcceptsCleanLedger(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ledger.jsonl")
	l, _ := Open(p)
	mustAppend(t, l, 2)
	_ = l.Close()
	lv, err := OpenVerified(p)
	if err != nil {
		t.Fatal(err)
	}
	lv.Close()
}

// ── helpers ──

func readLines(t *testing.T, p string) [][]byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var out [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			out = append(out, b[start:i])
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, b[start:])
	}
	return out
}

func writeLines(t *testing.T, p string, lines [][]byte) {
	t.Helper()
	var sb strings.Builder
	for _, l := range lines {
		sb.Write(l)
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(p, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeFacts(t *testing.T, p string, facts []Fact) {
	t.Helper()
	var sb strings.Builder
	for _, f := range facts {
		b, _ := json.Marshal(f)
		sb.Write(b)
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(p, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
