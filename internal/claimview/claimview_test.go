package claimview

import (
	"encoding/json"
	"testing"

	"ykc/internal/domain"
	"ykc/internal/eventledger"
	"ykc/internal/ledger"
)

// legacyFact 模擬舊格式（扁平 payload）的帳本事實。
func legacyFact(t *testing.T, typ string, payload any) ledger.Fact {
	t.Helper()
	b, _ := json.Marshal(payload)
	return ledger.Fact{Type: typ, Payload: b}
}

// bridgeFact 模擬新格式（eventledger bridge 信封）的帳本事實。
func bridgeFact(t *testing.T, kind domain.EventKind, payload any) ledger.Fact {
	t.Helper()
	env, err := domain.NewEnvelope(kind, "sess-1", "/ws", payload)
	if err != nil {
		t.Fatal(err)
	}
	bp := eventledger.NewBridgePayload(env)
	b, _ := json.Marshal(bp)
	return ledger.Fact{Type: eventledger.FactType(kind), Payload: b}
}

func TestParseBothShapesClaimVerdict(t *testing.T) {
	payload := map[string]any{
		"agent_id": "a1", "claim_id": "c1", "text": "t", "feature": "compiles",
		"verdict": "contradicted", "evidence": "e", "severity": 3,
	}
	for name, f := range map[string]ledger.Fact{
		"legacy": legacyFact(t, KindClaimVerdict, payload),
		"bridge": bridgeFact(t, domain.EventClaimVerdict, payload),
	} {
		v, ok := Parse(f)
		if !ok {
			t.Fatalf("%s: parse failed", name)
		}
		if v.Kind != KindClaimVerdict || v.AgentID != "a1" || v.Verdict != "contradicted" || v.Severity != 3 || v.ClaimID != "c1" {
			t.Fatalf("%s: unexpected view %+v", name, v)
		}
	}
}

func TestParseBothShapesTrustEvent(t *testing.T) {
	payload := map[string]any{"agent_id": "a1", "kind": "欺騙", "intent": "存心", "from": 3, "to": 0, "action": "a"}
	for name, f := range map[string]ledger.Fact{
		"legacy": legacyFact(t, KindTrustEvent, payload),
		"bridge": bridgeFact(t, domain.EventTrustEvent, payload),
	} {
		v, ok := Parse(f)
		if !ok || v.Kind != KindTrustEvent || v.From != 3 || v.To != 0 {
			t.Fatalf("%s: unexpected %+v ok=%v", name, v, ok)
		}
	}
}

// TestTrustLevelMixedShapes 是 D4 的關鍵鎖：舊格式與 bridge 格式混合
// 出現時，等級重建仍以「帳本順序最後一條」為準。
func TestTrustLevelMixedShapes(t *testing.T) {
	facts := []ledger.Fact{
		legacyFact(t, KindTrustEvent, map[string]any{"agent_id": "a1", "from": 3, "to": 2}),
		bridgeFact(t, domain.EventTrustEvent, map[string]any{"agent_id": "a1", "from": 2, "to": 1}),
		bridgeFact(t, domain.EventTrustReset, map[string]any{"agent_id": "a1", "to": 3, "reason": "human"}),
		bridgeFact(t, domain.EventTrustEvent, map[string]any{"agent_id": "a2", "from": 3, "to": 0}), // 其他代理，不影響 a1
	}
	if got := TrustLevel(facts, "a1", 3); got != 3 {
		t.Fatalf("a1 should end at 3 (reset wins by order), got %d", got)
	}
	if got := TrustLevel(facts, "a2", 3); got != 0 {
		t.Fatalf("a2 should end at 0, got %d", got)
	}
	if got := TrustLevel(facts, "unknown", 3); got != 3 {
		t.Fatalf("no record → default 3, got %d", got)
	}
}

func TestParseIgnoresOtherFacts(t *testing.T) {
	if _, ok := Parse(legacyFact(t, "event.workspace.snapshot", map[string]any{})); ok {
		t.Fatal("non-trust fact must not parse")
	}
	if _, ok := Parse(ledger.Fact{Type: "claim.verdict", Payload: []byte(`{"agent_id":""}`)}); ok {
		t.Fatal("empty agent_id must not parse (untrackable)")
	}
}
