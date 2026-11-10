package quest

import "testing"

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
	for _, id := range []string{"storm_dispatch", "storm_report"} {
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
