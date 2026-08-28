// Package reports 把 YKC 報告 Markdown（根目錄的 YKC_*.md）解析成結構化元數據，
// 並提供審計規則（Audit）與確定性產物生成（Artifacts/Build/Verify）。
//
// 設計紀律：全部輸出僅為報告內容的函數——無時間戳、無隨機數——因此
// verify 能把「報告改了但產物沒重生成」變成機械可判的失敗（與帳本
// 可重放同一哲學）。
package reports

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Schema 是索引與 manifest 的 schema 標識。
const Schema = "ykc-reports/v1"

// auditSchema 是審計報告的 schema 標識。
const auditSchema = "ykc-reports-audit/v1"

// 報告種類。
const (
	// KindSeries 是編號系列報告（YKC_NN_*.md）。
	KindSeries = "series"
	// KindFoundation 是無編號奠基文檔。
	KindFoundation = "foundation"
)

// Heading 是一個 Markdown ATX 標題。
type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	Line  int    `json:"line"`
}

// Ref 是對另一份 YKC 報告檔名的引用。
type Ref struct {
	Target string `json:"target"`
	Line   int    `json:"line"`
}

// Link 是一個相對 Markdown 鏈接（http/https/mailto/錨點不算）。
type Link struct {
	Target string `json:"target"`
	Line   int    `json:"line"`
}

// Report 是一份已解析報告的全部結構化元數據。
type Report struct {
	Path      string    `json:"path"`               // 相對 root 的路徑（'/' 分隔）
	Base      string    `json:"base"`               // 檔名（含 .md）
	Kind      string    `json:"kind"`               // series|foundation
	Number    int       `json:"number"`             // 系列編號；foundation 為 -1
	Title     string    `json:"title,omitempty"`    // 第一個 # 標題
	Date      string    `json:"date,omitempty"`     // YYYY-MM-DD（元數據區首個可解析日期）
	Baseline  string    `json:"baseline,omitempty"` // 分支/基線
	Tasks     []string  `json:"tasks,omitempty"`    // 任務 ID（T-nn / T-nna）
	SizeBytes int64     `json:"size_bytes"`         // 檔案位元組數
	SHA256    string    `json:"sha256"`             // 內容 sha256（hex）
	Summary   string    `json:"summary,omitempty"`  // 首段摘要
	Headings  []Heading `json:"headings"`           // fence-aware 標題樹
	Refs      []Ref     `json:"refs,omitempty"`     // 對其他報告的引用
	Links     []Link    `json:"links,omitempty"`    // 相對鏈接
}

// TitleOrBase 回傳標題；無標題時退回檔名。
func (r Report) TitleOrBase() string {
	if r.Title != "" {
		return r.Title
	}
	return r.Base
}

// Index 是一次 Scan 的全部結果。
type Index struct {
	Schema  string   `json:"schema"`
	Root    string   `json:"root"`
	Reports []Report `json:"reports"`
}

// SeriesNumbers 回傳系列報告的編號（升序、去重）。
func (idx Index) SeriesNumbers() []int {
	seen := map[int]bool{}
	var nums []int
	for _, r := range idx.Reports {
		if r.Kind == KindSeries && !seen[r.Number] {
			seen[r.Number] = true
			nums = append(nums, r.Number)
		}
	}
	sort.Ints(nums)
	return nums
}

var (
	seriesNumRe = regexp.MustCompile(`^YKC_(\d+)_`)
	headingRe   = regexp.MustCompile(`^(#{1,6})[ \t]+(.*)$`)
	dateRe      = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	baselineRe  = regexp.MustCompile("(?:分支|基線)\\**[：:]\\s*`?([A-Za-z0-9][A-Za-z0-9._/-]*)")
	taskRe      = regexp.MustCompile(`T-\d+[a-z]?`)
	refRe       = regexp.MustCompile(`YKC_[^\s` + "`" + `"'"（）()，。、；：*]+\.md`)
	linkRe      = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)\)`)
	metaParaRe  = regexp.MustCompile(`^\*\*(日期|分支|基線|狀態)\*\*`)
	hrRe        = regexp.MustCompile(`^(-{3,}|\*{3,}|_{3,})$`)
)

// Scan 掃描 root 下的全部 YKC_*.md 並解析。系列報告按編號升序在前，
// 奠基文檔按檔名字節序在後。
func Scan(root string) (Index, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return Index{}, err
	}
	var reps []Report
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "YKC_") || !strings.HasSuffix(name, ".md") {
			continue
		}
		r, err := parseFile(filepath.Join(root, name), name)
		if err != nil {
			return Index{}, err
		}
		reps = append(reps, r)
	}
	sort.SliceStable(reps, func(i, j int) bool {
		a, b := reps[i], reps[j]
		if (a.Kind == KindSeries) != (b.Kind == KindSeries) {
			return a.Kind == KindSeries
		}
		if a.Kind == KindSeries {
			return a.Number < b.Number
		}
		return a.Base < b.Base
	})
	return Index{Schema: Schema, Root: normalizeRoot(root), Reports: reps}, nil
}

// normalizeRoot 把 root 規整為相對 CWD 的 '/' 路徑；'.' 變成空字串。
func normalizeRoot(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return filepath.ToSlash(root)
	}
	wd, err := os.Getwd()
	if err == nil {
		if rel, err := filepath.Rel(wd, abs); err == nil {
			rel = filepath.ToSlash(rel)
			if rel == "." {
				return ""
			}
			return rel
		}
	}
	return filepath.ToSlash(abs)
}

func parseFile(full, base string) (Report, error) {
	data, err := os.ReadFile(full)
	if err != nil {
		return Report{}, err
	}
	sum := sha256.Sum256(data)
	r := Report{
		Path:      filepath.ToSlash(base),
		Base:      base,
		Kind:      KindFoundation,
		Number:    -1,
		SizeBytes: int64(len(data)),
		SHA256:    hex.EncodeToString(sum[:]),
	}
	if m := seriesNumRe.FindStringSubmatch(base); m != nil {
		r.Kind = KindSeries
		r.Number, _ = strconv.Atoi(m[1])
	}

	lines := strings.Split(string(data), "\n")
	fence := fenceMask(lines)

	// 標題（fence-aware）。
	for i, line := range lines {
		if fence[i] {
			continue
		}
		if h := headingRe.FindStringSubmatch(line); h != nil {
			level := len(h[1])
			text := strings.TrimSpace(h[2])
			r.Headings = append(r.Headings, Heading{Level: level, Text: text, Line: i + 1})
			if level == 1 && r.Title == "" {
				r.Title = text
			}
		}
	}

	// 元數據區 = 首個 level-2 標題之前的區域：日期、分支、任務。
	zoneEnd := len(lines)
	for _, h := range r.Headings {
		if h.Level == 2 {
			zoneEnd = h.Line - 1
			break
		}
	}
	for i := 0; i < zoneEnd; i++ {
		if fence[i] {
			continue
		}
		line := lines[i]
		// 日期兩種慣例：含「日期」關鍵詞的元數據行，或副標題標題行。
		if r.Date == "" && (strings.Contains(line, "日期") || headingRe.MatchString(line)) {
			if m := dateRe.FindString(line); m != "" {
				r.Date = m
			}
		}
		if r.Baseline == "" {
			if m := baselineRe.FindStringSubmatch(line); m != nil {
				r.Baseline = m[1]
			}
		}
	}
	// 任務 ID 掃首 20 行（fence-aware，lexicographic 排序）：報告在頭部
	// （標題/元數據/首個交付表）宣告所屬任務；深層正文的提及不算。
	taskEnd := min(len(lines), 20)
	for i := 0; i < taskEnd; i++ {
		if fence[i] {
			continue
		}
		for _, t := range taskRe.FindAllString(lines[i], -1) {
			if !contains(r.Tasks, t) {
				r.Tasks = append(r.Tasks, t)
			}
		}
	}
	sort.Strings(r.Tasks)

	r.Summary = firstParagraph(lines, fence)

	// 引用與相對鏈接（fence-aware）。
	for i, line := range lines {
		if fence[i] {
			continue
		}
		for _, m := range refRe.FindAllString(line, -1) {
			r.Refs = append(r.Refs, Ref{Target: m, Line: i + 1})
		}
		for _, m := range linkRe.FindAllStringSubmatch(line, -1) {
			t := m[1]
			if strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") ||
				strings.HasPrefix(t, "mailto:") || strings.HasPrefix(t, "#") ||
				strings.HasPrefix(t, "/") {
				continue
			}
			r.Links = append(r.Links, Link{Target: t, Line: i + 1})
		}
	}
	return r, nil
}

// fenceMask 標記每一行是否位於 ``` / ~~~ fence 內部（fence 行本身算內部）。
func fenceMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	open := false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			open = !open
			mask[i] = true
			continue
		}
		mask[i] = open
	}
	return mask
}

// firstParagraph 回傳首個「普通段落」：跳過標題、引用塊、水平線、表格與
// fence 後的第一個非空行塊；塊內再逐行濾掉元數據行（**日期**：…），
// 餘行以空格連接。
func firstParagraph(lines []string, fence []bool) string {
	var block []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		skip := fence[i] || trimmed == "" || headingRe.MatchString(line) ||
			strings.HasPrefix(trimmed, ">") || hrRe.MatchString(trimmed) ||
			strings.HasPrefix(trimmed, "|")
		if skip {
			if s := flushBlock(block); s != "" {
				return s
			}
			block = nil
			continue
		}
		block = append(block, trimmed)
	}
	return flushBlock(block)
}

func flushBlock(block []string) string {
	var keep []string
	for _, line := range block {
		if metaParaRe.MatchString(line) {
			continue
		}
		keep = append(keep, line)
	}
	if len(keep) == 0 {
		return ""
	}
	return truncateRunes(strings.TrimSpace(strings.Join(keep, " ")), 220)
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
