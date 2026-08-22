package rustutil

import (
	"os"
	"testing"
)

// TestSubcommandsSnapshot 是「clap help 文本格式」的鎖定測試（RK4）：
// subcommand 比對依賴 clap 的人類可讀 help 格式——clap 若改版，
// 此測試先紅，強制評估影響面（而非線上悄悄失靈）。
func TestSubcommandsSnapshot(t *testing.T) {
	help := `Demo CLI

Usage: demo-cli [OPTIONS] <COMMAND>

Commands:
  greet  Greet someone
  sum    Sum a list of integers
  help   Print this message or the help of the given subcommand(s)

Options:
      --verbose  Print extra info
  -h, --help     Print help
  -V, --version  Print version
`
	got := Subcommands(help)
	want := []string{"greet", "sum"}
	if len(got) != len(want) {
		t.Fatalf("subcommands snapshot drifted:\n got  %v\n want %v\n（clap help 格式若改變，需評估比對器影響）", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("subcommands snapshot drifted:\n got  %v\n want %v", got, want)
		}
	}
}

func TestSubcommandsEmpty(t *testing.T) {
	if got := Subcommands("no commands section here"); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestPackageName(t *testing.T) {
	dir := t.TempDir()
	write(t, dir+"/Cargo.toml", "[package]\nname = \"demo-cli\"\nversion = \"0.1.0\"\n")
	if got := PackageName(dir); got != "demo-cli" {
		t.Fatalf("PackageName = %q, want demo-cli", got)
	}
	if got := PackageName(t.TempDir()); got != "" {
		t.Fatalf("missing Cargo.toml should give empty name, got %q", got)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
