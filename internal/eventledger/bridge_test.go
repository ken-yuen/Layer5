package eventledger

import (
	"testing"

	"ykc/internal/domain"
	"ykc/internal/eventstore"
	"ykc/internal/ledger"
)

func TestAppendProjectsEventToLedger(t *testing.T) {
	b, err := Open(t.TempDir(), "test-actor")
	if err != nil {
		t.Fatal(err)
	}
	e, err := domain.NewEnvelope(domain.EventAgentClaim, "epoch-1", "/tmp/ws", domain.AgentClaim{Kind: domain.ClaimTestsPassed})
	if err != nil {
		t.Fatal(err)
	}
	res, err := b.Append(e)
	if err != nil {
		t.Fatal(err)
	}
	if res.Event.ID == "" || res.LedgerSeq == 0 || res.LedgerHash == "" {
		t.Fatalf("unexpected append result: %+v", res)
	}
	facts := ledger.ReadAll(b.LedgerPath)
	if len(facts) != 1 || !IsBridgeFact(facts[0]) {
		t.Fatalf("expected one bridge fact, got %+v", facts)
	}
	ok, _, err := b.VerifyLedger()
	if err != nil || !ok {
		t.Fatalf("ledger verify failed ok=%v err=%v", ok, err)
	}
}

func TestSyncMissingProjectsCommittedEvent(t *testing.T) {
	state := t.TempDir()
	es, err := eventstore.New(state + "/events")
	if err != nil {
		t.Fatal(err)
	}
	e, err := domain.NewEnvelope(domain.EventCommandResult, "epoch-1", "/tmp/ws", domain.CommandResult{Class: domain.CommandClassCheck, Name: "cargo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := es.Append(e); err != nil {
		t.Fatal(err)
	}
	b, err := Open(state, "sync-test")
	if err != nil {
		t.Fatal(err)
	}
	res, err := b.SyncMissing()
	if err != nil {
		t.Fatal(err)
	}
	if res.EventsSeen != 1 || res.Projected != 1 || res.AlreadyProjected != 0 {
		t.Fatalf("unexpected sync result: %+v", res)
	}
	res, err = b.SyncMissing()
	if err != nil {
		t.Fatal(err)
	}
	if res.Projected != 0 || res.AlreadyProjected != 1 {
		t.Fatalf("sync should be idempotent, got %+v", res)
	}
}
