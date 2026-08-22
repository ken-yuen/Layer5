package kb

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	persistentCacheVersion  = 1
	maxPersistentCacheFiles = 256
	maxPersistentCacheBytes = 8 << 20
	maxPersistentCacheFile  = 256 << 10
)

// persistentCache 是可選的跨程序上下文快取。它從不修改 KB blob 或種子資料；只在
// 使用者明確設置 YKC_KB_CACHE_DIR 時啟用，避免將代理查詢（可能含私有程式片段）
// 在未知位置落盤。快取檔 0600、目錄 0700、內容失效或損毀一律安全 miss。
type persistentCache struct {
	dir     string
	version string
	mu      sync.Mutex
	hits    uint64
	misses  uint64
	writes  uint64
	errors  uint64
}

type persistedContext struct {
	CacheVersion int            `json:"cache_version"`
	Key          string         `json:"key"`
	KeySHA256    string         `json:"key_sha256"`
	Version      string         `json:"version"`
	Query        string         `json:"query"`
	Hits         []persistedHit `json:"hits"`
	AtomIDs      []string       `json:"atom_ids"`
	TotalBytes   int            `json:"total_bytes"`
	BudgetBytes  int            `json:"budget_bytes"`
	Truncated    bool           `json:"truncated"`
	EstTokens    int            `json:"est_tokens"`
}

type persistedHit struct {
	ID      string  `json:"id"`
	Score   float64 `json:"score"`
	BM25    float64 `json:"bm25"`
	Shingle float64 `json:"shingle"`
}

type persistentCacheStats struct {
	Enabled bool   `json:"enabled"`
	Dir     string `json:"dir,omitempty"`
	Hits    uint64 `json:"hits"`
	Misses  uint64 `json:"misses"`
	Writes  uint64 `json:"writes"`
	Errors  uint64 `json:"errors"`
}

func newPersistentCache(datasetVersion string) *persistentCache {
	dir := strings.TrimSpace(os.Getenv("YKC_KB_CACHE_DIR"))
	if dir == "" || strings.EqualFold(dir, "off") || strings.EqualFold(dir, "disabled") {
		return nil
	}
	return &persistentCache{dir: filepath.Clean(dir), version: datasetVersion}
}

func (p *persistentCache) get(key string, store *Store) (*ContextBundle, bool) {
	if p == nil {
		return nil, false
	}
	if err := requirePrivateCacheDir(p.dir); err != nil {
		p.noteMiss(err)
		return nil, false
	}
	path := p.pathFor(key)
	f, err := os.Open(path)
	if err != nil {
		p.noteMiss(err)
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(f, maxPersistentCacheFile+1))
	_ = f.Close()
	if err != nil || len(data) > maxPersistentCacheFile {
		p.noteError()
		_ = os.Remove(path)
		return nil, false
	}
	var entry persistedContext
	if err := json.Unmarshal(data, &entry); err != nil {
		p.noteError()
		_ = os.Remove(path)
		return nil, false
	}
	bundle, ok := p.rehydrate(key, entry, store)
	if !ok {
		p.noteError()
		_ = os.Remove(path)
		return nil, false
	}
	p.mu.Lock()
	p.hits++
	p.mu.Unlock()
	return bundle, true
}

func (p *persistentCache) put(key string, bundle *ContextBundle) {
	if p == nil || bundle == nil {
		return
	}
	entry := persistedContext{
		CacheVersion: persistentCacheVersion,
		Key:          key,
		KeySHA256:    keyDigest(key),
		Version:      p.version,
		Query:        bundle.Query,
		TotalBytes:   bundle.TotalBytes,
		BudgetBytes:  bundle.BudgetBytes,
		Truncated:    bundle.Truncated,
		EstTokens:    bundle.EstTokens,
		Hits:         make([]persistedHit, 0, len(bundle.Hits)),
		AtomIDs:      make([]string, 0, len(bundle.Atoms)),
	}
	for _, hit := range bundle.Hits {
		if hit.Atom == nil {
			return
		}
		entry.Hits = append(entry.Hits, persistedHit{ID: hit.Atom.ID, Score: hit.Score, BM25: hit.BM25, Shingle: hit.Shingle})
	}
	for _, atom := range bundle.Atoms {
		if atom == nil {
			return
		}
		entry.AtomIDs = append(entry.AtomIDs, atom.ID)
	}
	data, err := json.Marshal(entry)
	if err != nil || len(data) > maxPersistentCacheFile {
		p.noteError()
		return
	}
	if err := os.MkdirAll(p.dir, 0o700); err != nil {
		p.noteError()
		return
	}
	if err := requirePrivateCacheDir(p.dir); err != nil {
		p.noteError()
		return
	}
	if err := writePersistentAtomic(p.pathFor(key), data); err != nil {
		p.noteError()
		return
	}
	p.mu.Lock()
	p.writes++
	p.mu.Unlock()
	p.prune()
}

func (p *persistentCache) rehydrate(key string, entry persistedContext, store *Store) (*ContextBundle, bool) {
	if entry.CacheVersion != persistentCacheVersion || entry.Key != key || entry.KeySHA256 != keyDigest(key) || entry.Version != store.Version() || entry.Version != p.version || entry.BudgetBytes <= 0 || entry.TotalBytes < 0 {
		return nil, false
	}
	atoms := make([]*Atom, 0, len(entry.AtomIDs))
	for _, id := range entry.AtomIDs {
		atom, ok := store.byID[id]
		if !ok {
			return nil, false
		}
		atoms = append(atoms, atom)
	}
	hits := make([]Hit, 0, len(entry.Hits))
	for _, hit := range entry.Hits {
		atom, ok := store.byID[hit.ID]
		if !ok {
			return nil, false
		}
		hits = append(hits, Hit{Atom: atom, Score: hit.Score, BM25: hit.BM25, Shingle: hit.Shingle})
	}
	return &ContextBundle{
		Query:       entry.Query,
		Version:     entry.Version,
		Hits:        hits,
		Atoms:       atoms,
		TotalBytes:  entry.TotalBytes,
		BudgetBytes: entry.BudgetBytes,
		Truncated:   entry.Truncated,
		EstTokens:   entry.EstTokens,
		CacheHit:    true,
	}, true
}

func (p *persistentCache) pathFor(key string) string {
	return filepath.Join(p.dir, keyDigest(key)+".json")
}

func keyDigest(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func (p *persistentCache) noteMiss(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.misses++
	if err != nil && !os.IsNotExist(err) {
		p.errors++
	}
}

func (p *persistentCache) noteError() {
	p.mu.Lock()
	p.errors++
	p.misses++
	p.mu.Unlock()
}

func (p *persistentCache) stats() persistentCacheStats {
	if p == nil {
		return persistentCacheStats{}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return persistentCacheStats{Enabled: true, Dir: p.dir, Hits: p.hits, Misses: p.misses, Writes: p.writes, Errors: p.errors}
}

func (p *persistentCache) prune() {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		return
	}
	type cacheFile struct {
		path string
		mod  time.Time
		size int64
	}
	files := make([]cacheFile, 0, len(entries))
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, cacheFile{path: filepath.Join(p.dir, entry.Name()), mod: info.ModTime(), size: info.Size()})
		total += info.Size()
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].mod.Equal(files[j].mod) {
			return files[i].path < files[j].path
		}
		return files[i].mod.Before(files[j].mod)
	})
	for (len(files) > maxPersistentCacheFiles || total > maxPersistentCacheBytes) && len(files) > 0 {
		victim := files[0]
		files = files[1:]
		if os.Remove(victim.path) == nil {
			total -= victim.size
		}
	}
}

func requirePrivateCacheDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return os.ErrNotExist
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("persistent cache directory %s must not be group/world accessible", dir)
	}
	return nil
}

func writePersistentAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
