package aiciv

import (
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aiagent"
)

func yekt(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, time.FixedZone("YEKT", 5*60*60))
}

func TestWindowsFollowTheDay(t *testing.T) {
	wedMorning := yekt(2026, 11, 11, 9, 0) // среда
	wedEvening := yekt(2026, 11, 11, 19, 30)
	wedNight := yekt(2026, 11, 11, 23, 15)
	wedMidday := yekt(2026, 11, 11, 13, 0)
	satDay := yekt(2026, 11, 14, 13, 0) // суббота
	if !windowOpen(ShiftLark, wedMorning) || windowOpen(ShiftLark, wedEvening) {
		t.Fatal("жаворонок только утром")
	}
	if !windowOpen(ShiftEvening, wedEvening) || windowOpen(ShiftEvening, wedMorning) {
		t.Fatal("вечерний играет вечером")
	}
	if !windowOpen(ShiftNight, wedNight) {
		t.Fatal("ночной играет ночью")
	}
	if windowOpen(ShiftNight, wedMidday) {
		t.Fatal("ночной в будний день не сидит онлайн")
	}
	if !windowOpen(ShiftNight, satDay) {
		t.Fatal("ночной в выходные играет и днём")
	}
	if !windowOpen(ShiftDay, wedMidday) || windowOpen(ShiftDay, wedNight) {
		t.Fatal("дневной играет днём")
	}
}

func TestShiftIsStableAndSpread(t *testing.T) {
	if ShiftOf("cpt-7") != ShiftOf("cpt-7") {
		t.Fatal("смена не должна меняться")
	}
	seen := map[Shift]bool{}
	for i := 0; i < 40; i++ {
		seen[ShiftOf(string(rune('a'+i))+"-cap")] = true
	}
	if len(seen) < 3 {
		t.Fatalf("смены не разъехались: %v", seen)
	}
}

func TestPickDueStaysPutUntilTheWindowEnds(t *testing.T) {
	var profiles []aiagent.Profile
	for i := 0; i < 30; i++ {
		id := string(rune('a'+i)) + "-pilot"
		profiles = append(profiles, aiagent.Profile{ID: id, Name: id, Profession: "scout"})
	}
	morning := yekt(2026, 11, 11, 8, 30)
	a := PickDue(profiles, 3, morning)
	b := PickDue(profiles, 3, morning.Add(2*time.Hour))
	if len(a) == 0 || len(a) > 3 {
		t.Fatalf("утром ждали 1..3, получили %d", len(a))
	}
	if len(a) != len(b) {
		t.Fatalf("состав прыгнул: %d и %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Fatalf("за два часа утро сменило капитана %s на %s", a[i].ID, b[i].ID)
		}
		if !OnShift(a[i].ID, morning) {
			t.Fatalf("%s не должен быть в мире утром", a[i].ID)
		}
	}
	night := yekt(2026, 11, 11, 23, 40)
	late := PickDue(profiles, 3, night)
	if len(late) == 0 {
		t.Fatal("ночью никого нет в мире")
	}
	same := 0
	for _, p := range late {
		for _, q := range a {
			if p.ID == q.ID {
				same++
			}
		}
	}
	if len(late) > 0 && same == len(late) && same == len(a) {
		t.Fatal("ночью в мире те же, что утром")
	}
}
