package aiciv

import (
	"github.com/maillunex-lang/CosmocRPG/server/internal/aiagent"
)

// GoalForProfession returns a long-term goal based on the captain's profession.
// Unknown professions fall back to idle_dock (do not wander).
func GoalForProfession(profession string) GoalKind {
	switch aiagent.BehaviorFor(profession) {
	case "patrol":
		return GoalFarmCredits
	case "miner":
		return GoalMineAndSell
	case "trader":
		return GoalTradeRun
	case "scout":
		return GoalExplore
	default:
		return GoalIdleDock
	}
}

// GoalForProfile is a convenience over Profile.
func GoalForProfile(p aiagent.Profile) GoalKind {
	return GoalForProfession(p.Profession)
}
