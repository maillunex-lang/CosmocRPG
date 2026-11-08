package aistudio

import (
	"strings"
	"testing"
)

func engineWith(t *testing.T) (*Engine, *Config) {
	t.Helper()
	c := NewConfig()
	Generate(&c, GenerateOptions{Seed: 3, Count: 8})
	return NewEngine(&c), &c
}

func TestNormalizeRussian(t *testing.T) {
	cases := map[string]string{
		"  Привет,   КАПИТАН!!! ": "привет капитан",
		"Ёлка, ёж":                "елка еж",
		"что?! где... как":        "что где как",
		"":                        "",
		"123 abc":                 "123 abc",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, ожидалось %q", in, got, want)
		}
	}
}

func TestTyposSlangSynonyms(t *testing.T) {
	e, _ := engineWith(t)
	cases := []struct{ in, intent string }{
		{"превет", "greeting"}, {"ЗДАРОВА!", "greeting"}, {"спс", "thanks"}, {"спосибо", "thanks"},
		{"как включить атопилот", "ask_autopilot"}, {"расскажи про афтопилот", "ask_autopilot"},
		{"где станцыя рубеж", "ask_station"}, {"что за кантракт", "ask_contract"}, {"какие кридиты", "ask_currency"},
		{"пока", "farewell"}, {"бывай", "farewell"}, {"как стрелять", "ask_combat"},
	}
	for _, c := range cases {
		got, conf, _ := e.Match(e.Canonical(c.in))
		if got != c.intent || conf < threshold {
			t.Errorf("«%s»: ожидалось %s, получено %s (%.2f)", c.in, c.intent, got, conf)
		}
	}
}

func TestNeverInventsFacts(t *testing.T) {
	e, c := engineWith(t)
	conv := NewConversation("cpt-0001")
	// вопросы вне фактов не получают уверенного ответа
	for _, s := range []string{"какая награда за убийство босса", "сколько людей живёт на станции", "когда начнётся война", "расскажи про древний храм"} {
		turn := e.Reply(conv, s)
		if !turn.Clarify {
			t.Errorf("«%s»: ответ не уточняющий: %q (%s)", s, turn.Reply, turn.Intent)
		}
		if strings.ContainsAny(turn.Reply, "0123456789") {
			t.Errorf("«%s»: в ответе цифры: %q", s, turn.Reply)
		}
	}
	// шаблон с неизвестным фактом отбрасывается движком, а не выводится
	c2 := c.Clone()
	c2.Templates = append(c2.Templates, Template{ID: "x1", Intent: "ask_currency", Text: "Курс: {fact:нет_такого}", Weight: 10})
	c2.Templates = append(c2.Templates, Template{ID: "x2", Intent: "ask_currency", Text: "Курс {fact:zzz_unknown}", Weight: 10})
	e2 := NewEngine(&c2)
	conv2 := NewConversation("cpt-0001")
	turn := e2.Reply(conv2, "какая тут валюта")
	if strings.Contains(turn.Reply, "{") || strings.Contains(turn.Reply, "Курс") {
		t.Errorf("выведен текст с неизвестным фактом: %q", turn.Reply)
	}
	// ответ о бое опирается на факт
	conv3 := NewConversation("cpt-0002")
	if r := e.Reply(conv3, "как стрелять").Reply; !containsFold(r, []string{"не готов", "подождать", "не выйдет", "не вышло"}) && !strings.Contains(r, "пока") {
		t.Errorf("ответ о бое не опирается на факт: %q", r)
	}
}

func TestAmbiguityAsksToClarify(t *testing.T) {
	c := NewConfig()
	Generate(&c, GenerateOptions{Seed: 3, Count: 3})
	c.Intents = append(c.Intents,
		Intent{ID: "ask_a", Title: "первая тема", Examples: []string{"расскажи про орбиту", "что такое орбита"}, Keywords: []string{"орбита"}},
		Intent{ID: "ask_b", Title: "вторая тема", Examples: []string{"расскажи про орбиту", "что такое орбита"}, Keywords: []string{"орбита"}})
	e := NewEngine(&c)
	conv := NewConversation("cpt-0001")
	turn := e.Reply(conv, "орбита")
	if !turn.Clarify || !strings.Contains(turn.Reply, "первая тема") || !strings.Contains(turn.Reply, "вторая тема") {
		t.Errorf("неоднозначный вопрос должен давать уточнение с двумя вариантами: %q", turn.Reply)
	}
}

func TestStyleFollowsProfile(t *testing.T) {
	c := NewConfig()
	Generate(&c, GenerateOptions{Seed: 9, Count: 120})
	e := NewEngine(&c)
	sl := slangRe()
	checked := 0
	for id, cp := range c.Captains {
		if cp.Speech.Formality != "formal" {
			continue
		}
		conv := NewConversation(id)
		for _, s := range []string{"привет", "как дела", "чем занимаешься", "какие цели", "спасибо", "пока"} {
			if r := e.Reply(conv, s).Reply; sl.MatchString(r) {
				t.Errorf("%s (formal): сленг в ответе на «%s»: %q", id, s, r)
			}
		}
		checked++
	}
	if checked < 10 {
		t.Fatalf("проверено формальных капитанов: %d", checked)
	}
}

func TestDifferentCaptainsSoundDifferent(t *testing.T) {
	c := NewConfig()
	Generate(&c, GenerateOptions{Seed: 11, Count: 60})
	e := NewEngine(&c)
	seen := map[string]int{}
	for id := range c.Captains {
		conv := NewConversation(id)
		seen[e.Reply(conv, "чем занимаешься").Reply]++
	}
	if len(seen) < 25 {
		t.Errorf("на один вопрос 60 капитанов дали только %d разных ответов", len(seen))
	}
}

func TestGroupConversation(t *testing.T) {
	c := NewConfig()
	Generate(&c, GenerateOptions{Seed: 11, Count: 6})
	rep, err := SimulateGroup(&c, []string{"cpt-0001", "cpt-0002", "cpt-0003"}, "привет всем", 9)
	if err != nil || len(rep.Transcript) < 10 {
		t.Fatalf("групповой разговор: %v, реплик %d", err, len(rep.Transcript))
	}
	if len(rep.Issues) > 0 {
		t.Errorf("замечания: %v", rep.Issues)
	}
	if _, err := SimulateGroup(&c, []string{"cpt-0001"}, "x", 3); err == nil {
		t.Error("одного капитана недостаточно")
	}
	if _, err := SimulateGroup(&c, []string{"cpt-0001", "cpt-9999"}, "x", 3); err == nil {
		t.Error("неизвестный капитан")
	}
}

func TestSimulateTranscript(t *testing.T) {
	c := NewConfig()
	Generate(&c, GenerateOptions{Seed: 11, Count: 3})
	tr, err := Simulate(&c, "cpt-0002", []string{"привет", "меня зовут Алекс", "как меня зовут"})
	if err != nil || len(tr) != 6 {
		t.Fatalf("%v %d", err, len(tr))
	}
	if !strings.Contains(tr[5].Text, "Алекс") {
		t.Errorf("не запомнил имя: %q", tr[5].Text)
	}
	if _, err := Simulate(&c, "cpt-9999", []string{"привет"}); err == nil {
		t.Error("неизвестный капитан")
	}
}
