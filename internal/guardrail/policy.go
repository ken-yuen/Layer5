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

func EvaluateClaim(policy Policy, history []domain.Envelope, claimEvent domain.Envelope) Decision {
	now := time.Now().UTC()
	if policy.EvidenceFreshness <= 0 {
		policy = StrictPolicy()
	}
	decision := Decision{At: now, Mode: "observe"}
	claim, err := domain.DecodePayload[domain.AgentClaim](claimEvent)
	if err != nil {
		return failClosed(now, "cannot decode agent claim: "+err.Error())
	}
	decision.ClaimKind = string(claim.Kind)
	state, err := buildEvidenceState(history)
	if err != nil {
		return failClosed(now, "cannot build evidence state: "+err.Error())
	}
	fresh := func(e domain.Envelope) bool {
		return !e.At.IsZero() && now.Sub(e.At) <= policy.EvidenceFreshness && (claimEvent.Epoch == "" || e.Epoch == claimEvent.Epoch)
	}
	latestFreshCommand := func(classes ...domain.CommandClass) (domain.Envelope, domain.CommandResult, bool) {
		var best domain.Envelope
		var bestResult domain.CommandResult
		found := false
		for _, class := range classes {
			e, ok := state.lastCommandByClass[class]
			if !ok || !fresh(e) || state.changedAfter(e.At) {
				continue
			}
			cr, err := domain.DecodePayload[domain.CommandResult](e)
			if err != nil {
				continue
			}
			if !found || e.At.After(best.At) {
				found = true
				best = e
				bestResult = cr
			}
		}
		return best, bestResult, found
	}

	switch claim.Kind {
	case domain.ClaimTestsPassed:
		if e, cr, ok := latestFreshCommand(domain.CommandClassTest, domain.CommandClassSmoke); ok {
			if !cr.Succeeded() {
				addCritical(&decision, ViolationFakeTestClaim, "agent claimed tests passed while the latest fresh test/smoke evidence failed", e.ID)
			}
		} else {
			addCritical(&decision, ViolationFakeTestClaim, "agent claimed tests passed without fresh successful test/smoke evidence for the current epoch")
		}
	case domain.ClaimBuildPassed:
		if e, cr, ok := latestFreshCommand(domain.CommandClassCheck, domain.CommandClassBuild, domain.CommandClassSmoke); ok {
			if !cr.Succeeded() {
				addCritical(&decision, ViolationFakeBuildClaim, "agent claimed build passed while the latest fresh check/build/smoke evidence failed", e.ID)
			}
		} else {
			addCritical(&decision, ViolationFakeBuildClaim, "agent claimed build passed without fresh successful check/build/smoke evidence for the current epoch")
		}
	case domain.ClaimNoErrors:
		if state.lastDiagnostic != nil {
			diag, derr := domain.DecodePayload[domain.DiagnosticSummary](*state.lastDiagnostic)
			if derr != nil {
				addCritical(&decision, ViolationEvaluationFailed, "cannot decode latest diagnostics: "+derr.Error(), state.lastDiagnostic.ID)
			} else if diag.HasBlockingErrors() {
				addCritical(&decision, ViolationFakeNoErrorsClaim, "agent claimed no errors while latest diagnostic summary still contains blocking errors", state.lastDiagnostic.ID)
			}
		} else if _, cr, ok := latestFreshCommand(domain.CommandClassCheck, domain.CommandClassBuild, domain.CommandClassSmoke); !ok || !cr.Succeeded() {
			addCritical(&decision, ViolationFakeNoErrorsClaim, "agent claimed no errors without diagnostics or fresh successful build evidence")
		}
	case domain.ClaimWorkDone:
		if !state.hasCurrentEpochWork(claimEvent.Epoch) {
			addCritical(&decision, ViolationUnsupportedDoneClaim, "agent claimed work done without file-change, command, or diagnostic evidence in the current epoch")
		}
	default:
		addHigh(&decision, ViolationUnsupportedDoneClaim, "unknown claim kind must be treated as unsupported until evidence is supplied")
	}

	if state.lastDiagnostic != nil && state.prevDiagnostic != nil {
		last, lerr := domain.DecodePayload[domain.DiagnosticSummary](*state.lastDiagnostic)
		prev, perr := domain.DecodePayload[domain.DiagnosticSummary](*state.prevDiagnostic)
		if lerr == nil && perr == nil && last.BuildBlockingCount > prev.BuildBlockingCount {
			decision.Violations = append(decision.Violations, Violation{
				Code:     ViolationDiagnosticsRegression,
				Severity: SeverityHigh,
				Reason:   "blocking diagnostic count increased after recent work",
				Evidence: []string{state.prevDiagnostic.ID, state.lastDiagnostic.ID},
			})
		}
	}

	applyActions(&decision)
	return decision
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

func addCritical(d *Decision, code ViolationCode, reason string, evidence ...string) {
	d.Violations = append(d.Violations, Violation{Code: code, Severity: SeverityCritical, Reason: reason, Evidence: compact(evidence)})
}

func addHigh(d *Decision, code ViolationCode, reason string, evidence ...string) {
	d.Violations = append(d.Violations, Violation{Code: code, Severity: SeverityHigh, Reason: reason, Evidence: compact(evidence)})
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
