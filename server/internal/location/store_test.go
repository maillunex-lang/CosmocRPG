package location

import (
	"context"
	"testing"
)

func TestJournalOnce(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	v := Visit{Player: "p", System: "meridian", ID: "an-1", Name: "Гравитационная аномалия", Kind: "anomaly"}
	if err := s.Record(ctx, v); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(ctx, v); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(ctx, Visit{System: "meridian", ID: "an-1"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.List(ctx, "p")
	if err != nil || len(got) != 1 || got[0].Kind != "anomaly" || got[0].Name == "" {
		t.Fatalf("журнал: %+v %v", got, err)
	}
	other, err := s.List(ctx, "q")
	if err != nil || len(other) != 0 {
		t.Fatalf("чужой журнал: %+v %v", other, err)
	}
}
