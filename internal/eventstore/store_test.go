package eventstore

import (
	"testing"

	"ykc/internal/domain"
)

func TestStoreAppendReplayCompleteEvents(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e, err := domain.NewEnvelope(domain.EventAgentClaim, "epoch", "/tmp/ws", domain.AgentClaim{Kind: domain.ClaimWorkDone})
	if err != nil {
		t.Fatal(err)
	}
	appended, err := store.Append(e)
	if err != nil {
		t.Fatal(err)
	}
	if appended.ID == "" {
		t.Fatal("expected generated event id")
	}
	events, err := store.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != appended.ID {
		t.Fatalf("unexpected replay: %+v", events)
	}
}
