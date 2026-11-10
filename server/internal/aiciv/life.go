package aiciv

import (
	"hash/fnv"
	"math"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// Life — где капитану быть и чем заниматься. Система зависит от прогресса контрактов,
// а точка внутри системы — от идентификатора, чтобы люди не вставали друг на друга.
type Life struct {
	System string
	Goal   GoalKind
	JumpTo string
}

// LifeOf строит план. haveQuest=false — контракты ещё не загружены, тогда решает число
// завершённых и профессия. Прыжок не телепортирует: JumpTo значит «лететь к воротам».
func LifeOf(captainID string, done int, haveQuest bool, kind, questSystem, target, current string, fallback GoalKind) Life {
	if current == "" {
		current = "aurora"
	}
	if !haveQuest || kind == "" {
		return Life{System: systemForProgress(done), Goal: orGoal(fallback, captainID)}
	}
	if kind == "jump" && target != "" {
		return Life{System: current, Goal: GoalJump, JumpTo: target}
	}
	if questSystem != "" && questSystem != current {
		return Life{System: current, Goal: GoalJump, JumpTo: questSystem}
	}
	sys := questSystem
	if sys == "" {
		sys = current
	}
	return Life{System: sys, Goal: goalForQuest(kind, fallback)}
}

func systemForProgress(done int) string {
	switch {
	case done >= 9:
		return "redstorm"
	case done >= 5:
		return "meridian"
	default:
		return "aurora"
	}
}

func goalForQuest(kind string, fallback GoalKind) GoalKind {
	switch kind {
	case "kill":
		return GoalFarmCredits
	case "mine", "deliver":
		return GoalMineAndSell
	case "sell", "reach", "talk":
		return GoalTradeRun
	case "visit":
		return GoalExplore
	case "jump":
		return GoalJump
	default:
		return orGoal(fallback, "")
	}
}

func orGoal(fallback GoalKind, id string) GoalKind {
	if fallback != "" && fallback != GoalIdleDock {
		return fallback
	}
	switch hashID(id) % 4 {
	case 0:
		return GoalMineAndSell
	case 1:
		return GoalFarmCredits
	case 2:
		return GoalExplore
	default:
		return GoalTradeRun
	}
}

func hashID(id string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return h.Sum32()
}

// Scatter — точка вокруг станции. Близко, в поясе и дальше по кругу, но не в одной координате.
func Scatter(captainID string, station [2]float64) world.Vec {
	h := hashID(captainID)
	var radius float64
	switch h % 4 {
	case 0:
		radius = 70 + float64(h%90)
	case 1:
		radius = 200 + float64((h/8)%260)
	case 2:
		radius = 480 + float64((h/16)%420)
	default:
		radius = 320 + float64((h/32)%300)
	}
	angle := float64(h%6283) / 1000
	return world.Vec{X: station[0] + math.Cos(angle)*radius, Y: station[1] + math.Sin(angle)*radius}
}
