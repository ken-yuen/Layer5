package sandbox

import (
	"strings"
	"testing"
)

func TestTailBufferKeepsLastBytesAndFullHash(t *testing.T) {
	b := newTailBuffer(5)
	_, _ = b.Write([]byte("hello"))
	_, _ = b.Write([]byte(" world"))
	if !b.Truncated() {
		t.Fatal("expected truncation")
	}
	if got := string(b.Bytes()); got != "world" {
		t.Fatalf("expected tail world, got %q", got)
	}
	if !strings.Contains(b.String(), "kept last 5 of 11 bytes") {
		t.Fatalf("expected truncation marker, got %q", b.String())
	}
}

func TestBuildEnvOverridesHostAndOmitsProxyWhenOffline(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://proxy.invalid")
	env := buildEnv(Config{Network: NetworkNone, Env: map[string]string{"HOME": "/work/.ykc/home"}})
	joined := "\n" + strings.Join(env, "\n") + "\n"
	if !strings.Contains(joined, "\nHOME=/work/.ykc/home\n") {
		t.Fatalf("expected HOME override, got %v", env)
	}
	if strings.Contains(joined, "HTTP_PROXY=") {
		t.Fatalf("proxy leaked into offline env: %v", env)
	}
	if !strings.Contains(joined, "\nCARGO_NET_OFFLINE=true\n") {
		t.Fatalf("expected cargo offline env, got %v", env)
	}
}
