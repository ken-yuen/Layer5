// Package precompile runs YKC's rustc/cargo pre-compilation pipeline and
// converts cargo JSON into actionable diagnostics.
package precompile

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ykc/internal/domain"
	"ykc/internal/sandbox"
)

type Status string

const (
	StatusPassed      Status = "passed"
	StatusFailed      Status = "failed"
	StatusSkipped     Status = "skipped"
	StatusUnsupported Status = "unsupported"
)

type Options struct {
	ProjectDir      string
	Backend         sandbox.Backend
	AllowNative     bool
	Timeout         time.Duration
	Fetch           bool
	Locked          bool
	LockedAuto      bool
	TestsNoRun      bool
	Clippy          bool
	Image           string
	NetworkForFetch bool
	StateDir        string
}

type Stage struct {
	Name       string             `json:"name"`
	Status     Status             `json:"status"`
	Command    []string           `json:"command,omitempty"`
	Sandbox    sandbox.Backend    `json:"sandbox,omitempty"`
	Isolation  sandbox.TrustLevel `json:"isolation,omitempty"`
	StartedAt  time.Time          `json:"started_at,omitempty"`
	FinishedAt time.Time          `json:"finished_at,omitempty"`
	ExitCode   int                `json:"exit_code,omitempty"`
	TimedOut   bool               `json:"timed_out,omitempty"`
	// Diagnostics 是正規化的 domain.DiagnosticSummary——與護欄/事件流同一 schema
	// （S3 修復：消除 precompile 自訂 DiagnosticSummary 與 domain 版的字段斷層）。
	Diagnostics     domain.DiagnosticSummary `json:"diagnostics,omitempty"`
	Messages        []Diagnostic             `json:"messages,omitempty"`
	RawJSONLines    int                      `json:"raw_json_lines,omitempty"`
	NonJSONLines    int                      `json:"non_json_lines,omitempty"`
	Notes           []string                 `json:"notes,omitempty"`
	StdoutTail      string                   `json:"stdout_tail,omitempty"`
	StderrTail      string                   `json:"stderr_tail,omitempty"`
	StdoutTruncated bool                     `json:"stdout_truncated,omitempty"`
	StderrTruncated bool                     `json:"stderr_truncated,omitempty"`
}

// ParseResult 是一次診斷解析的完整產出：正規化摘要（domain schema）+ 明細 + 統計。
type ParseResult struct {
	Summary      domain.DiagnosticSummary
	Messages     []Diagnostic
	RawJSONLines int
	NonJSONLines int
}

type Diagnostic struct {
	Tool     string `json:"tool,omitempty"`
	Level    string `json:"level,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Rendered string `json:"rendered,omitempty"`
}

type UnsupportedReason struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
	Action string `json:"action"`
}

type Report struct {
	ProjectDir   string               `json:"project_dir"`
	ManifestPath string               `json:"manifest_path,omitempty"`
	CreatedAt    time.Time            `json:"created_at"`
	Overall      Status               `json:"overall"`
	Sandbox      sandbox.Capability   `json:"sandbox"`
	Capabilities []sandbox.Capability `json:"capabilities,omitempty"`
	Unsupported  []UnsupportedReason  `json:"unsupported,omitempty"`
	Warnings     []string             `json:"warnings,omitempty"`
	Stages       []Stage              `json:"stages"`
}

func DefaultOptions(project string) Options {
	return Options{ProjectDir: project, Backend: sandbox.BackendAuto, Timeout: 3 * time.Minute, Fetch: true, LockedAuto: true, TestsNoRun: true, NetworkForFetch: true, StateDir: ".ykc"}
}

func Run(ctx context.Context, opt Options) (Report, error) {
	if opt.ProjectDir == "" {
		opt.ProjectDir = "."
	}
	if opt.Timeout <= 0 {
		opt.Timeout = 3 * time.Minute
	}
	abs, err := filepath.Abs(opt.ProjectDir)
	if err != nil {
		return Report{}, err
	}
	opt.ProjectDir = abs
	if err := prepareSandboxDirs(opt); err != nil {
		return Report{}, err
	}
	cap, caps := sandbox.Select(sandbox.Config{Backend: opt.Backend, ProjectDir: abs, AllowNative: opt.AllowNative, Image: opt.Image})
	rep := Report{ProjectDir: abs, CreatedAt: time.Now().UTC(), Overall: StatusPassed, Sandbox: cap, Capabilities: caps}
	manifest := filepath.Join(abs, "Cargo.toml")
	if _, err := os.Stat(manifest); err == nil {
		rep.ManifestPath = manifest
		if opt.LockedAuto {
			_, lockErr := os.Stat(filepath.Join(abs, "Cargo.lock"))
			opt.Locked = lockErr == nil
		}
		rep.Unsupported = append(rep.Unsupported, inspectCompileTimeRisks(abs, cap)...)
		if !cap.Available && !opt.AllowNative {
			rep.Overall = StatusUnsupported
			rep.Unsupported = append(rep.Unsupported, UnsupportedReason{Code: "sandbox_required", Reason: cap.Reason, Action: "install gVisor/runsc, bubblewrap, or rerun with -allow-native only for trusted projects"})
			return rep, nil
		}
		runCargoPlan(ctx, opt, &rep)
		finalize(&rep)
		return rep, nil
	}
	// No Cargo project: try single-file rustc metadata precheck.
	files, err := findRustFiles(abs)
	if err != nil {
		return rep, err
	}
	if len(files) == 0 {
		rep.Overall = StatusUnsupported
		rep.Unsupported = append(rep.Unsupported, UnsupportedReason{Code: "no_rust_entrypoint", Reason: "no Cargo.toml and no .rs file found", Action: "create a Cargo project or pass a directory containing Rust source"})
		return rep, nil
	}
	if !cap.Available && !opt.AllowNative {
		rep.Overall = StatusUnsupported
		rep.Unsupported = append(rep.Unsupported, UnsupportedReason{Code: "sandbox_required", Reason: cap.Reason, Action: "install sandbox backend or rerun with -allow-native for trusted single-file checks"})
		return rep, nil
	}
	runSingleRustc(ctx, opt, &rep, files[0])
	finalize(&rep)
	return rep, nil
}

func prepareSandboxDirs(opt Options) error {
	base := precompileBase(opt, false)
	for _, rel := range []string{"cargo-home", "target", "home"} {
		if err := os.MkdirAll(filepath.Join(base, rel), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func sandboxEnv(opt Options, backend sandbox.Backend) map[string]string {
	base := precompileBase(opt, backend != sandbox.BackendNative)
	return map[string]string{
		"CARGO_HOME":        filepath.Join(base, "cargo-home"),
		"CARGO_TARGET_DIR":  filepath.Join(base, "target"),
		"HOME":              filepath.Join(base, "home"),
		"CARGO_INCREMENTAL": "0",
	}
}

func precompileBase(opt Options, insideSandbox bool) string {
	state := opt.StateDir
	if state == "" {
		state = ".ykc"
	}
	if insideSandbox {
		if filepath.IsAbs(state) {
			return filepath.Join("/work", ".ykc", "precompile")
		}
		return filepath.Join("/work", state, "precompile")
	}
	if filepath.IsAbs(state) {
		return filepath.Join(state, "precompile")
	}
	return filepath.Join(opt.ProjectDir, state, "precompile")
}

func runCargoPlan(ctx context.Context, opt Options, rep *Report) {
	locked := []string{}
	if opt.Locked {
		locked = append(locked, "--locked")
	}
	runStage(ctx, opt, rep, "cargo-metadata", sandbox.NetworkHost, "cargo", append([]string{"metadata", "--format-version=1"}, locked...)...)
	if shouldStop(rep) {
		return
	}
	if opt.Fetch {
		net := sandbox.NetworkNone
		if opt.NetworkForFetch {
			net = sandbox.NetworkHost
		}
		runStage(ctx, opt, rep, "cargo-fetch", net, "cargo", append([]string{"fetch"}, locked...)...)
		if shouldStop(rep) {
			return
		}
	}
	checkArgs := []string{"check", "--workspace", "--all-targets", "--message-format=json"}
	checkArgs = append(checkArgs, locked...)
	runStage(ctx, opt, rep, "cargo-check-precompile", sandbox.NetworkNone, "cargo", checkArgs...)
	if opt.TestsNoRun {
		testArgs := []string{"test", "--workspace", "--all-targets", "--no-run", "--message-format=json"}
		testArgs = append(testArgs, locked...)
		runStage(ctx, opt, rep, "cargo-test-no-run", sandbox.NetworkNone, "cargo", testArgs...)
	}
	if opt.Clippy {
		clippyArgs := []string{"clippy", "--workspace", "--all-targets", "--message-format=json"}
		clippyArgs = append(clippyArgs, locked...)
		runStage(ctx, opt, rep, "cargo-clippy", sandbox.NetworkNone, "cargo", clippyArgs...)
	}
}

func runSingleRustc(ctx context.Context, opt Options, rep *Report, file string) {
	out := filepath.Join(os.TempDir(), "ykc-rustc-precompile.rmeta")
	rel, _ := filepath.Rel(opt.ProjectDir, file)
	runStage(ctx, opt, rep, "rustc-single-file-metadata", sandbox.NetworkNone, "rustc", "--edition=2021", "--error-format=json", "--emit=metadata", "-o", out, rel)
}

func runStage(ctx context.Context, opt Options, rep *Report, name string, network sandbox.NetworkMode, command string, args ...string) {
	cap, _ := sandbox.Select(sandbox.Config{Backend: opt.Backend, ProjectDir: opt.ProjectDir, AllowNative: opt.AllowNative, Image: opt.Image})
	cfg := sandbox.Config{Backend: opt.Backend, ProjectDir: opt.ProjectDir, Timeout: opt.Timeout, Network: network, AllowNative: opt.AllowNative, Image: opt.Image, MaxOutput: 512 * 1024, Env: sandboxEnv(opt, cap.Backend)}
	res := sandbox.Run(ctx, cfg, command, args...)
	stage := Stage{Name: name, Command: append([]string{command}, args...), Sandbox: res.Backend, Isolation: res.Isolation, StartedAt: res.StartedAt, FinishedAt: res.FinishedAt, ExitCode: res.ExitCode, TimedOut: res.TimedOut, StdoutTail: res.StdoutTail, StderrTail: res.StderrTail, StdoutTruncated: res.StdoutTruncated, StderrTruncated: res.StderrTruncated}
	parsed := ParseDiagnostics(res.StdoutTail + "\n" + res.StderrTail)
	stage.Diagnostics = parsed.Summary
	stage.Diagnostics.At = res.FinishedAt
	stage.Messages = parsed.Messages
	stage.RawJSONLines = parsed.RawJSONLines
	stage.NonJSONLines = parsed.NonJSONLines
	if res.Succeeded() && stage.Diagnostics.ErrorCount == 0 {
		stage.Status = StatusPassed
	} else {
		stage.Status = StatusFailed
		stage.Notes = append(stage.Notes, classifyFailure(res.StdoutTail+"\n"+res.StderrTail, res.Err)...)
		if res.Err != "" {
			stage.Notes = append(stage.Notes, res.Err)
		}
	}
	rep.Stages = append(rep.Stages, stage)
}

// ParseDiagnostics 解析 cargo/rustc 的 JSON 診斷流。
// 回傳的 Summary 是 domain.DiagnosticSummary——與護欄（guardrail.EvaluateClaim）
// 解碼 diagnostic.summary 事件所用的型別一致，接線時零轉換。
func ParseDiagnostics(text string) ParseResult {
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	var out ParseResult
	out.Summary.Tool = "rustc"
	h := sha256.New()
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "{") {
			out.NonJSONLines++
			continue
		}
		out.RawJSONLines++
		h.Write([]byte(line))
		if d, ok := parseOne(line); ok {
			if d.Level == "error" {
				out.Summary.ErrorCount++
			} else if d.Level == "warning" {
				out.Summary.WarningCount++
			}
			if d.Level == "error" || d.Level == "warning" {
				out.Messages = append(out.Messages, d)
			}
		}
	}
	// build-blocking = 所有 error 級診斷（rustc 語境）
	out.Summary.BuildBlockingCount = out.Summary.ErrorCount
	if out.RawJSONLines > 0 {
		out.Summary.Digest = hex.EncodeToString(h.Sum(nil))
	}
	return out
}

func parseOne(line string) (Diagnostic, bool) {
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		return Diagnostic{}, false
	}
	if reason, _ := m["reason"].(string); reason == "compiler-message" {
		if msg, ok := m["message"].(map[string]any); ok {
			return diagnosticFromMessage(msg), true
		}
		return Diagnostic{}, false
	}
	if _, ok := m["level"].(string); ok {
		return diagnosticFromMessage(m), true
	}
	return Diagnostic{}, false
}

func diagnosticFromMessage(msg map[string]any) Diagnostic {
	d := Diagnostic{Tool: "rustc"}
	d.Level, _ = msg["level"].(string)
	d.Message, _ = msg["message"].(string)
	d.Rendered, _ = msg["rendered"].(string)
	if code, ok := msg["code"].(map[string]any); ok {
		d.Code, _ = code["code"].(string)
	}
	if spans, ok := msg["spans"].([]any); ok {
		for _, raw := range spans {
			sp, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			primary, _ := sp["is_primary"].(bool)
			if !primary && d.File != "" {
				continue
			}
			d.File, _ = sp["file_name"].(string)
			d.Line = intNum(sp["line_start"])
			d.Column = intNum(sp["column_start"])
			if primary {
				break
			}
		}
	}
	return d
}

func intNum(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

func inspectCompileTimeRisks(dir string, cap sandbox.Capability) []UnsupportedReason {
	var out []UnsupportedReason
	if _, err := os.Stat(filepath.Join(dir, "build.rs")); err == nil {
		out = append(out, UnsupportedReason{Code: "build_script_executes", Reason: "project has build.rs; cargo check will compile and execute build script", Action: "compile only inside network-disabled sandbox; audit build.rs and generated OUT_DIR files"})
	}
	if hasText(filepath.Join(dir, "Cargo.toml"), "proc-macro = true") {
		out = append(out, UnsupportedReason{Code: "proc_macro_executes", Reason: "crate declares proc-macro; procedural macros execute during compilation", Action: "compile only inside sandbox and treat macro crates as supply-chain risk"})
	}
	if cap.Backend == sandbox.BackendNative || cap.Trust == sandbox.TrustNone {
		out = append(out, UnsupportedReason{Code: "native_not_isolated", Reason: "native cargo/rustc cannot block build.rs, proc-macro, filesystem or network side effects", Action: "use gVisor/runsc, Firecracker/Kata, or bubblewrap; native only for trusted local projects"})
	}
	return out
}

func classifyFailure(text, errText string) []string {
	joined := strings.ToLower(text + "\n" + errText)
	var notes []string
	add := func(code, action string) { notes = append(notes, code+": "+action) }
	switch {
	case strings.Contains(joined, "failed to run custom build command"):
		add("build_script_failed", "inspect build.rs output; ensure required native tools/libs exist inside sandbox")
	case strings.Contains(joined, "could not find `cargo.toml`"):
		add("manifest_missing", "run from Cargo project root or use single-file rustc mode")
	case strings.Contains(joined, "failed to get") && strings.Contains(joined, "as a dependency"):
		add("dependency_fetch_failed", "run cargo fetch with controlled network or vendor dependencies before offline compile")
	case strings.Contains(joined, "linker `cc` not found") || strings.Contains(joined, "no such file or directory") && strings.Contains(joined, "cc"):
		add("system_tool_missing", "install C toolchain in sandbox image or choose a pure-Rust dependency path")
	case strings.Contains(joined, "pkg-config") || strings.Contains(joined, "openssl"):
		add("native_dependency_missing", "install pkg-config/native dev libraries in sandbox image or switch to vendored/rustls feature")
	case strings.Contains(joined, "can't find crate"):
		add("crate_resolution_failed", "check target/features and dependency resolution")
	}
	return notes
}

func shouldStop(rep *Report) bool {
	if len(rep.Stages) == 0 {
		return false
	}
	last := rep.Stages[len(rep.Stages)-1]
	return last.Status == StatusFailed && (last.Name == "cargo-metadata" || last.Name == "cargo-fetch")
}

func finalize(rep *Report) {
	if rep.Overall == StatusUnsupported {
		return
	}
	rep.Overall = StatusPassed
	for _, st := range rep.Stages {
		if st.Status == StatusFailed {
			rep.Overall = StatusFailed
			return
		}
		if st.Status == StatusUnsupported {
			rep.Overall = StatusUnsupported
			return
		}
	}
}

func findRustFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "target" || d.Name() == ".git" || d.Name() == ".ykc" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".rs") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func hasText(path, needle string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(b), needle)
}

func WriteReport(path string, rep Report) error {
	if path == "" {
		return errors.New("report path is required")
	}
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
