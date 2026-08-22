package kb

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Store 是嵌入式唯讀知識庫的執行期實體：內容定址原子 + 倒排索引 +
// 依賴項圖 + 上下文緩存，全部只讀（無任何寫入 API）。
//
// 兩種取得方式：
//
//	Open()          — 由 go:embed 種子資料建構（嵌入式，零外部檔案）。
//	OpenFile(path)  — 由 ykc-know build 產出的單一 blob 建構（磁碟唯讀資料庫，
//	                  開檔時 sha256 校驗防竄改）。
//
// 併發安全：所有查詢路徑唯讀；cache 內部有鎖。Store 建立後不可變。
type Store struct {
	atoms       []*Atom
	byID        map[string]*Atom
	byCode      map[string]*Atom // code（原樣 + 大寫）→ atom
	index       *index
	graph       *graph
	cache       *cache
	version     string
	priority    map[string]float64  // 錯誤碼 → 靜態優先級（tier）
	termDomains map[string][]string // 詞元 → 領域（檢索加權）
	nDocs       int
	source      string // "embedded" 或 blob 路徑
}

// ---------- 建構 ----------

// Open 由內嵌種子資料建構唯讀知識庫。
func Open() (*Store, error) {
	atoms, _, err := buildAtoms()
	if err != nil {
		return nil, err
	}
	return newStore(atoms, "embedded")
}

func newStore(atoms []*Atom, source string) (*Store, error) {
	s := &Store{
		atoms:  atoms,
		byID:   map[string]*Atom{},
		byCode: map[string]*Atom{},
		source: source,
		nDocs:  len(atoms),
	}
	for _, a := range atoms {
		s.byID[a.ID] = a
		s.byCode[a.Code] = a
		s.byCode[strings.ToUpper(a.Code)] = a
		s.byCode[strings.ToLower(a.Code)] = a
	}
	s.version = versionOf(atoms)
	s.index = newIndex(atoms)
	s.graph = newGraph(atoms)
	s.cache = newCache(256, 8<<20)
	prio, terms, err := loadBoost()
	if err != nil {
		return nil, err
	}
	s.priority = prio
	s.termDomains = terms
	return s, nil
}

func loadBoost() (map[string]float64, map[string][]string, error) {
	var sb seedBoost
	f, err := dataFS.Open("data/boost.json.gz")
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, nil, err
	}
	defer zr.Close()
	b, err := io.ReadAll(zr)
	if err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(b, &sb); err != nil {
		return nil, nil, err
	}
	prio := map[string]float64{}
	for _, c := range sb.Tier1 {
		prio[strings.ToUpper(c)] = 1.0
	}
	for _, c := range sb.Tier2 {
		u := strings.ToUpper(c)
		if _, ok := prio[u]; !ok {
			prio[u] = 0.5
		}
	}
	terms := map[string][]string{}
	for t, d := range sb.Terms {
		terms[strings.ToLower(t)] = d
	}
	return prio, terms, nil
}

// ---------- blob（磁碟上的唯讀資料庫） ----------

const (
	blobMagic   = "YKCKB\x00"
	blobVersion = uint32(1)
)

// Build 把當前種子資料序列化為單一不可變 blob 位元組（含 sha256 尾章）。
// 這一步是「唯讀資料庫」唯一的寫入時機（離線建庫）。
func Build() ([]byte, error) {
	atoms, _, err := buildAtoms()
	if err != nil {
		return nil, err
	}
	return encodeBlob(atoms)
}

// Save 建庫並寫入 path（原子性：先寫暫存再 rename）。
func Save(path string) error {
	b, err := Build()
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// OpenFile 開啟磁碟 blob（唯讀）：校驗 magic、版本與 sha256 尾章，
// 任何不符（含竄改）即報錯。資料載入記憶體後供唯讀查詢。
func OpenFile(path string) (*Store, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("kb: open blob: %w", err)
	}
	atoms, err := decodeBlob(b)
	if err != nil {
		return nil, fmt.Errorf("kb: %s: %w", path, err)
	}
	return newStore(atoms, path)
}

func encodeBlob(atoms []*Atom) ([]byte, error) {
	var out []byte
	out = append(out, []byte(blobMagic)...)
	var vb [4]byte
	binary.BigEndian.PutUint32(vb[:], blobVersion)
	out = append(out, vb[:]...)
	var cb [8]byte
	binary.BigEndian.PutUint64(cb[:], uint64(len(atoms)))
	out = append(out, cb[:]...)

	var payload []byte
	for _, a := range atoms {
		j, err := json.Marshal(a)
		if err != nil {
			return nil, err
		}
		var lb [4]byte
		binary.BigEndian.PutUint32(lb[:], uint32(len(j)))
		payload = append(payload, lb[:]...)
		payload = append(payload, j...)
	}
	out = append(out, payload...)
	sum := sha256.Sum256(payload)
	out = append(out, sum[:]...)
	return out, nil
}

func decodeBlob(b []byte) ([]*Atom, error) {
	if len(b) < 6+4+8+32 {
		return nil, fmt.Errorf("truncated blob")
	}
	if string(b[:6]) != blobMagic {
		return nil, fmt.Errorf("bad magic")
	}
	if ver := binary.BigEndian.Uint32(b[6:10]); ver != blobVersion {
		return nil, fmt.Errorf("unsupported version %d", ver)
	}
	count := binary.BigEndian.Uint64(b[10:18])
	payload := b[18 : len(b)-32]
	want := b[len(b)-32:]
	sum := sha256.Sum256(payload)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), hex.EncodeToString(want)) {
		return nil, fmt.Errorf("checksum mismatch: blob tampered or corrupted")
	}
	atoms := make([]*Atom, 0, count)
	off := 0
	for off < len(payload) {
		if off+4 > len(payload) {
			return nil, fmt.Errorf("truncated record")
		}
		n := int(binary.BigEndian.Uint32(payload[off : off+4]))
		off += 4
		if off+n > len(payload) {
			return nil, fmt.Errorf("record overruns payload")
		}
		var a Atom
		if err := json.Unmarshal(payload[off:off+n], &a); err != nil {
			return nil, fmt.Errorf("decode record: %w", err)
		}
		atoms = append(atoms, &a)
		off += n
	}
	if uint64(len(atoms)) != count {
		return nil, fmt.Errorf("record count mismatch")
	}
	return atoms, nil
}

// ---------- 唯讀查詢 ----------

// Count 回傳原子總數。
func (s *Store) Count() int { return len(s.atoms) }

// Version 回傳資料版本（內容定址資料集的 sha256 指紋）。
func (s *Store) Version() string { return s.version }

// Source 回傳資料來源描述（embedded 或 blob 路徑）。
func (s *Store) Source() string { return s.source }

// ByCode 依邏輯鍵（錯誤碼 E0382 / 規則 id / 章節 id，大小寫不敏感）取原子。
func (s *Store) ByCode(code string) (*Atom, bool) {
	a, ok := s.byCode[code]
	if !ok {
		a, ok = s.byCode[strings.ToUpper(code)]
	}
	return a, ok
}

// ByID 依內容定址 ID 取原子。
func (s *Store) ByID(id string) (*Atom, bool) {
	a, ok := s.byID[id]
	return a, ok
}

// AtomIDs 回傳全部原子 ID（排序，決定論）。
func (s *Store) AtomIDs() []string {
	ids := make([]string, 0, len(s.atoms))
	for _, a := range s.atoms {
		ids = append(ids, a.ID)
	}
	sort.Strings(ids)
	return ids
}

// AtomsByKind 回傳指定種類的原子（依 Code 排序，決定論）。
func (s *Store) AtomsByKind(k Kind) []*Atom {
	var out []*Atom
	for _, a := range s.atoms {
		if a.Kind == k {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// ExpandContext 以原子 ID 為根做依賴項圖 BFS 展開（見 graph.ExpandContext）。
func (s *Store) ExpandContext(ids []string, depth int) []*Atom {
	order := s.graph.ExpandContext(ids, depth)
	out := make([]*Atom, 0, len(order))
	for _, id := range order {
		if a, ok := s.byID[id]; ok {
			out = append(out, a)
		}
	}
	return out
}

// Deps 回傳指定原子 ID 的依賴原子（前驅）。
func (s *Store) Deps(id string) []*Atom {
	var out []*Atom
	for _, rid := range s.graph.Deps(id) {
		if a, ok := s.byID[rid]; ok {
			out = append(out, a)
		}
	}
	return out
}

// Dependents 回傳引用指定原子 ID 的原子（後繼）。
func (s *Store) Dependents(id string) []*Atom {
	var out []*Atom
	for _, rid := range s.graph.Dependents(id) {
		if a, ok := s.byID[rid]; ok {
			out = append(out, a)
		}
	}
	return out
}

// Cycles 回傳依賴項圖的強連通分量（診斷用，見 graph.Cycles）。
func (s *Store) Cycles() [][]string { return s.graph.Cycles() }

// CacheStats 回傳上下文緩存統計（命中/未命中/條目/位元組）。
func (s *Store) CacheStats() cacheStats { return s.cache.stats() }

// RulesByDomain 回傳指定領域的規則原子（依 Code 排序）。
func (s *Store) RulesByDomain(domain string) []*Atom {
	var out []*Atom
	for _, a := range s.atoms {
		if a.Kind == KindRule && strings.EqualFold(a.Domain, domain) {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// Domains 回傳全部規則領域（排序，去重）。
func (s *Store) Domains() []string {
	seen := map[string]bool{}
	for _, a := range s.atoms {
		if a.Kind == KindRule && a.Domain != "" {
			seen[a.Domain] = true
		}
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}
