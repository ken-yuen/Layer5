package reports

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ykc/internal/atomicfile"
)

// generatedBy 是 manifest 的生成器標識。
const generatedBy = "ykc-reportbook/1"

// 產物檔名（順序即 Build 回傳順序）。
const (
	indexName    = "INDEX.md"
	manifestName = "manifest.json"
	outlineName  = "OUTLINE.md"
)

// artifactHeader 是產物開頭的勿手改注記。
const artifactHeader = "<!-- 本檔由 ykc-reportbook 確定性生成——請勿手改；改報告後執行 make reportbook 重生成。 -->"

// VerifyResult 是單件產物的比對結果。
type VerifyResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // ok|missing|drift
	Message string `json:"message,omitempty"`
}

// Artifacts 確定性生成三件產物的內容（僅為報告內容的函數）。
func Artifacts(idx Index) map[string][]byte {
	return map[string][]byte{
		indexName:    []byte(renderIndex(idx)),
		manifestName: renderManifest(idx),
		outlineName:  []byte(renderOutline(idx)),
	}
}

// Build 生成三件產物到 outDir（原子寫入），回傳寫入路徑。
func Build(idx Index, outDir string) ([]string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	arts := Artifacts(idx)
	var written []string
	for _, name := range []string{indexName, manifestName, outlineName} {
		p := filepath.Join(outDir, name)
		if err := atomicfile.WriteFileSync(p, arts[name], 0o644); err != nil {
			return nil, err
		}
		written = append(written, p)
	}
	return written, nil
}

// Verify 重生成並逐字節比對 outDir 現狀；任何 missing/drift 都令 ok=false。
func Verify(idx Index, outDir string) ([]VerifyResult, bool) {
	arts := Artifacts(idx)
	var results []VerifyResult
	ok := true
	for _, name := range []string{indexName, manifestName, outlineName} {
		p := filepath.Join(outDir, name)
		cur, err := os.ReadFile(p)
		switch {
		case os.IsNotExist(err):
			ok = false
			results = append(results, VerifyResult{Name: name, Status: "missing",
				Message: "產物不存在——執行 make reportbook 生成"})
		case err != nil:
			ok = false
			results = append(results, VerifyResult{Name: name, Status: "drift", Message: err.Error()})
		case !bytes.Equal(cur, arts[name]):
			ok = false
			results = append(results, VerifyResult{Name: name, Status: "drift", Message: "內容與重生成結果不一致"})
		default:
			results = append(results, VerifyResult{Name: name, Status: "ok"})
		}
	}
	return results, ok
}

func renderIndex(idx Index) string {
	var b strings.Builder
	b.WriteString("# YKC 報告總索引\n\n")
	b.WriteString(artifactHeader + "\n\n")

	var series, foundation []Report
	var total int64
	for _, r := range idx.Reports {
		total += r.SizeBytes
		if r.Kind == KindSeries {
			series = append(series, r)
		} else {
			foundation = append(foundation, r)
		}
	}
	kb := float64(total) / 1024
	fmt.Fprintf(&b, "> 系列報告 **%d** 份 ｜ 奠基文檔 **%d** 份 ｜ 合計 **%.1f KB** ｜ schema `%s`\n\n",
		len(series), len(foundation), kb, Schema)

	b.WriteString("## 系列報告（按編號）\n\n")
	b.WriteString("| # | 報告 | 日期 | 基線 | 任務 | 摘要 |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, r := range series {
		baseline := "—"
		if r.Baseline != "" {
			baseline = "`" + r.Baseline + "`"
		}
		tasks := "—"
		if len(r.Tasks) > 0 {
			tasks = strings.Join(r.Tasks, "、")
		}
		fmt.Fprintf(&b, "| %02d | [%s](../../%s) | %s | %s | %s | %s |\n",
			r.Number, r.TitleOrBase(), r.Path, orDash(r.Date), baseline, tasks, truncateRunes(r.Summary, 80))
	}

	b.WriteString("\n## 奠基文檔\n\n")
	b.WriteString("| 文檔 | 日期 | 摘要 |\n")
	b.WriteString("|---|---|---|\n")
	for _, r := range foundation {
		// 奠基表不截斷摘要（系列表截 80 runes）。
		fmt.Fprintf(&b, "| [%s](../../%s) | %s | %s |\n",
			r.TitleOrBase(), r.Path, orDash(r.Date), r.Summary)
	}

	b.WriteString("\n## 完整性指紋\n\n")
	b.WriteString("| 報告 | SHA-256（前 16 位） | 位元組 |\n")
	b.WriteString("|---|---|---|\n")
	for _, r := range idx.Reports {
		fmt.Fprintf(&b, "| %s | `%s` | %d |\n", r.Base, r.SHA256[:16], r.SizeBytes)
	}
	return b.String()
}

func renderManifest(idx Index) []byte {
	m := struct {
		Schema      string   `json:"schema"`
		Root        string   `json:"root"`
		Reports     []Report `json:"reports"`
		GeneratedBy string   `json:"generated_by"`
	}{Schema: idx.Schema, Root: idx.Root, Reports: idx.Reports, GeneratedBy: generatedBy}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		// Report 全部欄位皆為可序列化型別；此路徑實務不可達。
		panic(fmt.Sprintf("reports: manifest marshal: %v", err))
	}
	return append(data, '\n')
}

func renderOutline(idx Index) string {
	var b strings.Builder
	b.WriteString("# YKC 報告大綱\n\n")
	b.WriteString(artifactHeader + "\n\n")
	b.WriteString("> 用途：代理與人類在讀全文之前先定位章節。每份報告列出全部標題層級。\n")
	for _, r := range idx.Reports {
		header := r.TitleOrBase()
		if r.Kind == KindSeries {
			header = fmt.Sprintf("YKC_%02d — %s", r.Number, header)
		}
		var meta []string
		if r.Date != "" {
			meta = append(meta, "日期 "+r.Date)
		}
		if r.Baseline != "" {
			meta = append(meta, "基線 `"+r.Baseline+"`")
		}
		if len(r.Tasks) > 0 {
			meta = append(meta, "任務 "+strings.Join(r.Tasks, "、"))
		}
		meta = append(meta, "路徑 `"+r.Path+"`")
		fmt.Fprintf(&b, "\n## %s\n\n_%s_\n\n", header, strings.Join(meta, " ｜ "))
		for _, h := range r.Headings {
			if h.Level < 2 {
				continue
			}
			b.WriteString(strings.Repeat("  ", h.Level-2) + "- " + h.Text + "\n")
		}
	}
	b.WriteString("\n")
	return b.String()
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
