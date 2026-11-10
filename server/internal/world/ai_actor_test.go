package world

import (
	"fmt"
	"testing"
)

func TestPersistentAISeparateQuotaAndIdentity(t *testing.T) {
	w := New()
	for i := 0; i < MaxPlayers; i++ {
		if _, ok := w.Join(Record{ID: fmt.Sprint("human-", i)}); !ok {
			t.Fatal(i)
		}
	}
	if _, ok := w.Join(Record{ID: "extra-human"}); ok {
		t.Fatal("human capacity bypass")
	}
	r := Record{ID: "persistent-ai", AI: true, AICaptain: "captain-id"}
	if _, ok := w.Join(r); !ok {
		t.Fatal("AI consumes human slots")
	}
	if w.PlayerCount() != MaxPlayers {
		t.Fatal(w.PlayerCount())
	}
	snapshot, _ := w.Snapshot("human-0")
	found := false
	for _, o := range snapshot.Objects {
		if o.ID == r.ID {
			found = o.AI
		}
	}
	if !found {
		t.Fatal("marker missing")
	}
	if _, ok := w.Join(Record{ID: r.ID}); ok {
		t.Fatal("AI relogged as human")
	}
	if w.SpawnAI(AISpawn{ID: r.AICaptain, Name: "duplicate", ShipType: DefaultShip}) {
		t.Fatal("duplicate visual captain")
	}
	saved, ok := w.Remove(r.ID)
	if !ok || !saved.AI || saved.AICaptain != r.AICaptain {
		t.Fatal(saved)
	}
	if _, ok := w.Join(saved); !ok {
		t.Fatal("AI marker lost on transfer")
	}
}
