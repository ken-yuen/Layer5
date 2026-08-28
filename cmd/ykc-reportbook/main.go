// ykc-reportbook — YKC 報告重構 CLI（build / verify）。
//
// 把根目錄的 YKC_*.md 系列報告確定性重構為三份產物（實作在 internal/reports）：
//
//	INDEX.md       人讀總索引（統計 + 系列表 + 奠基文檔表 + 完整性指紋）
//	manifest.json  機器可讀全量索引（ykc-reports/v1）
//	OUTLINE.md     每份報告的標題大綱
//
// 用法：
//
//	ykc-reportbook build  [-root .] [-out docs/reports]   生成產物（原子寫入）
//	ykc-reportbook verify [-root .] [-out docs/reports]   重生成並比對現狀；漂移退出 1（CI 閘門）
//
// 決定論紀律：產物僅為報告內容的函數（無時間戳、無隨機數）——所以 verify
// 能把「報告改了但產物沒重生成」變成機械可判的失敗。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"ykc/internal/reports"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "build":
		os.Exit(cmdBuild(args))
	case "verify":
		os.Exit(cmdVerify(args))
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "ykc-reportbook: 未知子命令 %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `ykc-reportbook — YKC 報告重構（確定性生成）

用法：
  ykc-reportbook build  [-root .] [-out docs/reports]   生成 INDEX.md + manifest.json + OUTLINE.md
  ykc-reportbook verify [-root .] [-out docs/reports]   比對產物與報告現狀；漂移退出 1
`)
}

func mustScan(root string) reports.Index {
	idx, err := reports.Scan(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ykc-reportbook:", err)
		os.Exit(1)
	}
	if len(idx.Reports) == 0 {
		fmt.Fprintf(os.Stderr, "ykc-reportbook: %s 下找不到任何 YKC_*.md\n", root)
		os.Exit(1)
	}
	return idx
}

func cmdBuild(args []string) int {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	root := fs.String("root", ".", "報告根目錄")
	out := fs.String("out", "docs/reports", "產物輸出目錄")
	_ = fs.Parse(args)
	idx := mustScan(*root)
	written, err := reports.Build(idx, *out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ykc-reportbook:", err)
		return 1
	}
	fmt.Printf("✅ 已重構 %d 份報告 → %s\n", len(idx.Reports), *out)
	for _, p := range written {
		fmt.Printf("  - %s\n", p)
	}
	return 0
}

func cmdVerify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	root := fs.String("root", ".", "報告根目錄")
	out := fs.String("out", "docs/reports", "產物目錄")
	asJSON := fs.Bool("json", false, "JSON 輸出")
	_ = fs.Parse(args)
	idx := mustScan(*root)
	results, ok := reports.Verify(idx, *out)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"ok": ok, "artifacts": results})
	} else {
		for _, r := range results {
			icon := "✅"
			switch r.Status {
			case "missing":
				icon = "🈳"
			case "drift":
				icon = "❌"
			}
			line := fmt.Sprintf("%s %s — %s", icon, r.Name, r.Status)
			if r.Message != "" {
				line += "：" + r.Message
			}
			fmt.Println(line)
		}
		if ok {
			fmt.Println("\n✅ 產物與報告現狀一致")
		} else {
			fmt.Println("\n❌ 產物漂移——執行 make reportbook 重生成")
		}
	}
	if !ok {
		return 1
	}
	return 0
}
