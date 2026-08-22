package tail

import (
	"strings"
	"testing"
)

func TestBufferKeepsLastBytesAndFullHash(t *testing.T) {
	b := NewBuffer(5)
	_, _ = b.Write([]byte("hello"))
	_, _ = b.Write([]byte(" world"))
	if !b.Truncated() {
		t.Fatal("expected truncation")
	}
	if got := b.String(); !strings.Contains(got, "kept last 5 of 11 bytes") {
		t.Fatalf("expected truncation marker, got %q", got)
	}
	if !strings.HasSuffix(strings.TrimPrefix(b.String(), "...<truncated; kept last 5 of 11 bytes>\n"), "world") {
		t.Fatalf("expected tail world, got %q", b.String())
	}
	if b.Total() != 11 {
		t.Fatalf("total should count all bytes, got %d", b.Total())
	}
}

func TestBufferHashCoversAllBytesNotJustTail(t *testing.T) {
	small := NewBuffer(3)
	_, _ = small.Write([]byte("abcdef"))
	big := NewBuffer(1000)
	_, _ = big.Write([]byte("abcdef"))
	// 全量 hash 與保留長度無關——收據可比性的根基
	if small.SumHex() != big.SumHex() {
		t.Fatal("full-stream hash must be identical regardless of retained tail size")
	}
}

func TestBufferNoLimitKeepsAll(t *testing.T) {
	b := NewBuffer(0)
	_, _ = b.Write([]byte("everything"))
	if b.String() != "everything" || b.Truncated() {
		t.Fatalf("limit 0 must keep everything: %q", b.String())
	}
}
