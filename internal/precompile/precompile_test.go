package precompile

import (
	"testing"

	"ykc/internal/domain"
	"ykc/internal/guardrail"
)

func TestParseCargoCompilerMessage(t *testing.T) {
	input := `{"reason":"compiler-message","message":{"message":"cannot find value ` + "`x`" + ` in this scope","code":{"code":"E0425"},"level":"error","spans":[{"file_name":"src/main.rs","line_start":3,"column_start":5,"is_primary":true}],"rendered":"error[E0425]"}}`
	pr := ParseDiagnostics(input)
	if pr.Summary.ErrorCount != 1 || len(pr.Messages) != 1 {
		t.Fatalf("expected one error, got %+v", pr.Summary)
	}
	d := pr.Messages[0]
	if d.Code != "E0425" || d.File != "src/main.rs" || d.Line != 3 || d.Column != 5 {
		t.Fatalf("unexpected diagnostic: %+v", d)
	}
	if pr.Summary.BuildBlockingCount != 1 {
		t.Fatalf("blocking count must equal error count, got %+v", pr.Summary)
	}
}

func TestParseIgnoresNonJSONLines(t *testing.T) {
	pr := ParseDiagnostics("Compiling demo\n{\"level\":\"warning\",\"message\":\"careful\"}\n")
	if pr.NonJSONLines != 1 || pr.Summary.WarningCount != 1 {
		t.Fatalf("unexpected summary: %+v", pr)
	}
}

// TestDiagnosticsFeedGuardrailNoErrorsClaim 是 S3 的鎖死測試：
// precompile 的診斷 → domain schema → diagnostic.summary 事件 → 護欄判定。
// 若兩套 DiagnosticSummary 的字段再次走樣，此測試必紅（護欄 fail-open 前線防線）。
func TestDiagnosticsFeedGuardrailNoErrorsClaim(t *testing.T) {
	input := `{"reason":"compiler-message","message":{"message":"cannot find value ` + "`x`" + ` in this scope","code":{"code":"E0425"},"level":"error","spans":[{"file_name":"src/main.rs","line_start":3,"column_start":5,"is_primary":true}]}}`
	pr := ParseDiagnostics(input)

	diagEvent, err := domain.NewEnvelope(domain.EventDiagnosticSummary, "epoch-1", "/ws", pr.Summary)
	if err != nil {
		t.Fatal(err)
	}
	claimEvent, err := domain.NewEnvelope(domain.EventAgentClaim, "epoch-1", "/ws", domain.AgentClaim{Kind: domain.ClaimNoErrors})
	if err != nil {
		t.Fatal(err)
	}
	decision := guardrail.EvaluateClaim(guardrail.StrictPolicy(), []domain.Envelope{diagEvent}, claimEvent)

	found := false
	for _, v := range decision.Violations {
		if v.Code == guardrail.ViolationFakeNoErrorsClaim {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected fake_no_errors_claim violation, got %+v", decision.Violations)
	}
	if !decision.BlockWrites {
		t.Fatal("critical violation must block writes")
	}
}
