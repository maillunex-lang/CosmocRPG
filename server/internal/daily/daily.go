// Package daily defines deterministic daily activities and return rewards.
package daily

import "time"

type Activity struct {
	ID string `json:"id"`
	Title string `json:"title"`
	Kind string `json:"kind"`
	Goal int `json:"goal"`
	Reward int64 `json:"reward"`
}

type Snapshot struct {
	Day string `json:"day"`
	Activities []Activity `json:"activities"`
	Streak int `json:"streak"`
	CanClaimReturn bool `json:"can_claim_return"`
	ReturnReward int64 `json:"return_reward"`
}

func Day(now time.Time) string { return now.UTC().Format("2006-01-02") }

func Activities(now time.Time) []Activity {
	sets := [][]Activity{
		{{"daily_mine", "Сбор ресурсов", "mine", 3, 180}, {"daily_trade", "Торговый маршрут", "sell", 1, 260}, {"daily_jump", "Дальний прыжок", "jump", 1, 220}},
		{{"daily_mine", "Сбор ресурсов", "mine", 4, 220}, {"daily_trade", "Торговый маршрут", "sell", 2, 420}, {"daily_visit", "Исследователь", "visit", 1, 240}},
		{{"daily_mine", "Сбор ресурсов", "mine", 5, 280}, {"daily_contract", "Курьерская смена", "deliver", 1, 500}, {"daily_jump", "Дальний прыжок", "jump", 2, 420}},
	}
	week := int(now.UTC().Unix() / 86400)
	return append([]Activity(nil), sets[week%len(sets)]...)
}

func ReturnReward(streak int) int64 {
	if streak < 1 { streak = 1 }
	if streak > 7 { streak = 7 }
	return int64(150 + streak*50)
}
