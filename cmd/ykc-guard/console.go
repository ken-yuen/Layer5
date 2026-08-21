// 用家控制台 v1（T-16）：紅黃綠摘要 + 證據報告（markdown）。
// 非技術用家只需看「等級 + 紅黃綠 + 證據」，無需懂 Rust。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type ConsoleData struct {
	AgentID   string
	SessionID string
	Project   string
	Level     TrustLevel
	Verdicts  []Verdict
	Events    []RatchetEvent
}

// overallOf：以「信任等級」為唯一權威（判決是歷史記錄、永久留帳本）。
func overallOf(d ConsoleData) string {
	switch d.Level {
	case T0:
		return "TAKEOVER"
	case T1:
		return "FAIL"
	case T2:
		return "WARN"
	default:
		return "PASS"
	}
}

func printConsole(d ConsoleData) {
	fmt.Println("===============================================")
	fmt.Println("YKC 用家控制台 — 可信裁判摘要")
	fmt.Println("===============================================")
	fmt.Printf(" 代理           : %s\n", d.AgentID)
	fmt.Printf(" 專案           : %s\n", d.Project)
	fmt.Printf(" 信任等級       : %s\n", d.Level)
	overall := overallOf(d)
	color := "🟢"
	switch overall {
	case "WARN":
		color = "🟡"
	case "FAIL":
		color = "🟠"
	case "TAKEOVER":
		color = "🔴"
	}
	fmt.Printf(" 整體判定       : %s %s\n", color, overall)

	fmt.Println("--- 聲明 vs 事實（歷史判決記錄，永久留帳本）---")
	for _, v := range d.Verdicts {
		switch v.Verdict {
		case "verified":
			fmt.Printf("   ✅ %s\n      ↳ %s\n", v.Text, v.Evidence)
		case "unverifiable":
			fmt.Printf("   ❓ %s\n      ↳ %s（不視為成立）\n", v.Text, v.Evidence)
		case "contradicted":
			fmt.Printf("   🛑 %s  [嚴重度%d:%s]\n      ↳ %s\n", v.Text, v.Severity, severityKind(v.Severity), v.Evidence)
		}
	}

	if len(d.Events) > 0 {
		fmt.Println("--- 信任棘輪（只降不升）---")
		for _, ev := range d.Events {
			fmt.Printf("   %s → %s   %s(%s)\n      ↳ %s\n", ev.From, ev.To, ev.Kind, ev.Intent, ev.Action)
		}
	}
	fmt.Println("------------------------------------------------")
	fmt.Println(" 人類操作：ykc-guard reset -agent X -reason \"...\"   ← 唯一回升入口")
	fmt.Println("===============================================")
}

// mdEscape 轉義 markdown 表格單元格中的 | 與換行，避免破壞表格。
func mdEscape(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// writeEvidenceReport 產出給非技術用家的證據報告。
func writeEvidenceReport(d ConsoleData, path string) error {
	var b strings.Builder
	b.WriteString("# YKC 可信裁判證據報告\n\n")
	b.WriteString("| 項目 | 值 |\n|---|---|\n")
	b.WriteString(fmt.Sprintf("| 代理 | %s |\n", mdEscape(d.AgentID)))
	b.WriteString(fmt.Sprintf("| 專案 | %s |\n", mdEscape(d.Project)))
	b.WriteString(fmt.Sprintf("| 信任等級 | %s |\n", d.Level))
	b.WriteString(fmt.Sprintf("| 整體判定 | %s |\n", overallOf(d)))
	b.WriteString("\n## 聲明 vs 事實\n\n")
	b.WriteString("| 聲明 | 判定 | 證據 |\n|---|---|---|\n")
	for _, v := range d.Verdicts {
		icon := "✅ verified"
		if v.Verdict == "contradicted" {
			icon = fmt.Sprintf("🛑 contradicted（%s 嚴重度%d）", severityKind(v.Severity), v.Severity)
		} else if v.Verdict == "unverifiable" {
			icon = "❓ unverifiable"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", mdEscape(v.Text), icon, mdEscape(v.Evidence)))
	}
	if len(d.Events) > 0 {
		b.WriteString("\n## 信任棘輪\n\n")
		for _, ev := range d.Events {
			b.WriteString(fmt.Sprintf("- %s → **%s**：%s（%s）— %s\n", ev.From, ev.To, ev.Kind, ev.Intent, mdEscape(ev.Action)))
		}
	}
	b.WriteString("\n## 人類可執行操作\n\n- 接管/重置代理：`ykc-guard reset -agent <id> -reason \"<理由>\"`（永久留審計）\n- 重跑驗證：`ykc-guard score -dir <專案> -claims <claims.json>`\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// readConsoleData：從帳本重建控制台資料（最近一次 score 的判決 + 現等級）。
func readConsoleData(dir, agentID string) ConsoleData {
	facts := readAll(dir)
	d := ConsoleData{AgentID: agentID, Project: dir, Level: currentTrust(facts, agentID)}
	for _, f := range facts {
		if f.Type == "claim.verdict" {
			var p struct {
				AgentID string `json:"agent_id"`
			}
			if json.Unmarshal(f.Payload, &p) != nil || p.AgentID != agentID {
				continue
			}
			var v struct {
				ClaimID  string `json:"claim_id"`
				Text     string `json:"text"`
				Feature  string `json:"feature"`
				Verdict  string `json:"verdict"`
				Evidence string `json:"evidence"`
				Severity int    `json:"severity"`
			}
			if json.Unmarshal(f.Payload, &v) == nil {
				d.Verdicts = append(d.Verdicts, Verdict{ClaimID: v.ClaimID, Text: v.Text, Feature: v.Feature, Verdict: v.Verdict, Evidence: v.Evidence, Severity: v.Severity})
			}
		}
	}
	return d
}
