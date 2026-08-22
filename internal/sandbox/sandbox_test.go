package sandbox

import (
	"strings"
	"testing"
)

// tail buffer 的測試已移至 internal/tail（唯一實作處，S2/D5 合一）。

func TestBuildEnvDropsInvalidKeys(t *testing.T) {
	env := buildEnv(Config{Network: NetworkNone, Env: map[string]string{
		"GOOD_KEY":   "1",
		"bad key":    "x",
		"LEAD=DASH":  "y",
		"-injection": "z",
		"123NUMERIC": "w",
		"GOOD_KEY2_": "ok",
	}})
	joined := "\n" + strings.Join(env, "\n") + "\n"
	if !strings.Contains(joined, "\nGOOD_KEY=1\n") {
		t.Fatalf("valid key dropped: %v", env)
	}
	if !strings.Contains(joined, "\nGOOD_KEY2_=ok\n") {
		t.Fatalf("valid key dropped: %v", env)
	}
	for _, bad := range []string{"bad key=", "LEAD=DASH=", "-injection=", "123NUMERIC="} {
		if strings.Contains(joined, bad) {
			t.Fatalf("invalid env key leaked: %q in %v", bad, env)
		}
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
