package ledger

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "ykc-ledger-anchor-test-*")
	if err != nil {
		panic(err)
	}
	oldDir, hadDir := os.LookupEnv("YKC_ANCHOR_DIR")
	oldKey, hadKey := os.LookupEnv("YKC_ANCHOR_KEY_FILE")
	_ = os.Setenv("YKC_ANCHOR_DIR", filepath.Join(root, "anchors"))
	_ = os.Setenv("YKC_ANCHOR_KEY_FILE", filepath.Join(root, "anchor.key"))
	code := m.Run()
	if hadDir {
		_ = os.Setenv("YKC_ANCHOR_DIR", oldDir)
	} else {
		_ = os.Unsetenv("YKC_ANCHOR_DIR")
	}
	if hadKey {
		_ = os.Setenv("YKC_ANCHOR_KEY_FILE", oldKey)
	} else {
		_ = os.Unsetenv("YKC_ANCHOR_KEY_FILE")
	}
	_ = os.RemoveAll(root)
	os.Exit(code)
}

func newAnchoredLedger(t *testing.T, n int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	mustAppend(t, l, n)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHeadAnchorRoundTrip(t *testing.T) {
	path := newAnchoredLedger(t, 3)
	ok, head, status, err := VerifyAnchored(path)
	if err != nil || !ok {
		t.Fatalf("VerifyAnchored ok=%v status=%+v err=%v", ok, status, err)
	}
	if status.State != "anchored" || status.AnchorSeq != 3 || status.CurrentSeq != 3 || status.AnchorHead != head {
		t.Fatalf("unexpected anchor status: %+v", status)
	}
	if _, err := os.Stat(status.AnchorPath); err != nil {
		t.Fatalf("anchor file missing: %v", err)
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(status.AnchorPath)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm()&0o077 != 0 {
			t.Fatalf("anchor is not private: mode=%o", fi.Mode().Perm())
		}
	}
}

func TestHeadAnchorDetectsTruncationRollback(t *testing.T) {
	path := newAnchoredLedger(t, 3)
	lines := readLines(t, path)
	if len(lines) != 3 {
		t.Fatalf("line count = %d", len(lines))
	}
	// 保留完整、合法的前兩行；單看 hash chain 仍然成立，只有獨立 anchor 能知悉
	// 第三行曾經存在。
	writeLines(t, path, lines[:2])
	if ok, _, err := VerifyChain(path); err != nil || !ok {
		t.Fatalf("truncated prefix must still be internally valid: ok=%v err=%v", ok, err)
	}
	ok, _, status, err := VerifyAnchored(path)
	if ok || !errors.Is(err, ErrAnchorRollback) || status.State != "rollback" {
		t.Fatalf("want rollback detection, ok=%v status=%+v err=%v", ok, status, err)
	}
	if _, err := Open(path); !errors.Is(err, ErrAnchorRollback) {
		t.Fatalf("writer must fail closed on rollback, got %v", err)
	}
}

func TestHeadAnchorDetectsResignedTail(t *testing.T) {
	path := newAnchoredLedger(t, 3)
	facts := make([]Fact, 0, 3)
	for _, line := range readLines(t, path) {
		var fact Fact
		if err := json.Unmarshal(line, &fact); err != nil {
			t.Fatal(err)
		}
		facts = append(facts, fact)
	}
	// 攻擊者重寫最後一條並重新計算 hash；裸 chain 仍會綠，但其 head 不再等於
	// 專案外 anchor。
	facts[2].Payload = []byte(`{"i":999}`)
	facts[2].Hash = factHash(facts[2].PrevHash, facts[2].Type, facts[2].Actor, facts[2].Seq, facts[2].Payload)
	writeFacts(t, path, facts)
	if ok, _, err := VerifyChain(path); err != nil || !ok {
		t.Fatalf("resigned tail should remain chain-valid in isolation: ok=%v err=%v", ok, err)
	}
	ok, _, status, err := VerifyAnchored(path)
	if ok || !errors.Is(err, ErrAnchorMismatch) || status.State != "mismatch" {
		t.Fatalf("want anchor mismatch, ok=%v status=%+v err=%v", ok, status, err)
	}
}

func TestHeadAnchorDetectsAnchorTamper(t *testing.T) {
	path := newAnchoredLedger(t, 1)
	_, _, status, err := VerifyAnchored(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(status.AnchorPath)
	if err != nil {
		t.Fatal(err)
	}
	var anchor HeadAnchor
	if err := json.Unmarshal(b, &anchor); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(anchor.Signature, "00") {
		anchor.Signature = "ff" + anchor.Signature[2:]
	} else {
		anchor.Signature = "00" + anchor.Signature[2:]
	}
	b, err = json.Marshal(anchor)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(status.AnchorPath, b, 0o600); err != nil {
		t.Fatal(err)
	}
	ok, _, got, err := VerifyAnchored(path)
	if ok || !errors.Is(err, ErrAnchorInvalid) || got.State != "invalid" {
		t.Fatalf("want invalid anchor, ok=%v status=%+v err=%v", ok, got, err)
	}
}

func TestOptionalRemoteWitnessRecordsAndVerifiesAnchor(t *testing.T) {
	var stored HeadAnchor
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&stored); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			if r.URL.Query().Get("ledger_id") != stored.LedgerID {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]HeadAnchor{"anchor": stored})
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()
	t.Setenv("YKC_ANCHOR_WITNESS_URL", server.URL)
	t.Setenv("YKC_ANCHOR_WITNESS_VERIFY", "true")

	path := newAnchoredLedger(t, 2)
	ok, _, status, err := VerifyAnchored(path)
	if err != nil || !ok || status.WitnessState != "verified" || status.WitnessSeq != 2 {
		t.Fatalf("remote witness verification = ok %v status %+v err %v", ok, status, err)
	}
}

func TestRequiredRemoteWitnessFailureIsReportedAfterCommittedAppend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	t.Setenv("YKC_ANCHOR_WITNESS_URL", server.URL)
	t.Setenv("YKC_ANCHOR_WITNESS_REQUIRED", "true")
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	seq, err := l.Append("test", "witness-test", map[string]bool{"ok": true})
	_ = l.Close()
	if err == nil || seq != 1 {
		t.Fatalf("required witness failure must surface committed seq, seq=%d err=%v", seq, err)
	}
	// Local independent anchor was committed before remote transport failed, so a later
	// non-required verification remains able to protect the chain.
	t.Setenv("YKC_ANCHOR_WITNESS_REQUIRED", "")
	ok, _, status, err := VerifyAnchored(path)
	if err != nil || !ok || status.State != "anchored" {
		t.Fatalf("local anchor should survive witness outage: ok=%v status=%+v err=%v", ok, status, err)
	}
}

func TestLegacyChainBootstrapsAnchorOnNextAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	payload := []byte(`{"legacy":true}`)
	fact := Fact{Seq: 1, Type: "legacy", Actor: "test", Payload: payload, PrevHash: Genesis}
	fact.Hash = factHash(fact.PrevHash, fact.Type, fact.Actor, fact.Seq, fact.Payload)
	writeFacts(t, path, []Fact{fact})

	ok, _, status, err := VerifyAnchored(path)
	if err != nil || !ok || status.State != "unanchored" {
		t.Fatalf("legacy ledger should be allowed to bootstrap: ok=%v status=%+v err=%v", ok, status, err)
	}
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Append("post.upgrade", "test", map[string]bool{"ok": true}); err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
	ok, _, status, err = VerifyAnchored(path)
	if err != nil || !ok || status.State != "anchored" || status.AnchorSeq != 2 {
		t.Fatalf("legacy anchor bootstrap failed: ok=%v status=%+v err=%v", ok, status, err)
	}
}

func TestVerifyChainRejectsSequenceGap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	f1 := Fact{Seq: 1, Type: "x", Actor: "t", Payload: []byte(`{"n":1}`), PrevHash: Genesis}
	f1.Hash = factHash(f1.PrevHash, f1.Type, f1.Actor, f1.Seq, f1.Payload)
	f3 := Fact{Seq: 3, Type: "x", Actor: "t", Payload: []byte(`{"n":3}`), PrevHash: f1.Hash}
	f3.Hash = factHash(f3.PrevHash, f3.Type, f3.Actor, f3.Seq, f3.Payload)
	writeFacts(t, path, []Fact{f1, f3})
	if ok, _, err := VerifyChain(path); err != nil || ok {
		t.Fatalf("sequence gap must fail chain verification: ok=%v err=%v", ok, err)
	}
}
