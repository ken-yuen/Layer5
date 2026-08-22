package kb

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	persistent  *persistentCache
	version     string
	priority    map[string]float64  // 錯誤碼 → 靜態優先級（tier）
	termDomains map[string][]string // 詞元 → 領域（檢索加權）
	nDocs       int
	source      string // "embedded" 或 blob 路徑
	meta        DatasetMeta
}

// ---------- 建構 ----------

// Open 由內嵌種子資料建構唯讀知識庫。
func Open() (*Store, error) {
	atoms, _, err := buildAtoms()
	if err != nil {
		return nil, err
	}
	return newStore(atoms, "embedded", embeddedDatasetMeta)
}

func newStore(atoms []*Atom, source string, meta DatasetMeta) (*Store, error) {
	s := &Store{
		atoms:  atoms,
		byID:   map[string]*Atom{},
		byCode: map[string]*Atom{},
		source: source,
		meta:   normalizeMeta(meta),
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
	// 跨程序快取是可選的：只在 YKC_KB_CACHE_DIR 明確設定時落盤，且資料版本
	// 已含於 key，升版後不會讀到舊上下文。
	s.persistent = newPersistentCache(s.version)
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
	blobMagic        = "YKCKB\x00"
	blobVersionV1    = uint32(1) // 舊格式：僅校驗 payload，無來源版本 metadata。
	blobVersion      = uint32(2) // metadata + count + payload 一起被 sha256 覆蓋。
	blobMaxMetaBytes = 64 * 1024
)

// Build 把當前內嵌種子資料序列化為單一不可變 blob 位元組（含已校驗 metadata
// 標頭與 sha256 尾章）。metadata 會鎖定資料對應的 rustc 錯誤索引版本。
func Build() ([]byte, error) {
	atoms, _, err := buildAtoms()
	if err != nil {
		return nil, err
	}
	return encodeBlob(atoms, embeddedDatasetMeta)
}

// Save 建庫並寫入 path（原子性：先寫暫存再 rename）。
func Save(path string) error {
	b, err := Build()
	if err != nil {
		return err
	}
	return saveBlob(path, b)
}

func saveBlob(path string, b []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
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
	return os.Rename(tmpName, path)
}

// OpenFile 開啟磁碟 blob（唯讀）：校驗 magic、格式版本、metadata 與 payload
// 的 sha256，任何不符（含竄改）即報錯。v1 blob 仍可開啟，但沒有 rustc
// 來源版本，呼叫端可藉 RustcVersion()=="" 辨識並重新 import。
func OpenFile(path string) (*Store, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("kb: open blob: %w", err)
	}
	atoms, meta, err := decodeBlob(b)
	if err != nil {
		return nil, fmt.Errorf("kb: %s: %w", path, err)
	}
	return newStore(atoms, path, meta)
}

func encodeBlob(atoms []*Atom, meta DatasetMeta) ([]byte, error) {
	meta = normalizeMeta(meta)
	mb, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("encode metadata: %w", err)
	}
	if len(mb) > blobMaxMetaBytes {
		return nil, fmt.Errorf("metadata %d bytes exceeds blob limit %d", len(mb), blobMaxMetaBytes)
	}

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

	// v2 的 body（metadata 長度 + metadata + count + payload）整段納入 checksum，
	// 避免攻擊者只改 rustc_version 標頭就偽造一份「同內容、不同來源」的 KB。
	body := make([]byte, 0, 4+len(mb)+8+len(payload))
	var ml [4]byte
	binary.BigEndian.PutUint32(ml[:], uint32(len(mb)))
	body = append(body, ml[:]...)
	body = append(body, mb...)
	var cb [8]byte
	binary.BigEndian.PutUint64(cb[:], uint64(len(atoms)))
	body = append(body, cb[:]...)
	body = append(body, payload...)
	sum := sha256.Sum256(body)

	out := make([]byte, 0, len(blobMagic)+4+len(body)+len(sum))
	out = append(out, []byte(blobMagic)...)
	var vb [4]byte
	binary.BigEndian.PutUint32(vb[:], blobVersion)
	out = append(out, vb[:]...)
	out = append(out, body...)
	out = append(out, sum[:]...)
	return out, nil
}

func decodeBlob(b []byte) ([]*Atom, DatasetMeta, error) {
	if len(b) < len(blobMagic)+4+8+32 {
		return nil, DatasetMeta{}, fmt.Errorf("truncated blob")
	}
	if string(b[:len(blobMagic)]) != blobMagic {
		return nil, DatasetMeta{}, fmt.Errorf("bad magic")
	}
	ver := binary.BigEndian.Uint32(b[len(blobMagic) : len(blobMagic)+4])
	switch ver {
	case blobVersionV1:
		return decodeBlobV1(b)
	case blobVersion:
		return decodeBlobV2(b)
	default:
		return nil, DatasetMeta{}, fmt.Errorf("unsupported version %d", ver)
	}
}

// decodeBlobV1 保留對 v1 blob 的唯讀相容性。v1 的 checksum 僅覆蓋 payload，
// 因此 metadata 無從復原；使用者應以 ykc-know import 重建為 v2。
func decodeBlobV1(b []byte) ([]*Atom, DatasetMeta, error) {
	const header = 6 + 4 + 8
	if len(b) < header+32 {
		return nil, DatasetMeta{}, fmt.Errorf("truncated blob")
	}
	count := binary.BigEndian.Uint64(b[10:18])
	payload := b[header : len(b)-32]
	want := b[len(b)-32:]
	sum := sha256.Sum256(payload)
	if !bytes.Equal(sum[:], want) {
		return nil, DatasetMeta{}, fmt.Errorf("checksum mismatch: blob tampered or corrupted")
	}
	atoms, err := decodeRecords(payload, count)
	return atoms, DatasetMeta{}, err
}

func decodeBlobV2(b []byte) ([]*Atom, DatasetMeta, error) {
	const fixed = 6 + 4
	if len(b) < fixed+4+8+32 {
		return nil, DatasetMeta{}, fmt.Errorf("truncated v2 blob")
	}
	body := b[fixed : len(b)-32]
	want := b[len(b)-32:]
	sum := sha256.Sum256(body)
	if !bytes.Equal(sum[:], want) {
		return nil, DatasetMeta{}, fmt.Errorf("checksum mismatch: blob tampered or corrupted")
	}
	if len(body) < 4 {
		return nil, DatasetMeta{}, fmt.Errorf("truncated metadata")
	}
	metaLen := int(binary.BigEndian.Uint32(body[:4]))
	if metaLen < 0 || metaLen > blobMaxMetaBytes || len(body) < 4+metaLen+8 {
		return nil, DatasetMeta{}, fmt.Errorf("invalid metadata length %d", metaLen)
	}
	var meta DatasetMeta
	if err := json.Unmarshal(body[4:4+metaLen], &meta); err != nil {
		return nil, DatasetMeta{}, fmt.Errorf("decode metadata: %w", err)
	}
	meta = normalizeMeta(meta)
	off := 4 + metaLen
	count := binary.BigEndian.Uint64(body[off : off+8])
	atoms, err := decodeRecords(body[off+8:], count)
	return atoms, meta, err
}

func decodeRecords(payload []byte, count uint64) ([]*Atom, error) {
	// 每筆至少有 4 bytes length，先以此上限拒絕偽造的巨大 count，避免 int
	// 轉換或 make 容量造成 panic/記憶體配置攻擊。
	if count > uint64(len(payload)/4) {
		return nil, fmt.Errorf("record count exceeds payload")
	}
	atoms := make([]*Atom, 0, int(count))
	off := 0
	for off < len(payload) {
		if off+4 > len(payload) {
			return nil, fmt.Errorf("truncated record")
		}
		n := int(binary.BigEndian.Uint32(payload[off : off+4]))
		off += 4
		if n < 0 || off+n > len(payload) {
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

// Metadata 回傳資料集可追溯中繼資料的副本。
func (s *Store) Metadata() DatasetMeta { return s.meta }

// RustcVersion 回傳此資料集所對應的 rustc 錯誤索引版本。legacy v1 blob 沒有這個
// 欄位時回傳空字串，呼叫端應提醒使用者重新 import。
func (s *Store) RustcVersion() string { return s.meta.RustcVersion }

// ErrorIndexURL 回傳匯入時使用的官方錯誤索引 URL（若 legacy blob 未記錄則為空）。
func (s *Store) ErrorIndexURL() string { return s.meta.ErrorIndexURL }

// ErrorIndexSHA256 回傳匯入頁面的 SHA-256（若無則為空）。
func (s *Store) ErrorIndexSHA256() string { return s.meta.ErrorIndexSHA256 }

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

// CacheStats 回傳上下文緩存統計（記憶體 LRU + 可選跨程序磁碟層）。
func (s *Store) CacheStats() cacheStats {
	stats := s.cache.stats()
	if persistent := s.persistent.stats(); persistent.Enabled {
		stats.Persistent = persistent
	}
	return stats
}

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

// TranslatedErrorCount 回傳此資料集內實際帶繁中摘要的錯誤卡數量。它按 Store
// 內容計算，故可正確區分舊 v2 blob（無翻譯）與新資料集。
func (s *Store) TranslatedErrorCount() int {
	count := 0
	for _, atom := range s.atoms {
		if atom.Kind == KindError && strings.TrimSpace(atom.ZH) != "" {
			count++
		}
	}
	return count
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
