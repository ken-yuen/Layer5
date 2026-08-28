package reports

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 審計嚴重度。
const (
	// SeverityError 是 error 級發現（check 退出 1）。
	SeverityError = "error"
	// SeverityWarn 是 warn 級發現（僅 -strict 時退出 1）。
	SeverityWarn = "warn"
)

// Finding 是一條審計發現。
type Finding struct {
	Severity string `json:"severity"`
	Rule     string `json:"rule"`
	Report   string `json:"report,omitempty"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message"`
}

// AuditReport 是一次審計的完整結果。
type AuditReport struct {
	Schema   string    `json:"schema"`
	Root     string    `json:"root"`
	Total    int       `json:"total"`
	Series   int       `json:"series"`
	Findings []Finding `json:"findings"`
	Errors   int       `json:"errors"`
	Warnings int       `json:"warnings"`
}

// HasErrors 回報是否存在 error 級發現。
func (a AuditReport) HasErrors() bool { return a.Errors > 0 }

// Audit 對索引跑全部審計規則：series-gap、dup-number、dup-title（全局），
// missing-title、missing-date、broken-ref、broken-link、date-order（逐份）。
// 規則順序與發現順序都是確定的。
func Audit(idx Index, root string) AuditReport {
	a := AuditReport{Schema: auditSchema, Root: idx.Root, Total: len(idx.Reports)}

	// 全庫檔案集合（broken-ref / broken-link 的存在性依據）。
	exists := map[string]bool{}
	if entries, err := os.ReadDir(root); err == nil {
		for _, e := range entries {
			exists[e.Name()] = true
		}
	}

	// 全局：dup-number / dup-title / series-gap。
	byNumber := map[int]string{}
	byTitle := map[string]string{}
	for _, r := range idx.Reports {
		if r.Kind == KindSeries {
			a.Series++
			if prev, ok := byNumber[r.Number]; ok {
				a.add(SeverityError, "dup-number", r.Base, 0,
					fmt.Sprintf("編號 %02d 與 %s 重複", r.Number, prev))
			} else {
				byNumber[r.Number] = r.Base
			}
		}
		if r.Title != "" {
			if prev, ok := byTitle[r.Title]; ok {
				a.add(SeverityWarn, "dup-title", r.Base, 0,
					fmt.Sprintf("標題與 %s 重複", prev))
			} else {
				byTitle[r.Title] = r.Base
			}
		}
	}
	nums := idx.SeriesNumbers()
	for i := 1; i < len(nums); i++ {
		for n := nums[i-1] + 1; n < nums[i]; n++ {
			a.add(SeverityError, "series-gap", "", 0,
				fmt.Sprintf("系列編號斷層：缺 %02d（%02d 與 %02d 之間）", n, nums[i-1], nums[i]))
		}
	}

	// 逐份：missing-title / missing-date / broken-ref / broken-link / date-order。
	prevDate := ""
	prevBase := ""
	for _, r := range idx.Reports {
		if r.Title == "" {
			a.add(SeverityError, "missing-title", r.Base, 0, "缺少 # 標題")
		}
		if r.Date == "" && r.Kind == KindSeries {
			// 奠基文檔無日期慣例，不在此規則管轄。
			a.add(SeverityWarn, "missing-date", r.Base, 0, "缺少可解析日期（YYYY-MM-DD）")
		}
		for _, ref := range r.Refs {
			if !exists[ref.Target] {
				a.add(SeverityError, "broken-ref", r.Base, ref.Line,
					fmt.Sprintf("引用的報告不存在：%s", ref.Target))
			}
		}
		for _, ln := range r.Links {
			target := strings.SplitN(ln.Target, "#", 2)[0]
			if target == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(target))); err != nil {
				a.add(SeverityWarn, "broken-link", r.Base, ln.Line,
					fmt.Sprintf("相對鏈接目標不存在：%s", ln.Target))
			}
		}
		if r.Kind == KindSeries && r.Date != "" {
			if prevDate != "" && r.Date < prevDate {
				a.add(SeverityWarn, "date-order", r.Base, 0,
					fmt.Sprintf("日期 %s 早於前一份 %s（%s）——疑似倒掛", r.Date, prevBase, prevDate))
			}
			prevDate = r.Date
			prevBase = r.Base
		}
	}
	return a
}

func (a *AuditReport) add(sev, rule, report string, line int, msg string) {
	a.Findings = append(a.Findings, Finding{
		Severity: sev, Rule: rule, Report: report, Line: line, Message: msg,
	})
	if sev == SeverityError {
		a.Errors++
	} else {
		a.Warnings++
	}
}
