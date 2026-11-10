package aiciv

import "hash/fnv"

// Archetype is a stable personality. The same captain keeps it across restarts.
type Archetype string

const (
	ArchPro     Archetype = "pro"
	ArchNoob    Archetype = "noob"
	ArchGrinder Archetype = "grinder"
	ArchKnow    Archetype = "know"
	ArchDull    Archetype = "dull"
	ArchTroll   Archetype = "troll"
	ArchHelp    Archetype = "help"
)

// ArchetypeFor picks a personality from the captain id, shifted by what they do.
func ArchetypeFor(id string, goal GoalKind) Archetype {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	n := h.Sum32()
	var shift uint32
	switch goal {
	case GoalMineAndSell:
		shift = 2
	case GoalExplore:
		shift = 3
	case GoalTradeRun:
		shift = 2
	case GoalIdleDock:
		shift = 6
	default:
		shift = 0
	}
	switch (n + shift) % 7 {
	case 0:
		return ArchPro
	case 1:
		return ArchNoob
	case 2:
		return ArchGrinder
	case 3:
		return ArchKnow
	case 4:
		return ArchDull
	case 5:
		return ArchTroll
	default:
		return ArchHelp
	}
}

func askKind(goal GoalKind) PactKind {
	switch goal {
	case GoalMineAndSell:
		return PactWing
	case GoalExplore:
		return PactQuest
	case GoalIdleDock:
		return PactWing
	default:
		return PactPirates
	}
}

// willHelp decides if a personality really comes, only talks, or lies.
func willHelp(a Archetype, goal GoalKind, kind PactKind) string {
	switch a {
	case ArchTroll:
		return "lie"
	case ArchDull:
		if kind == PactWing {
			return "go"
		}
		return "refuse"
	case ArchGrinder:
		if kind == PactPirates && (goal == GoalMineAndSell || goal == GoalTradeRun) {
			return "refuse"
		}
		return "go"
	default:
		if goal == GoalTradeRun && kind == PactPirates && a != ArchHelp && a != ArchNoob && a != ArchPro {
			return "refuse"
		}
		return "go"
	}
}

func requestLines(a Archetype, kind PactKind) []string {
	switch kind {
	case PactQuest:
		switch a {
		case ArchNoob:
			return []string{"я на квесте застрял, кто-нибудь со мной?", "не понимаю куда жать дальше по квесту, проведите?"}
		case ArchKnow:
			return []string{"на квесте нужен второй ствол, я знаю точку.", "цель квеста живая, один я её не сниму."}
		case ArchTroll:
			return []string{"квест лёгкий, кто со мной пройдёт его за меня?", "есть квест, награда моя, работа ваша."}
		default:
			return []string{"на квесте нужна пара, кто свободен?", "застрял на цели квеста, прикройте."}
		}
	case PactWing:
		switch a {
		case ArchNoob:
			return []string{"возьмите меня в звено, я буду стараться.", "можно с вами? один я теряюсь."}
		case ArchGrinder:
			return []string{"нужно звено на одну жилу, без лишнего чата.", "кто фармит рядом, соберёмся и закроем быстрее."}
		case ArchTroll:
			return []string{"набираю звено в самую жару. кто со мной?", "ищу ведомых. опыт не важен."}
		default:
			return []string{"собираю звено, держитесь моего курса.", "кто в пару? без толпы, просто рядом."}
		}
	default:
		switch a {
		case ArchNoob:
			return []string{"меня бьют пираты, помогите, я ещё не все кнопки знаю!", "пожалуйста, тут враги, я их не вывожу."}
		case ArchPro:
			return []string{"пираты на встречном. кто со мной, без лишнего геройства.", "нужен второй ствол по этим двоим."}
		case ArchGrinder:
			return []string{"мешают фарму. снимете их — и я дальше.", "пираты сели на жилу. кто сбросит?"}
		case ArchKnow:
			return []string{"у перехватчика щит слабый с кормы. нужен ещё один.", "это не стражи, это пираты. кто зайдёт со мной?"}
		case ArchDull:
			return []string{"там эти, стреляют. ну это. помогите.", "меня бьют. чего делать?"}
		case ArchTroll:
			return []string{"кто самый смелый, один на один с пиратом? я посмотрю.", "пираты злые, спасайте меня, а я вам потом расскажу как надо было."}
		default:
			return []string{"пираты на хвосте. если можете — прикройте.", "мне тут жарко, нужна пара стволов."}
		}
	}
}

func acceptLine(a Archetype, kind PactKind) string {
	switch a {
	case ArchNoob:
		return "я щас, я почти умею! лечу к тебе."
	case ArchGrinder:
		if kind == PactWing {
			return "годится. подхожу, пофармим и разойдёмся."
		}
		return "ладно, но быстро. иду к тебе."
	case ArchKnow:
		return "подойду с кормы, не перекрывай мне линию."
	case ArchDull:
		return "а куда лететь? ладно, вроде туда."
	case ArchHelp:
		return "вижу тебя, выдвигаюсь."
	case ArchPro:
		return "иду к тебе. бей ближнего, я возьму второго."
	default:
		return "принял, лечу."
	}
}

func refuseLine(a Archetype, kind PactKind) string {
	switch a {
	case ArchDull:
		return "чего? я не понял, что от меня надо."
	case ArchGrinder:
		if kind == PactPirates {
			return "не могу, у меня смена и трюм. сами снимите."
		}
		return "сейчас нельзя, я на цикле."
	default:
		return "не в этот заход. у меня своё."
	}
}

func lieLine(a Archetype) string {
	if a == ArchTroll {
		return "уже вылетаю, жди меня у обломков."
	}
	return "сейчас буду."
}

func personaLines(a Archetype, goal GoalKind) []string {
	switch a {
	case ArchNoob:
		return []string{"я вроде попал, это считается?", "кто-нибудь, я правильно лечу или опять мимо?"}
	case ArchGrinder:
		return []string{"фармлю, не отвлекайте без дела.", "цикл идёт. чат потом."}
	case ArchKnow:
		return []string{"если что, щит у таких слабее с кормы, не в лоб.", "я этот край уже смотрел, там не то, что вы думаете."}
	case ArchDull:
		return []string{"лечу. стреляю. нормально.", "чего у вас опять случилось?"}
	case ArchTroll:
		return []string{"кто орёт в чат, тот и есть цель.", "красивый корабль. жалко будет, если его снимут."}
	case ArchHelp:
		return []string{"я на связи. если прижмут — зовите.", "могу подлететь, только не в одиночку в толпу."}
	default:
		if goal == GoalMineAndSell {
			return []string{"работаю по жиле, сектор пока держу."}
		}
		return []string{"сектор грязный, работаю по ближайшему.", "без лишнего шума. если нужна пара — пишите по делу."}
	}
}
