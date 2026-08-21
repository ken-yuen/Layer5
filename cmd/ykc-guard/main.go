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

	doc, err := loadClaims(*claims)
	if err != nil {
		fmt.Fprintln(os.Stderr, "載入聲明失敗:", err)
		os.Exit(1)
	}
	p := Project{Dir: absDir(*dir)}
	led, err := openLedger(p.Dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "帳本:", err)
		os.Exit(1)
	}
	defer led.Close()

	// 1) 比對
	var verdicts []Verdict
	for _, c := range doc.Claims {
		v := p.Verify(c)
		verdicts = append(verdicts, v)
		appendFact(led, "claim.verdict", "ykc-guard", map[string]any{
			"agent_id": doc.AgentID, "session_id": doc.SessionID,
			"claim_id": v.ClaimID, "text": v.Text, "feature": v.Feature,
			"verdict": v.Verdict, "evidence": v.Evidence, "severity": v.Severity,
		})
	}

	// 2) 棘輪
	from := currentTrust(readAll(p.Dir), doc.AgentID)
	to, events := ApplyAll(from, verdicts)
	for _, ev := range events {
		appendFact(led, "trust.event", "ykc-guard", map[string]any{
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
	p := Project{Dir: absDir(*dir)}
	led, err := openLedger(p.Dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "帳本:", err)
		os.Exit(1)
	}
	defer led.Close()
	appendFact(led, "trust.reset", "ykc-guard", map[string]any{
		"agent_id": *agent, "to": 3, "reason": *reason,
	})
	fmt.Printf("✅ 已重置代理 %s → T3 高度信任\n   理由: %s（已永久寫入帳本）\n", *agent, *reason)
}

func cmdMCP(args []string) {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	dir := fs.String("dir", ".", "專案目錄")
	_ = fs.Parse(args)
	serveMCP(absDir(*dir))
}
