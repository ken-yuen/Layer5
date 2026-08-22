// ykc-know — YKC 嵌入式唯讀知識庫 + 代理上下文引擎 CLI。
//
// 把 Rust 官方教學文檔、rustc 全部編譯錯誤碼（含錯誤範例與正解）、
// Rust 規則抽象組織成內容定址的「上下文原子」，提供精準檢索、依賴項圖展開、
// 上下文緩存與預算截斷——給生成式代理一個零依賴、可驗證的 Rust 知識面。
//
// 用法：
//
//	ykc-know code E0382                    # 精確查錯誤碼卡（含錯誤範例 + 正解）
//	ykc-know search "borrow after move"    # 精準檢索（BM25 + shingle + 領域加權）
//	ykc-know rule BRW-01                   # 查規則抽象
//	ykc-know rules -domain borrowing       # 列領域規則
//	ykc-know graph E0382 -depth 2          # 依賴項圖展開（代理用）
//	ykc-know book                          # 官方教學文檔目錄
//	ykc-know codes                         # 全部錯誤碼
//	ykc-know build -o kb.ykc               # 由內嵌、鎖定的種子建單一唯讀 blob
//	ykc-know import "$(rustc --version)" -o kb.ykc # 抽取對應版本官方錯誤索引、blob + manifest
//	ykc-know replay kb.ykc.manifest.json -o replay.ykc # 重抓來源並逐項可重放驗證
//	ykc-know diff old.ykc new.ykc           # 原子／metadata 差異（release 審計）
//	ykc-know open kb.ykc search "..."      # 對 blob 做唯讀查詢
//	ykc-know serve -addr 127.0.0.1 -port 8090   # 唯讀 HTTP API
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"ykc/internal/kb"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, rest := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "code", "rule", "search", "rules", "graph", "book", "codes", "stats":
		st, e := kb.Open()
		if e != nil {
			fatal(e)
		}
		err = dispatch(st, cmd, rest)
	case "build":
		err = cmdBuild(rest)
	case "import":
		err = cmdImport(rest)
	case "replay":
		err = cmdReplay(rest)
	case "diff":
		err = cmdDiff(rest)
	case "open":
		if len(rest) < 2 {
			fmt.Fprintln(os.Stderr, "用法: ykc-know open <blob> <subcommand> [args]")
			os.Exit(2)
		}
		st, e := kb.OpenFile(rest[0])
		if e != nil {
			fatal(e)
		}
		err = dispatch(st, rest[1], rest[2:])
	case "serve":
		err = cmdServe(rest)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ykc-know:", err)
	os.Exit(1)
}

// reorderArgs 把「出現在位置參數之後」的旗標搬到前面：
// Go 的 flag 套件預設停在首個非旗標參數，因此 `search "query" -k 5` 的 -k 會被當成 query。
// boolFlags 為不取值的旗標集合。
func reorderArgs(args []string, boolFlags map[string]bool) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			if !boolFlags[a] && !strings.Contains(a, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
		} else {
			pos = append(pos, a)
		}
	}
	return append(flags, pos...)
}

// boolFlags 是不取值的旗標集合（供 reorderArgs 使用）。
var boolFlags = map[string]bool{"-json": true}

func dispatch(st *kb.Store, cmd string, args []string) error {
	switch cmd {
	case "code":
		fs := flag.NewFlagSet("code", flag.ExitOnError)
		fs.Parse(args)
		if fs.NArg() < 1 {
			return fmt.Errorf("用法: ykc-know code <E0382>")
		}
		a, ok := st.ByCode(fs.Arg(0))
		if !ok || a.Kind != kb.KindError {
			return fmt.Errorf("找不到錯誤碼 %s", fs.Arg(0))
		}
		fmt.Print(kb.RenderMarkdown(&kb.ContextBundle{Query: a.Code, Version: st.Version(), Hits: []kb.Hit{{Atom: a, Score: 1}}, Atoms: []*kb.Atom{a}, TotalBytes: a.Size(), BudgetBytes: a.Size(), EstTokens: a.Size() / 4}))
		return nil
	case "rule":
		fs := flag.NewFlagSet("rule", flag.ExitOnError)
		fs.Parse(args)
		if fs.NArg() < 1 {
			return fmt.Errorf("用法: ykc-know rule <OWN-01>")
		}
		a, ok := st.ByCode(fs.Arg(0))
		if !ok || a.Kind != kb.KindRule {
			return fmt.Errorf("找不到規則 %s", fs.Arg(0))
		}
		writeAtom(a)
		return nil
	case "rules":
		fs := flag.NewFlagSet("rules", flag.ExitOnError)
		domain := fs.String("domain", "", "領域篩選")
		fs.Parse(reorderArgs(args, boolFlags))
		var rs []*kb.Atom
		if *domain != "" {
			rs = st.RulesByDomain(*domain)
		} else {
			rs = st.AtomsByKind(kb.KindRule)
		}
		for _, a := range rs {
			fmt.Printf("%-8s %-14s %s\n", a.Code, a.Domain, a.Title)
		}
		fmt.Printf("（共 %d 條規則；領域：%s）\n", len(rs), strings.Join(st.Domains(), ", "))
		return nil
	case "search":
		fs := flag.NewFlagSet("search", flag.ExitOnError)
		k := fs.Int("k", 8, "命中數")
		expand := fs.Int("expand", 2, "依賴項展開深度")
		budget := fs.Int("budget", 12000, "上下文預算 bytes")
		asJSON := fs.Bool("json", false, "輸出 JSON")
		fs.Parse(reorderArgs(args, boolFlags))
		q := strings.Join(fs.Args(), " ")
		if q == "" {
			return fmt.Errorf("用法: ykc-know search <query> [-k 8 -expand 2 -budget 12000]")
		}
		b := st.Retrieve(q, kb.SearchOpts{K: *k, ExpandDepth: *expand, BudgetBytes: *budget})
		if *asJSON {
			writeJSON(b)
		} else {
			fmt.Print(kb.RenderMarkdown(b))
		}
		return nil
	case "graph":
		fs := flag.NewFlagSet("graph", flag.ExitOnError)
		depth := fs.Int("depth", 2, "展開深度")
		fs.Parse(reorderArgs(args, boolFlags))
		if fs.NArg() < 1 {
			return fmt.Errorf("用法: ykc-know graph <CODE|ID> [-depth 2]")
		}
		a, ok := st.ByCode(fs.Arg(0))
		if !ok {
			a, ok = st.ByID(fs.Arg(0))
		}
		if !ok {
			return fmt.Errorf("找不到 %s", fs.Arg(0))
		}
		atoms := st.ExpandContext([]string{a.ID}, *depth)
		for i, n := range atoms {
			kind := string(n.Kind)
			code := n.Code
			if code == "" {
				code = n.ID[:12]
			}
			fmt.Printf("[%d] %-8s %-14s %s\n", i, kind, code, n.Title)
		}
		return nil
	case "book":
		fs := flag.NewFlagSet("book", flag.ExitOnError)
		fs.Parse(reorderArgs(args, boolFlags))
		if fs.NArg() >= 1 {
			a, ok := st.ByCode(fs.Arg(0))
			if !ok || (a.Kind != kb.KindBook && a.Kind != kb.KindPart) {
				return fmt.Errorf("找不到章節 %s", fs.Arg(0))
			}
			writeAtom(a)
			return nil
		}
		for _, p := range st.AtomsByKind(kb.KindPart) {
			fmt.Printf("▍%s\n", p.Title)
			for _, dep := range st.Dependents(p.ID) {
				if dep.Kind == kb.KindBook {
					fmt.Printf("   - %-48s %s\n", dep.Code, dep.Title)
				}
			}
		}
		return nil
	case "codes":
		for _, a := range st.AtomsByKind(kb.KindError) {
			fmt.Printf("%-8s %s\n", a.Code, a.Title)
		}
		return nil
	case "stats":
		cs := st.CacheStats()
		meta := st.Metadata()
		rustcVersion := meta.RustcVersion
		if rustcVersion == "" {
			rustcVersion = "unknown (legacy v1 blob; please re-import)"
		}
		fmt.Printf("version   = %s\n", st.Version())
		fmt.Printf("source    = %s\n", st.Source())
		fmt.Printf("rustc     = %s\n", rustcVersion)
		if meta.ErrorIndexURL != "" {
			fmt.Printf("index URL = %s\n", meta.ErrorIndexURL)
		}
		if meta.ErrorIndexSHA256 != "" {
			fmt.Printf("index sha = %s\n", meta.ErrorIndexSHA256)
		}
		if meta.TranslationVersion != "" {
			fmt.Printf("zh-Hant   = %s (%d translated error cards)\n", meta.TranslationVersion, st.TranslatedErrorCount())
		}
		fmt.Printf("atoms     = %d\n", st.Count())
		for _, k := range []kb.Kind{kb.KindError, kb.KindRule, kb.KindBook, kb.KindPart, kb.KindTOC} {
			fmt.Printf("  %-10s = %d\n", k, len(st.AtomsByKind(k)))
		}
		fmt.Printf("domains   = %d (%s)\n", len(st.Domains()), strings.Join(st.Domains(), ", "))
		fmt.Printf("cache     = hits %d / misses %d (entries %d)\n", cs.Hits, cs.Misses, cs.Entries)
		if cs.Persistent.Enabled {
			fmt.Printf("disk cache= hits %d / misses %d / writes %d / errors %d (%s)\n", cs.Persistent.Hits, cs.Persistent.Misses, cs.Persistent.Writes, cs.Persistent.Errors, cs.Persistent.Dir)
		}
		fmt.Printf("cycle SCC = %d（相關概念互引，屬預期）\n", len(st.Cycles()))
		return nil
	}
	return fmt.Errorf("未知子命令 %s", cmd)
}

func writeAtom(a *kb.Atom) {
	fmt.Printf("%s [%s]\n", a.Code, a.Kind)
	fmt.Printf("title: %s\n", a.Title)
	if a.ZH != "" {
		fmt.Printf("規則:  %s\n", a.ZH)
	}
	if a.Why != "" {
		fmt.Printf("為什麼: %s\n", a.Why)
	}
	if a.Body != "" {
		fmt.Printf("\n%s\n", strings.TrimSpace(a.Body))
	}
	if a.Err != "" {
		fmt.Printf("\n錯誤範例:\n%s\n", a.Err)
	}
	if a.Fix != "" {
		fmt.Printf("\n正解:\n%s\n", a.Fix)
	}
	if len(a.Fixes) > 0 {
		fmt.Println("\n修法:")
		for i, f := range a.Fixes {
			fmt.Printf("  %d. %s\n", i+1, f)
		}
	}
	if a.Source != "" {
		fmt.Printf("\n出處: %s\n", a.Source)
	}
}

func cmdBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	out := fs.String("o", "kb.ykc", "輸出路徑")
	fs.Parse(reorderArgs(args, boolFlags))
	if err := kb.Save(*out); err != nil {
		return err
	}
	fi, _ := os.Stat(*out)
	fmt.Printf("已建庫：%s（%.1f KB；rustc=%s）\n", *out, float64(fi.Size())/1024, kb.EmbeddedRustcVersion())
	return nil
}

// cmdImport 以指定 rustc 版本的官方 error_codes/print.html 重建可追溯 blob。
// 版本參數可直接傳 `rustc --version` 的完整輸出，例如：
// ykc-know import "$(rustc --version)" -o bin/kb.ykc
func cmdImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	out := fs.String("o", "kb.ykc", "輸出路徑")
	manifestPath := fs.String("manifest", "", "release manifest 輸出路徑（預設 <blob>.manifest.json）")
	indexURL := fs.String("url", "", "error_codes/print.html URL（預設官方對應版本）")
	fs.Parse(reorderArgs(args, boolFlags))
	if fs.NArg() < 1 {
		return fmt.Errorf("用法: ykc-know import <rustc版本|`rustc --version`輸出> [-o kb.ykc -manifest kb.ykc.manifest.json]")
	}
	result, err := kb.ImportErrorIndex(context.Background(), kb.ImportOptions{
		RustcVersion:  strings.Join(fs.Args(), " "),
		ErrorIndexURL: *indexURL,
	})
	if err != nil {
		return err
	}
	manifest, err := result.SaveWithManifest(*out, *manifestPath)
	if err != nil {
		return err
	}
	fi, err := os.Stat(*out)
	if err != nil {
		return err
	}
	meta := result.Metadata()
	if *manifestPath == "" {
		*manifestPath = *out + ".manifest.json"
	}
	fmt.Printf("已匯入 rustc %s：%d 條錯誤碼、%d 原子 → %s（%.1f KB）\n", result.Metadata().RustcVersion, result.ErrorCount(), result.Count(), *out, float64(fi.Size())/1024)
	fmt.Printf("來源：%s\n內容 SHA-256：%s\nETag：%s\n", meta.ErrorIndexURL, meta.ErrorIndexSHA256, result.SourceETag())
	fmt.Printf("manifest：%s（dataset=%s，blob SHA-256=%s）\n", *manifestPath, manifest.DatasetVersion, manifest.BlobSHA256)
	return nil
}

func cmdReplay(args []string) error {
	fs := flag.NewFlagSet("replay", flag.ExitOnError)
	out := fs.String("o", "", "重建 blob 輸出路徑（必填）")
	outManifest := fs.String("manifest", "", "重建後 manifest 輸出路徑（預設 <blob>.manifest.json）")
	fs.Parse(reorderArgs(args, boolFlags))
	if fs.NArg() != 1 || *out == "" {
		return fmt.Errorf("用法: ykc-know replay <manifest.json> -o <replayed.ykc>")
	}
	manifest, err := kb.ReadImportManifest(fs.Arg(0))
	if err != nil {
		return err
	}
	result, err := kb.ReplayImport(context.Background(), manifest, nil)
	if err != nil {
		return err
	}
	actual, err := result.SaveWithManifest(*out, *outManifest)
	if err != nil {
		return err
	}
	if *outManifest == "" {
		*outManifest = *out + ".manifest.json"
	}
	fmt.Printf("✅ manifest 可重放：%s → %s\n", fs.Arg(0), *out)
	fmt.Printf("dataset=%s · blob SHA-256=%s · manifest=%s\n", actual.DatasetVersion, actual.BlobSHA256, *outManifest)
	return nil
}

func cmdDiff(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "輸出完整 JSON")
	limit := fs.Int("limit", 200, "Markdown 每類最多列數；0=不截斷")
	fs.Parse(reorderArgs(args, boolFlags))
	if fs.NArg() != 2 {
		return fmt.Errorf("用法: ykc-know diff <base.ykc> <target.ykc> [-json -limit 200]")
	}
	report, err := kb.DiffFiles(fs.Arg(0), fs.Arg(1))
	if err != nil {
		return err
	}
	if *asJSON {
		writeJSON(report)
		return nil
	}
	fmt.Print(kb.RenderDiffMarkdown(report, *limit))
	return nil
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1", "綁定位址")
	port := fs.Int("port", 8090, "監聽埠")
	blob := fs.String("blob", "", "已校驗 KB blob 路徑（預設內嵌鎖版資料）")
	fs.Parse(reorderArgs(args, boolFlags))

	var (
		st  *kb.Store
		err error
	)
	if *blob == "" {
		st, err = kb.Open()
	} else {
		st, err = kb.OpenFile(*blob)
	}
	if err != nil {
		return err
	}
	listen := net.JoinHostPort(*addr, fmt.Sprintf("%d", *port))
	log.Printf("ykc-know 唯讀知識庫 API listening on %s（version=%s, rustc=%s, atoms=%d）", listen, st.Version(), st.RustcVersion(), st.Count())
	return http.ListenAndServe(listen, kb.Handler(st))
}

func writeJSON(v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "json:", err)
		return
	}
	fmt.Println(string(b))
}

func usage() {
	fmt.Fprint(os.Stderr, `YKC 知識庫（嵌入式唯讀 + 代理上下文引擎）

Commands:
  code <E0382>            錯誤碼卡（含錯誤範例 + 正解）
  rule <OWN-01>           規則抽象
  rules [-domain d]       列出規則（領域：ownership/borrowing/lifetime/...）
  search <query> [...]    精準檢索 + 依賴項展開 + 預算截斷
  graph <CODE|ID> [...]   依賴項圖展開（代理用）
  book [id]               官方教學文檔目錄 / 章節
  codes                   全部錯誤碼（518 條）
  stats                   統計與緩存狀態
  build -o kb.ykc         由內嵌、鎖定種子建單一唯讀 blob
  import <rustc版本> [...] 抽取官方 error index，輸出 blob + 可重放 manifest
  replay <manifest> -o X  重抓來源、核對 SHA/ETag/原子/blob 後重建 X
  diff <base> <target>    比較兩 blob 的 metadata、原子內容及 graph refs
  open <blob> <cmd> [...] 對 blob 做唯讀查詢（同上子命令）
  serve [-addr -port -blob] 唯讀 HTTP API（可提供已校驗鎖版 blob）
`)
}
