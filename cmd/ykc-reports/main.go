// ykc-reports — YKC 報告健檢 CLI（list / check / show）。
//
// 把根目錄的 YKC_*.md 系列報告解析成結構化元數據（編號、標題、日期、基線、
// 任務、引用、鏈接、sha256），並跑審計規則（實作在 internal/reports）。
//
// 用法：
//
//	ykc-reports list  [-root .] [-json]                列出全部報告（表格或 JSON）
//	ykc-reports check [-root .] [-json] [-strict]      審計；有 error 退出 1，-strict 連 warn 也擋
//	ykc-reports show  <編號|檔名> [-root .] [-json]     單一報告詳情
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"ykc/internal/reports"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "list":
		os.Exit(cmdList(args))
	case "check":
		os.Exit(cmdCheck(args))
	case "show":
		os.Exit(cmdShow(args))
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "ykc-reports: 未知子命令 %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `ykc-reports — YKC 報告健檢

用法：
  ykc-reports list  [-root .] [-json]              列出全部報告
  ykc-reports check [-root .] [-json] [-strict]    審計（error 退出 1；-strict 連 warn 也擋）
  ykc-reports show  <編號|檔名> [-root .] [-json]   單一報告詳情
`)
}

func mustScan(root string) reports.Index {
	idx, err := reports.Scan(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ykc-reports:", err)
		os.Exit(1)
	}
	if len(idx.Reports) == 0 {
		fmt.Fprintf(os.Stderr, "ykc-reports: %s 下找不到任何 YKC_*.md\n", root)
		os.Exit(1)
	}
	return idx
}

func emitJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, "ykc-reports: json encode:", err)
		os.Exit(1)
	}
}

func cmdList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	root := fs.String("root", ".", "報告根目錄")
	asJSON := fs.Bool("json", false, "JSON 輸出")
	_ = fs.Parse(args)
	idx := mustScan(*root)
	if *asJSON {
		emitJSON(idx)
		return 0
	}
	fmt.Printf("%-4s %-54s %-12s %-22s %s\n", "#", "報告", "日期", "基線", "任務")
	for _, r := range idx.Reports {
		num := "—"
		if r.Kind == reports.KindSeries {
			num = fmt.Sprintf("%02d", r.Number)
		}
		fmt.Printf("%-4s %-54s %-12s %-22s %s\n",
			num, truncateRunes(r.TitleOrBase(), 52), orDash(r.Date), orDash(r.Baseline), strings.Join(r.Tasks, ","))
	}
	series := len(idx.SeriesNumbers())
	fmt.Printf("\n合計 %d 份（系列 %d ｜ 奠基 %d）\n", len(idx.Reports), series, len(idx.Reports)-series)
	return 0
}

func cmdCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	root := fs.String("root", ".", "報告根目錄")
	asJSON := fs.Bool("json", false, "JSON 輸出")
	strict := fs.Bool("strict", false, "warn 也擋（退出 1）")
	_ = fs.Parse(args)
	idx := mustScan(*root)
	audit := reports.Audit(idx, *root)
	if *asJSON {
		emitJSON(audit)
	} else {
		printAudit(audit)
	}
	if audit.HasErrors() || (*strict && len(audit.Findings) > 0) {
		return 1
	}
	return 0
}

func printAudit(a reports.AuditReport) {
	fmt.Printf("YKC 報告健檢：%d 份（系列 %d）｜ error %d ｜ warn %d\n\n", a.Total, a.Series, a.Errors, a.Warnings)
	if len(a.Findings) == 0 {
		fmt.Println("✅ 全部規則通過")
		return
	}
	for _, f := range a.Findings {
		icon := "⚠️ "
		if f.Severity == reports.SeverityError {
			icon = "❌"
		}
		loc := f.Report
		if f.Report != "" && f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.Report, f.Line)
		}
		fmt.Printf("%s [%s] %s %s\n", icon, f.Rule, padRunes(loc, 46), f.Message)
	}
}

func cmdShow(args []string) int {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	root := fs.String("root", ".", "報告根目錄")
	asJSON := fs.Bool("json", false, "JSON 輸出")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "ykc-reports show：需要 <編號|檔名> 一個參數")
		return 2
	}
	idx := mustScan(*root)
	sel := fs.Arg(0)
	for _, r := range idx.Reports {
		if r.Base == sel || r.Path == sel || (r.Kind == reports.KindSeries && fmt.Sprintf("%02d", r.Number) == sel) {
			if *asJSON {
				emitJSON(r)
				return 0
			}
			printReport(r)
			return 0
		}
	}
	fmt.Fprintf(os.Stderr, "ykc-reports: 找不到報告 %q\n", sel)
	return 1
}

func printReport(r reports.Report) {
	fmt.Printf("報告：%s\n", r.Path)
	fmt.Printf("標題：%s\n", r.TitleOrBase())
	kind := r.Kind
	if r.Kind == reports.KindSeries {
		kind = fmt.Sprintf("%s（編號 %02d）", r.Kind, r.Number)
	}
	fmt.Printf("種類：%s\n", kind)
	fmt.Printf("日期：%s ｜ 基線：%s\n", orDash(r.Date), orDash(r.Baseline))
	if len(r.Tasks) > 0 {
		fmt.Printf("任務：%s\n", strings.Join(r.Tasks, "、"))
	}
	fmt.Printf("大小：%d bytes ｜ sha256：%s\n", r.SizeBytes, r.SHA256)
	if r.Summary != "" {
		fmt.Printf("\n摘要：%s\n", r.Summary)
	}
	if len(r.Headings) > 0 {
		fmt.Println("\n大綱：")
		for _, h := range r.Headings {
			fmt.Printf("%s%s\n", strings.Repeat("  ", h.Level-1), h.Text)
		}
	}
	if len(r.Refs) > 0 {
		fmt.Printf("\n引用：%d 處\n", len(r.Refs))
		seen := map[string]bool{}
		for _, ref := range r.Refs {
			if !seen[ref.Target] {
				seen[ref.Target] = true
				fmt.Printf("  - %s\n", ref.Target)
			}
		}
	}
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func padRunes(s string, n int) string {
	if len([]rune(s)) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len([]rune(s)))
}
