// Package guardrail 是行為驅動的動態護欄：聲明評估、宣告式 Datalog 規則與裁決。
package guardrail

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	"ykc/internal/domain"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type ViolationCode string

const (
	ViolationFakeTestClaim         ViolationCode = "fake_test_claim"
	ViolationFakeBuildClaim        ViolationCode = "fake_build_claim"
	ViolationFakeNoErrorsClaim     ViolationCode = "fake_no_errors_claim"
	ViolationUnsupportedDoneClaim  ViolationCode = "unsupported_done_claim"
	ViolationDiagnosticsRegression ViolationCode = "diagnostics_regression"
	ViolationStaleEvidence         ViolationCode = "stale_evidence"
	ViolationEvaluationFailed      ViolationCode = "evaluation_failed_fail_closed"
)

type ActionKind string

const (
	ActionWarn             ActionKind = "warn"
	ActionRequireEvidence  ActionKind = "require_evidence"
	ActionBlockAgentWrites ActionKind = "block_agent_writes"
	ActionSmokeTakeover    ActionKind = "smoke_takeover"
	ActionHumanReview      ActionKind = "human_review"
)

type Violation struct {
	Code     ViolationCode `json:"code"`
	Severity Severity      `json:"severity"`
	Reason   string        `json:"reason"`
	Evidence []string      `json:"evidence,omitempty"`
}

type Action struct {
	Kind   ActionKind `json:"kind"`
	Reason string     `json:"reason"`
}

type Decision struct {
	At          time.Time   `json:"at"`
	ClaimKind   string      `json:"claim_kind,omitempty"`
	Mode        string      `json:"mode"`
	Violations  []Violation `json:"violations,omitempty"`
	Actions     []Action    `json:"actions,omitempty"`
	BlockWrites bool        `json:"block_writes"`
	RunSmoke    bool        `json:"run_smoke"`
	FailClosed  bool        `json:"fail_closed"`
}

type Policy struct {
	// EvidenceFreshness is the maximum age of command evidence. Keep it short:
	// AI claims must be backed by current facts, not yesterday's green build.
	EvidenceFreshness time.Duration
}

func StrictPolicy() Policy {
	return Policy{EvidenceFreshness: 15 * time.Minute}
}

type evidenceState struct {
	lastSnapshotEpoch   string
	lastSnapshotDigest  string
	lastFileChangeAt    time.Time
	lastFileChangeEpoch string
	lastCommandByClass  map[domain.CommandClass]domain.Envelope
	lastSuccessByClass  map[domain.CommandClass]domain.Envelope
	lastFailureByClass  map[domain.CommandClass]domain.Envelope
	lastDiagnostic      *domain.Envelope
	prevDiagnostic      *domain.Envelope
}

// EvaluateClaim 以「預設規則集」評估聲明。
//
// 遷移聲明（YKC_14）：違規判定自 v0.0.3 起全部由宣告式 Datalog 規則產生
// （見 rules.go / dl.go）；本函數是唯一入口的薄包裝，不存在第二套判定實作。
// 需要附加用家規則時走 EvaluateClaimWithRules（ykc serve -rules）。
func EvaluateClaim(policy Policy, history []domain.Envelope, claimEvent domain.Envelope) Decision {
	return EvaluateClaimWithRules(policy, history, claimEvent, nil)
}

func buildEvidenceState(history []domain.Envelope) (evidenceState, error) {
	state := evidenceState{
		lastCommandByClass: make(map[domain.CommandClass]domain.Envelope),
		lastSuccessByClass: make(map[domain.CommandClass]domain.Envelope),
		lastFailureByClass: make(map[domain.CommandClass]domain.Envelope),
	}
	sorted := append([]domain.Envelope(nil), history...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].At.Before(sorted[j].At) })
	for _, e := range sorted {
		switch e.Kind {
		case domain.EventWorkspaceSnapshot:
			var snap struct {
				Digest string `json:"digest"`
			}
			if err := json.Unmarshal(e.Payload, &snap); err != nil {
				return state, err
			}
			if state.lastSnapshotDigest != "" && snap.Digest != "" && snap.Digest != state.lastSnapshotDigest {
				state.lastFileChangeAt = e.At
				state.lastFileChangeEpoch = e.Epoch
			}
			state.lastSnapshotEpoch = e.Epoch
			state.lastSnapshotDigest = snap.Digest
		case domain.EventCommandResult:
			cr, err := domain.DecodePayload[domain.CommandResult](e)
			if err != nil {
				return state, err
			}
			state.lastCommandByClass[cr.Class] = e
			if cr.Succeeded() {
				state.lastSuccessByClass[cr.Class] = e
			} else {
				state.lastFailureByClass[cr.Class] = e
			}
		case domain.EventDiagnosticSummary:
			if state.lastDiagnostic != nil {
				prev := *state.lastDiagnostic
				state.prevDiagnostic = &prev
			}
			cur := e
			state.lastDiagnostic = &cur
		}
	}
	return state, nil
}

func (s evidenceState) changedAfter(t time.Time) bool {
	return !s.lastFileChangeAt.IsZero() && s.lastFileChangeAt.After(t)
}

func (s evidenceState) hasCurrentEpochWork(epoch string) bool {
	if epoch == "" {
		return len(s.lastSuccessByClass) > 0 || s.lastDiagnostic != nil || !s.lastFileChangeAt.IsZero()
	}
	if s.lastFileChangeEpoch == epoch {
		return true
	}
	for _, e := range s.lastSuccessByClass {
		if e.Epoch == epoch {
			return true
		}
	}
	for _, e := range s.lastFailureByClass {
		if e.Epoch == epoch {
			return true
		}
	}
	return s.lastDiagnostic != nil && s.lastDiagnostic.Epoch == epoch
}

func applyActions(d *Decision) {
	if len(d.Violations) == 0 {
		d.Mode = "observe"
		return
	}
	max := SeverityInfo
	for _, v := range d.Violations {
		if severityRank(v.Severity) > severityRank(max) {
			max = v.Severity
		}
	}
	if severityRank(max) >= severityRank(SeverityCritical) {
		d.Mode = "evidence_only_smoke_takeover"
		d.BlockWrites = true
		d.RunSmoke = true
		d.Actions = append(d.Actions,
			Action{Kind: ActionSmokeTakeover, Reason: "critical behavior violation: YKC must verify the project, not the agent"},
			Action{Kind: ActionBlockAgentWrites, Reason: "freeze agent writes until smoke evidence is produced"},
			Action{Kind: ActionRequireEvidence, Reason: "future success claims require fresh command evidence in the same epoch"},
		)
		return
	}
	if severityRank(max) >= severityRank(SeverityHigh) {
		d.Mode = "evidence_required"
		d.Actions = append(d.Actions,
			Action{Kind: ActionRequireEvidence, Reason: "high-risk claim requires command evidence"},
			Action{Kind: ActionWarn, Reason: "guardrail threshold exceeded"},
		)
		return
	}
	d.Mode = "warn"
	d.Actions = append(d.Actions, Action{Kind: ActionWarn, Reason: "non-critical guardrail finding"})
}

func severityRank(s Severity) int {
	switch s {
	case SeverityInfo:
		return 0
	case SeverityWarning:
		return 1
	case SeverityHigh:
		return 2
	case SeverityCritical:
		return 3
	default:
		return -1
	}
}

func failClosed(now time.Time, reason string) Decision {
	d := Decision{
		At:          now,
		Mode:        "fail_closed_smoke_takeover",
		BlockWrites: true,
		RunSmoke:    true,
		FailClosed:  true,
		Violations:  []Violation{{Code: ViolationEvaluationFailed, Severity: SeverityCritical, Reason: reason}},
	}
	applyActions(&d)
	return d
}

func compact(in []string) []string {
	out := in[:0]
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func MarshalDecision(d Decision) ([]byte, error) {
	if d.At.IsZero() {
		return nil, errors.New("decision timestamp is required")
	}
	return json.MarshalIndent(d, "", "  ")
}
