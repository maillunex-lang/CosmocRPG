package aistudio

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func gen300(t *testing.T) Config {
	t.Helper()
	c := NewConfig()
	res := Generate(&c, GenerateOptions{Seed: 20261108, Count: 300})
	if len(res.Added) != 300 || len(c.Captains) != 300 {
		t.Fatalf("создано %d, в реестре %d", len(res.Added), len(c.Captains))
	}
	return c
}

func TestGenerated300PassSchemaAndValidation(t *testing.T) {
	c := gen300(t)
	bad := 0
	for id, cp := range c.Captains {
		b, _ := json.Marshal(cp)
		if msgs := ValidateJSON("captain", b); len(msgs) > 0 {
			bad++
			if bad <= 3 {
				t.Errorf("%s не проходит схему: %v", id, msgs)
			}
		}
	}
	for _, is := range Validate(c) {
		if is.Level == "error" {
			t.Errorf("ошибка согласованности: %+v", is)
		}
	}
	// базовые разделы тоже должны проходить схемы
	for kind, v := range map[string]any{
		"intents": map[string]any{"intents": c.Intents}, "templates": map[string]any{"templates": c.Templates},
		"facts": map[string]any{"facts": c.Facts}, "factions": map[string]any{"factions": c.Factions},
		"corporations": map[string]any{"corporations": c.Corporations}, "strategies": map[string]any{"strategies": c.Strategies},
		"relations": map[string]any{"relations": c.Relations}, "rules": map[string]any{"rules": c.Rules},
		"emotions": map[string]any{"emotions": c.Emotions}, "memory_rules": map[string]any{"rules": c.MemoryRules},
		"lessons": map[string]any{"lessons": c.Lessons}, "schedules": map[string]any{"schedules": c.Schedules},
		"dialogue_tests": map[string]any{"tests": c.Tests},
		"synonyms":       map[string]any{"synonyms": c.Synonyms.Synonyms, "slang": c.Synonyms.Slang, "typos": c.Synonyms.Typos},
	} {
		b, _ := json.Marshal(v)
		if msgs := ValidateJSON(kind, b); len(msgs) > 0 {
			t.Errorf("раздел %s не проходит схему: %v", kind, msgs)
		}
	}
}

func TestUniquenessOf300(t *testing.T) {
	c := gen300(t)
	names, bios, sigs, callsigns := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]int{}
	profs, facs := map[string]int{}, map[string]int{}
	forms := map[string]int{}
	for _, cp := range c.Captains {
		if names[cp.Name] {
			t.Errorf("повтор имени %s", cp.Name)
		}
		names[cp.Name] = true
		if bios[cp.Bio] {
			t.Errorf("повтор биографии %s", cp.ID)
		}
		bios[cp.Bio] = true
		if sigs[cp.Signature()] {
			t.Errorf("повтор характера %s", cp.ID)
		}
		sigs[cp.Signature()] = true
		if cp.CallSign != "" {
			callsigns[cp.CallSign]++
		}
		profs[cp.Profession]++
		facs[cp.Faction]++
		forms[cp.Speech.Formality]++
		if len([]rune(cp.Bio)) < 150 {
			t.Errorf("слишком короткая биография %s: %q", cp.ID, cp.Bio)
		}
		if strings.Contains(cp.Bio, "⟨") || strings.Contains(cp.Bio, "(а)") || strings.Contains(cp.Bio, "(ла)") || strings.Contains(cp.Bio, "[[") {
			t.Errorf("не раскрыты подстановки в %s: %q", cp.ID, cp.Bio)
		}
		for intent, rs := range cp.Replies {
			for _, r := range rs {
				if strings.Contains(r, "[[") || strings.Contains(r, "⟨") {
					t.Errorf("не раскрыта подстановка в ответе %s/%s: %q", cp.ID, intent, r)
				}
			}
		}
	}
	for cs, n := range callsigns {
		if n > 1 {
			t.Errorf("позывной «%s» у %d капитанов", cs, n)
		}
	}
	if len(profs) < 18 {
		t.Errorf("профессий всего %d, ожидалось не меньше 18", len(profs))
	}
	if len(facs) < 6 {
		t.Errorf("задействовано фракций %d", len(facs))
	}
	if forms["formal"] < 30 || forms["casual"] < 30 || forms["neutral"] < 30 {
		t.Errorf("перекос стилей речи: %v", forms)
	}
	// ни одна пара не должна быть слишком похожей
	ids := make([]string, 0, 300)
	for id := range c.Captains {
		ids = append(ids, id)
	}
	maxSim := 0.0
	var pair string
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if s := Similarity(c.Captains[ids[i]], c.Captains[ids[j]]); s > maxSim {
				maxSim, pair = s, ids[i]+"/"+ids[j]
			}
		}
	}
	if maxSim > 0.92 {
		t.Errorf("слишком похожие капитаны %s: %.2f", pair, maxSim)
	}
	t.Logf("профессии: %d, фракции: %v, стили: %v, максимальная схожесть %.2f (%s)", len(profs), facs, forms, maxSim, pair)
}

func TestGeneratorDeterministicAndRerunSafe(t *testing.T) {
	a, b := NewConfig(), NewConfig()
	Generate(&a, GenerateOptions{Seed: 42, Count: 60})
	Generate(&b, GenerateOptions{Seed: 42, Count: 60})
	if !reflect.DeepEqual(a.Captains, b.Captains) {
		t.Fatal("одинаковый seed должен давать одинаковый результат")
	}
	// повторный запуск ничего не меняет и не создаёт
	before := a.Clone()
	res := Generate(&a, GenerateOptions{Seed: 42, Count: 60})
	if len(res.Added) != 0 || len(res.Skipped) != 60 {
		t.Fatalf("повторный запуск: добавлено %d, пропущено %d", len(res.Added), len(res.Skipped))
	}
	if !reflect.DeepEqual(a.Captains, before.Captains) {
		t.Fatal("повторный запуск изменил профили")
	}
	// ручные правки сохраняются при добавлении новых капитанов
	cp := a.Captains["cpt-0005"]
	cp.Bio = "Особая биография, которую владелец написал вручную и которую нельзя перезаписывать никакой генерацией, чтобы сохранить авторский текст."
	cp.Meta.Manual = true
	a.Captains["cpt-0005"] = cp
	Generate(&a, GenerateOptions{Seed: 42, Count: 80})
	if a.Captains["cpt-0005"].Bio != cp.Bio || !a.Captains["cpt-0005"].Meta.Manual {
		t.Fatal("вручную правленный профиль перезаписан")
	}
	if len(a.Captains) != 80 {
		t.Fatalf("в реестре %d капитанов", len(a.Captains))
	}
	// стабильность: профиль №10 одинаков независимо от общего числа
	c3 := NewConfig()
	Generate(&c3, GenerateOptions{Seed: 42, Count: 10})
	if !reflect.DeepEqual(c3.Captains["cpt-0003"].Bio, a.Captains["cpt-0003"].Bio) {
		t.Fatal("профиль зависит от размера партии")
	}
}

func TestGenderForms(t *testing.T) {
	cases := []struct{ in, g, want string }{
		{"Начинал(а) вторым пилотом.", "m", "Начинал вторым пилотом."},
		{"Начинал(а) вторым пилотом.", "f", "Начинала вторым пилотом."},
		{"Пришёл(ла) на астероиды.", "f", "Пришла на астероиды."},
		{"Пришёл(ла) на астероиды.", "m", "Пришёл на астероиды."},
		{"Вырос(ла) в посёлке.", "f", "Выросла в посёлке."},
		{"Учился(лась) у мастера.", "f", "Училась у мастера."},
		{"⟨Родился|Родилась⟩ на станции.", "f", "Родилась на станции."},
		{"Знакомые называют ⟨его|её⟩ добрым.", "m", "Знакомые называют его добрым."},
	}
	for _, c := range cases {
		if got := gender(c.in, c.g); got != c.want {
			t.Errorf("gender(%q,%s) = %q, ожидалось %q", c.in, c.g, got, c.want)
		}
	}
}

func TestSampleBiographyReadable(t *testing.T) {
	c := NewConfig()
	Generate(&c, GenerateOptions{Seed: 20261108, Count: 4})
	for _, id := range []string{"cpt-0001", "cpt-0002", "cpt-0003", "cpt-0004"} {
		cp := c.Captains[id]
		t.Log(fmt.Sprintf("%s %s «%s» (%s, %s/%s): %s", id, cp.Name, cp.CallSign, cp.Profession, cp.Speech.Formality, cp.Speech.Humor, cp.Bio))
		t.Log("  ", cp.Replies["greeting"], cp.Replies["ask_job"][0])
	}
}
