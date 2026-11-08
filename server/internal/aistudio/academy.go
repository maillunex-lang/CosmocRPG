package aistudio

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// AI Civilization Academy: симуляции и экзамены. Всё локально и детерминированно, без LLM и без сети.

// TestResult — итог группы проверок.
type TestResult struct {
	Kind     string   `json:"kind"`
	Title    string   `json:"title"`
	Total    int      `json:"total"`
	Passed   int      `json:"passed"`
	Failures []string `json:"failures"`
	Gate     bool     `json:"gate"` // провал блокирует публикацию
}

func (r *TestResult) check(ok bool, format string, a ...any) {
	r.Total++
	if ok {
		r.Passed++
		return
	}
	if len(r.Failures) < 25 {
		r.Failures = append(r.Failures, fmt.Sprintf(format, a...))
	}
}

// AllKinds — виды проверок.
var AllKinds = []string{"structure", "uniqueness", "intents", "character", "facts", "context", "memory", "repeats", "safety", "regression"}

var kindTitles = map[string]string{
	"structure": "Структура и ссылки", "uniqueness": "Уникальность капитанов", "intents": "Распознавание намерений",
	"character": "Согласованность характера", "facts": "Отсутствие выдуманных игровых фактов", "context": "Работа контекста",
	"memory": "Память о собеседнике", "repeats": "Защита от повторов", "safety": "Безопасные уточняющие ответы", "regression": "Регрессионные диалоги",
}

// sampleIDs выбирает до n капитанов равномерно по отсортированному реестру.
func sampleIDs(c *Config, n int) []string {
	ids := make([]string, 0, len(c.Captains))
	for id := range c.Captains {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if n <= 0 || len(ids) <= n {
		return ids
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, ids[i*len(ids)/n])
	}
	return out
}

func containsFold(text string, subs []string) bool {
	lt := strings.ToLower(text)
	for _, s := range subs {
		if strings.Contains(lt, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

// RunSuite запускает выбранные проверки. sample: сколько капитанов проверять в диалоговых тестах (0 — всех).
func RunSuite(c *Config, kinds []string, sample int) []TestResult {
	if len(kinds) == 0 {
		kinds = AllKinds
	}
	e := NewEngine(c)
	ids := sampleIDs(c, sample)
	var out []TestResult
	for _, k := range kinds {
		r := TestResult{Kind: k, Title: kindTitles[k], Failures: []string{}}
		switch k {
		case "structure":
			r.Gate = true
			for _, is := range Validate(*c) {
				if is.Level == "error" {
					r.check(false, "%s: %s", is.Path, is.Message)
				}
			}
			r.check(true, "")
			for id, cp := range c.Captains {
				b := mustJSON(cp)
				if msgs := ValidateJSON("captain", b); len(msgs) > 0 {
					r.check(false, "%s: %s", id, msgs[0])
				} else {
					r.check(true, "")
				}
			}
		case "uniqueness":
			testUniqueness(c, &r)
		case "intents":
			testIntents(c, e, ids, &r)
		case "character":
			testCharacter(c, e, ids, &r)
		case "facts":
			r.Gate = true
			testFacts(c, &r)
		case "context":
			r.Gate = true
			testContext(c, e, ids, &r)
		case "memory":
			testMemory(c, e, ids, &r)
		case "repeats":
			testRepeats(c, e, ids, &r)
		case "safety":
			testSafety(c, e, ids, &r)
		case "regression":
			r.Gate = true
			testRegression(c, e, ids, &r)
		default:
			continue
		}
		out = append(out, r)
	}
	return out
}

func mustJSON(v any) []byte {
	b, _ := jsonMarshal(v)
	return b
}

func testUniqueness(c *Config, r *TestResult) {
	names, bios, sigs := map[string]string{}, map[string]string{}, map[string]string{}
	ids := sampleIDs(c, 0)
	for _, id := range ids {
		cp := c.Captains[id]
		for _, p := range []struct {
			m map[string]string
			k string
			w string
		}{{names, strings.ToLower(cp.Name), "имя"}, {bios, cp.Bio, "биография"}, {sigs, cp.Signature(), "характер и речь"}} {
			if o, dup := p.m[p.k]; dup {
				r.check(false, "%s совпадает с %s (%s)", id, o, p.w)
			} else {
				r.check(true, "")
			}
			p.m[p.k] = id
		}
	}
	// слишком похожие соседи по реестру
	limit := 0.95
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids) && j < i+40; j++ {
			if s := Similarity(c.Captains[ids[i]], c.Captains[ids[j]]); s >= limit {
				r.check(false, "%s и %s слишком похожи (%.0f%%)", ids[i], ids[j], s*100)
			}
		}
	}
	r.check(true, "")
}

func testIntents(c *Config, e *Engine, ids []string, r *TestResult) {
	// каждый пример намерения должен распознаваться как своё намерение
	for _, it := range c.Intents {
		for _, ex := range it.Examples {
			if it.ID == "introduce" || it.ID == "ask_my_name" {
				continue // зависят от памяти и имени: проверяются в разделе «память»
			}
			got, conf, _ := e.Match(e.Canonical(ex))
			r.check(got == it.ID && conf >= threshold, "«%s»: ожидалось %s, получено %s (%.2f)", ex, it.ID, got, conf)
		}
	}
	// опечатки и сленг из словаря: подставляются в пример и должны давать то же намерение
	for _, ty := range c.Synonyms.Typos {
		if ty.Wrong == ty.Right {
			continue
		}
		for _, it := range c.Intents {
			if it.ID == "introduce" || it.ID == "ask_my_name" {
				continue
			}
			for _, ex := range it.Examples {
				n := Normalize(ex)
				if !strings.Contains(" "+n+" ", " "+Normalize(ty.Right)+" ") {
					continue
				}
				broken := strings.ReplaceAll(" "+n+" ", " "+Normalize(ty.Right)+" ", " "+Normalize(ty.Wrong)+" ")
				got, conf, _ := e.Match(e.Canonical(strings.TrimSpace(broken)))
				r.check(got == it.ID && conf >= threshold, "опечатка «%s» в «%s»: ожидалось %s, получено %s", ty.Wrong, ex, it.ID, got)
			}
		}
	}
}

func slangRe() *regexp.Regexp {
	return regexp.MustCompile(`(?i)(^|[^\p{L}])(` + strings.Join(slangWords, "|") + `)([^\p{L}]|$)`)
}

func testCharacter(c *Config, e *Engine, ids []string, r *TestResult) {
	sl := slangRe()
	for _, id := range ids {
		cp := c.Captains[id]
		var total, n int
		for intent, rs := range cp.Replies {
			for _, t := range rs {
				total += len([]rune(t))
				n++
				if cp.Speech.Formality == "formal" && sl.MatchString(t) {
					r.check(false, "%s: формальный капитан использует сленг в ответе «%s» (%s)", id, t, intent)
				}
				for _, w := range cp.Speech.AvoidWords {
					r.check(!containsFold(t, []string{w}), "%s: ответ содержит запретное слово «%s»", id, w)
				}
			}
		}
		r.check(n >= 3, "%s: слишком мало собственных ответов (%d)", id, n)
		// формальные не обращаются на «ты» в приветствиях по умолчанию: проверяем обращение игроку
		if cp.Speech.Formality == "formal" {
			r.check(cp.Speech.SlangLevel == 0, "%s: формальный стиль и уровень сленга %d", id, cp.Speech.SlangLevel)
		}
		if cp.Speech.Formality == "casual" {
			r.check(cp.Speech.SlangLevel >= 1, "%s: неформальный стиль без сленга", id)
		}
		if cp.Corporation != nil {
			ok := false
			for _, k := range c.Corporations {
				if k.ID == *cp.Corporation && k.Faction == cp.Faction {
					ok = true
				}
			}
			r.check(ok, "%s: корпорация не согласована с фракцией", id)
		}
	}
	// лаконичные говорят в среднем короче разговорчивых
	var terse, talk, nt, nk int
	for _, id := range ids {
		cp := c.Captains[id]
		conv := NewConversation(id)
		l := len([]rune(e.Reply(conv, "расскажи про автопилот").Reply))
		switch cp.Speech.Verbosity {
		case "terse":
			terse += l
			nt++
		case "talkative":
			talk += l
			nk++
		}
	}
	if nt > 0 && nk > 0 {
		r.check(terse/nt <= talk/nk+10, "лаконичные капитаны не короче разговорчивых (%d против %d знаков)", terse/nt, talk/nk)
	}
}

var digitRe = regexp.MustCompile(`\d`)

func testFacts(c *Config, r *TestResult) {
	facts := map[string]bool{}
	for _, f := range c.Facts {
		facts[f.ID] = true
	}
	check := func(where, text string) {
		for _, f := range FactRefs(text) {
			r.check(facts[f], "%s: неизвестный факт «%s»", where, f)
		}
		for _, b := range BadPlaceholders(text) {
			r.check(false, "%s: недопустимая подстановка %s", where, b)
		}
		stripped := factRe.ReplaceAllString(text, "")
		r.check(!digitRe.MatchString(stripped), "%s: число вне разрешённых фактов в «%s»", where, text)
	}
	for _, t := range c.Templates {
		check("шаблон "+t.ID, t.Text)
	}
	for id, cp := range c.Captains {
		for intent, rs := range cp.Replies {
			for _, t := range rs {
				check(id+"/"+intent, t)
			}
		}
	}
	// факты не должны ссылаться на несуществующее: у фактов нет подстановок
	for _, f := range c.Facts {
		r.check(!strings.Contains(f.Text, "{"), "факт %s содержит подстановку", f.ID)
	}
	// ответы на вопросы о нереализованном обязаны опираться на факт «пока не готово»
	for _, it := range []struct{ intent, fact string }{{"ask_combat", "combat_not_ready"}, {"ask_trade", "trade_not_ready"}} {
		found := false
		for _, t := range c.Templates {
			if t.Intent == it.intent && strings.Contains(t.Text, "{fact:"+it.fact+"}") {
				found = true
			}
		}
		if hasIntent(c, it.intent) {
			r.check(found, "ответ на «%s» должен опираться на факт «%s»", it.intent, it.fact)
		}
	}
}

func hasIntent(c *Config, id string) bool {
	for _, it := range c.Intents {
		if it.ID == id {
			return true
		}
	}
	return false
}

func testContext(c *Config, e *Engine, ids []string, r *TestResult) {
	for _, id := range ids {
		// уточнение «подробнее» после темы не теряет тему и не уходит в уточняющий вопрос
		for _, topic := range []struct{ say, intent string }{
			{"что такое автопилот", "ask_autopilot"}, {"где станция рубеж", "ask_station"}, {"что за контракт первый полёт", "ask_contract"},
		} {
			if !hasIntent(c, topic.intent) {
				continue
			}
			conv := NewConversation(id)
			e.Reply(conv, topic.say)
			t := e.Reply(conv, "расскажи подробнее")
			r.check(t.Intent == topic.intent && !t.Clarify, "%s: «подробнее» после «%s» дало %s (уточнение=%v)", id, topic.say, t.Intent, t.Clarify)
		}
		// оскорбление портит настроение, затем оно проходит
		if len(c.Emotions) > 0 {
			conv := NewConversation(id)
			t := e.Reply(conv, "ты дурак")
			r.check(t.Mood == "annoyed", "%s: после оскорбления настроение «%s»", id, t.Mood)
			for i := 0; i < 4; i++ {
				t = e.Reply(conv, "как дела")
			}
			r.check(t.Mood == "", "%s: настроение не вернулось к нейтральному (%s)", id, t.Mood)
		}
	}
}

func testMemory(c *Config, e *Engine, ids []string, r *TestResult) {
	if !hasIntent(c, "introduce") || !hasIntent(c, "ask_my_name") {
		r.check(false, "нет намерений introduce и ask_my_name")
		return
	}
	for _, id := range ids {
		conv := NewConversation(id)
		t0 := e.Reply(conv, "как меня зовут")
		r.check(!strings.Contains(t0.Reply, "Алекс"), "%s: капитан назвал имя, которого не знает", id)
		e.Reply(conv, "меня зовут Алекс")
		e.Reply(conv, "привет")
		t := e.Reply(conv, "как меня зовут")
		r.check(strings.Contains(t.Reply, "Алекс"), "%s: капитан не запомнил имя: «%s»", id, t.Reply)
		// память одного разговора не просачивается в другой
		other := NewConversation(id)
		t2 := e.Reply(other, "как меня зовут")
		r.check(!strings.Contains(t2.Reply, "Алекс"), "%s: память просочилась в новый разговор", id)
		// срок хранения
		ttl := 50
		for _, m := range c.MemoryRules {
			if m.Remember == "player_name" {
				ttl = m.TTLTurns
			}
		}
		for i := 0; i < ttl+2; i++ {
			e.Reply(conv, "привет")
		}
		t3 := e.Reply(conv, "как меня зовут")
		r.check(!strings.Contains(t3.Reply, "Алекс"), "%s: имя не забыто по истечении срока памяти", id)
	}
}

func testRepeats(c *Config, e *Engine, ids []string, r *TestResult) {
	for _, id := range ids {
		for _, say := range []string{"привет", "как дела", "чем занимаешься"} {
			conv := NewConversation(id)
			var replies []string
			for i := 0; i < 8; i++ {
				replies = append(replies, e.Reply(conv, say).Reply)
			}
			bad := false
			for i := 1; i < len(replies); i++ {
				if replies[i] == replies[i-1] {
					bad = true
				}
			}
			distinct := map[string]bool{}
			for _, x := range replies {
				distinct[x] = true
			}
			r.check(!bad, "%s: подряд повторяется один и тот же ответ на «%s»", id, say)
			r.check(len(distinct) >= 3, "%s: на «%s» только %d разных ответов из 8", id, say, len(distinct))
		}
	}
}

func testSafety(c *Config, e *Engine, ids []string, r *TestResult) {
	nonsense := []string{"расскажи про древний храм на луне", "кто победил в большой войне", "сколько стоит корабль класса дредноут", "ыыыы", "123 456", "где спрятан клад"}
	for _, id := range ids {
		for _, s := range nonsense {
			conv := NewConversation(id)
			t := e.Reply(conv, s)
			r.check(t.Clarify, "%s: на «%s» дан уверенный ответ (%s) вместо уточнения", id, s, t.Intent)
			r.check(!digitRe.MatchString(t.Reply), "%s: в ответе на «%s» появились цифры: «%s»", id, s, t.Reply)
		}
	}
}

func testRegression(c *Config, e *Engine, ids []string, r *TestResult) {
	run := func(id string, tc DialogueTest) {
		if _, ok := c.Captains[id]; !ok {
			r.check(false, "%s: капитана %s нет", tc.ID, id)
			return
		}
		conv := NewConversation(id)
		var last Turn
		for _, s := range tc.Say {
			last = e.Reply(conv, s)
		}
		ex := tc.Expect
		if ex.Intent != "" {
			r.check(last.Intent == ex.Intent, "%s (%s): ожидалось намерение %s, получено %s", tc.ID, id, ex.Intent, last.Intent)
		}
		if len(ex.ContainsAny) > 0 {
			r.check(containsFold(last.Reply, ex.ContainsAny), "%s (%s): ответ «%s» не содержит ни одного из %v", tc.ID, id, last.Reply, ex.ContainsAny)
		}
		if len(ex.NotContains) > 0 {
			r.check(!containsFold(last.Reply, ex.NotContains), "%s (%s): ответ содержит запрещённое: «%s»", tc.ID, id, last.Reply)
		}
		if ex.NoClarify {
			r.check(!last.Clarify, "%s (%s): ответ уточняющий, а ожидался ответ по теме", tc.ID, id)
		}
	}
	for _, tc := range c.Tests {
		if tc.CaptainID != "" {
			run(tc.CaptainID, tc)
			continue
		}
		for _, id := range ids {
			run(id, tc)
		}
	}
	r.check(true, "")
}

// ---- симулятор и групповые разговоры ----

// Transcript — запись разговора.
type Transcript struct {
	Speaker string  `json:"speaker"`
	Name    string  `json:"name"`
	Text    string  `json:"text"`
	Intent  string  `json:"intent,omitempty"`
	Conf    float64 `json:"confidence,omitempty"`
	Mood    string  `json:"mood,omitempty"`
	Clarify bool    `json:"clarify,omitempty"`
	Source  string  `json:"source,omitempty"`
}

// Simulate прогоняет реплики игрока через капитана.
func Simulate(c *Config, captainID string, say []string) ([]Transcript, error) {
	cp, ok := c.Captains[captainID]
	if !ok {
		return nil, fmt.Errorf("капитана %s нет", captainID)
	}
	e := NewEngine(c)
	conv := NewConversation(captainID)
	var out []Transcript
	for _, s := range say {
		t := e.Reply(conv, s)
		out = append(out, Transcript{Speaker: "player", Name: "Игрок", Text: s})
		out = append(out, Transcript{Speaker: captainID, Name: cp.Name, Text: t.Reply, Intent: t.Intent, Conf: t.Confidence, Mood: t.Mood, Clarify: t.Clarify, Source: t.Source})
	}
	return out, nil
}

// GroupReport — итог группового разговора.
type GroupReport struct {
	Transcript []Transcript `json:"transcript"`
	Issues     []string     `json:"issues"`
	Distinct   int          `json:"distinct_replies"`
}

// SimulateGroup: капитаны по очереди отвечают на реплику предыдущего; первая реплика задаёт тему.
// Если предыдущий ответ уточняющий (капитаны не поняли друг друга), ведущий задаёт следующий вопрос из набора тем.
func SimulateGroup(c *Config, ids []string, opening string, turns int) (GroupReport, error) {
	if len(ids) < 2 || len(ids) > 6 {
		return GroupReport{}, fmt.Errorf("нужно от 2 до 6 капитанов")
	}
	e := NewEngine(c)
	convs := map[string]*Conversation{}
	for _, id := range ids {
		if _, ok := c.Captains[id]; !ok {
			return GroupReport{}, fmt.Errorf("капитана %s нет", id)
		}
		convs[id] = NewConversation(id)
	}
	var topics []string
	for _, in := range []string{"how_are_you", "ask_job", "ask_goals", "ask_faction", "ask_autopilot", "ask_station", "ask_contract"} {
		for _, it := range c.Intents {
			if it.ID == in && len(it.Examples) > 0 {
				topics = append(topics, it.Examples[0])
			}
		}
	}
	var rep GroupReport
	rep.Transcript = append(rep.Transcript, Transcript{Speaker: "player", Name: "Игрок", Text: opening})
	text := opening
	seen := map[string]bool{}
	var prev string
	hostTurn := 0
	for i := 0; i < turns; i++ {
		id := ids[i%len(ids)]
		t := e.Reply(convs[id], text)
		cp := c.Captains[id]
		rep.Transcript = append(rep.Transcript, Transcript{Speaker: id, Name: cp.Name, Text: t.Reply, Intent: t.Intent, Conf: t.Confidence, Mood: t.Mood, Clarify: t.Clarify, Source: t.Source})
		if t.Reply == prev {
			rep.Issues = append(rep.Issues, fmt.Sprintf("реплика %d повторяет предыдущую", i+1))
		}
		seen[t.Reply] = true
		prev = t.Reply
		text = t.Reply
		if (t.Clarify || t.Intent == "greeting" || t.Intent == "farewell") && len(topics) > 0 && i < turns-1 {
			q := topics[hostTurn%len(topics)]
			hostTurn++
			rep.Transcript = append(rep.Transcript, Transcript{Speaker: "host", Name: "Ведущий", Text: q})
			text = q
		}
	}
	rep.Distinct = len(seen)
	if turns >= 4 && len(seen) < turns/2 {
		rep.Issues = append(rep.Issues, "в разговоре слишком много повторяющихся реплик")
	}
	return rep, nil
}

// Quality — сводные показатели качества реестра.
func Quality(c *Config) map[string]any {
	n := len(c.Captains)
	var bioLen, replies, manual, chatgpt, gen int
	prof, fac, form, humor := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	catch := map[string]bool{}
	for _, cp := range c.Captains {
		bioLen += len([]rune(cp.Bio))
		for _, rs := range cp.Replies {
			replies += len(rs)
		}
		prof[cp.Profession]++
		fac[cp.Faction]++
		form[cp.Speech.Formality]++
		humor[cp.Speech.Humor]++
		for _, ph := range cp.Speech.Catchphrases {
			catch[ph] = true
		}
		switch {
		case cp.Meta.Manual:
			manual++
		case cp.Meta.Source == "chatgpt":
			chatgpt++
		default:
			gen++
		}
	}
	avg := func(x int) int {
		if n == 0 {
			return 0
		}
		return x / n
	}
	return map[string]any{
		"captains": n, "avg_bio_chars": avg(bioLen), "avg_replies": avg(replies), "distinct_catchphrases": len(catch),
		"professions": prof, "factions": fac, "formality": form, "humor": humor,
		"by_source": map[string]int{"generator": gen, "chatgpt": chatgpt, "manual": manual},
		"intents":   len(c.Intents), "templates": len(c.Templates), "facts": len(c.Facts), "tests": len(c.Tests),
	}
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
