package aistudio

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// CaptainChange — изменение профиля капитана при импорте.
type CaptainChange struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Kind   string   `json:"kind"` // added | changed | unchanged | conflict
	Fields []string `json:"fields,omitempty"`
	Reason string   `json:"reason,omitempty"`
}

// SectionChange — изменения раздела данных.
type SectionChange struct {
	Name      string `json:"name"`
	Added     int    `json:"added"`
	Changed   int    `json:"changed"`
	Unchanged int    `json:"unchanged"`
}

// Plan — что изменится после публикации (предпросмотр).
type Plan struct {
	Captains  []CaptainChange `json:"captains"`
	Sections  []SectionChange `json:"sections"`
	Added     int             `json:"added"`
	Changed   int             `json:"changed"`
	Unchanged int             `json:"unchanged"`
	Conflicts int             `json:"conflicts"`
}

// MergeOptions — параметры слияния.
type MergeOptions struct {
	// OverwriteManual разрешает перезаписать профили, правленные вручную (по умолчанию они защищены).
	OverwriteManual bool `json:"overwrite_manual"`
}

func changedFields(a, b Captain) []string {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	var ma, mb map[string]json.RawMessage
	_ = json.Unmarshal(ja, &ma)
	_ = json.Unmarshal(jb, &mb)
	var out []string
	for k := range mb {
		if k == "revision" {
			continue
		}
		if string(ma[k]) != string(mb[k]) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func mergeByKey[T any](base, in []T, key func(T) string) ([]T, SectionChange) {
	var sc SectionChange
	out := append([]T{}, base...)
	idx := map[string]int{}
	for i, v := range out {
		idx[key(v)] = i
	}
	for _, v := range in {
		k := key(v)
		if i, ok := idx[k]; ok {
			if reflect.DeepEqual(out[i], v) {
				sc.Unchanged++
			} else {
				out[i] = v
				sc.Changed++
			}
		} else {
			idx[k] = len(out)
			out = append(out, v)
			sc.Added++
		}
	}
	return out, sc
}

// Merge накладывает разобранный пакет на базовую конфигурацию. Неразрушающе: ничего не удаляется,
// вручную правленные капитаны защищены, существующие данные заменяются только одноимёнными.
func Merge(base Config, p *Parsed, opt MergeOptions) (Config, Plan) {
	out := base.Clone()
	var plan Plan
	ids := make([]string, 0, len(p.Partial.Captains))
	for id := range p.Partial.Captains {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		in := p.Partial.Captains[id]
		old, exists := out.Captains[id]
		ch := CaptainChange{ID: id, Name: in.Name}
		switch {
		case !exists:
			ch.Kind = "added"
			if in.Revision < 1 {
				in.Revision = 1
			}
			out.Captains[id] = in
			plan.Added++
		default:
			fields := changedFields(old, in)
			if len(fields) == 0 {
				ch.Kind = "unchanged"
				plan.Unchanged++
			} else if old.Meta.Manual && !opt.OverwriteManual {
				ch.Kind, ch.Fields = "conflict", fields
				ch.Reason = "профиль правился вручную и защищён от перезаписи; разрешите перезапись, если хотите заменить"
				plan.Conflicts++
			} else {
				ch.Kind, ch.Fields = "changed", fields
				in.Revision = max(in.Revision, old.Revision+1)
				if in.Meta.Source == "" {
					in.Meta.Source = "chatgpt"
				}
				out.Captains[id] = in
				plan.Changed++
			}
		}
		plan.Captains = append(plan.Captains, ch)
	}
	sec := func(name string, sc SectionChange) {
		plan.Sections = append(plan.Sections, SectionChange{name, sc.Added, sc.Changed, sc.Unchanged})
	}
	pc := p.Partial
	if p.Sections["intents"] {
		var sc SectionChange
		out.Intents, sc = mergeByKey(out.Intents, pc.Intents, func(x Intent) string { return x.ID })
		sec("dialogue/intents", sc)
	}
	if p.Sections["synonyms"] {
		var a, b, c SectionChange
		out.Synonyms.Synonyms, a = mergeByKey(out.Synonyms.Synonyms, pc.Synonyms.Synonyms, func(x Synonym) string { return x.Canonical })
		out.Synonyms.Slang, b = mergeByKey(out.Synonyms.Slang, pc.Synonyms.Slang, func(x SlangEntry) string { return x.Slang })
		out.Synonyms.Typos, c = mergeByKey(out.Synonyms.Typos, pc.Synonyms.Typos, func(x Typo) string { return x.Wrong })
		sec("dialogue/synonyms", SectionChange{"", a.Added + b.Added + c.Added, a.Changed + b.Changed + c.Changed, a.Unchanged + b.Unchanged + c.Unchanged})
	}
	if p.Sections["templates"] {
		var sc SectionChange
		out.Templates, sc = mergeByKey(out.Templates, pc.Templates, func(x Template) string { return x.ID })
		sec("dialogue/templates", sc)
	}
	if p.Sections["rules"] {
		var sc SectionChange
		out.Rules, sc = mergeByKey(out.Rules, pc.Rules, func(x Rule) string { return x.ID })
		sec("dialogue/rules", sc)
	}
	if p.Sections["emotions"] {
		var sc SectionChange
		out.Emotions, sc = mergeByKey(out.Emotions, pc.Emotions, func(x Emotion) string { return x.ID })
		sec("dialogue/emotions", sc)
	}
	if p.Sections["facts"] {
		var sc SectionChange
		out.Facts, sc = mergeByKey(out.Facts, pc.Facts, func(x Fact) string { return x.ID })
		sec("knowledge/facts", sc)
	}
	if p.Sections["factions"] {
		var sc SectionChange
		out.Factions, sc = mergeByKey(out.Factions, pc.Factions, func(x Faction) string { return x.ID })
		sec("society/factions", sc)
	}
	if p.Sections["corporations"] {
		var sc SectionChange
		out.Corporations, sc = mergeByKey(out.Corporations, pc.Corporations, func(x Corporation) string { return x.ID })
		sec("society/corporations", sc)
	}
	if p.Sections["relations"] {
		var sc SectionChange
		out.Relations, sc = mergeByKey(out.Relations, pc.Relations, func(x FactionRelation) string { return x.A + "|" + x.B })
		sec("society/relations", sc)
	}
	if p.Sections["strategies"] {
		var sc SectionChange
		out.Strategies, sc = mergeByKey(out.Strategies, pc.Strategies, func(x Strategy) string { return x.ID })
		sec("behavior/strategies", sc)
	}
	if p.Sections["lessons"] {
		var sc SectionChange
		out.Lessons, sc = mergeByKey(out.Lessons, pc.Lessons, func(x Lesson) string { return x.ID })
		sec("learning/lessons", sc)
	}
	if p.Sections["memory_rules"] {
		var sc SectionChange
		out.MemoryRules, sc = mergeByKey(out.MemoryRules, pc.MemoryRules, func(x MemoryRule) string { return x.ID })
		sec("memory/memory_rules", sc)
	}
	if p.Sections["schedules"] {
		var sc SectionChange
		out.Schedules, sc = mergeByKey(out.Schedules, pc.Schedules, func(x Schedule) string { return x.CaptainID + "|" + x.Activity })
		sec("scheduler/schedules", sc)
	}
	if p.Sections["dialogue_tests"] {
		var sc SectionChange
		out.Tests, sc = mergeByKey(out.Tests, pc.Tests, func(x DialogueTest) string { return x.ID })
		sec("tests/dialogue_tests", sc)
	}
	// разделы без изменений не показываем
	kept := plan.Sections[:0]
	for _, s := range plan.Sections {
		if s.Added+s.Changed > 0 {
			kept = append(kept, s)
		}
	}
	plan.Sections = kept
	return out, plan
}

// Summary — краткая строка о плане.
func (p Plan) Summary() string {
	return fmt.Sprintf("добавится %d, изменится %d, без изменений %d, конфликтов %d", p.Added, p.Changed, p.Unchanged, p.Conflicts)
}
