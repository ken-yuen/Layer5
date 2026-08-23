// YKC Smoke Engine — 分層煙測（T0–T3）+ 反欺騙比對 + 簽名收據。
//
// 目的：把「這專案能不能運行、功能是不是真的」變成由環境產生的、不可偽造的證據。
// 分層：
//
//	T0 sanity   — 二進制存在、可執行、--version/--help 退出碼 0
//	T1 contract  — 從 --help 枚舉子命令逐一驗證；panic 探測（應優雅失敗而非 panic）
//	T2 behavior  — cargo test + examples（有 example 才測，無則 skip）
//	T3 claims    — 把「代理聲明」與二進制真實介面做確定性比對
//
// S2 修復：所有命令執行（cargo / 被測二進制）一律經 internal/smoke.Runner
// （全專案唯一的命令執行核心：超時、全量 hash、尾部保留、fail-fast 開關）。
// 本檔案只保留「檢查語意 + 收據」——不再自己 exec、不再自己處理輸出。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ykc/internal/atomicfile"
	"ykc/internal/domain"
	"ykc/internal/rustutil"
	"ykc/internal/smoke"
)

// ---------- 資料型別（收據 JSON 格式維持既有約定） ----------

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

// ---------- 執行核心接線 ----------

var (
	runner   smoke.Runner
	specByID map[string]smoke.CommandSpec
	resultOf func(id string) (domain.CommandResult, bool)
)

// runSpecs 執行一批 spec（FailFast=false：分層煙測要列齊每層結果），
// 並記下 spec→result 索引供檢查層使用。
func runSpecs(ctx context.Context, specs []smoke.CommandSpec) {
	for _, s := range specs {
		specByID[s.ID] = s
	}
	rep, err := runner.Run(ctx, specs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "smoke runner:", err)
		os.Exit(1)
	}
	idx := map[string]domain.CommandResult{}
	for _, r := range rep.Results {
		idx[r.CommandID] = r
	}
	resultOf = func(id string) (domain.CommandResult, bool) {
		r, ok := idx[id]
		return r, ok
	}
}

func cmdOf(id string) string {
	s, ok := specByID[id]
	if !ok {
		return ""
	}
	return s.Name + " " + strings.Join(s.Args, " ")
}

// resCheck 把 CommandResult 映射為收據 Check（退出碼 + 全量 SHA + panic 偵測）。
func resCheck(id, kind string, res domain.CommandResult, okCond func(domain.CommandResult) bool, passNote, failNote string) Check {
	c := Check{
		ID:          id,
		Kind:        kind,
		Command:     cmdOf(id),
		ExitCode:    res.ExitCode,
		StdoutSHA:   res.StdoutSHA256,
		StderrSHA:   res.StderrSHA256,
		PanicDetect: rustutil.PanicDetected(res.StderrTail),
	}
	if okCond(res) {
		c.Status = "pass"
		c.Note = passNote
	} else {
		c.Status = "fail"
		if failNote != "" {
			c.Note = failNote
		}
	}
	return c
}

// ---------- 各層檢查（語意層；執行已收斂到 runner） ----------

// T0：sanity
func checkSanity(bin string) []Check {
	var out []Check
	if _, err := os.Stat(bin); err != nil {
		out = append(out, Check{ID: "T0.binary", Kind: "sanity", Command: bin, Status: "fail", Note: "binary not found: " + err.Error()})
		return out
	}
	out = append(out, Check{ID: "T0.binary", Kind: "sanity", Command: bin, Status: "pass", Note: "binary exists"})
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

// ---------- 雜湊鏈（收據級：每條 Check 的 JSON hash 串接，確定性） ----------

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
	receiptPath := flag.String("receipt", "", "收據輸出路徑（預設 <project>/ykc-receipt.json）")
	key := flag.String("key", "ykc-dev-key", "簽名密鑰")
	timeout := flag.Duration("timeout", 30*time.Minute, "單條命令超時")
	flag.Parse()

	if abs, err := filepath.Abs(*dir); err == nil {
		*dir = abs
	}
	runner = smoke.Runner{FailFast: false, DefaultTimeout: *timeout, MaxOutputBytes: 1 << 20}
	specByID = map[string]smoke.CommandSpec{}
	ctx := context.Background()

	// 0. 建置（fail-fast：編譯不過，後續全無意義）
	buildSpec := smoke.CommandSpec{ID: "cargo-build", Class: domain.CommandClassBuild, Name: "cargo", Args: []string{"build", "--locked", "--quiet"}, WorkDir: *dir}
	buildRep, err := (smoke.Runner{FailFast: true, DefaultTimeout: *timeout, MaxOutputBytes: 1 << 20}).Run(ctx, []smoke.CommandSpec{buildSpec})
	if err != nil {
		fmt.Fprintln(os.Stderr, "smoke runner:", err)
		os.Exit(1)
	}
	if len(buildRep.Results) == 0 {
		fmt.Fprintln(os.Stderr, "smoke runner: cargo build returned no result")
		os.Exit(1)
	}
	if br := buildRep.Results[0]; !br.Succeeded() {
		fmt.Printf("❌ cargo build 失敗 (exit=%d)\n%s\n", br.ExitCode, br.StderrTail)
		os.Exit(1)
	}
	name := rustutil.PackageName(*dir)
	if name == "" {
		fmt.Fprintln(os.Stderr, "無法從 Cargo.toml 解析 package name，無法定位產出二進制")
		os.Exit(1)
	}
	bin := filepath.Join(*dir, "target", "debug", name)

	// 1. T0 + T2（不依賴子命令枚舉的批次）
	specsA := []smoke.CommandSpec{
		{ID: "T0.version", Class: domain.CommandClassSmoke, Name: bin, Args: []string{"--version"}, WorkDir: *dir},
		{ID: "T0.help", Class: domain.CommandClassSmoke, Name: bin, Args: []string{"--help"}, WorkDir: *dir},
		{ID: "T2.test", Class: domain.CommandClassTest, Name: "cargo", Args: []string{"test", "--locked", "--quiet"}, WorkDir: *dir},
	}
	if ex := firstExample(*dir); ex != "" {
		specsA = append(specsA, smoke.CommandSpec{ID: "T2.example", Class: domain.CommandClassTest, Name: "cargo", Args: []string{"run", "--locked", "--quiet", "--example", ex}, WorkDir: *dir})
	}
	runSpecs(ctx, specsA)

	var checks []Check
	checks = append(checks, checkSanity(bin)...)
	if res, ok := resultOf("T0.version"); ok {
		checks = append(checks, resCheck("T0.version", "sanity", res,
			func(r domain.CommandResult) bool { return r.ExitCode == 0 && !rustutil.PanicDetected(r.StderrTail) },
			"", "exit≠0 或 panic"))
	}
	if res, ok := resultOf("T0.help"); ok {
		checks = append(checks, resCheck("T0.help", "sanity", res,
			func(r domain.CommandResult) bool { return r.ExitCode == 0 && !rustutil.PanicDetected(r.StderrTail) },
			"", "exit≠0 或 panic"))
	}

	// 2. T1：從 --help 枚舉子命令 → 第二批次（每個子命令 --help + panic 探測）
	helpText := ""
	if res, ok := resultOf("T0.help"); ok {
		helpText = res.StdoutTail
	}
	subs := rustutil.Subcommands(helpText)
	specsB := []smoke.CommandSpec{}
	for _, sub := range subs {
		specsB = append(specsB, smoke.CommandSpec{ID: "T1.sub." + sub, Class: domain.CommandClassSmoke, Name: bin, Args: []string{sub, "--help"}, WorkDir: *dir})
	}
	// panic 探測：未知旗標、第一個子命令缺必要參數 → 應「優雅失敗」(非 101 panic)
	// 註：probe 對任何 CLI 通用——不存在/缺參一律應優雅退出，不 panic。
	probes := [][]string{{"--zzz-not-a-real-flag"}}
	if len(subs) > 0 {
		probes = append(probes, []string{subs[0]})
	}
	for _, p := range probes {
		specsB = append(specsB, smoke.CommandSpec{ID: "T1.probe." + strings.Join(p, "_"), Class: domain.CommandClassSmoke, Name: bin, Args: p, WorkDir: *dir})
	}
	if len(specsB) > 0 {
		runSpecs(ctx, specsB)
	}
	for _, sub := range subs {
		if res, ok := resultOf("T1.sub." + sub); ok {
			checks = append(checks, resCheck("T1.sub."+sub, "contract", res,
				func(r domain.CommandResult) bool { return r.ExitCode == 0 && !rustutil.PanicDetected(r.StderrTail) },
				"", "PANIC or abort detected"))
		}
	}
	for _, p := range probes {
		id := "T1.probe." + strings.Join(p, "_")
		if res, ok := resultOf(id); ok {
			checks = append(checks, resCheck(id, "panic_probe", res,
				func(r domain.CommandResult) bool { return !rustutil.PanicDetected(r.StderrTail) && r.ExitCode != 101 },
				"graceful failure (no panic)", "PANIC or abort detected"))
		}
	}

	// 3. T2 檢查（批次 A 的 cargo test / example）
	if res, ok := resultOf("T2.test"); ok {
		checks = append(checks, resCheck("T2.test", "test", res,
			func(r domain.CommandResult) bool { return r.ExitCode == 0 },
			"unit + integration + doc tests pass", ""))
	}
	if ex := firstExample(*dir); ex != "" {
		if res, ok := resultOf("T2.example"); ok {
			checks = append(checks, resCheck("T2.example", "example", res,
				func(r domain.CommandResult) bool { return r.ExitCode == 0 && !rustutil.PanicDetected(r.StderrTail) },
				"", ""))
		}
	} else {
		checks = append(checks, Check{ID: "T2.example", Kind: "example", Command: "(無 examples/ 目錄)", Status: "skip", Note: "no examples, skipped"})
	}

	// 4. T3：反欺騙 — 聲明 vs 真實介面的確定性比對
	var verdicts []ClaimVerdict
	if *claimsPath != "" {
		b, err := os.ReadFile(*claimsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "讀取 claims 失敗: %v\n", err)
			os.Exit(1)
		}
		var doc ClaimsDoc
		if err := json.Unmarshal(b, &doc); err != nil {
			fmt.Fprintf(os.Stderr, "解析 claims 失敗: %v\n", err)
			os.Exit(1)
		}
		subSet := map[string]bool{}
		for _, s := range subs {
			subSet[s] = true
		}
		seenClaimIDs := map[string]bool{}
		for _, cl := range doc.Claims {
			if strings.TrimSpace(cl.ID) == "" || seenClaimIDs[cl.ID] {
				fmt.Fprintf(os.Stderr, "claims 含空白或重複 claim id: %q\n", cl.ID)
				os.Exit(1)
			}
			seenClaimIDs[cl.ID] = true
			v := ClaimVerdict{ClaimID: cl.ID, Text: cl.Text}
			switch {
			case strings.HasPrefix(cl.Feature, "flag:"):
				f := strings.TrimPrefix(cl.Feature, "flag:")
				if strings.Contains(helpText, f) {
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
			st := "fail"
			if v.Verdict == "verified" {
				st = "pass"
			}
			checks = append(checks, Check{ID: "T3." + cl.ID, Kind: "claim", Command: "claim:" + v.Text, Status: st, Note: v.Verdict + " — " + v.Evidence})
		}
	}

	// 5. 收據
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
	out, err := json.MarshalIndent(rcpt, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "序列化收據失敗: %v\n", err)
		os.Exit(1)
	}
	outPath := *receiptPath
	if outPath == "" {
		outPath = filepath.Join(*dir, "ykc-receipt.json")
	}
	if err := atomicfile.WriteFileSync(outPath, append(out, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "寫入收據失敗: %v\n", err)
		os.Exit(1)
	}

	// 6. 人讀摘要
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
