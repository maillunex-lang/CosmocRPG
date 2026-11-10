package aiciv

import "testing"

func TestSimilarLinesAreBlocked(t *testing.T) {
	same := "На станции опять каша. Кто-нибудь ел?"
	punct := "на станции опять каша, кто-нибудь ел"
	if !tooClose(normLine(same), normLine(punct)) {
		t.Fatal("та же фраза с другой пунктуацией должна отсекаться")
	}
	other := "щит сел, я ухожу на край сектора"
	if tooClose(normLine(same), normLine(other)) {
		t.Fatal("разные реплики не должны считаться копией")
	}
}

func TestEachVoiceHasItsOwnWording(t *testing.T) {
	styles := []VoiceStyle{StyleShort, StyleWarm, StyleDry, StyleJoke, StyleTalk, StyleCare}
	for _, topic := range chainTopics() {
		turns := chains[topic]
		for step, turn := range turns {
			for i := 0; i < len(styles); i++ {
				for j := i + 1; j < len(styles); j++ {
					if tooClose(normLine(turn[i]), normLine(turn[j])) {
						t.Fatalf("%s шаг %d: манеры %s и %s слишком похожи\n%q\n%q", topic, step, styles[i], styles[j], turn[i], turn[j])
					}
				}
			}
			if step == 0 {
				continue
			}
			for i := 0; i < 6; i++ {
				for j := 0; j < 6; j++ {
					if tooClose(normLine(turns[step-1][i]), normLine(turn[j])) {
						t.Fatalf("%s ответ %d похож на прошлую реплику\n%q\n%q", topic, step, turns[step-1][i], turn[j])
					}
				}
			}
		}
	}
}

func TestChainFollowsThePreviousLine(t *testing.T) {
	a := banterLines("dock", 0, ArchPro, "alpha", 1)
	b := banterLines("dock", 1, ArchHelp, "beta", 2)
	if len(a) < 6 || len(b) < 6 {
		t.Fatalf("связка короткая: %d и %d", len(a), len(b))
	}
	if a[0] == b[0] || tooClose(normLine(a[0]), normLine(b[0])) {
		t.Fatal("следующая реплика повторяет первую")
	}
	if StyleOf("alpha") == StyleOf("alpha-other") && a[0] == banterLines("dock", 0, ArchPro, "alpha", 9)[0] {
		// same captain keeps the same first line
	}
	if banterLines("dock", 0, ArchPro, "alpha", 1)[0] != a[0] {
		t.Fatal("один капитан не должен менять свою первую формулировку")
	}
}
