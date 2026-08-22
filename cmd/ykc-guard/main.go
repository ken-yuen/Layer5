// YKC Guard — P2 動態護欄（反欺騙裁判閉環）
//
// 子命令：
//
//	verify  -dir <dir> -claims <claims.json>   聲明 → 確定性比對（純，不改狀態）
//	score   -dir <dir> -claims <claims.json>   比對 + 信任棘輪 + 落帳（判決與事件）
//	console -dir <dir> -agent <id> [-out e.md] 用家控制台 + 證據報告
//	reset   -dir <dir> -agent <id> -reason ...  人類唯一回升入口（留審計）
//	mcp     -dir <dir>                          MCP server（stdio）
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"ykc/internal/claimview"
	"ykc/internal/domain"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "verify":
		cmdVerify(os.Args[2:])
	case "score":
		cmdScore(os.Args[2:])
	case "console":
		cmdConsole(os.Args[2:])
	case "reset":
		cmdReset(os.Args[2:])
	case "mcp":
		cmdMCP(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`用法：
  ykc-guard verify  -dir <專案> -claims <claims.json>
  ykc-guard score   -dir <專案> -claims <claims.json>
  ykc-guard console -dir <專案> -agent <id> [-out evidence.md]
  ykc-guard reset   -dir <專案> -agent <id> -reason "理由"
  ykc-guard mcp     -dir <專案>`)
}

func absDir(dir string) string {
	if a, err := filepath.Abs(dir); err == nil {
		return a
	}
	return dir
}

func cmdVerify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	dir := fs.String("dir", ".", "專案目錄")
	claims := fs.String("claims", "", "claims.json")
	_ = fs.Parse(args)

	if err := requireDir(*dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	doc, err := loadClaims(*claims)
	if err != nil {
		fmt.Fprintln(os.Stderr, "載入聲明失敗:", err)
		os.Exit(1)
	}
	p := Project{Dir: absDir(*dir)}
	fmt.Printf("===== YKC 確定性比對（代理 %s）=====\n", doc.AgentID)
	for _, c := range doc.Claims {
		v := p.Verify(c)
		switch v.Verdict {
		case "verified":
			fmt.Printf("  ✅ %s\n     ↳ %s\n", v.Text, v.Evidence)
		case "contradicted":
			fmt.Printf("  🛑 %s [嚴重度%d:%s]\n     ↳ %s\n", v.Text, v.Severity, severityKind(v.Severity), v.Evidence)
		default:
			fmt.Printf("  ❓ %s\n     ↳ %s（不視為成立）\n", v.Text, v.Evidence)
		}
	}
}

func cmdScore(args []string) {
	fs := flag.NewFlagSet("score", flag.ExitOnError)
	dir := fs.String("dir", ".", "專案目錄")
	claims := fs.String("claims", "", "claims.json")
	_ = fs.Parse(args)

	if err := requireDir(*dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	doc, err := loadClaims(*claims)
	if err != nil {
		fmt.Fprintln(os.Stderr, "載入聲明失敗:", err)
		os.Exit(1)
	}
	p := Project{Dir: absDir(*dir)}
	br, err := openBridge(p.Dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "帳本:", err)
		os.Exit(1)
	}
	defer br.Close()

	// 1) 比對（每條判決經 bridge 落帳：原子事件庫 + hash 鏈投影）
	var verdicts []Verdict
	for _, c := range doc.Claims {
		v := p.Verify(c)
		verdicts = append(verdicts, v)
		appendTrustEvent(br, p.Dir, domain.EventClaimVerdict, doc.SessionID, map[string]any{
			"agent_id": doc.AgentID, "session_id": doc.SessionID,
			"claim_id": v.ClaimID, "text": v.Text, "feature": v.Feature,
			"verdict": v.Verdict, "evidence": v.Evidence, "severity": v.Severity,
		})
	}

	// 2) 棘輪
	from := TrustLevel(claimview.TrustLevel(readAll(p.Dir), doc.AgentID, int(T3)))
	to, events := ApplyAll(from, verdicts)
	for _, ev := range events {
		appendTrustEvent(br, p.Dir, domain.EventTrustEvent, doc.SessionID, map[string]any{
			"agent_id": doc.AgentID, "session_id": doc.SessionID,
			"claim_id": ev.ClaimID, "severity": ev.Severity, "kind": ev.Kind,
			"intent": ev.Intent, "from": int(ev.From), "to": int(ev.To), "action": ev.Action,
		})
	}

	// 3) 控制台摘要
	printConsole(ConsoleData{
		AgentID: doc.AgentID, SessionID: doc.SessionID,
		Project: p.Dir, Level: to, Verdicts: verdicts, Events: events,
	})
}

func cmdConsole(args []string) {
	fs := flag.NewFlagSet("console", flag.ExitOnError)
	dir := fs.String("dir", ".", "專案目錄")
	agent := fs.String("agent", "", "代理 id")
	out := fs.String("out", "evidence.md", "證據報告路徑")
	_ = fs.Parse(args)

	if err := requireDir(*dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	d := readConsoleData(absDir(*dir), *agent)
	printConsole(d)
	if err := writeEvidenceReport(d, *out); err != nil {
		fmt.Fprintln(os.Stderr, "寫報告失敗:", err)
		os.Exit(1)
	}
	fmt.Printf("\n📄 證據報告已寫入: %s\n", *out)
}

func cmdReset(args []string) {
	fs := flag.NewFlagSet("reset", flag.ExitOnError)
	dir := fs.String("dir", ".", "專案目錄")
	agent := fs.String("agent", "", "代理 id")
	reason := fs.String("reason", "", "人類放行理由（必填，留審計）")
	_ = fs.Parse(args)

	if *reason == "" {
		fmt.Fprintln(os.Stderr, "❌ 必須提供 -reason（人類放行理由，永久留審計）")
		os.Exit(1)
	}
	if *agent == "" {
		fmt.Fprintln(os.Stderr, "❌ 必須提供 -agent（代理 id）")
		os.Exit(1)
	}
	if err := requireDir(*dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	p := Project{Dir: absDir(*dir)}
	br, err := openBridge(p.Dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "帳本:", err)
		os.Exit(1)
	}
	defer br.Close()
	appendTrustEvent(br, p.Dir, domain.EventTrustReset, "", map[string]any{
		"agent_id": *agent, "to": int(T3), "reason": *reason,
	})
	fmt.Printf("✅ 已重置代理 %s → T3 高度信任\n   理由: %s（已永久寫入帳本）\n", *agent, *reason)
}

// requireDir：邊界加固——專案目錄必須存在且為目錄。
func requireDir(dir string) error {
	fi, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("專案目錄不存在: %s", dir)
	}
	if !fi.IsDir() {
		return fmt.Errorf("專案路徑不是目錄: %s", dir)
	}
	return nil
}

func cmdMCP(args []string) {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	dir := fs.String("dir", ".", "專案目錄")
	_ = fs.Parse(args)
	serveMCP(absDir(*dir))
}
