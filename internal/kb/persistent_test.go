package kb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPersistentCacheSharesOnlyVersionMatchedContexts(t *testing.T) {
	dir := t.TempDir()
	// t.TempDir 的 mode 受平台／umask 影響；persistent cache 明確要求私有目錄。
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("YKC_KB_CACHE_DIR", dir)
	opts := SearchOpts{K: 1, ExpandDepth: 1, BudgetBytes: 4096}

	firstStore, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	first := firstStore.Retrieve("E0382", opts)
	if first.CacheHit {
		t.Fatal("first process query must not be a cache hit")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("persistent cache files = %v, err=%v", files, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(files[0])
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("persistent cache must be private: %o", info.Mode().Perm())
		}
	}

	secondStore, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	second := secondStore.Retrieve("E0382", opts)
	if !second.CacheHit || len(second.Atoms) == 0 || second.Atoms[0].Code != "E0382" {
		t.Fatalf("second process did not rehydrate disk cache: %+v", second)
	}
	if stats := secondStore.CacheStats().Persistent; !stats.Enabled || stats.Hits != 1 {
		t.Fatalf("persistent stats = %+v", stats)
	}

	// 偽造另一資料版本：快取不是權威資料，版本不符時必須丟棄並重新組裝。
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var entry persistedContext
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatal(err)
	}
	entry.Version = "different-dataset-version"
	data, err = json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(files[0], data, 0o600); err != nil {
		t.Fatal(err)
	}
	thirdStore, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	third := thirdStore.Retrieve("E0382", opts)
	if third.CacheHit {
		t.Fatal("version-mismatched disk cache must not be accepted")
	}
	if stats := thirdStore.CacheStats().Persistent; stats.Errors == 0 {
		t.Fatalf("invalid entry should be counted as safe cache error: %+v", stats)
	}
}

func TestPersistentCacheRejectsInsecureDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission semantics do not apply")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YKC_KB_CACHE_DIR", dir)
	store, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	bundle := store.Retrieve("E0382", SearchOpts{K: 1, ExpandDepth: 0, BudgetBytes: 4096})
	if bundle.CacheHit {
		t.Fatal("insecure directory must not be accepted as cache hit")
	}
	if stats := store.CacheStats().Persistent; stats.Errors == 0 {
		t.Fatalf("insecure cache directory must be recorded as error: %+v", stats)
	}
}

func TestPersistentCacheDisabledWithoutExplicitDirectory(t *testing.T) {
	t.Setenv("YKC_KB_CACHE_DIR", "")
	store, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if stats := store.CacheStats().Persistent; stats.Enabled {
		t.Fatalf("persistent cache must be opt-in: %+v", stats)
	}
}
