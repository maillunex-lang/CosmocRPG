package aistudio

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var factRe = regexp.MustCompile(`\{fact:([a-z][a-z0-9_\-]*)\}`)

var anyBraceRe = regexp.MustCompile(`\{[^{}]*\}`)
var allowedBraceRe = regexp.MustCompile(`^\{(player|captain|topics|fact:[a-z][a-z0-9_\-]*)\}$`)

// BadPlaceholders возвращает недопустимые подстановки в тексте (разрешены {player}, {captain}, {topics}, {fact:id}).
func BadPlaceholders(text string) []string {
	var out []string
	for _, m := range anyBraceRe.FindAllString(text, -1) {
		if !allowedBraceRe.MatchString(m) {
			out = append(out, m)
		}
	}
	if strings.Count(text, "{") != strings.Count(text, "}") && len(out) == 0 {
		out = append(out, "непарная фигурная скобка")
	}
	return out
}

// FactRefs возвращает идентификаторы фактов, на которые ссылается текст.
func FactRefs(text string) []string {
	var out []string
	for _, m := range factRe.FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
	}
	return out
}

// Validate проверяет ссылочную целостность и согласованность полной конфигурации.
func Validate(c Config) []Issue {
	var out []Issue
	add := func(level, code, path, format string, a ...any) {
		out = append(out, Issue{level, code, path, fmt.Sprintf(format, a...)})
	}
	dupes := func(kind string, ids []string) {
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				add("error", "duplicate_id", kind, "повторяющийся идентификатор «%s»", id)
			}
			seen[id] = true
		}
	}
	intents := map[string]bool{"unknown": true, "no_name": true} // встроенные псевдонамерения движка
	var ids []string
	for _, it := range c.Intents {
		intents[it.ID] = true
		ids = append(ids, it.ID)
	}
	dupes("dialogue/intents.json", ids)
	facts := map[string]bool{}
	ids = nil
	for _, f := range c.Facts {
		facts[f.ID] = true
		ids = append(ids, f.ID)
	}
	dupes("knowledge/facts.json", ids)
	factions := map[string]bool{}
	ids = nil
	for _, f := range c.Factions {
		factions[f.ID] = true
		ids = append(ids, f.ID)
	}
	dupes("society/factions.json", ids)
	corps := map[string]Corporation{}
	ids = nil
	for _, k := range c.Corporations {
		corps[k.ID] = k
		ids = append(ids, k.ID)
		if !factions[k.Faction] {
			add("error", "ref", "society/corporations.json", "корпорация «%s» ссылается на неизвестную фракцию «%s»", k.ID, k.Faction)
		}
	}
	dupes("society/corporations.json", ids)
	for _, r := range c.Relations {
		if !factions[r.A] || !factions[r.B] {
			add("error", "ref", "society/relations.json", "отношение %s—%s: неизвестная фракция", r.A, r.B)
		}
	}
	strategies := map[string]bool{}
	ids = nil
	for _, s := range c.Strategies {
		strategies[s.ID] = true
		ids = append(ids, s.ID)
	}
	dupes("behavior/strategies.json", ids)
	ids = nil
	for _, t := range c.Templates {
		ids = append(ids, t.ID)
		if !intents[t.Intent] {
			add("error", "ref", "dialogue/templates.json", "шаблон «%s»: неизвестное намерение «%s»", t.ID, t.Intent)
		}
		for _, f := range FactRefs(t.Text) {
			if !facts[f] {
				add("error", "unknown_fact", "dialogue/templates.json", "шаблон «%s» ссылается на неизвестный игровой факт «%s»", t.ID, f)
			}
		}
		for _, b := range BadPlaceholders(t.Text) {
			add("error", "bad_placeholder", "dialogue/templates.json", "шаблон «%s»: недопустимая подстановка %s", t.ID, b)
		}
	}
	dupes("dialogue/templates.json", ids)
	for _, r := range c.Rules {
		for _, id := range []string{r.When.PrevIntent, r.When.Intent, r.Then.IntentOverride} {
			if id != "" && !intents[id] {
				add("error", "ref", "dialogue/rules.json", "правило «%s»: неизвестное намерение «%s»", r.ID, id)
			}
		}
	}
	for _, e := range c.Emotions {
		for _, t := range e.Triggers {
			if !intents[t] {
				add("error", "ref", "dialogue/emotions.json", "реакция «%s»: неизвестное намерение «%s»", e.ID, t)
			}
		}
	}
	names := map[string]string{}
	signatures := map[string]string{}
	capIDs := make([]string, 0, len(c.Captains))
	for id := range c.Captains {
		capIDs = append(capIDs, id)
	}
	sort.Strings(capIDs)
	for _, id := range capIDs {
		cp := c.Captains[id]
		p := "captains/" + id + ".json"
		if cp.ID != id {
			add("error", "captain_id", p, "идентификатор не совпадает с ключом")
		}
		if !factions[cp.Faction] {
			add("error", "ref", p, "неизвестная фракция «%s»", cp.Faction)
		}
		if cp.Corporation != nil {
			k, ok := corps[*cp.Corporation]
			switch {
			case !ok:
				add("error", "ref", p, "неизвестная корпорация «%s»", *cp.Corporation)
			case k.Faction != cp.Faction:
				add("warning", "faction_mismatch", p, "корпорация «%s» принадлежит фракции «%s», а капитан — «%s»", k.ID, k.Faction, cp.Faction)
			}
		}
		for _, s := range cp.Strategies {
			if !strategies[s] {
				add("error", "ref", p, "неизвестная стратегия «%s»", s)
			}
		}
		for _, r := range cp.Relationships {
			if r.CaptainID == id {
				add("error", "ref", p, "капитан не может иметь отношение к самому себе")
			} else if _, ok := c.Captains[r.CaptainID]; !ok {
				add("error", "ref", p, "отношение к неизвестному капитану «%s»", r.CaptainID)
			}
		}
		for intent, replies := range cp.Replies {
			if !intents[intent] {
				add("error", "ref", p, "ответы для неизвестного намерения «%s»", intent)
			}
			for _, t := range replies {
				for _, b := range BadPlaceholders(t) {
					add("error", "bad_placeholder", p, "ответ содержит недопустимую подстановку %s", b)
				}
				for _, f := range FactRefs(t) {
					if !facts[f] {
						add("error", "unknown_fact", p, "ответ ссылается на неизвестный игровой факт «%s»", f)
					}
				}
			}
		}
		key := strings.ToLower(cp.Name)
		if other, dup := names[key]; dup {
			add("error", "duplicate_name", p, "имя «%s» уже у капитана %s", cp.Name, other)
		}
		names[key] = id
		if cp.CallSign != "" {
			ck := "cs:" + strings.ToLower(cp.CallSign)
			if other, dup := names[ck]; dup {
				add("warning", "duplicate_call_sign", p, "позывной «%s» уже у капитана %s", cp.CallSign, other)
			}
			names[ck] = id
		}
		sig := cp.Signature()
		if other, dup := signatures[sig]; dup {
			add("warning", "similar_character", p, "характер, речь и цели почти совпадают с капитаном %s", other)
		}
		signatures[sig] = id
	}
	return out
}

// Signature — отпечаток характера для проверки уникальности: профессия, черты, стиль речи, главная цель.
func (c Captain) Signature() string {
	traits := append([]string{}, c.Personality.Traits...)
	sort.Strings(traits)
	goal := ""
	if len(c.Goals) > 0 {
		goal = c.Goals[0].ID
	}
	return fmt.Sprintf("%s|%s|%s|%s|%s|%d|%s|%s", c.Profession, strings.Join(traits, ","), c.Speech.Formality, c.Speech.Verbosity,
		c.Speech.Humor, c.Speech.SlangLevel, c.Personality.Temper, goal)
}

// Similarity — доля совпавших смысловых признаков двух капитанов (0..1), для отчёта об уникальности.
func Similarity(a, b Captain) float64 {
	score, total := 0.0, 0.0
	eq := func(x, y string) {
		total++
		if x == y {
			score++
		}
	}
	eq(a.Profession, b.Profession)
	eq(a.Faction, b.Faction)
	eq(a.Speech.Formality, b.Speech.Formality)
	eq(a.Speech.Verbosity, b.Speech.Verbosity)
	eq(a.Speech.Humor, b.Speech.Humor)
	eq(a.Personality.Temper, b.Personality.Temper)
	eq(fmt.Sprint(a.Speech.SlangLevel), fmt.Sprint(b.Speech.SlangLevel))
	set := map[string]bool{}
	for _, t := range a.Personality.Traits {
		set[t] = true
	}
	inter, union := 0, len(set)
	for _, t := range b.Personality.Traits {
		if set[t] {
			inter++
		} else {
			union++
		}
	}
	total++
	if union > 0 {
		score += float64(inter) / float64(union)
	}
	return score / total
}
