package monitor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ykc/internal/atomicfile"
)

var DefaultExcludeDirs = []string{
	".git", ".ykc", "target", "node_modules", "dist", "build", "coverage",
	".next", ".turbo", ".cache", "__pycache__",
}

type Snapshotter struct {
	Root        string
	ExcludeDirs []string
}

type FileRecord struct {
	Path            string `json:"path"`
	Size            int64  `json:"size"`
	Mode            string `json:"mode"`
	ModTimeUnixNano int64  `json:"mod_time_unix_nano"`
	SHA256          string `json:"sha256"`
}

type Snapshot struct {
	ID        string       `json:"id"`
	Root      string       `json:"root"`
	CreatedAt time.Time    `json:"created_at"`
	Files     []FileRecord `json:"files"`
	Digest    string       `json:"digest"`
}

type Diff struct {
	Added    []string `json:"added,omitempty"`
	Modified []string `json:"modified,omitempty"`
	Deleted  []string `json:"deleted,omitempty"`
}

func (d Diff) HasChanges() bool {
	return len(d.Added)+len(d.Modified)+len(d.Deleted) > 0
}

func (s Snapshotter) Capture() (Snapshot, error) {
	root := s.Root
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Snapshot{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Snapshot{}, err
	}
	if !info.IsDir() {
		return Snapshot{}, fmt.Errorf("snapshot root is not a directory: %s", abs)
	}
	excludes := make(map[string]struct{})
	for _, d := range append(DefaultExcludeDirs, s.ExcludeDirs...) {
		excludes[d] = struct{}{}
	}
	var files []FileRecord
	if err := filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if _, ok := excludes[name]; ok && path != abs {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		st, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		h, err := fileSHA256(path)
		if err != nil {
			return err
		}
		files = append(files, FileRecord{
			Path:            rel,
			Size:            st.Size(),
			Mode:            st.Mode().String(),
			ModTimeUnixNano: st.ModTime().UnixNano(),
			SHA256:          h,
		})
		return nil
	}); err != nil {
		return Snapshot{}, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	digest := digestFiles(files)
	created := time.Now().UTC()
	return Snapshot{
		ID:        fmt.Sprintf("%020d-%s", created.UnixNano(), digest[:16]),
		Root:      abs,
		CreatedAt: created,
		Files:     files,
		Digest:    digest,
	}, nil
}

func (s Snapshot) WriteAtomic(stateDir string) error {
	if stateDir == "" {
		return errors.New("state directory is required")
	}
	if s.ID == "" {
		return errors.New("snapshot id is required")
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(stateDir, "snapshots", s.ID+".json")
	if err := atomicfile.WriteFileSync(path, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return atomicfile.WriteFileSync(filepath.Join(stateDir, "snapshots", "current.json"), append(b, '\n'), 0o644)
}

func ReadCurrentSnapshot(stateDir string) (Snapshot, error) {
	b, err := os.ReadFile(filepath.Join(stateDir, "snapshots", "current.json"))
	if err != nil {
		return Snapshot{}, err
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}

func DiffSnapshots(prev, cur Snapshot) Diff {
	pm := make(map[string]FileRecord, len(prev.Files))
	cm := make(map[string]FileRecord, len(cur.Files))
	for _, f := range prev.Files {
		pm[f.Path] = f
	}
	for _, f := range cur.Files {
		cm[f.Path] = f
	}
	var d Diff
	for p, cf := range cm {
		pf, ok := pm[p]
		if !ok {
			d.Added = append(d.Added, p)
			continue
		}
		if pf.SHA256 != cf.SHA256 || pf.Size != cf.Size {
			d.Modified = append(d.Modified, p)
		}
	}
	for p := range pm {
		if _, ok := cm[p]; !ok {
			d.Deleted = append(d.Deleted, p)
		}
	}
	sort.Strings(d.Added)
	sort.Strings(d.Modified)
	sort.Strings(d.Deleted)
	return d
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func digestFiles(files []FileRecord) string {
	h := sha256.New()
	for _, f := range files {
		_, _ = io.WriteString(h, strings.Join([]string{f.Path, fmt.Sprint(f.Size), f.SHA256}, "\x00"))
		_, _ = io.WriteString(h, "\n")
	}
	return hex.EncodeToString(h.Sum(nil))
}
