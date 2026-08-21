package eventstore

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ykc/internal/atomicfile"
	"ykc/internal/domain"
)

// Store persists each event as one immutable JSON file using atomic rename.
// This avoids JSONL partial-write ambiguity and makes concurrent writers safer:
// an event is either absent or complete. Ordering is by filename timestamp.
type Store struct {
	Dir string
}

func New(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("event store directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{Dir: dir}, nil
}

func (s *Store) Append(e domain.Envelope) (domain.Envelope, error) {
	if s == nil || s.Dir == "" {
		return domain.Envelope{}, errors.New("nil event store")
	}
	if e.Kind == "" {
		return domain.Envelope{}, errors.New("event kind is required")
	}
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	if e.ID == "" {
		e.ID = newEventID(e.At)
	}
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return domain.Envelope{}, err
	}
	day := e.At.UTC().Format("20060102")
	path := filepath.Join(s.Dir, day, sanitizeFileName(e.ID)+".json")
	if _, err := os.Stat(path); err == nil {
		return domain.Envelope{}, fmt.Errorf("event id collision: %s", e.ID)
	}
	if err := atomicfile.WriteFileSync(path, append(b, '\n'), 0o644); err != nil {
		return domain.Envelope{}, err
	}
	return e, nil
}

func (s *Store) Replay() ([]domain.Envelope, error) {
	if s == nil || s.Dir == "" {
		return nil, errors.New("nil event store")
	}
	var files []string
	if err := filepath.WalkDir(s.Dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".json") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(files)
	events := make([]domain.Envelope, 0, len(files))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var e domain.Envelope
		if err := json.Unmarshal(b, &e); err != nil {
			return nil, fmt.Errorf("decode %s: %w", f, err)
		}
		events = append(events, e)
	}
	return events, nil
}

func newEventID(t time.Time) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%020d-%s", t.UTC().UnixNano(), hex.EncodeToString(b[:]))
}

func sanitizeFileName(s string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "\x00", "_")
	return replacer.Replace(s)
}
