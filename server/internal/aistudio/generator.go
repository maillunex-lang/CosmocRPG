package aistudio

import (
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Генератор стартовых профилей. Детерминирован: профиль номера N зависит только от seed и N.
// Повторный запуск ничего не перезаписывает: существующие капитаны (особенно правленные вручную) пропускаются.

var (
	genderPair = regexp.MustCompile(`⟨([^|⟩]*)\|([^⟩]*)⟩`)
	suffixLa   = regexp.MustCompile(`(\p{L}+)\(ла\)`)
	suffixLas  = regexp.MustCompile(`(\p{L}+)\(лась\)`)
	suffixA    = regexp.MustCompile(`(\p{L}+)\(а\)`)
)

// gender выбирает формы слов по полу: ⟨м|ж⟩ и сокращённые окончания (а), (ла), (лась).
func gender(s, g string) string {
	if g == "f" {
		s = genderPair.ReplaceAllString(s, "$2")
		s = suffixLas.ReplaceAllStringFunc(s, func(m string) string {
			w := suffixLas.FindStringSubmatch(m)[1]
			return strings.TrimSuffix(w, "ся") + "ась"
		})
		s = suffixLa.ReplaceAllStringFunc(s, func(m string) string {
			w := suffixLa.FindStringSubmatch(m)[1]
			if strings.HasSuffix(w, "ёл") {
				return strings.TrimSuffix(w, "ёл") + "ла"
			}
			return w + "ла"
		})
		return suffixA.ReplaceAllString(s, "${1}а")
	}
	s = genderPair.ReplaceAllString(s, "$1")
	s = suffixLas.ReplaceAllString(s, "$1")
	s = suffixLa.ReplaceAllString(s, "$1")
	return suffixA.ReplaceAllString(s, "$1")
}

type rng struct{ *rand.Rand }

func (r rng) pick(n int) int { return r.Intn(n) }

func pickS(r rng, s []string) string { return s[r.Intn(len(s))] }

func (r rng) sample(src []string, n int) []string {
	idx := r.Perm(len(src))
	if n > len(src) {
		n = len(src)
	}
	out := make([]string, 0, n)
	for _, i := range idx[:n] {
		out = append(out, src[i])
	}
	return out
}

func weighted(r rng, w []int) int {
	t := 0
	for _, x := range w {
		t += x
	}
	if t == 0 {
		return 0
	}
	v := r.Intn(t)
	for i, x := range w {
		if v < x {
			return i
		}
		v -= x
	}
	return len(w) - 1
}

func capFirst(s string) string {
	if s == "" {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return strings.ToUpper(string(r)) + s[n:]
}

func pronounCap(g string) string {
	if g == "f" {
		return "Она"
	}
	return "Он"
}

// adjForm возвращает форму прилагательного по полу: -ый/-ой/-ий → -ая для женского рода.
func adjForm(a, g string) string {
	if g != "f" {
		return a
	}
	for _, e := range []string{"ый", "ой", "ий"} {
		if strings.HasSuffix(a, e) {
			return strings.TrimSuffix(a, e) + "ая"
		}
	}
	return a
}

func professionKeys() []string {
	return append([]string{}, Professions...)
}

// GenerateOptions — параметры генерации.
type GenerateOptions struct {
	Seed  int64
	Count int // сколько капитанов в реестре должно быть (1..400)
}

// GenerateResult — итог генерации.
type GenerateResult struct {
	Added   []string `json:"added"`
	Skipped []string `json:"skipped"` // уже существовали (в том числе правленные вручную)
}

// Generate добавляет в конфигурацию недостающих капитанов с номерами 1..Count.
// Существующие профили не изменяются и не перезаписываются.
func Generate(c *Config, o GenerateOptions) GenerateResult {
	EnsureDefaults(c)
	if o.Count < 1 {
		o.Count = 1
	}
	if o.Count > 400 {
		o.Count = 400
	}
	used := map[string]bool{}
	usedCS := map[string]bool{}
	usedBio := map[string]bool{}
	usedSig := map[string]bool{}
	for _, e := range c.Captains {
		used[strings.ToLower(e.Name)] = true
		usedCS[strings.ToLower(e.CallSign)] = true
		usedBio[e.Bio] = true
		usedSig[e.Signature()] = true
	}
	var res GenerateResult
	var created []string
	for i := 1; i <= o.Count; i++ {
		id := fmt.Sprintf("cpt-%04d", i)
		if _, ok := c.Captains[id]; ok {
			res.Skipped = append(res.Skipped, id)
			continue
		}
		var cp Captain
		for attempt := 0; attempt < 60; attempt++ {
			cp = genCaptain(c, o.Seed, i, attempt)
			if used[strings.ToLower(cp.Name)] || usedBio[cp.Bio] || usedSig[cp.Signature()] {
				continue
			}
			if cp.CallSign != "" && usedCS[strings.ToLower(cp.CallSign)] {
				cp.CallSign = ""
				if attempt < 30 {
					continue
				}
			}
			break
		}
		used[strings.ToLower(cp.Name)] = true
		usedCS[strings.ToLower(cp.CallSign)] = true
		usedBio[cp.Bio] = true
		usedSig[cp.Signature()] = true
		c.Captains[id] = cp
		res.Added = append(res.Added, id)
		created = append(created, id)
	}
	assignRelationships(c, created, o.Seed)
	return res
}

func genCaptain(c *Config, seed int64, idx, attempt int) Captain {
	r := rng{rand.New(rand.NewSource(seed*1000003 + int64(idx)*7919 + int64(attempt)*104729))}
	id := fmt.Sprintf("cpt-%04d", idx)
	g := "m"
	if r.Intn(100) < 46 {
		g = "f"
	}
	// игровой ник вместо «имя фамилия»: русские и английские вперемешку, по номеру капитана
	nick := NickFor(seed, idx, attempt)
	prof := Professions[r.Intn(len(Professions))]
	pd := profTable[prof]

	// фракция и корпорация согласованы с профессией
	fac := pd.factions[r.Intn(len(pd.factions))]
	var corp *string
	var corpName string
	if r.Intn(100) < 70 {
		var opts []Corporation
		for _, k := range c.Corporations {
			if k.Faction == fac {
				opts = append(opts, k)
			}
		}
		if len(opts) > 0 {
			k := opts[r.Intn(len(opts))]
			v := k.ID
			corp, corpName = &v, k.Name
		}
	}

	// характер: 3-5 несовместимых черт
	n := 3 + r.Intn(3)
	var traits []string
	for _, t := range r.sample(traitPool, len(traitPool)) {
		bad := false
		for _, have := range traits {
			for _, cf := range traitConflict[have] {
				if cf == t {
					bad = true
				}
			}
			for _, cf := range traitConflict[t] {
				if cf == have {
					bad = true
				}
			}
		}
		if !bad {
			traits = append(traits, t)
		}
		if len(traits) == n {
			break
		}
	}
	scores := map[string]int{}
	for _, k := range ScoreKeys {
		scores[k] = 50 + r.Intn(21) - 10
	}
	for _, t := range traits {
		for k, d := range traitScores[t] {
			scores[k] += d
		}
	}
	for k, v := range scores {
		scores[k] = max(0, min(100, v))
	}
	has := func(t string) bool {
		for _, x := range traits {
			if x == t {
				return true
			}
		}
		return false
	}
	temper := "calm"
	switch {
	case has("вспыльчивый") || has("дерзкий") || has("импульсивный"):
		temper = "hot"
	case has("хладнокровный") || has("циничный") || has("молчаливый") || has("недоверчивый"):
		temper = "cold"
	case has("жизнерадостный") || has("добродушный") || has("общительный") || has("романтичный"):
		temper = "cheerful"
	}

	// речь
	forms := []string{"formal", "neutral", "casual"}
	formality := forms[weighted(r, pd.formal[:])]
	slang := 0
	switch formality {
	case "neutral":
		slang = r.Intn(2)
		if has("болтливый") || has("азартный") {
			slang++
		}
	case "casual":
		slang = 1 + r.Intn(3)
	}
	verbosity := "normal"
	switch {
	case has("молчаливый") || has("хладнокровный") || has("скрытный"):
		verbosity = "terse"
	case has("болтливый") || has("общительный"):
		verbosity = "talkative"
	case r.Intn(100) < 15:
		verbosity = []string{"terse", "talkative"}[r.Intn(2)]
	}
	humor := "none"
	switch {
	case has("язвительный") || has("циничный"):
		humor = "sarcastic"
	case has("ироничный"):
		humor = "dry"
	case has("жизнерадостный") || has("добродушный") || has("азартный"):
		humor = "playful"
	case r.Intn(100) < 35:
		humor = []string{"dry", "playful"}[r.Intn(2)]
	}
	if formality == "formal" && humor == "sarcastic" {
		humor = "dry"
	}
	addr := map[string][]string{
		"formal":  {"капитан", "коллега", "уважаемый"},
		"neutral": {"капитан", "коллега", "пилот"},
		"casual":  {"друг", "дружище", "капитан", "приятель", "пилот"},
	}[formality]
	catch := []string{pickS(r, pd.proverbs), pickS(r, genericCatch[formality])}
	if r.Intn(100) < 40 {
		extra := pickS(r, genericCatch[formality])
		if extra != catch[1] {
			catch = append(catch, extra)
		}
	}
	var avoid []string
	if formality == "formal" {
		avoid = r.sample(slangWords, 3)
	}
	speech := Speech{Formality: formality, Verbosity: verbosity, SlangLevel: slang, Humor: humor, Catchphrases: catch,
		Quirks: r.sample(quirkPool, 1+r.Intn(2)), AddressPlayer: pickS(r, addr), AvoidWords: avoid}
	if avoid == nil {
		speech.AvoidWords = []string{}
	}

	// интересы, цели, стратегии
	interests := r.sample(pd.interests, 2)
	addInterest := func() {
		extra := pickS(r, generalInterests)
		for _, x := range interests {
			if x == extra {
				return
			}
		}
		interests = append(interests, extra)
	}
	addInterest()
	if len(interests) < 3 {
		addInterest()
	}
	if r.Intn(100) < 40 {
		addInterest()
	}
	var goals []Goal
	gl := r.Perm(len(pd.goals))
	ng := 2
	if r.Intn(100) < 40 {
		ng = 3
	}
	for k := 0; k < min(ng, len(gl)); k++ {
		gg := pd.goals[gl[k]]
		goals = append(goals, Goal{ID: gg[0], Text: capFirst(gg[1]), Priority: k + 1})
	}
	strategies := r.sample(pd.strategy, 2+r.Intn(2))

	// биография
	careerLine := pd.career[r.Intn(len(pd.career))]
	t1, t2 := adjForm(traits[0], g), adjForm(traits[1], g)
	topGoal := strings.ToLower(pd.goals[gl[0]][1])
	bio := strings.Join([]string{
		fmt.Sprintf("%s %s.", "⟨Родился|Родилась⟩", pickS(r, originPlaces)),
		careerLine,
		fmt.Sprintf("%s — %s и %s: %s, а %s.", pronounCap(g), t1, t2, traitClauses[traits[0]], traitClauses[traits[1]]),
		fmt.Sprintf("Любит %s и %s.", interests[0], interests[1]),
		fmt.Sprintf("Сейчас главная цель — %s.", topGoal),
	}, " ")
	bio = gender(bio, g)

	age := 22 + r.Intn(40)
	if prof == "учёный" || prof == "дипломат" {
		age += 8
	}
	if age > 90 {
		age = 90
	}
	callSign := "" // ник уже является игровым именем; отдельный позывной не нужен

	cp := Captain{
		ID: id, Revision: 1, Name: nick, CallSign: callSign, Gender: g, Age: age, Bio: bio, Profession: prof,
		Faction: fac, Corporation: corp,
		Personality: Personality{Traits: traits, Scores: scores, Temper: temper},
		Speech:      speech, Interests: interests, Goals: goals, Strategies: strategies, Relationships: []Relationship{},
		Meta: CaptainMeta{Manual: false, Source: "generator", Seed: seed},
	}
	cp.Replies = buildReplies(r, cp, pd, corpName, c)
	return cp
}

func buildReplies(r rng, cp Captain, pd profData, corpName string, c *Config) map[string][]string {
	facName := cp.Faction
	for _, f := range c.Factions {
		if f.ID == cp.Faction {
			facName = f.Name
		}
	}
	corpLine := "Работаю сам по себе."
	if corpName != "" {
		corpLine = "Работаю на «" + corpName + "»."
	}
	job := pd.job
	goal := strings.ToLower(cp.Goals[0].Text)
	short := capFirst(pd.job) + "."
	rep := strings.NewReplacer("[[job]]", job, "[[job_cap]]", capFirst(job), "[[job_short]]", short,
		"[[goal]]", goal, "[[goal_cap]]", capFirst(goal), "[[faction]]", facName, "[[corp_line]]", corpLine)
	out := map[string][]string{}
	intents := make([]string, 0, len(coreReplies))
	for k := range coreReplies {
		intents = append(intents, k)
	}
	sort.Strings(intents) // порядок обхода фиксирован: иначе результат зависел бы от порядка обхода карты
	for _, intent := range intents {
		byForm := coreReplies[intent]
		cores := byForm[cp.Speech.Formality]
		order := r.Perm(len(cores))
		count := min(3, len(cores))
		for k := 0; k < count; k++ {
			text := rep.Replace(cores[order[k]])
			// префикс по характеру: не у всех и не всегда
			if intent != "unknown" && intent != "insult" && r.Intn(100) < 45 {
				pre := prefixTable[cp.Speech.Formality][cp.Personality.Temper]
				p := pre[r.Intn(len(pre))]
				if strings.HasSuffix(p, ",") || strings.HasSuffix(p, ":") {
					text = p + " " + lowerFirst(text)
				} else {
					text = p + " " + text
				}
			}
			out[intent] = append(out[intent], strings.TrimSpace(text))
		}
	}
	return out
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	// {player}, {captain} и т.п. не трогаем
	if strings.HasPrefix(s, "{") {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return strings.ToLower(string(r)) + s[n:]
}

// assignRelationships задаёт новым капитанам 2-4 отношения к существующим: общие корпорации и фракции дружат,
// враждебные фракции соперничают. Существующие капитаны не изменяются.
func assignRelationships(c *Config, created []string, seed int64) {
	if len(created) == 0 {
		return
	}
	all := make([]string, 0, len(c.Captains))
	for id := range c.Captains {
		all = append(all, id)
	}
	sort.Strings(all)
	hostile := map[string]string{}
	for _, rel := range c.Relations {
		hostile[rel.A+"|"+rel.B] = rel.Kind
		hostile[rel.B+"|"+rel.A] = rel.Kind
	}
	for _, id := range created {
		cp := c.Captains[id]
		r := rng{rand.New(rand.NewSource(seed*7 + int64(len(id)) + int64(hash(id))))}
		n := 2 + r.Intn(3)
		seen := map[string]bool{id: true}
		for tries := 0; tries < 80 && len(cp.Relationships) < n; tries++ {
			oid := all[r.Intn(len(all))]
			if seen[oid] {
				continue
			}
			seen[oid] = true
			o := c.Captains[oid]
			kind := "neutral"
			switch {
			case cp.Corporation != nil && o.Corporation != nil && *cp.Corporation == *o.Corporation:
				kind = []string{"partner", "friend", "friend"}[r.Intn(3)]
			case cp.Faction == o.Faction:
				kind = []string{"friend", "friend", "rival", "neutral"}[r.Intn(4)]
			case hostile[cp.Faction+"|"+o.Faction] == "hostile":
				kind = []string{"enemy", "rival", "rival"}[r.Intn(3)]
			case hostile[cp.Faction+"|"+o.Faction] == "tense":
				kind = []string{"rival", "neutral"}[r.Intn(2)]
			case hostile[cp.Faction+"|"+o.Faction] == "allied" || hostile[cp.Faction+"|"+o.Faction] == "friendly":
				kind = []string{"friend", "neutral", "partner"}[r.Intn(3)]
			}
			note := relationNotes[kind][r.Intn(len(relationNotes[kind]))]
			cp.Relationships = append(cp.Relationships, Relationship{CaptainID: oid, Kind: kind, Note: note})
		}
		c.Captains[id] = cp
	}
}
