package kb

import (
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// 種子資料以 go:embed 編入二進制（嵌入式、唯讀）。
// 來源：
//   - errcodes.json.gz — rustc 錯誤索引全文（doc.rust-lang.org/error_codes/，518 條）
//   - book.json.gz     — The Rust Programming Language（官方教學文檔）結構 + 章節摘要
//   - rules.json.gz    — Rust 規則抽象（54 條領域規則）
//   - boost.json.gz    — 常見錯誤碼優先級 + 領域詞彙加權表
//
//go:embed data/*.json.gz
var dataFS embed.FS

type seedError struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Explanation string `json:"explanation"`
	ErrExample  string `json:"err_example"`
	FixExample  string `json:"fix_example"`
}

type seedRule struct {
	ID     string   `json:"id"`
	Domain string   `json:"domain"`
	Title  string   `json:"title"`
	ZH     string   `json:"zh"`
	Why    string   `json:"why"`
	Codes  []string `json:"codes"`
	Book   []string `json:"book"`
	Refs   []string `json:"refs"`
	Fixes  []string `json:"fixes"`
}

type seedPart struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type seedChapter struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Part     string    `json:"part"`
	Summary  string    `json:"summary"`
	Sections []Section `json:"sections"`
}

type seedBook struct {
	Parts    []seedPart    `json:"parts"`
	Chapters []seedChapter `json:"chapters"`
}

type seedBoost struct {
	Tier1 []string            `json:"tier1"`
	Tier2 []string            `json:"tier2"`
	Terms map[string][]string `json:"terms"`
}

const (
	errSourceBase  = "https://doc.rust-lang.org/error_codes/%s.html"
	bookSourceBase = "https://doc.rust-lang.org/book/%s.html"
	bookRootSource = "https://doc.rust-lang.org/book/"
)

// loadSeed 解壓並反序列化內嵌種子資料。
func loadSeed() ([]seedError, []seedRule, seedBook, seedBoost, error) {
	var errs []seedError
	var rules []seedRule
	var book seedBook
	var boost seedBoost

	read := func(name string, v any) error {
		f, err := dataFS.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		zr, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer zr.Close()
		b, err := io.ReadAll(zr)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, v)
	}
	if err := read("data/errcodes.json.gz", &errs); err != nil {
		return nil, nil, book, boost, fmt.Errorf("kb: load errcodes: %w", err)
	}
	if err := read("data/rules.json.gz", &rules); err != nil {
		return nil, nil, book, boost, fmt.Errorf("kb: load rules: %w", err)
	}
	if err := read("data/book.json.gz", &book); err != nil {
		return nil, nil, book, boost, fmt.Errorf("kb: load book: %w", err)
	}
	if err := read("data/boost.json.gz", &boost); err != nil {
		return nil, nil, book, boost, fmt.Errorf("kb: load boost: %w", err)
	}
	return errs, rules, book, boost, nil
}

// buildAtoms 由種子資料組裝上下文原子並解析依賴項圖（Refs）。
func buildAtoms() ([]*Atom, map[string]string, error) {
	errs, rules, book, _, err := loadSeed()
	if err != nil {
		return nil, nil, err
	}

	// 1) 建立原子（尚未解析 Refs）。
	atoms := make([]*Atom, 0, len(errs)+len(rules)+len(book.Chapters)+len(book.Parts)+1)

	for _, e := range errs {
		atoms = append(atoms, &Atom{
			Kind:   KindError,
			Code:   strings.ToUpper(e.Code),
			Title:  e.Title,
			Body:   e.Explanation,
			Err:    e.ErrExample,
			Fix:    e.FixExample,
			Tags:   []string{strings.ToUpper(e.Code), "error"},
			Source: fmt.Sprintf(errSourceBase, strings.ToUpper(e.Code)),
		})
	}
	for _, r := range rules {
		tags := []string{r.ID, "rule", r.Domain}
		atoms = append(atoms, &Atom{
			Kind:   KindRule,
			Code:   r.ID,
			Domain: r.Domain,
			Title:  r.Title,
			ZH:     r.ZH,
			Why:    r.Why,
			Fixes:  r.Fixes,
			Tags:   tags,
		})
	}
	// 章節 + 部。
	for _, p := range book.Parts {
		atoms = append(atoms, &Atom{
			Kind:   KindPart,
			Code:   p.ID,
			Title:  p.Title,
			Tags:   []string{p.ID, "book", "part"},
			Source: bookRootSource,
		})
	}
	for _, c := range book.Chapters {
		atoms = append(atoms, &Atom{
			Kind:     KindBook,
			Code:     c.ID,
			Title:    c.Title,
			Body:     c.Summary,
			Tags:     []string{c.ID, "book", "chapter"},
			Sections: c.Sections,
			Source:   fmt.Sprintf(bookSourceBase, c.ID),
		})
	}

	// 2) 內容定址：指派 ID（去重校驗）。byCode 同時登記原碼與大寫碼
	//（錯誤碼大小寫不敏感查詢；章節/規則 id 原樣登記）。
	byCode := map[string]string{}
	seen := map[string]string{}
	for _, a := range atoms {
		id := a.contentHash()
		if other, dup := seen[id]; dup {
			return nil, nil, fmt.Errorf("kb: content hash collision %s (%s vs %s)", id, other, a.Code)
		}
		seen[id] = a.Code
		a.ID = id
		byCode[a.Code] = id
		byCode[strings.ToUpper(a.Code)] = id
	}

	// 3) 解析依賴項（邏輯鍵 → 原子 ID）。
	for i, a := range atoms {
		switch a.Kind {
		case KindRule:
			r := rulesByID(rules)[a.Code]
			var refs []string
			for _, k := range r.Refs {
				refs = appendRef(refs, byCode, strings.ToUpper(k))
			}
			for _, k := range r.Book {
				refs = appendRef(refs, byCode, k)
			}
			for _, k := range r.Codes {
				refs = appendRef(refs, byCode, strings.ToUpper(k))
			}
			atoms[i].Refs = dedupe(refs)
		case KindBook:
			var refs []string
			if c := bookChapterByID(book, a.Code); c != nil && c.Part != "" {
				refs = appendRef(refs, byCode, c.Part)
			}
			atoms[i].Refs = refs
		case KindPart, KindError:
			// 無直接依賴；反向邊由 Backrefs 提供。
		}
	}

	// 4) 目錄原子（TOC）：依賴所有「部」。
	toc := &Atom{
		Kind:   KindTOC,
		Code:   "toc",
		Title:  "The Rust Programming Language — Table of Contents",
		Tags:   []string{"toc", "book"},
		Source: bookRootSource,
	}
	toc.ID = toc.contentHash()
	var tocRefs []string
	for _, p := range book.Parts {
		tocRefs = appendRef(tocRefs, byCode, p.ID)
	}
	toc.Refs = tocRefs
	atoms = append(atoms, toc)

	return atoms, byCode, nil
}

func appendRef(refs []string, byCode map[string]string, key string) []string {
	if key == "" {
		return refs
	}
	// 依原樣、大寫、小寫依序嘗試（涵蓋錯誤碼大小寫與章節 id 大小寫差異）。
	if id, ok := byCode[key]; ok {
		return append(refs, id)
	}
	if id, ok := byCode[strings.ToUpper(key)]; ok {
		return append(refs, id)
	}
	if id, ok := byCode[strings.ToLower(key)]; ok {
		return append(refs, id)
	}
	return refs
}

func rulesByID(rs []seedRule) map[string]*seedRule {
	m := make(map[string]*seedRule, len(rs))
	for i := range rs {
		m[strings.ToUpper(rs[i].ID)] = &rs[i]
	}
	return m
}

func bookChapterByID(b seedBook, id string) *seedChapter {
	for i := range b.Chapters {
		if b.Chapters[i].ID == id {
			return &b.Chapters[i]
		}
	}
	return nil
}

// versionOf 計算資料版本：對全部原子 ID 排序後取 sha256 前 16 hex。
// 用於緩存鍵與 blob 校驗（同一資料集永遠同一版本 → 緩存可安全命中共用）。
func versionOf(atoms []*Atom) string {
	ids := make([]string, len(atoms))
	for i, a := range atoms {
		ids[i] = a.ID
	}
	sort.Strings(ids)
	return hashString(strings.Join(ids, "\n"))
}
