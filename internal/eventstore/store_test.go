package eventstore

import (
	"testing"

	"ykc/internal/domain"
)

func TestAppendIsIdempotentForSameEnvelope(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e, err := domain.NewEnvelope(domain.EventCommandResult, "epoch", "/tmp/project", map[string]string{"name": "cargo"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Append(e)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Append(e)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.Kind != second.Kind {
		t.Fatalf("idempotent append changed envelope: first=%+v second=%+v", first, second)
	}
}
