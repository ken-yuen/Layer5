package serve

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ykc/internal/toolchain"
)

func TestBuildAttestUnavailable(t *testing.T) {
	pay := buildAttest(context.Background(), toolchain.NewUnavailable(""), t.TempDir(), PolicyDegrade)
	if pay.Available {
		t.Fatal("unavailable 工具鏈不得申報可用")
	}
	if pay.Reason == "" {
		t.Fatal("缺席必須附原因")
	}
	if pay.Policy != PolicyDegrade {
		t.Fatalf("policy = %q", pay.Policy)
	}
}

func TestBuildAttestMatchAgainstPinnedChannel(t *testing.T) {
	root := t.TempDir()
	// 鎖定 1.98.0；replay fixture 的 rustc 版本行含 "1.98.0" → match
	if err := os.WriteFile(filepath.Join(root, "rust-toolchain.toml"),
		[]byte("[toolchain]\nchannel = \"1.98.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := toolchain.NewReplay(fixtureDir(t, "e0502"))
	pay := buildAttest(context.Background(), r, root, PolicyStrict)
	if !pay.Available {
		t.Fatalf("replay 應可用: %s", pay.Reason)
	}
	if pay.ExpectedChannel != "1.98.0" {
		t.Fatalf("expected channel = %q", pay.ExpectedChannel)
	}
	if !pay.Match {
		t.Fatalf("rustc %q 應命中 1.98.0", pay.Rustc)
	}
}

func TestBuildAttestMismatch(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "rust-toolchain.toml"),
		[]byte("[toolchain]\nchannel = \"1.99.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := toolchain.NewReplay(fixtureDir(t, "e0502")) // fixture 是 1.98.0
	pay := buildAttest(context.Background(), r, root, PolicyStrict)
	if pay.Match {
		t.Fatalf("1.98.0 不應命中鎖定的 1.99.0: %+v", pay)
	}
}

func TestBuildAttestNoPinIsMatch(t *testing.T) {
	// 未鎖版環境不誤紅：無 rust-toolchain.toml → match = true
	r := toolchain.NewReplay(fixtureDir(t, "e0502"))
	pay := buildAttest(context.Background(), r, t.TempDir(), PolicyDegrade)
	if !pay.Match {
		t.Fatal("無鎖定時應視為 match")
	}
}

// fixtureDir 定位 internal/toolchain 的 testdata（相對本套件目錄）。
func fixtureDir(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "toolchain", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("fixture 缺失: %v", err)
	}
	return p
}
