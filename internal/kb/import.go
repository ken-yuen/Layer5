package kb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"net/http"
	urlpkg "net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const maxErrorIndexBytes = 8 << 20

// ImportOptions 控制從官方 rustc 錯誤索引建庫的行為。ErrorIndexURL 留空時，
// 會使用 https://doc.rust-lang.org/<RustcVersion>/error_codes/print.html；保留此
// 欄位是為了可重現測試與受管鏡像，不是執行期查詢的網路依賴。
type ImportOptions struct {
	RustcVersion  string
	ErrorIndexURL string
	HTTPClient    *http.Client
}

// ImportResult 是一次版本鎖定匯入的不可變結果。它包含新版錯誤碼卡，以及仍與
// 專案共同維護的規則抽象和官方教學文檔；Save 寫出的 v2 blob 會校驗全部 metadata。
type ImportResult struct {
	atoms              []*Atom
	meta               DatasetMeta
	errorCount         int
	sourceETag         string
	sourceLastModified string
}

// Metadata 回傳本次匯入要寫入 blob 標頭的可追溯資訊。
func (r *ImportResult) Metadata() DatasetMeta { return r.meta }

// ErrorCount 回傳從指定 rustc 錯誤索引抽出的錯誤碼數量。
func (r *ImportResult) ErrorCount() int { return r.errorCount }

// Count 回傳完整資料集（錯誤碼 + 規則 + 文檔）的原子數量。
func (r *ImportResult) Count() int { return len(r.atoms) }

// SourceETag 回傳匯入時 HTTP 回應的 ETag（若來源未提供則為空）。ETag 放在
// release manifest，不放進 blob metadata，避免 CDN header 漂移破壞內容可重現性。
func (r *ImportResult) SourceETag() string { return r.sourceETag }

// SourceLastModified 回傳匯入時 HTTP 回應的 Last-Modified（若來源未提供則為空）。
func (r *ImportResult) SourceLastModified() string { return r.sourceLastModified }

// Blob 把匯入結果編碼為已校驗的 v2 唯讀 blob。
func (r *ImportResult) Blob() ([]byte, error) { return encodeBlob(r.atoms, r.meta) }

// Save 原子寫入匯入結果；成功後可用 OpenFile 驗證/查詢。
func (r *ImportResult) Save(path string) error {
	b, err := r.Blob()
	if err != nil {
		return err
	}
	return saveBlob(path, b)
}

// ImportErrorIndex 從指定 rustc 版本的官方 print.html 抽取完整錯誤索引，並以
// 現有的規則/教學種子組成一個可離線使用的版本鎖定資料集。
//
// 這是明確的離線建庫動作；Store 查詢本身始終沒有網路寫入或下載路徑。
func ImportErrorIndex(ctx context.Context, opts ImportOptions) (*ImportResult, error) {
	version, err := normalizeRustcVersion(opts.RustcVersion)
	if err != nil {
		return nil, err
	}
	indexURL := strings.TrimSpace(opts.ErrorIndexURL)
	if indexURL == "" {
		indexURL = fmt.Sprintf("https://doc.rust-lang.org/%s/error_codes/print.html", version)
	}
	if err := validateIndexURL(indexURL); err != nil {
		return nil, err
	}

	fetched, err := fetchErrorIndex(ctx, opts.HTTPClient, indexURL)
	if err != nil {
		return nil, err
	}
	errs, err := parseErrorIndexHTML(fetched.body)
	if err != nil {
		return nil, fmt.Errorf("kb: parse %s: %w", indexURL, err)
	}
	if len(errs) == 0 {
		return nil, fmt.Errorf("kb: %s contained no error-code sections", indexURL)
	}
	errorSourceBase, err := errorSourceBaseForIndex(indexURL)
	if err != nil {
		return nil, err
	}

	_, rules, book, _, err := loadSeed()
	if err != nil {
		return nil, err
	}
	atoms, _, err := buildAtomsFromSeed(errs, rules, book, errorSourceBase)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(fetched.body)
	return &ImportResult{
		atoms:              atoms,
		errorCount:         len(errs),
		sourceETag:         fetched.etag,
		sourceLastModified: fetched.lastModified,
		meta: DatasetMeta{
			RustcVersion:       version,
			ErrorIndexURL:      indexURL,
			ErrorIndexSHA256:   hex.EncodeToString(sum[:]),
			TranslationVersion: ErrorTranslationVersion(),
		},
	}, nil
}

var exactRustcVersion = regexp.MustCompile(`^1\.[0-9]+\.[0-9]+$`)

// normalizeRustcVersion 接受 "1.98.0"、"v1.98.0" 或 `rustc --version` 的完整
// 輸出，並拒絕會改變 URL 路徑的任意字串。
func normalizeRustcVersion(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	fields := strings.Fields(v)
	if len(fields) >= 2 && fields[0] == "rustc" {
		v = fields[1]
	}
	v = strings.TrimPrefix(v, "v")
	if !exactRustcVersion.MatchString(v) {
		return "", fmt.Errorf("kb: 無效 rustc 版本 %q（預期 1.x.y 或 `rustc --version` 輸出）", raw)
	}
	return v, nil
}

func validateIndexURL(raw string) error {
	u, err := urlpkg.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return fmt.Errorf("kb: invalid error index URL %q", raw)
	}
	if !strings.HasSuffix(u.Path, "/error_codes/print.html") {
		return fmt.Errorf("kb: error index URL must end in /error_codes/print.html")
	}
	return nil
}

type fetchedErrorIndex struct {
	body         []byte
	etag         string
	lastModified string
}

func fetchErrorIndex(ctx context.Context, client *http.Client, rawURL string) (fetchedErrorIndex, error) {
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fetchedErrorIndex{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fetchedErrorIndex{}, fmt.Errorf("kb: fetch error index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fetchedErrorIndex{}, fmt.Errorf("kb: fetch error index: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorIndexBytes+1))
	if err != nil {
		return fetchedErrorIndex{}, fmt.Errorf("kb: read error index: %w", err)
	}
	if len(body) > maxErrorIndexBytes {
		return fetchedErrorIndex{}, fmt.Errorf("kb: error index exceeds %d bytes", maxErrorIndexBytes)
	}
	return fetchedErrorIndex{
		body:         body,
		etag:         strings.TrimSpace(resp.Header.Get("ETag")),
		lastModified: strings.TrimSpace(resp.Header.Get("Last-Modified")),
	}, nil
}

func errorSourceBaseForIndex(indexURL string) (string, error) {
	u, err := urlpkg.Parse(indexURL)
	if err != nil {
		return "", err
	}
	u.RawQuery = ""
	u.Fragment = ""
	base := u.String()
	if !strings.HasSuffix(base, "print.html") {
		return "", fmt.Errorf("kb: cannot derive error-card source URL from %q", indexURL)
	}
	return strings.TrimSuffix(base, "print.html") + "%s.html", nil
}

var (
	errorHeadingRE = regexp.MustCompile(`(?is)<h1\b[^>]*\bid\s*=\s*["']error-code-(e[0-9]{4})["'][^>]*>.*?</h1\s*>`)
	codeBlockRE    = regexp.MustCompile(`(?is)<pre\b[^>]*>\s*<code\b([^>]*)>(.*?)</code\s*>\s*</pre\s*>`)
	paragraphRE    = regexp.MustCompile(`(?is)<p\b[^>]*>(.*?)</p\s*>`)
	tagRE          = regexp.MustCompile(`(?is)<[^>]+>`)
	labelRE        = regexp.MustCompile(`(?i)\b(erroneous code examples?|error code examples?)\s*:`)
)

// parseErrorIndexHTML 擷取 mdBook print.html 中每個 error-code-E#### 區塊。這個
// 小型抽取器刻意只依賴 Rust 官方文件的穩定 h1/pre/code 結構，避免為一次性離線
// import 引入 HTML parser 第三方依賴到 YKC 的執行期供應鏈。
func parseErrorIndexHTML(doc []byte) ([]seedError, error) {
	matches := errorHeadingRE.FindAllSubmatchIndex(doc, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no error-code headings found")
	}
	seen := make(map[string]bool, len(matches))
	errs := make([]seedError, 0, len(matches))
	for i, m := range matches {
		code := strings.ToUpper(string(doc[m[2]:m[3]]))
		if seen[code] {
			return nil, fmt.Errorf("duplicate error code %s", code)
		}
		seen[code] = true
		end := len(doc)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		e, err := parseErrorBlock(code, string(doc[m[1]:end]))
		if err != nil {
			return nil, err
		}
		errs = append(errs, e)
	}
	// print.html 的頁面分段已排序；仍明確排序，確保鏡像重排 HTML 時 blob 內容不飄移。
	sort.Slice(errs, func(i, j int) bool { return errs[i].Code < errs[j].Code })
	return errs, nil
}

func parseErrorBlock(code, block string) (seedError, error) {
	var errExamples []string
	var fixExample string
	seenFailure := false
	for _, match := range codeBlockRE.FindAllStringSubmatch(block, -1) {
		attrs, source := strings.ToLower(match[1]), cleanCode(match[2])
		if source == "" {
			continue
		}
		if strings.Contains(attrs, "compile_fail") {
			seenFailure = true
			errExamples = append(errExamples, source)
			continue
		}
		// 正解優先取第一段「錯誤範例」後的 Rust 程式；若該碼沒有 compile_fail
		// 範例，仍保留第一段 Rust code，避免遺失官方給出的可運行示範。
		if fixExample == "" && (seenFailure || strings.Contains(attrs, "language-rust")) {
			fixExample = source
		}
	}

	textWithoutCode := codeBlockRE.ReplaceAllString(block, "\n")
	explanation := normalizeHTMLText(textWithoutCode)
	explanation = strings.TrimSpace(labelRE.ReplaceAllString(explanation, ""))
	title := firstParagraph(block)
	if title == "" {
		title = firstSentence(explanation)
	}
	if title == "" {
		title = "Rust compiler error " + code
	}
	return seedError{
		Code:        code,
		Title:       title,
		Explanation: explanation,
		ErrExample:  strings.Join(errExamples, "\n\n"),
		FixExample:  fixExample,
	}, nil
}

func firstParagraph(block string) string {
	for _, m := range paragraphRE.FindAllStringSubmatch(block, -1) {
		p := normalizeHTMLText(m[1])
		if p == "" || labelRE.MatchString(p) {
			continue
		}
		return p
	}
	return ""
}

func firstSentence(s string) string {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return strings.TrimSpace(s[:i+1])
	}
	return strings.TrimSpace(s)
}

func cleanCode(s string) string {
	return strings.TrimSpace(html.UnescapeString(s))
}

func normalizeHTMLText(s string) string {
	s = tagRE.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}
