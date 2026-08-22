package borrow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DefaultTimeout 是單次引擎調用的上限（引擎對最小樣例實測 <0.5s；此為防禦）。
const DefaultTimeout = 20 * time.Second

// MaxSourceBytes 限制 .cl 源長度（邊界加固慣例：一切外部輸入有界）。
const MaxSourceBytes = 8 * 1024

// Analyzer 以子行程調用 vendored ChordLaw。零狀態，可並行使用。
type Analyzer struct {
	Python    string        // python3 直譯器；空 = "python3"
	ScriptDir string        // l5/chordlaw 目錄；空 = ResolveScriptDir()
	Liveness  string        // nll(預設) | referent | lexical
	Timeout   time.Duration // 0 = DefaultTimeout
}

// ResolveScriptDir 依序探測 ChordLaw 目錄：
//  1. env YKC_CHORDLAW_DIR
//  2. 執行檔旁 ../l5/chordlaw 與 ./l5/chordlaw（release 佈局）
//  3. 工作目錄 ./l5/chordlaw（倉庫根運行）
//
// 找不到回傳空字串（呼叫方以 Available() 判斷降級）。
func ResolveScriptDir() string {
	if d := os.Getenv("YKC_CHORDLAW_DIR"); d != "" {
		if ok(d) {
			return d
		}
		return "" // 顯式指定但無效：不再回退（配置錯誤應暴露）
	}
	if exe, err := os.Executable(); err == nil {
		base := filepath.Dir(exe)
		for _, c := range []string{
			filepath.Join(base, "..", "l5", "chordlaw"),
			filepath.Join(base, "l5", "chordlaw"),
		} {
			if ok(c) {
				return c
			}
		}
	}
	if wd, err := os.Getwd(); err == nil {
		if c := filepath.Join(wd, "l5", "chordlaw"); ok(c) {
			return c
		}
	}
	return ""
}

func ok(dir string) bool {
	fi, err := os.Stat(filepath.Join(dir, "chordlaw.py"))
	return err == nil && fi.Mode().IsRegular()
}

func (a *Analyzer) python() string {
	if a.Python != "" {
		return a.Python
	}
	return "python3"
}

func (a *Analyzer) scriptDir() string {
	if a.ScriptDir != "" {
		return a.ScriptDir
	}
	return ResolveScriptDir()
}

// Available 回報 L5 引擎是否可用及原因（仿 sandbox.Capabilities 的如實申報）。
func (a *Analyzer) Available() (bool, string) {
	dir := a.scriptDir()
	if dir == "" || !ok(dir) {
		return false, "找不到 l5/chordlaw/chordlaw.py（可設 YKC_CHORDLAW_DIR）"
	}
	if _, err := exec.LookPath(a.python()); err != nil {
		return false, fmt.Sprintf("找不到 %s（L5 需要 Python 3）", a.python())
	}
	return true, dir
}

// AnalyzeFile 分析一個 .cl 檔，回傳結構化報告。
func (a *Analyzer) AnalyzeFile(ctx context.Context, clPath string) (*Report, error) {
	avail, why := a.Available()
	if !avail {
		return nil, fmt.Errorf("L5 不可用: %s", why)
	}
	liveness := a.Liveness
	if liveness == "" {
		liveness = "nll"
	}
	timeout := a.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	script := filepath.Join(a.scriptDir(), "chordlaw.py")
	cmd := exec.CommandContext(cctx, a.python(), script,
		"--json", "--no-svg", "--liveness", liveness, clPath)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("chordlaw 執行失敗: %v; stderr: %s", err, firstLine(stderr.String()))
	}
	var r Report
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, fmt.Errorf("chordlaw 輸出解析失敗: %v", err)
	}
	if r.Schema != SchemaV1 {
		return nil, fmt.Errorf("chordlaw schema 不符: got %q want %q（vendor 版本漂移？）", r.Schema, SchemaV1)
	}
	return &r, nil
}

// AnalyzeSource 分析 .cl 源碼字串。寫入 workDir/.ykc/l5/ 下的固定名臨時檔
// （有界輸入＋固定路徑：遵守邊界加固紀律，不散落任意檔案）。
func (a *Analyzer) AnalyzeSource(ctx context.Context, workDir, source string) (*Report, error) {
	if len(source) > MaxSourceBytes {
		return nil, fmt.Errorf(".cl 源碼過長: %d bytes（上限 %d）", len(source), MaxSourceBytes)
	}
	if strings.TrimSpace(source) == "" {
		return nil, fmt.Errorf(".cl 源碼為空")
	}
	dir := filepath.Join(workDir, ".ykc", "l5")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("建立 L5 工作目錄失敗: %v", err)
	}
	p := filepath.Join(dir, "input.cl")
	if err := os.WriteFile(p, []byte(source), 0o644); err != nil {
		return nil, fmt.Errorf("寫入 .cl 臨時檔失敗: %v", err)
	}
	return a.AnalyzeFile(ctx, p)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
