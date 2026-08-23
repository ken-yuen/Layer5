// Package sandbox provides YKC's controlled execution boundary for cargo/rustc.
// It deliberately separates backend selection from command execution so higher
// layers can fail closed when no real sandbox is available.
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ykc/internal/tail"
)

type Backend string

const (
	probeTimeout               = 2 * time.Second
	BackendAuto        Backend = "auto"
	BackendNative      Backend = "native"
	BackendBwrap       Backend = "bwrap"
	BackendDockerRunsc Backend = "docker-runsc"
	BackendPodmanRunsc Backend = "podman-runsc"
	BackendDocker      Backend = "docker"
	BackendPodman      Backend = "podman"
)

type NetworkMode string

const (
	NetworkNone NetworkMode = "none"
	NetworkHost NetworkMode = "host"
)

type TrustLevel string

const (
	TrustStrong   TrustLevel = "strong"
	TrustModerate TrustLevel = "moderate"
	TrustWeak     TrustLevel = "weak"
	TrustNone     TrustLevel = "none"
)

type Config struct {
	Backend     Backend
	ProjectDir  string
	Timeout     time.Duration
	Network     NetworkMode
	AllowNative bool
	Image       string
	Env         map[string]string
	MaxOutput   int
}

type Capability struct {
	Backend   Backend    `json:"backend"`
	Available bool       `json:"available"`
	Trust     TrustLevel `json:"trust"`
	Reason    string     `json:"reason,omitempty"`
}

type Result struct {
	Backend         Backend    `json:"backend"`
	Command         string     `json:"command"`
	Args            []string   `json:"args,omitempty"`
	WorkDir         string     `json:"work_dir,omitempty"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      time.Time  `json:"finished_at"`
	ExitCode        int        `json:"exit_code"`
	TimedOut        bool       `json:"timed_out"`
	StdoutSHA256    string     `json:"stdout_sha256,omitempty"`
	StderrSHA256    string     `json:"stderr_sha256,omitempty"`
	StdoutTail      string     `json:"stdout_tail,omitempty"`
	StderrTail      string     `json:"stderr_tail,omitempty"`
	StdoutTruncated bool       `json:"stdout_truncated,omitempty"`
	StderrTruncated bool       `json:"stderr_truncated,omitempty"`
	Err             string     `json:"err,omitempty"`
	Isolation       TrustLevel `json:"isolation"`
}

func (r Result) Succeeded() bool { return !r.TimedOut && r.ExitCode == 0 && r.Err == "" }

func NormalizeBackend(s string) Backend {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return BackendAuto
	}
	switch Backend(s) {
	case BackendAuto, BackendNative, BackendBwrap, BackendDockerRunsc, BackendPodmanRunsc, BackendDocker, BackendPodman:
		return Backend(s)
	default:
		return Backend(s)
	}
}

func Capabilities() []Capability {
	return []Capability{
		probeDockerRunsc(),
		probePodmanRunsc(),
		probeBwrap(),
		probeDocker(),
		probePodman(),
		{Backend: BackendNative, Available: true, Trust: TrustNone, Reason: "native execution is not a sandbox; only allowed when explicitly requested"},
	}
}

func Select(cfg Config) (Capability, []Capability) {
	caps := Capabilities()
	if cfg.Backend == "" {
		cfg.Backend = BackendAuto
	}
	if cfg.Backend != BackendAuto {
		for _, c := range caps {
			if c.Backend == cfg.Backend {
				if c.Backend == BackendNative && !cfg.AllowNative {
					c.Available = false
					c.Reason = "native backend requested but allow_native=false"
				}
				return c, caps
			}
		}
		return Capability{Backend: cfg.Backend, Available: false, Trust: TrustNone, Reason: "unknown sandbox backend"}, caps
	}
	for _, want := range []Backend{BackendDockerRunsc, BackendPodmanRunsc, BackendBwrap} {
		for _, c := range caps {
			if c.Backend == want && c.Available {
				return c, caps
			}
		}
	}
	if cfg.AllowNative {
		return Capability{Backend: BackendNative, Available: true, Trust: TrustNone, Reason: "falling back to native because allow_native=true"}, caps
	}
	return Capability{Backend: BackendAuto, Available: false, Trust: TrustNone, Reason: "no strong sandbox found; set up gVisor/runsc, bubblewrap, or explicitly allow native for trusted projects"}, caps
}

func Run(ctx context.Context, cfg Config, command string, args ...string) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Minute
	}
	if cfg.MaxOutput <= 0 {
		cfg.MaxOutput = 128 * 1024
	}
	if cfg.Network == "" {
		cfg.Network = NetworkNone
	}
	if cfg.Image == "" {
		cfg.Image = "rust:1.98"
	}
	abs, err := filepath.Abs(cfg.ProjectDir)
	if err != nil {
		return failedResult(cfg.Backend, command, args, err)
	}
	cfg.ProjectDir = abs
	cap, _ := Select(cfg)
	if !cap.Available {
		return failedResult(cap.Backend, command, args, errors.New(cap.Reason))
	}
	started := time.Now().UTC()
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	name, fullArgs, workdir, err := materializeCommand(cfg, cap.Backend, command, args)
	if err != nil {
		return failedResult(cap.Backend, command, args, err)
	}
	cmd := exec.CommandContext(ctx, name, fullArgs...)
	cmd.Dir = workdir
	cmd.Env = buildEnv(cfg)
	stdout := tail.NewBuffer(cfg.MaxOutput)
	stderr := tail.NewBuffer(cfg.MaxOutput)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	finished := time.Now().UTC()
	res := Result{
		Backend:         cap.Backend,
		Command:         command,
		Args:            args,
		WorkDir:         cfg.ProjectDir,
		StartedAt:       started,
		FinishedAt:      finished,
		ExitCode:        0,
		TimedOut:        ctx.Err() == context.DeadlineExceeded,
		StdoutSHA256:    stdout.SumHex(),
		StderrSHA256:    stderr.SumHex(),
		StdoutTail:      stdout.String(),
		StderrTail:      stderr.String(),
		StdoutTruncated: stdout.Truncated(),
		StderrTruncated: stderr.Truncated(),
		Isolation:       cap.Trust,
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

func materializeCommand(cfg Config, backend Backend, command string, args []string) (string, []string, string, error) {
	switch backend {
	case BackendNative:
		if !cfg.AllowNative {
			return "", nil, "", errors.New("native execution refused: allow_native=false")
		}
		return command, args, cfg.ProjectDir, nil
	case BackendDockerRunsc:
		name, allArgs, workdir := containerCommand("docker", true, cfg, command, args)
		return name, allArgs, workdir, nil
	case BackendPodmanRunsc:
		name, allArgs, workdir := containerCommand("podman", true, cfg, command, args)
		return name, allArgs, workdir, nil
	case BackendDocker:
		name, allArgs, workdir := containerCommand("docker", false, cfg, command, args)
		return name, allArgs, workdir, nil
	case BackendPodman:
		name, allArgs, workdir := containerCommand("podman", false, cfg, command, args)
		return name, allArgs, workdir, nil
	case BackendBwrap:
		return bwrapCommand(cfg, command, args)
	default:
		return "", nil, "", fmt.Errorf("unsupported backend %s", backend)
	}
}

func containerCommand(engine string, runsc bool, cfg Config, command string, args []string) (string, []string, string) {
	all := []string{"run", "--rm", "--workdir", "/work", "--volume", cfg.ProjectDir + ":/work:rw"}
	if user := containerUser(); user != "" {
		all = append(all, "--user", user)
	}
	if runsc {
		all = append(all, "--runtime=runsc")
	}
	if cfg.Network == NetworkNone {
		all = append(all, "--network=none")
	}
	for k, v := range cfg.Env {
		all = append(all, "-e", k+"="+v)
	}
	all = append(all, cfg.Image, command)
	all = append(all, args...)
	return engine, all, ""
}

func bwrapCommand(cfg Config, command string, args []string) (string, []string, string, error) {
	// Developer fallback. Production should prefer gVisor/Firecracker/Kata. We
	// keep enough host toolchain read-only for cargo/rustc to execute, but expose
	// only the project as read-write and drop network by default.
	all := []string{
		"--die-with-parent",
		"--unshare-pid",
		"--unshare-ipc",
		"--unshare-uts",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--bind", cfg.ProjectDir, "/work",
		"--chdir", "/work",
	}
	if cfg.Network == NetworkNone {
		all = append(all, "--unshare-net")
	}
	for _, p := range []string{"/usr", "/bin", "/lib", "/lib64", "/etc/ssl", "/etc/alternatives"} {
		if _, err := os.Stat(p); err == nil {
			all = append(all, "--ro-bind", p, p)
		}
	}
	// Bind local toolchains if YKC installed them outside /usr. CARGO_HOME is
	// mounted only to expose rustup proxy binaries on PATH; the sandboxed cargo
	// cache itself is supplied via cfg.Env and should remain project-local.
	if cargoHome := os.Getenv("CARGO_HOME"); cargoHome != "" {
		if _, err := os.Stat(cargoHome); err == nil {
			all = append(all, "--ro-bind", cargoHome, cargoHome)
		}
	}
	if rustupHome := os.Getenv("RUSTUP_HOME"); rustupHome != "" {
		if _, err := os.Stat(rustupHome); err == nil {
			all = append(all, "--ro-bind", rustupHome, rustupHome, "--setenv", "RUSTUP_HOME", rustupHome)
		}
	}
	if path := os.Getenv("PATH"); path != "" {
		all = append(all, "--setenv", "PATH", path)
	}
	all = append(all, "--", command)
	all = append(all, args...)
	return "bwrap", all, cfg.ProjectDir, nil
}

func buildEnv(cfg Config) []string {
	values := map[string]string{
		"CARGO_TERM_COLOR": "never",
		"RUST_BACKTRACE":   "0",
		"YKC_SANDBOX":      "1",
	}
	keep := []string{"PATH", "HOME", "CARGO_HOME", "RUSTUP_HOME", "SSL_CERT_FILE", "SSL_CERT_DIR"}
	if cfg.Network == NetworkHost {
		keep = append(keep, "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY")
	} else {
		values["CARGO_NET_OFFLINE"] = "true"
	}
	for _, k := range keep {
		if v := os.Getenv(k); v != "" {
			values[k] = v
		}
	}
	// 邊界加固：cfg.Env 的 key 必須是合法環境變數名——容器後端會把每個 key
	// 變成一個 `-e k=v` 參數，非法 key（含空白/`=`/前導 `-`）可被用來注入參數。
	for k, v := range cfg.Env {
		if !validEnvKey(k) {
			continue // 靜默丟棄非法 key（不影響安全：只是不被注入）
		}
		values[k] = v
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, k := range keys {
		env = append(env, k+"="+values[k])
	}
	return env
}

// validEnvKey：POSIX 環境變數名（字母/數字/底線，不以數字開頭）。
func validEnvKey(k string) bool {
	if k == "" {
		return false
	}
	for i, c := range k {
		switch {
		case c == '_':
		case c >= '0' && c <= '9':
			if i == 0 {
				return false
			}
		case (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z'):
		default:
			return false
		}
	}
	return true
}

func probeDockerRunsc() Capability {
	if _, err := exec.LookPath("docker"); err != nil {
		return Capability{Backend: BackendDockerRunsc, Available: false, Trust: TrustStrong, Reason: "docker not found"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "info", "--format", "{{json .Runtimes}}").CombinedOutput()
	if err != nil {
		return Capability{Backend: BackendDockerRunsc, Available: false, Trust: TrustStrong, Reason: "docker info failed: " + strings.TrimSpace(string(out))}
	}
	if !strings.Contains(string(out), "runsc") {
		return Capability{Backend: BackendDockerRunsc, Available: false, Trust: TrustStrong, Reason: "docker runtime runsc not configured"}
	}
	return Capability{Backend: BackendDockerRunsc, Available: true, Trust: TrustStrong, Reason: "docker runtime runsc available"}
}

func probePodmanRunsc() Capability {
	if _, err := exec.LookPath("podman"); err != nil {
		return Capability{Backend: BackendPodmanRunsc, Available: false, Trust: TrustStrong, Reason: "podman not found"}
	}
	if _, err := exec.LookPath("runsc"); err != nil {
		return Capability{Backend: BackendPodmanRunsc, Available: false, Trust: TrustStrong, Reason: "runsc not found"}
	}
	return Capability{Backend: BackendPodmanRunsc, Available: true, Trust: TrustStrong, Reason: "podman and runsc available; runtime must be configured"}
}

func probeBwrap() Capability {
	if _, err := exec.LookPath("bwrap"); err != nil {
		return Capability{Backend: BackendBwrap, Available: false, Trust: TrustModerate, Reason: "bwrap not found"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bwrap", "--ro-bind", "/usr", "/usr", "--", "/usr/bin/true")
	if err := cmd.Run(); err != nil {
		return Capability{Backend: BackendBwrap, Available: false, Trust: TrustModerate, Reason: "bwrap preflight failed: " + err.Error()}
	}
	return Capability{Backend: BackendBwrap, Available: true, Trust: TrustModerate, Reason: "bwrap preflight succeeded"}
}

func probeDocker() Capability {
	if _, err := exec.LookPath("docker"); err != nil {
		return Capability{Backend: BackendDocker, Available: false, Trust: TrustWeak, Reason: "docker not found"}
	}
	return Capability{Backend: BackendDocker, Available: true, Trust: TrustWeak, Reason: "plain container shares host kernel; not sufficient for untrusted code"}
}

func probePodman() Capability {
	if _, err := exec.LookPath("podman"); err != nil {
		return Capability{Backend: BackendPodman, Available: false, Trust: TrustWeak, Reason: "podman not found"}
	}
	return Capability{Backend: BackendPodman, Available: true, Trust: TrustWeak, Reason: "plain container shares host kernel; not sufficient for untrusted code"}
}

func failedResult(backend Backend, command string, args []string, err error) Result {
	now := time.Now().UTC()
	return Result{Backend: backend, Command: command, Args: args, StartedAt: now, FinishedAt: now, ExitCode: -1, Err: err.Error(), Isolation: TrustNone}
}
