package kb

import (
	"sort"
	"strings"
)

// Hit 是一條檢索命中。
type Hit struct {
	Atom    *Atom   `json:"atom"`
	Score   float64 `json:"score"`
	BM25    float64 `json:"bm25"`
	Shingle float64 `json:"shingle"`
}

// Search 執行精準檢索，回傳依相關性排序的前 k 條（k<=0 取預設 8）。
//
// 檢索管線：
//  1. 精確鍵短路：查詢本身是錯誤碼/規則 id/章節 id → 該原子置頂；
//  2. BM25 詞彙相關性（倒排索引）；
//  3. char-shingle 模糊相似度（拼寫誤差、程式碼片段、概念相近）；
//  4. 領域詞彙加權（如 "borrow" → borrowing/lifetime 規則加分）；
//  5. 靜態優先級（常見錯誤碼 tier 加權）。
//
// 全部權重為決定論函數：同一查詢、同一資料集 → 同一結果（可重放、可緩存）。
func (s *Store) Search(q string, k int) []Hit {
	if k <= 0 {
		k = 8
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}

	// 1) 精確鍵短路。
	exact := map[string]bool{}
	exactIDs := []string{}
	if a, ok := s.ByCode(q); ok {
		exactIDs = append(exactIDs, a.ID)
		exact[a.ID] = true
	}
	tokens := Tokenize(q)
	for _, t := range tokens {
		if a, ok := s.byCode[strings.ToUpper(t)]; ok {
			if !exact[a.ID] {
				exactIDs = append(exactIDs, a.ID)
				exact[a.ID] = true
			}
		}
	}

	// 2) BM25。
	bm25 := s.index.bm25(tokens, s.nDocs)
	maxB := 0.0
	for _, v := range bm25 {
		if v > maxB {
			maxB = v
		}
	}

	// 3+4+5) 綜合評分。
	queryShingles := Shingle(q)
	domainBoost := s.domainBoost(tokens)

	cands := make([]int, 0, len(bm25))
	for doc := range bm25 {
		cands = append(cands, doc)
	}
	sort.Ints(cands)

	// 模糊回退：詞彙完全無命中（拼寫誤差 / 新措辭）時，
	// 以 char-shingle 覆蓋率掃描全部原子，取覆蓋率最高者。
	if len(cands) == 0 {
		var fb []Hit
		for _, a := range s.atoms {
			sh := ShingleContainment(queryShingles, shingleOf(a))
			if sh > 0 {
				fb = append(fb, Hit{Atom: a, Score: sh, Shingle: sh})
			}
		}
		sort.SliceStable(fb, func(i, j int) bool { return fb[i].Score > fb[j].Score })
		if len(fb) > k {
			fb = fb[:k]
		}
		return fb
	}

	hits := make([]Hit, 0, len(cands)+len(exactIDs))
	for _, doc := range cands {
		a := s.atoms[doc]
		bm := bm25[doc]
		bmNorm := 0.0
		if maxB > 0 {
			bmNorm = bm / maxB
		}
		sh := ShingleContainment(queryShingles, shingleOf(a))
		db := domainBoost[a.Code]
		if a.Kind == KindRule {
			db += domainBoost[strings.ToLower(a.Domain)]
		}
		prio := s.priority[strings.ToUpper(a.Code)]
		score := bmNorm*(1+0.25*sh+0.2*db) + 0.1*prio
		hits = append(hits, Hit{Atom: a, Score: score, BM25: bm, Shingle: sh})
	}

	// 精確鍵原子置頂（分數 +1e9 保序；跳過已出現者避免重複）。
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	seen := map[string]bool{}
	for _, h := range hits {
		seen[h.Atom.ID] = true
	}
	var pinned []Hit
	for _, id := range exactIDs {
		if seen[id] {
			continue
		}
		if a, ok := s.byID[id]; ok {
			pinned = append(pinned, Hit{Atom: a, Score: 1e9, BM25: 0, Shingle: 1})
			seen[id] = true
		}
	}
	hits = append(pinned, hits...)

	if len(hits) > k {
		hits = hits[:k]
	}
	return hits
}

// shingleOf 計算原子文本的 shingle 集合（無快取：683 文件 × 少量候選，直接算）。
func shingleOf(a *Atom) map[uint64]bool {
	text := a.Title + " " + a.ZH + " " + a.Body + " " + a.Why + " " + a.Err + " " + a.Fix + " " + strings.Join(a.Tags, " ")
	return Shingle(text)
}

// domainBoost 依查詢詞元回傳「領域 → 加權分」。
func (s *Store) domainBoost(tokens []string) map[string]float64 {
	out := map[string]float64{}
	for _, t := range dedupe(tokens) {
		if ds, ok := s.termDomains[t]; ok {
			for _, d := range ds {
				out[d] += 1.0
				if out[d] > 1.0 {
					out[d] = 1.0
				}
			}
		}
	}
	return out
}
