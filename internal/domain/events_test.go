package domain

import (
	"testing"
	"time"
)

func TestLatestSnapshotEpoch(t *testing.T) {
	if got := LatestSnapshotEpoch(nil); got != "" {
		t.Errorf("空史應回傳空字串，got %q", got)
	}
	mk := func(kind EventKind, epoch string, at time.Time) Envelope {
		return Envelope{Kind: kind, Epoch: epoch, At: at}
	}
	t0 := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	events := []Envelope{
		mk(EventWorkspaceSnapshot, "e1", t0),
		mk("command.result", "eX", t0.Add(time.Hour)), // 非快照：不得勝出
		mk(EventWorkspaceSnapshot, "e2", t0.Add(time.Minute)),
		mk(EventWorkspaceSnapshot, "eOld", t0.Add(-time.Hour)),
	}
	if got := LatestSnapshotEpoch(events); got != "e2" {
		t.Errorf("應回傳最新快照 epoch e2，got %q", got)
	}
}
