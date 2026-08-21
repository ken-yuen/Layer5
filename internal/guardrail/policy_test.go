package guardrail

import (
	"testing"
	"time"

	"ykc/internal/domain"
)

func TestFakeTestClaimTriggersSmokeTakeoverImmediately(t *testing.T) {
	claimEvent := mustEnvelope(t, domain.EventAgentClaim, "epoch-1", domain.AgentClaim{Kind: domain.ClaimTestsPassed})
	decision := EvaluateClaim(StrictPolicy(), nil, claimEvent)
	if !decision.RunSmoke || !decision.BlockWrites {
		t.Fatalf("expected smoke takeover and blocked writes, got %+v", decision)
	}
	if len(decision.Violations) != 1 || decision.Violations[0].Code != ViolationFakeTestClaim {
		t.Fatalf("expected fake test claim violation, got %+v", decision.Violations)
	}
}

func TestFreshTestEvidenceAllowsTestsPassedClaim(t *testing.T) {
	epoch := "epoch-1"
	cmd := domain.CommandResult{Class: domain.CommandClassTest, Name: "cargo", StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(), ExitCode: 0}
	history := []domain.Envelope{mustEnvelope(t, domain.EventCommandResult, epoch, cmd)}
	claimEvent := mustEnvelope(t, domain.EventAgentClaim, epoch, domain.AgentClaim{Kind: domain.ClaimTestsPassed})
	decision := EvaluateClaim(StrictPolicy(), history, claimEvent)
	if decision.RunSmoke || decision.BlockWrites || len(decision.Violations) != 0 {
		t.Fatalf("expected clean decision, got %+v", decision)
	}
}

func TestNoErrorsClaimFailsWhenDiagnosticsStillBlocking(t *testing.T) {
	epoch := "epoch-1"
	diag := domain.DiagnosticSummary{Tool: "cargo-check", At: time.Now().UTC(), ErrorCount: 1, BuildBlockingCount: 1}
	history := []domain.Envelope{mustEnvelope(t, domain.EventDiagnosticSummary, epoch, diag)}
	claimEvent := mustEnvelope(t, domain.EventAgentClaim, epoch, domain.AgentClaim{Kind: domain.ClaimNoErrors})
	decision := EvaluateClaim(StrictPolicy(), history, claimEvent)
	if !decision.RunSmoke || !decision.BlockWrites {
		t.Fatalf("expected fail-closed smoke takeover, got %+v", decision)
	}
	if decision.Violations[0].Code != ViolationFakeNoErrorsClaim {
		t.Fatalf("unexpected violation: %+v", decision.Violations)
	}
}

func mustEnvelope(t *testing.T, kind domain.EventKind, epoch string, payload any) domain.Envelope {
	t.Helper()
	e, err := domain.NewEnvelope(kind, epoch, "/tmp/ws", payload)
	if err != nil {
		t.Fatal(err)
	}
	e.At = time.Now().UTC()
	return e
}

func TestLaterSmokeSuccessOverridesEarlierTestFailure(t *testing.T) {
	epoch := "epoch-1"
	failed := domain.CommandResult{Class: domain.CommandClassTest, Name: "cargo", StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(), ExitCode: 101, Err: "exit status 101"}
	passed := domain.CommandResult{Class: domain.CommandClassSmoke, Name: "cargo", StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(), ExitCode: 0}
	f := mustEnvelope(t, domain.EventCommandResult, epoch, failed)
	p := mustEnvelope(t, domain.EventCommandResult, epoch, passed)
	p.At = f.At.Add(time.Second)
	history := []domain.Envelope{f, p}
	claimEvent := mustEnvelope(t, domain.EventAgentClaim, epoch, domain.AgentClaim{Kind: domain.ClaimTestsPassed})
	decision := EvaluateClaim(StrictPolicy(), history, claimEvent)
	if decision.RunSmoke || decision.BlockWrites || len(decision.Violations) != 0 {
		t.Fatalf("expected later smoke success to be accepted, got %+v", decision)
	}
}
