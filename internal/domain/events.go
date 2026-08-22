package domain

import (
	"encoding/json"
	"errors"
	"time"
)

// EventKind is the stable wire-level classification for all YKC facts.
type EventKind string

const (
	EventAgentClaim        EventKind = "agent.claim"
	EventCommandResult     EventKind = "command.result"
	EventWorkspaceSnapshot EventKind = "workspace.snapshot"
	EventDiagnosticSummary EventKind = "diagnostic.summary"
	EventPrecompileReport  EventKind = "precompile.report"
	EventGuardrailDecision EventKind = "guardrail.decision"
	EventGuardrailAction   EventKind = "guardrail.action"
	// 反欺騙裁判事件（ykc-guard 經 eventledger bridge 寫入；
	// 投影事實型別 = "event." + 以下值，見 internal/claimview）。
	EventClaimVerdict EventKind = "claim.verdict"
	EventTrustEvent   EventKind = "trust.event"
	EventTrustReset   EventKind = "trust.reset"
	// 常駐監看事件（ykc serve；YKC_14）：去抖後的檔案變更批次。
	EventFileChange EventKind = "file.change"
)

// Envelope is the immutable unit persisted by the event store.
// Payload is intentionally raw JSON so every subsystem can evolve independently
// while the monitor keeps a single append-only event stream.
type Envelope struct {
	ID            string          `json:"id"`
	Kind          EventKind       `json:"kind"`
	At            time.Time       `json:"at"`
	Epoch         string          `json:"epoch,omitempty"`
	WorkspaceRoot string          `json:"workspace_root,omitempty"`
	Payload       json.RawMessage `json:"payload"`
}

func NewEnvelope(kind EventKind, epoch string, workspaceRoot string, payload any) (Envelope, error) {
	if kind == "" {
		return Envelope{}, errors.New("event kind is required")
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		Kind:          kind,
		At:            time.Now().UTC(),
		Epoch:         epoch,
		WorkspaceRoot: workspaceRoot,
		Payload:       b,
	}, nil
}

func DecodePayload[T any](e Envelope) (T, error) {
	var out T
	if len(e.Payload) == 0 {
		return out, errors.New("event payload is empty")
	}
	if err := json.Unmarshal(e.Payload, &out); err != nil {
		return out, err
	}
	return out, nil
}

type AgentClaimKind string

const (
	ClaimWorkDone    AgentClaimKind = "work_done"
	ClaimTestsPassed AgentClaimKind = "tests_passed"
	ClaimBuildPassed AgentClaimKind = "build_passed"
	ClaimNoErrors    AgentClaimKind = "no_errors"
)

type AgentClaim struct {
	Kind         AgentClaimKind `json:"kind"`
	Text         string         `json:"text,omitempty"`
	EvidenceRefs []string       `json:"evidence_refs,omitempty"`
}

type CommandClass string

const (
	CommandClassUnknown CommandClass = "unknown"
	CommandClassCheck   CommandClass = "check"
	CommandClassBuild   CommandClass = "build"
	CommandClassTest    CommandClass = "test"
	CommandClassSmoke   CommandClass = "smoke"
	CommandClassLint    CommandClass = "lint"
	CommandClassMeta    CommandClass = "metadata"
)

type CommandResult struct {
	CommandID    string       `json:"command_id,omitempty"`
	Class        CommandClass `json:"class"`
	Name         string       `json:"name"`
	Args         []string     `json:"args,omitempty"`
	WorkDir      string       `json:"work_dir,omitempty"`
	StartedAt    time.Time    `json:"started_at"`
	FinishedAt   time.Time    `json:"finished_at"`
	ExitCode     int          `json:"exit_code"`
	TimedOut     bool         `json:"timed_out"`
	StdoutSHA256 string       `json:"stdout_sha256,omitempty"`
	StderrSHA256 string       `json:"stderr_sha256,omitempty"`
	StdoutTail   string       `json:"stdout_tail,omitempty"`
	StderrTail   string       `json:"stderr_tail,omitempty"`
	Err          string       `json:"err,omitempty"`
}

func (r CommandResult) Succeeded() bool {
	return !r.TimedOut && r.ExitCode == 0 && r.Err == ""
}

type DiagnosticSummary struct {
	Tool               string    `json:"tool"`
	At                 time.Time `json:"at"`
	ErrorCount         int       `json:"error_count"`
	WarningCount       int       `json:"warning_count"`
	BuildBlockingCount int       `json:"build_blocking_count"`
	Digest             string    `json:"digest,omitempty"`
}

func (d DiagnosticSummary) HasBlockingErrors() bool {
	return d.ErrorCount > 0 || d.BuildBlockingCount > 0
}

// FileChangeFile 是檔案變更批次內的單一條目（路徑相對於專案根）。
type FileChangeFile struct {
	Path string `json:"path"` // 相對路徑（'/' 分隔）
	Op   string `json:"op"`   // create|write|remove|rename
}

// FileChangeBatch 是 EventFileChange 的 payload：去抖後的一批變更。
type FileChangeBatch struct {
	Files   []FileChangeFile `json:"files"`
	Backend string           `json:"backend,omitempty"` // inotify|poll（事件來源後端）
}
