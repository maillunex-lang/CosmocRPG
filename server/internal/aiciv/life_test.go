package aiciv

import (
	"math"
	"testing"
)

func TestLifeFollowsQuestProgress(t *testing.T) {
	aurora := LifeOf("cpt-1", 0, false, "", "", "", "aurora", GoalMineAndSell)
	if aurora.System != "aurora" || aurora.Goal != GoalMineAndSell {
		t.Fatalf("новичок: %+v", aurora)
	}
	mid := LifeOf("cpt-1", 6, false, "", "", "", "aurora", GoalMineAndSell)
	if mid.System != "meridian" {
		t.Fatalf("середина сюжета: %+v", mid)
	}
	far := LifeOf("cpt-1", 10, false, "", "", "", "meridian", GoalExplore)
	if far.System != "redstorm" {
		t.Fatalf("конец акта: %+v", far)
	}
	jump := LifeOf("cpt-1", 4, true, "jump", "", "meridian", "aurora", GoalMineAndSell)
	if jump.Goal != GoalJump || jump.JumpTo != "meridian" || jump.System != "aurora" {
		t.Fatalf("прыжок не телепорт: %+v", jump)
	}
	cross := LifeOf("cpt-1", 6, true, "deliver", "meridian", "iron_ore", "aurora", GoalMineAndSell)
	if cross.Goal != GoalJump || cross.JumpTo != "meridian" {
		t.Fatalf("чужая система — сначала ворота: %+v", cross)
	}
	mine := LifeOf("cpt-1", 2, true, "mine", "", "iron_ore", "aurora", GoalExplore)
	if mine.Goal != GoalMineAndSell || mine.System != "aurora" {
		t.Fatalf("добыча: %+v", mine)
	}
}

func TestScatterSeparatesCaptains(t *testing.T) {
	station := [2]float64{100, -40}
	a := Scatter("cpt-0001", station)
	b := Scatter("cpt-0042", station)
	if math.Hypot(a.X-b.X, a.Y-b.Y) < 40 {
		t.Fatalf("капитаны слишком близко: %+v %+v", a, b)
	}
	if math.Hypot(a.X-station[0], a.Y-station[1]) < 60 {
		t.Fatalf("слишком близко к станции: %+v", a)
	}
}
