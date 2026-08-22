// Package smoke 是 YKC 的「命令執行核心」：所有煙測/接管驗證的命令
// （cargo、rustc、被測二進制）一律經此執行，輸出處理全專案唯一
// （internal/tail：全量 hash + 尾部保留）。
//
// 兩種模式：
//   - FailFast=true（預設）：前一條失敗即停——「基本健康不成立，後續無意義」
//     （ykc-atom smoke takeover 用）。
//   - FailFast=false：全部執行完——分層煙測收據要列齊每一層結果
//     （cmd/ykc-smoke 用）。
package smoke

import (
	"context"
	"errors"
	"os/exec"
	"time"

	"ykc/internal/domain"
	"ykc/internal/tail"
)

type CommandSpec struct {
	ID      string              `json:"id,omitempty"` // 穩定的檢查 id（收據用，如 "T0.version"）
	Class   domain.CommandClass `json:"class"`
	Name    string              `json:"name"`
	Args    []string            `json:"args,omitempty"`
	WorkDir string              `json:"work_dir,omitempty"`
	Timeout time.Duration       `json:"timeout,omitempty"`
}

type Report struct {
	At      time.Time              `json:"at"`
	Passed  bool                   `json:"passed"`
	Results []domain.CommandResult `json:"results"`
}

// ResultOf 依 spec.ID 取結果（無則零值 + false）。
func (r Report) ResultOf(id string) (domain.CommandResult, bool) {
	for _, res := range r.Results {
		if res.CommandID == id {
			return res, true
		}
	}
	return domain.CommandResult{}, false
}

type Runner struct {
	DefaultTimeout time.Duration
	MaxOutputBytes int
	FailFast       bool // true（預設）：首個失敗即停
}

func DefaultRustSmoke(root string) []CommandSpec {
	return []CommandSpec{
		{ID: "cargo-metadata", Class: domain.CommandClassMeta, Name: "cargo", Args: []string{"metadata", "--format-version=1"}, WorkDir: root, Timeout: 60 * time.Second},
		{ID: "cargo-check", Class: domain.CommandClassCheck, Name: "cargo", Args: []string{"check", "--workspace", "--all-targets", "--message-format=json"}, WorkDir: root, Timeout: 120 * time.Second},
		{ID: "cargo-test-no-run", Class: domain.CommandClassTest, Name: "cargo", Args: []string{"test", "--workspace", "--all-targets", "--no-run", "--message-format=json"}, WorkDir: root, Timeout: 180 * time.Second},
	}
}

func (r Runner) Run(ctx context.Context, specs []CommandSpec) (Report, error) {
	if len(specs) == 0 {
		return Report{}, errors.New("at least one smoke command is required")
	}
	if r.DefaultTimeout <= 0 {
		r.DefaultTimeout = 120 * time.Second
	}
	if r.MaxOutputBytes <= 0 {
		r.MaxOutputBytes = 64 * 1024
	}
	report := Report{At: time.Now().UTC(), Passed: true}
	for _, spec := range specs {
		res := r.runOne(ctx, spec)
		if !res.Succeeded() {
			report.Passed = false
		}
		report.Results = append(report.Results, res)
		// Smoke takeover is fail-fast: later commands depend on earlier basic health.
		if r.FailFast && !res.Succeeded() {
			break
		}
	}
	return report, nil
}

func (r Runner) runOne(parent context.Context, spec CommandSpec) domain.CommandResult {
	started := time.Now().UTC()
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = r.DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Dir = spec.WorkDir
	stdout := tail.NewBuffer(r.MaxOutputBytes)
	stderr := tail.NewBuffer(r.MaxOutputBytes)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	finished := time.Now().UTC()
	res := domain.CommandResult{
		CommandID:    spec.ID,
		Class:        spec.Class,
		Name:         spec.Name,
		Args:         spec.Args,
		WorkDir:      spec.WorkDir,
		StartedAt:    started,
		FinishedAt:   finished,
		ExitCode:     0,
		TimedOut:     ctx.Err() == context.DeadlineExceeded,
		StdoutSHA256: stdout.SumHex(),
		StderrSHA256: stderr.SumHex(),
		StdoutTail:   stdout.String(),
		StderrTail:   stderr.String(),
	}
	if err != nil {
		res.Err = err.Error()
		if exit, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exit.ExitCode()
		} else {
			res.ExitCode = -1
		}
	}
	return res
}
