package aistudio

import (
	"hash/fnv"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Минимальный диалоговый движок на правилах. Без LLM и без сети: нормализация русского текста,
// синонимы, сленг и опечатки, намерения, контекст, настроение, память, речевые профили,
// проверка игровых фактов, защита от повторов и безопасные уточняющие ответы.

// SemanticSearcher — точка подключения локального семантического поиска (эмбеддинги) позже.
// Сейчас не используется: движок работает на правилах. Реализация должна быть локальной (без облака).
type SemanticSearcher interface {
	Search(query string, k int) []SemanticHit
}

// SemanticHit — найденное намерение и оценка близости (0..1).
type SemanticHit struct {
	IntentID string
	Score    float64
}

const (
	intentUnknown = "unknown"
	intentNoName  = "no_name" // спросили имя, которое капитан не знает
	threshold     = 0.40
	maxRecent     = 6
)

var stopWords = map[string]bool{
	"и": true, "в": true, "на": true, "я": true, "ты": true, "мне": true, "меня": true, "как": true, "что": true,
	"это": true, "а": true, "но": true, "по": true, "за": true, "из": true, "у": true, "к": true, "с": true,
	"про": true, "ну": true, "вот": true, "же": true, "ли": true, "бы": true, "то": true, "о": true, "об": true,
	"какой": true, "какая": true, "какие": true, "какое": true, "каких": true, "каким": true, "такой": true, "такое": true, "пожалуйста": true, "мой": true, "наш": true, "нам": true, "вы": true, "вас": true, "вам": true, "тут": true, "здесь": true,
}

var endings = []string{"ами", "ями", "ого", "его", "ому", "ему", "ыми", "ими", "ать", "ять", "ить", "еть", "ует", "уют",
	"ают", "яют", "ешь", "ишь", "ете", "ите", "ой", "ей", "ый", "ий", "ая", "яя", "ое", "ее", "ые", "ие", "ов", "ев",
	"ам", "ям", "ах", "ях", "ом", "ем", "ую", "юю", "ет", "ут", "ют", "ит", "ат", "ят", "ть", "ла", "ли", "ло",
	"а", "я", "о", "е", "ы", "и", "у", "ю", "ь"}

// Normalize приводит текст к виду для сравнения: нижний регистр, ё→е, без знаков препинания.
func Normalize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "ё", "е")
	var b strings.Builder
	space := true
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// stem — грубое отсечение окончаний (достаточно для сопоставления форм слова).
func stem(w string) string {
	r := []rune(w)
	if len(r) <= 3 {
		return w
	}
	for _, e := range endings {
		er := []rune(e)
		if len(r)-len(er) >= 3 && strings.HasSuffix(w, e) {
			return string(r[:len(r)-len(er)])
		}
	}
	return w
}

func lev(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

type intentIdx struct {
	intent   Intent
	examples []map[string]bool
	keywords map[string]bool
}

// Engine — движок диалога. Неизменяем после создания, безопасен для параллельного использования.
type Engine struct {
	cfg      *Config
	vocab    map[string]bool // нормализованные слова словаря намерений
	typos    map[string]string
	phrases  []phrasePair // сленг и синонимы: фраза -> каноническая форма
	intents  []intentIdx
	facts    map[string]string
	topics   []string
	Searcher SemanticSearcher // необязательный локальный семантический поиск (пока не используется)
}

type phrasePair struct{ from, to string }

// NewEngine строит индексы по конфигурации.
func NewEngine(cfg *Config) *Engine {
	e := &Engine{cfg: cfg, vocab: map[string]bool{}, typos: map[string]string{}, facts: map[string]string{}}
	for _, t := range cfg.Synonyms.Typos {
		e.typos[Normalize(t.Wrong)] = Normalize(t.Right)
	}
	for _, s := range cfg.Synonyms.Slang {
		if f := Normalize(s.Slang); f != "" {
			e.phrases = append(e.phrases, phrasePair{f, Normalize(s.Meaning)})
		}
	}
	for _, s := range cfg.Synonyms.Synonyms {
		c := Normalize(s.Canonical)
		for _, v := range s.Variants {
			if f := Normalize(v); f != "" && f != c {
				e.phrases = append(e.phrases, phrasePair{f, c})
			}
		}
		for _, w := range strings.Fields(c) {
			e.vocab[w] = true
		}
	}
	// длинные фразы заменяются раньше коротких
	sort.SliceStable(e.phrases, func(i, j int) bool { return len(e.phrases[i].from) > len(e.phrases[j].from) })
	for _, f := range cfg.Facts {
		e.facts[f.ID] = f.Text
	}
	for _, it := range cfg.Intents {
		ix := intentIdx{intent: it, keywords: map[string]bool{}}
		for _, ex := range it.Examples {
			set := map[string]bool{}
			for _, w := range strings.Fields(Normalize(ex)) {
				e.vocab[w] = true
				if !stopWords[w] {
					set[stem(w)] = true
				}
			}
			if len(set) > 0 {
				ix.examples = append(ix.examples, set)
			}
		}
		for _, k := range it.Keywords {
			for _, w := range strings.Fields(Normalize(k)) {
				e.vocab[w] = true
				ix.keywords[stem(w)] = true
			}
		}
		e.intents = append(e.intents, ix)
		if strings.HasPrefix(it.ID, "ask_") && len(e.topics) < 4 && it.ID != "ask_my_name" {
			e.topics = append(e.topics, it.Title)
		}
	}
	return e
}

// Canonical возвращает текст после нормализации, исправления опечаток, замены сленга и синонимов.
func (e *Engine) Canonical(input string) string {
	n := Normalize(input)
	toks := strings.Fields(n)
	for i, t := range toks {
		if r, ok := e.typos[t]; ok {
			toks[i] = r
		}
	}
	n = " " + strings.Join(toks, " ") + " "
	for _, p := range e.phrases {
		n = strings.ReplaceAll(n, " "+p.from+" ", " "+p.to+" ")
	}
	toks = strings.Fields(n)
	// неизвестные слова: ближайшее слово словаря (опечатка в одну-две буквы)
	for i, t := range toks {
		if e.vocab[t] || len([]rune(t)) < 4 || stopWords[t] {
			continue
		}
		maxD := 1
		if len([]rune(t)) >= 8 {
			maxD = 2
		}
		best, bestD, tie := "", 99, false
		for w := range e.vocab {
			if abs(len([]rune(w))-len([]rune(t))) > maxD {
				continue
			}
			d := lev(t, w)
			if d < bestD {
				best, bestD, tie = w, d, false
			} else if d == bestD && w != best {
				tie = true
			}
		}
		if bestD <= maxD && !tie {
			toks[i] = best
		}
	}
	return strings.Join(toks, " ")
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func stemSet(canonical string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(canonical) {
		if !stopWords[w] {
			out[stem(w)] = true
		}
	}
	return out
}

// Match определяет намерение. Возвращает идентификатор, уверенность и (если неоднозначно) второе намерение.
func (e *Engine) Match(canonical string) (best string, score float64, second string) {
	in := stemSet(canonical)
	if len(in) == 0 {
		return intentUnknown, 0, ""
	}
	type sc struct {
		id string
		s  float64
	}
	var all []sc
	for _, ix := range e.intents {
		s := 0.0
		for _, ex := range ix.examples {
			inter := 0
			for w := range in {
				if ex[w] {
					inter++
				}
			}
			if inter > 0 {
				d := 2 * float64(inter) / float64(len(in)+len(ex))
				// пример должен покрывать большую часть реплики: одно общее слово в длинной фразе — не совпадение
				if float64(inter)/float64(len(in)) < 0.5 {
					d *= 0.5
				}
				if d > s {
					s = d
				}
			}
		}
		hits := 0
		for w := range in {
			if ix.keywords[w] {
				hits++
			}
		}
		if k := float64(hits) * 0.45; k > 0 {
			// ключевое слово среди множества посторонних слов — не уверенное попадание
			if float64(hits)/float64(len(in)) < 0.34 {
				k *= 0.7
			}
			if k > s {
				s = min(k, 1)
			}
		}
		if s > 0 {
			all = append(all, sc{ix.intent.ID, s})
		}
	}
	if len(all) == 0 {
		return intentUnknown, 0, ""
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].s > all[j].s })
	best, score = all[0].id, all[0].s
	if len(all) > 1 && all[0].s-all[1].s < 0.03 && all[1].s >= threshold {
		second = all[1].id
	}
	return best, score, second
}

// Turn — реплика в истории.
type Turn struct {
	Player     string  `json:"player"`
	Reply      string  `json:"reply"`
	Intent     string  `json:"intent"`
	Confidence float64 `json:"confidence"`
	Mood       string  `json:"mood"`
	Clarify    bool    `json:"clarify"`
	Source     string  `json:"source"`
}

// Conversation — состояние разговора с одним капитаном.
type Conversation struct {
	CaptainID  string
	Turn       int
	LastIntent string
	Mood       string
	moodLeft   int
	Memory     map[string]string
	memTurn    map[string]int
	Recent     []string
	lastCatch  int
	History    []Turn
}

// NewConversation начинает разговор.
func NewConversation(captainID string) *Conversation {
	return &Conversation{CaptainID: captainID, Memory: map[string]string{}, memTurn: map[string]int{}, lastCatch: -10}
}

var nameRe = regexp.MustCompile(`(?:меня зовут|мое имя|моё имя|зови меня|я -|я —|я)\s+([A-ZА-ЯЁ][\p{L}\-]{1,20})`)

func hash(parts ...string) uint32 {
	h := fnv.New32a()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return h.Sum32()
}

// Reply обрабатывает реплику игрока и возвращает ответ капитана. Все факты берутся только из knowledge/facts.json.
func (e *Engine) Reply(conv *Conversation, input string) Turn {
	cp, ok := e.cfg.Captains[conv.CaptainID]
	if !ok {
		return Turn{Player: input, Reply: "…", Intent: intentUnknown, Source: "no_captain"}
	}
	conv.Turn++
	canon := e.Canonical(input)
	intent, conf, second := e.Match(canon)

	// память: имя собеседника
	if e.memoryEnabled("player_name") {
		if m := nameRe.FindStringSubmatch(input); m != nil && e.hasIntent("introduce") {
			conv.Memory["player_name"] = m[1]
			conv.memTurn["player_name"] = conv.Turn
			intent, conf, second = "introduce", 1, ""
		}
	}
	e.expireMemory(conv)

	// контекстные правила
	for _, r := range e.cfg.Rules {
		if !ruleMatches(r, conv, intent, canon) {
			continue
		}
		if r.Then.IntentOverride != "" {
			intent, conf, second = r.Then.IntentOverride, max(conf, 0.6), ""
		}
		if r.Then.SetMood != "" {
			conv.Mood, conv.moodLeft = r.Then.SetMood, 3
		}
	}

	clarify := false
	if conf < threshold || intent == intentUnknown {
		clarify = true
	} else if second != "" {
		clarify = true
	}
	if intent == "ask_my_name" && conv.Memory["player_name"] == "" {
		intent = intentNoName
		clarify = false
	}

	// настроение
	if conv.moodLeft > 0 {
		conv.moodLeft--
		if conv.moodLeft == 0 {
			conv.Mood = ""
		}
	}
	if !clarify {
		for _, em := range e.cfg.Emotions {
			for _, t := range em.Triggers {
				if t == intent {
					conv.Mood, conv.moodLeft = em.ID, em.Decay
				}
			}
		}
	}

	var text, source string
	var facts []string
	if clarify {
		text, source = e.clarifyText(cp, conv, intent, conf, second)
	} else {
		text, facts, source = e.pickReply(cp, conv, intent)
		if text == "" { // нет безопасного ответа: уточняем, а не выдумываем
			clarify = true
			text, source = e.clarifyText(cp, conv, intentUnknown, 0, "")
			facts = nil
		}
	}
	base := text
	text = e.decorate(cp, conv, text, clarify)
	conv.Recent = append(conv.Recent, base)
	if len(conv.Recent) > maxRecent {
		conv.Recent = conv.Recent[len(conv.Recent)-maxRecent:]
	}
	conv.LastIntent = intent
	t := Turn{Player: input, Reply: text, Intent: intent, Confidence: conf, Mood: conv.Mood, Clarify: clarify, Source: source}
	conv.History = append(conv.History, t)
	_ = facts
	return t
}

func (e *Engine) hasIntent(id string) bool {
	for _, it := range e.cfg.Intents {
		if it.ID == id {
			return true
		}
	}
	return false
}

func (e *Engine) memoryEnabled(kind string) bool {
	for _, r := range e.cfg.MemoryRules {
		if r.Remember == kind {
			return true
		}
	}
	return false
}

func (e *Engine) expireMemory(conv *Conversation) {
	for _, r := range e.cfg.MemoryRules {
		if r.Remember != "player_name" {
			continue
		}
		if t, ok := conv.memTurn["player_name"]; ok && conv.Turn-t > r.TTLTurns {
			delete(conv.Memory, "player_name")
			delete(conv.memTurn, "player_name")
		}
	}
}

func ruleMatches(r Rule, conv *Conversation, intent, canon string) bool {
	w := r.When
	if w.PrevIntent != "" && w.PrevIntent != conv.LastIntent {
		return false
	}
	if w.Intent != "" && w.Intent != intent {
		return false
	}
	if w.Mood != "" && w.Mood != conv.Mood {
		return false
	}
	if len(w.ContainsAny) > 0 {
		toks := " " + canon + " "
		found := false
		for _, c := range w.ContainsAny {
			if strings.Contains(toks, " "+Normalize(c)+" ") {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return w.PrevIntent != "" || w.Intent != "" || w.Mood != "" || len(w.ContainsAny) > 0
}

type candidate struct {
	text  string
	key   string
	score int
	facts []string
}

// resolve подставляет переменные; false — в тексте есть неизвестный факт (кандидат отбрасывается).
func (e *Engine) resolve(cp Captain, conv *Conversation, t string) (string, []string, bool) {
	var used []string
	ok := true
	t = factRe.ReplaceAllStringFunc(t, func(m string) string {
		id := factRe.FindStringSubmatch(m)[1]
		txt, has := e.facts[id]
		if !has {
			ok = false
			return ""
		}
		used = append(used, id)
		return strings.TrimRight(txt, ".")
	})
	if !ok {
		return "", nil, false
	}
	player := conv.Memory["player_name"]
	if player == "" {
		player = cp.Speech.AddressPlayer
	}
	callsign := cp.CallSign
	if callsign == "" {
		callsign = cp.Name
	}
	t = strings.NewReplacer("{player}", player, "{captain}", callsign, "{topics}", strings.Join(e.topics, ", ")).Replace(t)
	return strings.TrimSpace(t), used, true
}

func styleOK(s TemplateStyle, sp Speech) bool {
	return (s.Formality == "" || s.Formality == sp.Formality) &&
		(s.Verbosity == "" || s.Verbosity == sp.Verbosity) &&
		(s.Humor == "" || s.Humor == sp.Humor)
}

func (e *Engine) pickReply(cp Captain, conv *Conversation, intent string) (string, []string, string) {
	var cands []candidate
	for i, r := range cp.Replies[intent] {
		if txt, f, ok := e.resolve(cp, conv, r); ok {
			cands = append(cands, candidate{txt, "c:" + intent + ":" + itoa(i), 10, f})
		}
	}
	for _, t := range e.cfg.Templates {
		if t.Intent != intent || !styleOK(t.Style, cp.Speech) {
			continue
		}
		if t.Mood != "" && t.Mood != conv.Mood {
			continue
		}
		score := t.Weight
		if t.Mood != "" {
			score += 6
		}
		for _, s := range []string{t.Style.Formality, t.Style.Verbosity, t.Style.Humor} {
			if s != "" {
				score += 2
			}
		}
		if txt, f, ok := e.resolve(cp, conv, t.Text); ok {
			cands = append(cands, candidate{txt, "t:" + t.ID, score, f})
		}
	}
	if len(cands) == 0 {
		return "", nil, ""
	}
	// настроение сильнее личной манеры: если есть шаблоны под текущее настроение, говорим ими
	if conv.Mood != "" {
		var moody []candidate
		for _, c := range cands {
			if strings.HasPrefix(c.key, "t:") && e.templateMood(strings.TrimPrefix(c.key, "t:")) == conv.Mood {
				moody = append(moody, c)
			}
		}
		if len(moody) > 0 {
			cands = moody
		}
	}
	// защита от повторов: не повторять недавние ответы, пока есть другие
	recent := map[string]bool{}
	for _, r := range conv.Recent {
		recent[r] = true
	}
	fresh := cands[:0:0]
	for _, c := range cands {
		if !recent[c.text] {
			fresh = append(fresh, c)
		}
	}
	pool := fresh
	if len(pool) == 0 {
		// все варианты недавно использованы: берём самый давний (по кругу), но не последний ответ
		oldest, idx := cands[0], 1<<30
		for _, c := range cands {
			pos := -1
			for i, r := range conv.Recent {
				if r == c.text {
					pos = i
				}
			}
			if pos < idx {
				oldest, idx = c, pos
			}
		}
		return oldest.text, oldest.facts, sourceOf(oldest.key)
	}
	top := 0
	for _, c := range pool {
		top = max(top, c.score)
	}
	var best []candidate
	for _, c := range pool {
		if c.score >= top-4 {
			best = append(best, c)
		}
	}
	sort.SliceStable(best, func(i, j int) bool { return best[i].key < best[j].key })
	c := best[int(hash(conv.CaptainID, intent, itoa(conv.Turn))%uint32(len(best)))]
	return c.text, c.facts, sourceOf(c.key)
}

func (e *Engine) templateMood(id string) string {
	for _, t := range e.cfg.Templates {
		if t.ID == id {
			return t.Mood
		}
	}
	return ""
}

func sourceOf(key string) string {
	if strings.HasPrefix(key, "c:") {
		return "captain"
	}
	return "template"
}

func (e *Engine) clarifyText(cp Captain, conv *Conversation, intent string, conf float64, second string) (string, string) {
	// 1) неоднозначность двух намерений
	if second != "" {
		a, b := e.title(intent), e.title(second)
		if a != "" && b != "" {
			return "Уточните, пожалуйста: вас интересует «" + a + "» или «" + b + "»?", "clarify_ambiguous"
		}
	}
	// 2) вопрос из определения намерения, если оно почти распознано
	if conf >= 0.25 && intent != intentUnknown {
		for _, it := range e.cfg.Intents {
			if it.ID == intent && it.Clarify != "" {
				return it.Clarify, "clarify_intent"
			}
		}
	}
	// 3) личные и общие ответы «не понял»
	var cands []candidate
	for i, r := range cp.Replies[intentUnknown] {
		if txt, f, ok := e.resolve(cp, conv, r); ok {
			cands = append(cands, candidate{txt, "c:u:" + itoa(i), 10, f})
		}
	}
	for _, t := range e.cfg.Templates {
		if t.Intent == intentUnknown && styleOK(t.Style, cp.Speech) {
			if txt, f, ok := e.resolve(cp, conv, t.Text); ok {
				cands = append(cands, candidate{txt, "t:" + t.ID, t.Weight, f})
			}
		}
	}
	if len(cands) == 0 {
		return "Не совсем понял. Могу рассказать про: " + strings.Join(e.topics, ", ") + ".", "clarify_default"
	}
	recent := map[string]bool{}
	for _, r := range conv.Recent {
		recent[r] = true
	}
	var pool []candidate
	for _, c := range cands {
		if !recent[c.text] {
			pool = append(pool, c)
		}
	}
	if len(pool) == 0 {
		pool = cands
	}
	sort.SliceStable(pool, func(i, j int) bool { return pool[i].key < pool[j].key })
	return pool[int(hash(conv.CaptainID, "u", itoa(conv.Turn))%uint32(len(pool)))].text, "clarify"
}

func (e *Engine) title(intent string) string {
	for _, it := range e.cfg.Intents {
		if it.ID == intent {
			return it.Title
		}
	}
	return ""
}

// decorate добавляет индивидуальную манеру: коронную фразу (не чаще раза в три реплики, не в уточнениях у лаконичных).
func (e *Engine) decorate(cp Captain, conv *Conversation, text string, clarify bool) string {
	if clarify || len(cp.Speech.Catchphrases) == 0 || cp.Speech.Verbosity == "terse" || conv.Turn-conv.lastCatch < 3 {
		return text
	}
	chance := 25
	if cp.Speech.Verbosity == "talkative" {
		chance = 50
	}
	if int(hash(conv.CaptainID, "cp", itoa(conv.Turn))%100) >= chance {
		return text
	}
	conv.lastCatch = conv.Turn
	phrase := cp.Speech.Catchphrases[int(hash(conv.CaptainID, "cpi", itoa(conv.Turn)))%len(cp.Speech.Catchphrases)]
	return text + " " + phrase
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
