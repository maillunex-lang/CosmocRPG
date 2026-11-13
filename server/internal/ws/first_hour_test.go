package ws

import (
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
)

func TestFirstHourPlan(t *testing.T) {
	v := quest.View{Quests: []quest.QuestView{{ID: "ch01_q01"}}}
	p := firstHourPlan(v)
	if p["current"] != 0 {
		t.Fatalf("expected briefing step, got %#v", p["current"])
	}
	v.Quests[0].ID = "first_ore"
	p = firstHourPlan(v)
	if p["current"] != 2 {
		t.Fatalf("expected mining step, got %#v", p["current"])
	}
	steps, ok := p["steps"].([]map[string]any)
	if !ok || len(steps) != 7 {
		t.Fatalf("unexpected first-hour steps: %#v", p["steps"])
	}
}
