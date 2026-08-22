package toolchain

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// 契約測試（toolchain-lane）：以真實鎖版 cargo 驗證「cargo JSON 診斷格式」
// 未漂移——native 解析結果必須與 replay 錄音帶語義一致。
//
// 行為：
//   - 本機無 cargo：預設 skip（core-lane 保持零 Rust 依賴）；
//   - CI toolchain-lane 設 YKC_REQUIRE_TOOLCHAIN=1：無 cargo 視為失敗（不允許靜默降級）。

func requireToolchain(t *testing.T) *Native {
	t.Helper()
	n := NewNative()
	if ok, why := n.Available(); !ok {
		if os.Getenv("YKC_REQUIRE_TOOLCHAIN") == "1" {
			t.Fatalf("toolchain-lane 要求真實工具鏈，但: %s", why)
		}
		t.Skipf("無 Rust 工具鏈，跳過契約測試: %s", why)
	}
	return n
}

// writeCrate 產生一個帶 E0502 借用錯誤的最小 crate（與 e0502 fixture 同構）。
func writeCrate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(
		"[package]\nname = \"fix-e0502\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\n"), 0o644))
	must(os.MkdirAll(filepath.Join(dir, "src"), 0o755))
	must(os.WriteFile(filepath.Join(dir, "src", "main.rs"), []byte(
		"fn main() {\n    let total = 0;\n    let mut v = vec![1];\n    let first = &v[0];\n    v.push(2);\n    println!(\"{}\", first);\n}\n"), 0o644))
	return dir
}

func TestContractCargoCheckE0502(t *testing.T) {
	n := requireToolchain(t)
	dir := writeCrate(t)
	res, err := n.Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("cargo check 工具鏈級失敗: %v", err)
	}
	var found bool
	for _, e := range res.Errors {
		if e.Code == "E0502" {
			found = true
			if e.File == "" || e.Line <= 0 {
				t.Fatalf("E0502 primary span 缺位置（格式漂移？）: %+v", e)
			}
			if filepath.Base(e.File) != "main.rs" {
				t.Fatalf("file = %q", e.File)
			}
		}
	}
	if !found {
		t.Fatalf("真 cargo 未產出 E0502（診斷 JSON 契約漂移？）: %+v", res.Errors)
	}
}

func TestContractQuickCheckAndExplain(t *testing.T) {
	n := requireToolchain(t)
	dir := writeCrate(t)
	if ok, ev := n.QuickCheck(context.Background(), dir); ok || ev == "" {
		t.Fatalf("壞 crate 的 quickcheck 應失敗且附證據 (ok=%v)", ok)
	}
	if ex := n.Explain(context.Background(), dir, "E0502"); ex == "" {
		t.Fatal("rustc --explain E0502 應有內容")
	}
}

func TestContractVersionAttest(t *testing.T) {
	n := requireToolchain(t)
	info, err := n.Version(context.Background())
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if info.Rustc == "" || info.Cargo == "" {
		t.Fatalf("版本指紋不完整: %+v", info)
	}
	if info.CargoSHA256 == "" {
		t.Fatalf("cargo 二進制 sha256 缺失（attest 需要）: %+v", info)
	}
}
