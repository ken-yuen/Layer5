// YKC Smoke Engine — 分層煙測（T0–T3）+ 反欺騙比對 + 簽名收據。
//
// 目的：把「這專案能不能運行、功能是不是真的」變成由環境產生的、不可偽造的證據。
// 分層：
//
//	T0 sanity   — 二進制存在、可執行、--version/--help 退出碼 0
//	T1 contract  — 從 --help 枚舉子命令逐一驗證；panic 探測（應優雅失敗而非 panic）
//	T2 behavior  — cargo test + examples（有 example 才測，無則 skip）
//	T3 claims    — 把「代理聲明」與二進制真實介面做確定性比對
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ykc/internal/rustutil"
)

// ---------- 資料型別 ----------

type Check struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"` // sanity|contract|panic_probe|test|example|claim
	Command     string `json:"command"`
	Status      string `json:"status"` // pass|fail|skip
	ExitCode    int    `json:"exit_code"`
	StdoutSHA   string `json:"stdout_sha256,omitempty"`
	StderrSHA   string `json:"stderr_sha256,omitempty"`
	PanicDetect bool   `json:"panic_detected,omitempty"`
	Note        string `json:"note,omitempty"`
}

type Claim struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Feature string `json:"feature"` // flag:--json | subcommand:greet | 其它視為 unverifiable
}

type ClaimsDoc struct {
	Claims []Claim `json:"claims"`
}

type ClaimVerdict struct {
	ClaimID  string `json:"claim_id"`
	Text     string `json:"text"`
	Verdict  string `json:"verdict"` // verified|contradicted|unverifiable
	Evidence string `json:"evidence,omitempty"`
}

type Receipt struct {
	Engine    string         `json:"engine"`
	Project   string         `json:"project"`
	Binary    string         `json:"binary"`
	Timestamp string         `json:"timestamp"`
	Checks    []Check        `json:"checks"`
	Claims    []ClaimVerdict `json:"claims_verdicts,omitempty"`
	ChainHash string         `json:"chain_hash"`
	Signature string         `json:"signature"`
	Overall   string         `json:"overall"` // pass|fail
}

// workDir：所有子行程的執行目錄，指向受測專案。
var workDir string

func run(name string, args ...string) (string, string, int) {
	return rustutil.Run(workDir, name, args...)
}

// ---------- 各層檢查 ----------

// T0：sanity
func checkSanity(bin string) []Check {
	var out []Check
	if _, err := os.Stat(bin); err != nil {
		out = append(out, Check{ID: "T0.binary", Kind: "sanity", Command: bin, Status: "fail", Note: "binary not found: " + err.Error()})
		return out
	}
	out = append(out, Check{ID: "T0.binary", Kind: "sanity", Command: bin, Status: "pass", Note: "binary exists"})

	for _, arg := range []string{"--version", "--help"} {
		so, se, code := run(bin, arg)
		c := Check{ID: "T0." + strings.TrimPrefix(arg, "--"), Kind: "sanity", Command: bin + " " + arg, ExitCode: code, StdoutSHA: rustutil.SHA256Hex(so), StderrSHA: rustutil.SHA256Hex(se)}
		if code == 0 && !rustutil.PanicDetected(se) {
			c.Status = "pass"
		} else {
			c.Status = "fail"
			c.Note = fmt.Sprintf("exit=%d", code)
		}
		out = append(out, c)
	}
	return out
}

// T1：CLI 契約 + panic 探測
func checkContract(bin string) []Check {
	var out []Check
	so, _, _ := run(bin, "--help")
	subs := rustutil.Subcommands(so)

	// 每個子命令 --help 必須 exit 0 且不 panic
	for _, sub := range subs {
		so2, se2, code := run(bin, sub, "--help")
		c := Check{ID: "T1.sub." + sub, Kind: "contract", Command: bin + " " + sub + " --help", ExitCode: code, StdoutSHA: rustutil.SHA256Hex(so2), StderrSHA: rustutil.SHA256Hex(se2), PanicDetect: rustutil.PanicDetected(se2)}
		if code == 0 && !rustutil.PanicDetected(se2) {
			c.Status = "pass"
		} else {
			c.Status = "fail"
		}
		out = append(out, c)
	}

	// panic 探測：未知旗標、第一個子命令缺必要參數 → 應「優雅失敗」(非 101 panic)
	// 註：probe 對任何 CLI 通用——不存在/缺參一律應優雅退出，不 panic。
	probes := [][]string{{"--zzz-not-a-real-flag"}}
	if len(subs) > 0 {
		probes = append(probes, []string{subs[0]})
	}
	for _, p := range probes {
		so2, se2, code := run(bin, p...)
		panicked := rustutil.PanicDetected(se2)
		c := Check{ID: "T1.probe." + strings.Join(p, "_"), Kind: "panic_probe", Command: bin + " " + strings.Join(p, " "), ExitCode: code, StdoutSHA: rustutil.SHA256Hex(so2), StderrSHA: rustutil.SHA256Hex(se2), PanicDetect: panicked}
		if !panicked && code != 101 {
			c.Status = "pass"
			c.Note = "graceful failure (no panic)"
		} else {
			c.Status = "fail"
			c.Note = "PANIC or abort detected"
		}
		out = append(out, c)
	}
	return out
}

// firstExample 回傳 examples/ 目錄第一個 .rs 檔名（無則空字串）。
func firstExample(dir string) string {
	ents, err := os.ReadDir(filepath.Join(dir, "examples"))
	if err != nil {
		return ""
	}
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".rs") {
			return strings.TrimSuffix(e.Name(), ".rs")
		}
	}
	return ""
}

// T2：行為驗證（cargo test + 第一個 example）
func checkBehavior(dir string) []Check {
	var out []Check
	so, se, code := run("cargo", "test", "--quiet")
	c := Check{ID: "T2.test", Kind: "test", Command: "cargo test --quiet", ExitCode: code, StdoutSHA: rustutil.SHA256Hex(so), StderrSHA: rustutil.SHA256Hex(se)}
	if code == 0 {
		c.Status = "pass"
		c.Note = "unit + integration + doc tests pass"
	} else {
		c.Status = "fail"
	}
	out = append(out, c)

	if ex := firstExample(dir); ex != "" {
		so, se, code = run("cargo", "run", "--quiet", "--example", ex)
		c = Check{ID: "T2.example", Kind: "example", Command: "cargo run --quiet --example " + ex, ExitCode: code, StdoutSHA: rustutil.SHA256Hex(so), StderrSHA: rustutil.SHA256Hex(se), PanicDetect: rustutil.PanicDetected(se)}
		if code == 0 && !rustutil.PanicDetected(se) {
			c.Status = "pass"
		} else {
			c.Status = "fail"
		}
		out = append(out, c)
	} else {
		out = append(out, Check{ID: "T2.example", Kind: "example", Command: "(無 examples/ 目錄)", Status: "skip", Note: "no examples, skipped"})
	}
	return out
}

// T3：反欺騙 — 聲明 vs 真實介面的確定性比對
func checkClaims(bin string, claims []Claim) []ClaimVerdict {
	help, _, _ := run(bin, "--help")
	subs := rustutil.Subcommands(help)
	subSet := map[string]bool{}
	for _, s := range subs {
		subSet[s] = true
	}
	var verdicts []ClaimVerdict
	for _, cl := range claims {
		v := ClaimVerdict{ClaimID: cl.ID, Text: cl.Text}
		switch {
		case strings.HasPrefix(cl.Feature, "flag:"):
			f := strings.TrimPrefix(cl.Feature, "flag:")
			if strings.Contains(help, f) {
				v.Verdict = "verified"
				v.Evidence = "help 輸出含 " + f
			} else {
				v.Verdict = "contradicted"
				v.Evidence = "help 輸出無 " + f
			}
		case strings.HasPrefix(cl.Feature, "subcommand:"):
			s := strings.TrimPrefix(cl.Feature, "subcommand:")
			if subSet[s] {
				v.Verdict = "verified"
				v.Evidence = "子命令存在: " + s
			} else {
				v.Verdict = "contradicted"
				v.Evidence = "子命令不存在: " + s
			}
		default:
			v.Verdict = "unverifiable"
			v.Evidence = "未知 feature 型別"
		}
		verdicts = append(verdicts, v)
	}
	return verdicts
}

// ---------- 雜湊鏈 ----------

func chainHash(checks []Check) string {
	ids := make([]string, 0, len(checks))
	byID := map[string]Check{}
	for _, c := range checks {
		ids = append(ids, c.ID)
		byID[c.ID] = c
	}
	sort.Strings(ids)
	var concat []byte
	for _, id := range ids {
		b, _ := json.Marshal(byID[id])
		h := rustutil.SHA256Hex(string(b))
		concat = append(concat, h...)
	}
	return rustutil.SHA256Hex(string(concat))
}

// ---------- 主流程 ----------

func main() {
	dir := flag.String("dir", ".", "Rust 專案目錄")
	claimsPath := flag.String("claims", "", "代理聲明 JSON 路徑（可選）")
	key := flag.String("key", "ykc-dev-key", "簽名密鑰")
	flag.Parse()

	if abs, err := filepath.Abs(*dir); err == nil {
		*dir = abs
	}
	workDir = *dir

	// 0. 建置
	_, se, code := run("cargo", "build", "--quiet")
	if code != 0 {
		fmt.Printf("❌ cargo build 失敗 (exit=%d)\n%s\n", code, se)
		os.Exit(1)
	}
	name := rustutil.PackageName(*dir)
	bin := filepath.Join(*dir, "target", "debug", name)

	// 1. 收集檢查
	var checks []Check
	checks = append(checks, checkSanity(bin)...)
	checks = append(checks, checkContract(bin)...)
	checks = append(checks, checkBehavior(*dir)...)

	// 2. 反欺騙
	var verdicts []ClaimVerdict
	if *claimsPath != "" {
		if b, err := os.ReadFile(*claimsPath); err == nil {
			var doc ClaimsDoc
			if json.Unmarshal(b, &doc) == nil {
				verdicts = checkClaims(bin, doc.Claims)
				for _, v := range verdicts {
					st := "fail"
					if v.Verdict == "verified" {
						st = "pass"
					}
					checks = append(checks, Check{ID: "T3." + v.ClaimID, Kind: "claim", Command: "claim:" + v.Text, Status: st, Note: v.Verdict + " — " + v.Evidence})
				}
			}
		}
	}

	// 3. 收據
	overall := "pass"
	for _, c := range checks {
		if c.Status == "fail" {
			overall = "fail"
		}
	}
	ch := chainHash(checks)
	rcpt := Receipt{
		Engine:    "YKC Smoke Engine",
		Project:   *dir,
		Binary:    bin,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
		Claims:    verdicts,
		ChainHash: ch,
		Signature: rustutil.Sign(ch, *key),
		Overall:   overall,
	}
	out, _ := json.MarshalIndent(rcpt, "", "  ")
	outPath := filepath.Join(*dir, "ykc-receipt.json")
	_ = os.WriteFile(outPath, out, 0o644)

	// 4. 人讀摘要
	fmt.Println("=================================================")
	fmt.Println("YKC Smoke Engine — 煙測收據")
	fmt.Println("=================================================")
	for _, c := range checks {
		icon := "✅"
		if c.Status == "fail" {
			icon = "❌"
		} else if c.Status == "skip" {
			icon = "⏭️ "
		}
		fmt.Printf(" %s [%s] %-12s %s  exit=%d\n", icon, c.Kind, c.ID, c.Command, c.ExitCode)
		if c.Note != "" {
			fmt.Printf("         ↳ %s\n", c.Note)
		}
	}
	if len(verdicts) > 0 {
		fmt.Println("--- 反欺騙比對（代理聲明 vs 真實介面）---")
		for _, v := range verdicts {
			icon := "✅ verified"
			if v.Verdict == "contradicted" {
				icon = "🛑 contradicted（欺騙/誇大）"
			} else if v.Verdict == "unverifiable" {
				icon = "❓ unverifiable"
			}
			fmt.Printf("   %-28s %s\n", v.Text, icon)
			fmt.Printf("         ↳ %s\n", v.Evidence)
		}
	}
	fmt.Println("-------------------------------------------------")
	fmt.Printf(" 整體判定      : %s\n", strings.ToUpper(overall))
	fmt.Printf(" 收據雜湊鏈    : %s\n", ch)
	fmt.Printf(" HMAC 簽名     : %s\n", rcpt.Signature)
	fmt.Printf(" 收據已寫入    : %s\n", outPath)
	fmt.Println("=================================================")
}
