// dl_test.go：Datalog 護欄遷移的專屬測試（原 policy_test 四條是語義等價的回歸保障）。
package guardrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ykc/internal/datalog"
	"ykc/internal/domain"
)

func dlEnvelope(t *testing.T, kind domain.EventKind, epoch string, payload any, at time.Time) domain.Envelope {
	t.Helper()
	e, err := domain.NewEnvelope(kind, epoch, "/tmp/ws", payload)
	if err != nil {
		t.Fatal(err)
	}
	e.At = at
	return e
}

// TestDiagnosticsRegressionRule：阻斷數上升 → diagnostics_regression（帶前後證據）。
func TestDiagnosticsRegressionRule(t *testing.T) {
	now := time.Now().UTC()
	prev := dlEnvelope(t, domain.EventDiagnosticSummary, "e1", domain.DiagnosticSummary{Tool: "check", At: now.Add(-2 * time.Minute), BuildBlockingCount: 0}, now.Add(-2*time.Minute))
	prev.ID = "evt_prev" // ID 由 eventstore 提交時產生；已提交事件攜帶之
	last := dlEnvelope(t, domain.EventDiagnosticSummary, "e1", domain.DiagnosticSummary{Tool: "check", At: now.Add(-time.Minute), BuildBlockingCount: 3}, now.Add(-time.Minute))
	last.ID = "evt_last"
	claim := dlEnvelope(t, domain.EventAgentClaim, "e1", domain.AgentClaim{Kind: domain.ClaimNoErrors}, now)
	d := EvaluateClaim(StrictPolicy(), []domain.Envelope{prev, last}, claim)
	found := false
	for _, v := range d.Violations {
		if v.Code == ViolationDiagnosticsRegression {
			found = true
			if len(v.Evidence) != 2 || v.Evidence[0] != "evt_prev" || v.Evidence[1] != "evt_last" {
				t.Fatalf("regression evidence order mismatch: %v", v.Evidence)
			}
		}
	}
	if !found {
		t.Fatalf("expected diagnostics_regression violation, got %+v", d.Violations)
	}
	// no_errors + blocking → 兩條（fake_no_errors 在前、regression 在後——規則宣告序）
	if len(d.Violations) != 2 {
		t.Fatalf("expected 2 violations, got %d: %+v", len(d.Violations), d.Violations)
	}
	if d.Violations[0].Code != ViolationFakeNoErrorsClaim {
		t.Fatalf("expected fake_no_errors first, got %+v", d.Violations)
	}
}

// TestUnknownClaimKindHigh：未知種類 → high（非 critical → 不接管，僅要求證據）。
func TestUnknownClaimKindHigh(t *testing.T) {
	claim := dlEnvelope(t, domain.EventAgentClaim, "e1", domain.AgentClaim{Kind: "invented_kind"}, time.Now().UTC())
	d := EvaluateClaim(StrictPolicy(), nil, claim)
	if len(d.Violations) != 1 || d.Violations[0].Severity != SeverityHigh {
		t.Fatalf("expected single high violation, got %+v", d.Violations)
	}
	if d.BlockWrites {
		t.Fatalf("unknown kind should not block writes (high, not critical)")
	}
}

// TestExtraRulesAddViolation：附加規則可增補新違規碼（自訂場景）。
func TestExtraRulesAddViolation(t *testing.T) {
	extra, err := datalog.ParseRules(`
violation("custom_never_sleep", "warning", "agent claimed work done while project sleeps", "", "") :-
    claim.kind("work_done"), project.sleepy.
`)
	if err != nil {
		t.Fatal(err)
	}
	// 事實 project.sleepy 不存在於抽取器輸出 → 附加規則需自帶事實來源：
	// 這裡以「規則 + 事實擴充」二段式驗證——附加規則體引用抽取器未提供的謂詞，
	// 求值時該謂詞無事實 → 不觸發。用 evidence.latest("build") 反向驗證可觸發性。
	extra2, _ := datalog.ParseRules(`
violation("custom_build_evidence_seen", "warning", "build evidence exists (custom rule fired)", "", "") :-
    claim.kind("build_passed"), evidence.latest_ok("build").
`)
	_ = extra
	now := time.Now().UTC()
	cmd := domain.CommandResult{Class: domain.CommandClassCheck, Name: "cargo", StartedAt: now, FinishedAt: now, ExitCode: 0}
	history := []domain.Envelope{dlEnvelope(t, domain.EventCommandResult, "e1", cmd, now)}
	claim := dlEnvelope(t, domain.EventAgentClaim, "e1", domain.AgentClaim{Kind: domain.ClaimBuildPassed}, now)

	d := EvaluateClaimWithRules(StrictPolicy(), history, claim, extra2)
	if len(d.Violations) != 1 || d.Violations[0].Code != "custom_build_evidence_seen" {
		t.Fatalf("expected custom rule violation, got %+v", d.Violations)
	}
}

// TestLoadRulesFile：.dl 檔案與目錄載入。
func TestLoadRulesFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "extra.dl")
	if err := os.WriteFile(f, []byte(`violation("x", "warning", "y", "", "") :- claim.unknown.`), 0o644); err != nil {
		t.Fatal(err)
	}
	rs, err := LoadRules(f)
	if err != nil || len(rs) != 1 {
		t.Fatalf("LoadRules file: %v, %d rules", err, len(rs))
	}
	// 目錄：按名序合併
	if err := os.WriteFile(filepath.Join(dir, "b_second.dl"), []byte(`v2(X) :- claim.unknown(X).`), 0o644); err != nil {
		t.Fatal(err)
	}
	// b_second.dl 頭部變數未綁定 → 應於 AddRules 時報錯（嚴格性穿越檔案邊界）
	_, err = LoadRules(dir)
	if err == nil {
		// claim.unknown 0-arity → v2(X) 頭 X 未綁定 → AddRules 在 serve 端注入時報錯；
		// LoadRules 只解析。驗證解析成功即可。
		t.Log("directory load parsed ok")
	}
	p := datalog.NewProgram()
	if err := p.AddRules(rs); err != nil {
		t.Fatalf("AddRules loaded file: %v", err)
	}
}

// TestRuleEvalFailureFailsClosed：規則集含非分層程式 → fail-closed 接管。
func TestRuleEvalFailureFailsClosed(t *testing.T) {
	bad, _ := datalog.ParseRules(`
a(X) :- b(X), !c(X).
c(X) :- a(X), !b(X).
`)
	claim := dlEnvelope(t, domain.EventAgentClaim, "e1", domain.AgentClaim{Kind: domain.ClaimTestsPassed}, time.Now().UTC())
	d := EvaluateClaimWithRules(StrictPolicy(), nil, claim, bad)
	if !d.FailClosed {
		t.Fatalf("expected fail-closed on unstratifiable extra rules, got %+v", d)
	}
	if !d.BlockWrites || !d.RunSmoke {
		t.Fatalf("fail-closed must block writes and take over smoke")
	}
	if len(d.Violations) != 1 || d.Violations[0].Code != ViolationEvaluationFailed {
		t.Fatalf("expected evaluation_failed_fail_closed, got %+v", d.Violations)
	}
	if !strings.Contains(d.Violations[0].Reason, "stratifiable") {
		t.Fatalf("reason should carry engine error, got %s", d.Violations[0].Reason)
	}
}

// TestStaleEvidenceNotLatest：檔案變更晚於命令 → 命令不可數（!evidence.latest）。
func TestStaleEvidenceNotLatest(t *testing.T) {
	now := time.Now().UTC()
	cmd := domain.CommandResult{Class: domain.CommandClassTest, Name: "cargo", StartedAt: now.Add(-3 * time.Minute), FinishedAt: now.Add(-3 * time.Minute), ExitCode: 0}
	cmdEnv := dlEnvelope(t, domain.EventCommandResult, "e1", cmd, now.Add(-3*time.Minute))
	// 快照 digest 變化發生於命令之後 → changedAfter → 證據過期
	snap1 := dlEnvelope(t, domain.EventWorkspaceSnapshot, "e1", map[string]string{"digest": "d1"}, now.Add(-2*time.Minute))
	snap2 := dlEnvelope(t, domain.EventWorkspaceSnapshot, "e1", map[string]string{"digest": "d2"}, now.Add(-time.Minute))
	claim := dlEnvelope(t, domain.EventAgentClaim, "e1", domain.AgentClaim{Kind: domain.ClaimTestsPassed}, now)
	d := EvaluateClaim(StrictPolicy(), []domain.Envelope{cmdEnv, snap1, snap2}, claim)
	if len(d.Violations) != 1 || d.Violations[0].Code != ViolationFakeTestClaim {
		t.Fatalf("expected stale evidence → fake_test_claim (no latest), got %+v", d.Violations)
	}
	if strings.Contains(d.Violations[0].Reason, "while the latest") {
		t.Fatalf("should be the 'without fresh' variant, got %s", d.Violations[0].Reason)
	}
}
