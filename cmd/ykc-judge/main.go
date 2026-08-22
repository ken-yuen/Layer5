// YKC Judge — L4 除錯閉環（裁判核心）
//
// 模式：
//
//	judge（預設）：cargo check → 機械修復(cargo fix) → 再 check → 簽名收據 + 事實帳本
//	gate ：閘門（軌道 B）——只驗證(編譯+測試)、不修復，供 git pre-commit 呼叫，exit 0/1
//	verify：驗證帳本完整性（反竄改抽查）
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ykc/internal/ledger"
	"ykc/internal/rustutil"
)

type Receipt struct {
	Engine            string  `json:"engine"`
	Project           string  `json:"project"`
	Timestamp         string  `json:"timestamp"`
	ErrorsBefore      int     `json:"errors_before"`
	WarningsBefore    int     `json:"warnings_before"`
	ErrorsAfter       int     `json:"errors_after"`
	MechanicallyFixed []Error `json:"mechanically_fixed"`
	Remaining         []Error `json:"remaining"`
	ChainHash         string  `json:"chain_hash"`
	Signature         string  `json:"signature"`
	Overall           string  `json:"overall"`
}

func main() {
	dir := flag.String("dir", ".", "Rust 專案目錄")
	key := flag.String("key", "ykc-dev-key", "簽名密鑰")
	gate := flag.Bool("gate", false, "閘門模式：只驗證、不修復")
	verify := flag.Bool("verify", false, "驗證帳本完整性後退出")
	flag.Parse()
	if abs, err := filepath.Abs(*dir); err == nil {
		*dir = abs
	}
	// 邊界加固：專案目錄必須存在且為目錄
	if fi, err := os.Stat(*dir); err != nil || !fi.IsDir() {
		fmt.Fprintln(os.Stderr, "❌ 專案目錄不存在:", *dir)
		os.Exit(1)
	}

	switch {
	case *verify:
		runVerify(*dir)
	case *gate:
		runGate(*dir)
	default:
		runJudge(*dir, *key)
	}
}

// appendFact 追加事實；失敗時醒目告警（裁判完整性不應被靜默破壞）。
func appendFact(led *ledger.Ledger, typ, actor string, payload any) {
	if _, err := led.Append(typ, actor, payload); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ 帳本寫入失敗（%s）: %v\n", typ, err)
	}
}

// runJudge：除錯閉環。
func runJudge(dir, key string) {
	ledgerDir := filepath.Join(dir, ".ykc")
	_ = os.MkdirAll(ledgerDir, 0o755)
	// S4 修復：寫入前校驗全鏈（OpenVerified）——不在偽鏈/斷鏈之上繼續追加；
	// 併取得寫鎖（flock）強制單一寫者。
	led, err := ledger.OpenVerified(filepath.Join(ledgerDir, "ledger.jsonl"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "ledger:", err)
		os.Exit(1)
	}
	defer led.Close()

	appendFact(led, "judge.start", "ykc-judge", map[string]any{
		"project": dir, "time": time.Now().UTC().Format(time.RFC3339),
	})

	before, err := cargoCheck(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	appendFact(led, "crate.check.before", "ykc-judge", map[string]any{
		"errors": len(before.Errors), "warnings": before.Warnings, "fingerprints": fingerprints(before.Errors),
	})

	var fixed, remaining []Error
	if len(before.Errors) > 0 {
		if ferr := cargoFix(dir); ferr != nil {
			appendFact(led, "crate.fix", "ykc-judge", map[string]any{"ok": false, "note": ferr.Error()})
			remaining = before.Errors
		} else {
			after, aerr := cargoCheck(dir)
			if aerr != nil {
				appendFact(led, "crate.fix", "ykc-judge", map[string]any{"ok": false, "note": aerr.Error()})
				remaining = before.Errors
			} else {
				afterFP := map[string]bool{}
				for _, e := range after.Errors {
					afterFP[e.Fingerprint()] = true
				}
				for _, e := range before.Errors {
					if !afterFP[e.Fingerprint()] {
						fixed = append(fixed, e)
					} else {
						remaining = append(remaining, e)
					}
				}
				appendFact(led, "crate.fix", "ykc-judge", map[string]any{
					"ok": true, "fixed": len(fixed), "remaining": len(remaining),
				})
			}
		}
	}

	overall := "pass"
	if len(remaining) > 0 {
		overall = "fail"
	}
	chainHash := led.Head()
	rcpt := Receipt{
		Engine:            "YKC Judge (L4)",
		Project:           dir,
		Timestamp:         time.Now().UTC().Format(time.RFC3339),
		ErrorsBefore:      len(before.Errors),
		WarningsBefore:    before.Warnings,
		ErrorsAfter:       len(remaining),
		MechanicallyFixed: fixed,
		Remaining:         remaining,
		ChainHash:         chainHash,
		Signature:         rustutil.Sign(chainHash, key),
		Overall:           overall,
	}
	b, _ := json.MarshalIndent(rcpt, "", "  ")
	_ = os.WriteFile(filepath.Join(ledgerDir, "receipt.json"), b, 0o644)
	appendFact(led, "receipt.issue", "ykc-judge", map[string]any{"overall": overall, "chain_hash": chainHash})

	// ── 人讀摘要 ──
	fmt.Println("===============================================")
	fmt.Println("YKC Judge — L4 除錯閉環收據")
	fmt.Println("===============================================")
	fmt.Printf(" 修復前錯誤/警告 : %d / %d\n", len(before.Errors), before.Warnings)
	if len(fixed) > 0 {
		fmt.Println("--- ✅ 機械修復（rustc 建議自動套用）---")
		for _, e := range fixed {
			fmt.Printf("   %s %s (%s:%d)\n", e.Code, e.Message, filepath.Base(e.File), e.Line)
		}
	}
	if len(remaining) > 0 {
		fmt.Println("--- ⚠️ 剩餘語意錯誤（需 LLM/人類介入，附官方說明）---")
		for _, e := range remaining {
			fmt.Printf("   %s %s (%s:%d)\n", e.Code, e.Message, filepath.Base(e.File), e.Line)
			if ex := explain(dir, e.Code); ex != "" {
				fmt.Printf("      ↳ 官方說明: %s\n", ex)
			}
		}
		// L5: borrow 類錯誤 → 幾何解釋（規則卡+樣例拓撲; 解釋非判定）
		if l5 := l5Explain(dir, remaining, led); l5 != "" {
			fmt.Println("--- 📐 L5 借用幾何解釋（ChordLaw; 解釋非判定）---")
			for _, ln := range strings.Split(strings.TrimRight(l5, "\n"), "\n") {
				fmt.Println("   " + ln)
			}
		}
	}
	fmt.Println("------------------------------------------------")
	fmt.Printf(" 整體判定       : %s\n", strings.ToUpper(overall))
	fmt.Printf(" 收據雜湊鏈     : %s\n", chainHash)
	fmt.Printf(" HMAC 簽名      : %s\n", rcpt.Signature)
	fmt.Printf(" 帳本           : %s\n", filepath.Join(ledgerDir, "ledger.jsonl"))
	fmt.Printf(" 收據           : %s\n", filepath.Join(ledgerDir, "receipt.json"))
	fmt.Println("===============================================")
}

// runGate：閘門模式（軌道 B）——git pre-commit 呼叫。
func runGate(dir string) {
	res, err := cargoCheck(dir)
	if err != nil {
		fmt.Println("❌ YKC 閘門：無法執行 cargo check —", err)
		os.Exit(1)
	}
	if len(res.Errors) > 0 {
		fmt.Printf("❌ YKC 閘門攔截：編譯錯誤 %d 個\n", len(res.Errors))
		for _, e := range res.Errors {
			fmt.Printf("   %s %s (%s:%d)\n", e.Code, e.Message, filepath.Base(e.File), e.Line)
		}
		os.Exit(1)
	}
	_, se, code := rustutil.Run(dir, "cargo", "test", "--quiet")
	if code != 0 {
		fmt.Println("❌ YKC 閘門攔截：測試未通過")
		fmt.Println(se)
		os.Exit(1)
	}
	fmt.Println("✅ YKC 閘門通過：編譯 0 錯誤、測試全過")
}

// runVerify：驗證帳本完整性（反竄改抽查）。
func runVerify(dir string) {
	path := filepath.Join(dir, ".ykc", "ledger.jsonl")
	ok, head, err := ledger.VerifyChain(path)
	if err != nil {
		fmt.Println("❌ 讀取帳本失敗:", err)
		os.Exit(1)
	}
	if ok {
		fmt.Printf("✅ 帳本完整，鏈頭 = %s\n", head)
	} else {
		fmt.Println("🛑 帳本遭竄改！hash 鏈斷裂。")
		os.Exit(1)
	}
}
