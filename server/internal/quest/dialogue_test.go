package quest

import (
	"strings"
	"testing"
)

func TestMeridianDialogue(t *testing.T) {
	sc, ok := Dialogue("meridian_voice")
	if !ok {
		t.Fatal("нет сцены")
	}
	open := sc.Nodes[sc.Start]
	if open.Speaker != "Вора Кесс" || len(open.Choices) != 2 {
		t.Fatalf("открытие: %+v", open)
	}
	if open.Choices[0].Next != "who" || open.Choices[1].Next != "done" {
		t.Fatalf("переходы: %+v", open.Choices)
	}
	if _, ok := Dialogue("first_flight"); ok {
		t.Fatal("у полёта не должно быть диалога")
	}
	for _, id := range []string{"storm_dispatch", "storm_report", "ch01_q01", "ch01_q05", "ch01_q06", "ch01_q07"} {
		sc, ok := Dialogue(id)
		if !ok {
			t.Fatal("нет сцены", id)
		}
		seen := map[string]bool{}
		var walk func(string)
		walk = func(nodeID string) {
			if seen[nodeID] {
				return
			}
			seen[nodeID] = true
			node, ok := sc.Nodes[nodeID]
			if !ok || node.Speaker == "" || node.Text == "" || len(node.Choices) == 0 {
				t.Fatalf("%s узел %s пустой", id, nodeID)
			}
			for _, c := range node.Choices {
				if c.Next == "done" {
					continue
				}
				if _, ok := sc.Nodes[c.Next]; !ok {
					t.Fatalf("%s ответ %s ведёт в никуда", id, c.ID)
				}
				walk(c.Next)
			}
		}
		walk(sc.Start)
	}
}

func TestAuroraLoreBranchesKeepTheSameFacts(t *testing.T) {
	sc, ok := Dialogue("ch01_q05")
	if !ok {
		t.Fatal("нет сверки")
	}
	if !containsAll(sc.Nodes["names"].Text, "Даниэль Корвин", "не встреча") {
		t.Fatal(sc.Nodes["names"].Text)
	}
	if !containsAll(sc.Nodes["sponsors"].Text, "Конкордат", "Свободных Систем", "Гелиос", "Конклав Эхо", "Веспер") {
		t.Fatal(sc.Nodes["sponsors"].Text)
	}
	end, ok := Dialogue("ch01_q07")
	if !ok || !containsAll(end.Nodes["ship"].Text, "разные вещи", "не доказывает") {
		t.Fatal(end.Nodes["ship"].Text)
	}
	if !containsAll(end.Nodes["status"].Text, "пропал", "Не пиши", "удобные ответы") {
		t.Fatal(end.Nodes["status"].Text)
	}
}

func containsAll(text string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(text, p) {
			return false
		}
	}
	return true
}
