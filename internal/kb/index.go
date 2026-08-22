package kb

import (
	"math"
	"strings"
)

// 精準檢索（一）：倒排索引 + BM25 詞彙相關性。
//
// 索引欄位與權重（field weight，越大越重要）：
//
//	Code/Tags     3.0  （錯誤碼、規則 id、領域詞——精確鍵）
//	Title         2.5
//	ZH            2.0
//	Err/Fix 程式  1.6  （程式碼也參與檢索：搜 "&mut"、"clone()" 能命中範例）
//	Body/Why      1.0
type index struct {
	postings map[string][]int // token → atom 位置（依 atom 序）
	docLens  []int            // 每份文件的「加權詞元長度」
	avgLen   float64
	tf       []map[string]float64 // 每份文件的詞頻（加權）
	df       map[string]int       // 文件頻率
	fieldW   map[string]float64
}

const (
	b       = 0.75
	k1      = 1.2
	codeW   = 3.0
	titleW  = 2.5
	zhW     = 2.0
	codeExW = 1.6
	bodyW   = 1.0
)

func newIndex(atoms []*Atom) *index {
	ix := &index{
		postings: map[string][]int{},
		docLens:  make([]int, len(atoms)),
		tf:       make([]map[string]float64, len(atoms)),
		df:       map[string]int{},
		fieldW: map[string]float64{
			"code": codeW, "title": titleW, "zh": zhW,
			"codex": codeExW, "body": bodyW,
		},
	}
	totalLen := 0
	for i, a := range atoms {
		freq := map[string]float64{}
		add := func(tokens []string, w float64) {
			for _, t := range tokens {
				freq[t] += w
			}
		}
		add(Tokenize(a.Code), ix.fieldW["code"])
		add(Tokenize(strings.Join(a.Tags, " ")), ix.fieldW["code"])
		add(Tokenize(a.Title), ix.fieldW["title"])
		add(Tokenize(a.ZH), ix.fieldW["zh"])
		add(Tokenize(a.Err+"\n"+a.Fix), ix.fieldW["codex"])
		add(Tokenize(a.Body+"\n"+a.Why), ix.fieldW["body"])
		for _, f := range a.Fixes {
			add(Tokenize(f), ix.fieldW["codex"])
		}

		ix.tf[i] = freq
		// 文件長度 = 詞元加權總和（BM25 的 |D|）。
		l := 0
		for _, w := range freq {
			l += int(w)
		}
		if l < 1 {
			l = 1
		}
		ix.docLens[i] = l
		totalLen += l
		for t := range freq {
			ix.postings[t] = append(ix.postings[t], i)
			ix.df[t]++
		}
	}
	if len(atoms) > 0 {
		ix.avgLen = float64(totalLen) / float64(len(atoms))
	} else {
		ix.avgLen = 1
	}
	return ix
}

// bm25 對單一查詢詞元計算所有候選文件的 BM25 分數（稀疏——只走 postings）。
func (ix *index) bm25(queryTokens []string, nDocs int) map[int]float64 {
	scores := map[int]float64{}
	for _, t := range dedupe(queryTokens) {
		post := ix.postings[t]
		if len(post) == 0 {
			continue
		}
		idf := math.Log(1 + (float64(nDocs)-float64(len(post))+0.5)/(float64(len(post))+0.5))
		if idf < 0 {
			idf = 0
		}
		for _, doc := range post {
			tf := ix.tf[doc][t]
			denom := tf + k1*(1-b+b*float64(ix.docLens[doc])/ix.avgLen)
			scores[doc] += idf * (tf * (k1 + 1)) / denom
		}
	}
	return scores
}
