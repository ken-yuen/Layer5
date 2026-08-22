package kb

import (
	"regexp"
	"strings"
	"unicode"
)

// 精準檢索的地基：把文本切成「可索引的詞元」，並把文本切成 char-shingle
// （字元 3-gram 的雜湊集合）供模糊/概念相似度（無需外部 embedding 模型）。

var codeRe = regexp.MustCompile(`\b[eE]\d{4}\b`)

// Tokenize 把一段文本切成詞元：
//   - 全小寫；
//   - 依非字母數字切分；
//   - 對駝峰（BorrowChecker → borrow, checker）再切；
//   - 保留並正規化錯誤碼（E0382 → e0382）。
func Tokenize(s string) []string {
	s = strings.ToLower(s)
	var out []string
	for _, run := range alphaNumRuns(s) {
		for _, p := range splitCamel(run) {
			if p != "" {
				out = append(out, p)
			}
		}
	}
	for _, c := range codeRe.FindAllString(s, -1) {
		out = append(out, strings.ToLower(c))
	}
	return dedupe(out)
}

// alphaNumRuns 把字串切成連續的字母數字片段（其餘字元為分隔）。
func alphaNumRuns(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, string(cur))
			cur = nil
		}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur = append(cur, r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

// splitCamel 在「小寫→大寫」邊界處切分駝峰/帕斯卡命名（BorrowChecker → borrow, checker）。
func splitCamel(s string) []string {
	var parts []string
	start := 0
	rs := []rune(s)
	for i := 1; i < len(rs); i++ {
		if unicode.IsLower(rs[i-1]) && unicode.IsUpper(rs[i]) {
			parts = append(parts, string(rs[start:i]))
			start = i
		}
	}
	parts = append(parts, string(rs[start:]))
	return parts
}

func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, t := range in {
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// Shingle 回傳文本的字元 3-gram FNV-1a 雜湊集合（去重）。
// 用於模糊匹配：查詢與候選文的 containment = |Q∩D| / |Q|。
func Shingle(s string) map[uint64]bool {
	norm := normalizeForShingle(s)
	rs := []rune(norm)
	set := make(map[uint64]bool, len(rs))
	if len(rs) < 3 {
		return set
	}
	var h uint64
	for i := 0; i < 3; i++ {
		h = fnvAdd(h, uint64(rs[i]))
	}
	set[h] = true
	for i := 3; i < len(rs); i++ {
		h = fnvAdd(h, uint64(rs[i]))
		set[h] = true
	}
	return set
}

// ShingleContainment 回傳 query 的 shingle 被 doc 覆蓋的比例 ∈ [0,1]。
// 空 query shingle 集合回傳 0。
func ShingleContainment(q, doc map[uint64]bool) float64 {
	if len(q) == 0 {
		return 0
	}
	hit := 0
	for h := range q {
		if doc[h] {
			hit++
		}
	}
	return float64(hit) / float64(len(q))
}

func normalizeForShingle(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSpace = false
		} else if !prevSpace {
			b.WriteByte(' ')
			prevSpace = true
		}
	}
	return b.String()
}

// fnvAdd 是「滾動」不了的簡單 FNV-1a 累加（我們對每 3-gram 重算，683 份文件規模無需滾動）。
func fnvAdd(h, c uint64) uint64 {
	h ^= c
	h *= 0x100000001b3
	return h
}
