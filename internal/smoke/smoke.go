package smoke

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"os/exec"
	"time"

	"ykc/internal/domain"
)

type CommandSpec struct {
	Class   domain.CommandClass `json:"class"`
	Name    string              `json:"name"`
	Args    []string            `json:"args,omitempty"`
	WorkDir string              `json:"work_dir,omitempty"`
	Timeout time.Duration       `json:"timeout"`
}

type Report struct {
	At      time.Time              `json:"at"`
	Passed  bool                   `json:"passed"`
	Results []domain.CommandResult `json:"results"`
}

type Runner struct {
	DefaultTimeout time.Duration
	MaxOutputBytes int
}

func DefaultRustSmoke(root string) []CommandSpec {
	return []CommandSpec{
		{Class: domain.CommandClassMeta, Name: "cargo", Args: []string{"metadata", "--format-version=1"}, WorkDir: root, Timeout: 60 * time.Second},
		{Class: domain.CommandClassCheck, Name: "cargo", Args: []string{"check", "--workspace", "--all-targets", "--message-format=json"}, WorkDir: root, Timeout: 120 * time.Second},
		{Class: domain.CommandClassTest, Name: "cargo", Args: []string{"test", "--workspace", "--all-targets", "--no-run", "--message-format=json"}, WorkDir: root, Timeout: 180 * time.Second},
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
		if !res.Succeeded() {
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
	stdout := newTailBuffer(r.MaxOutputBytes)
	stderr := newTailBuffer(r.MaxOutputBytes)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	finished := time.Now().UTC()
	res := domain.CommandResult{
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

type tailBuffer struct {
	Limit int
	total int64
	h     hash.Hash
	buf   []byte
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{Limit: limit, h: sha256.New()}
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	_, _ = b.h.Write(p)
	b.total += int64(len(p))
	if b.Limit <= 0 {
		b.buf = append(b.buf, p...)
		return len(p), nil
	}
	if len(p) >= b.Limit {
		b.buf = append(b.buf[:0], p[len(p)-b.Limit:]...)
		return len(p), nil
	}
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.Limit {
		copy(b.buf, b.buf[len(b.buf)-b.Limit:])
		b.buf = b.buf[:b.Limit]
	}
	return len(p), nil
}

func (b *tailBuffer) SumHex() string  { return hex.EncodeToString(b.h.Sum(nil)) }
func (b *tailBuffer) Truncated() bool { return b.Limit > 0 && b.total > int64(len(b.buf)) }
func (b *tailBuffer) String() string {
	if b.Truncated() {
		return fmt.Sprintf("...<truncated; kept last %d of %d bytes>\n%s", len(b.buf), b.total, string(b.buf))
	}
	return string(b.buf)
}
