package aiciv

import (
	"hash/fnv"
	"sort"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aiagent"
)

// Shift — когда капитан сам заходит в игру. Часы по Уфе (UTC+5), как у живых игроков оттуда.
type Shift string

const (
	ShiftLark    Shift = "lark"    // утро
	ShiftDay     Shift = "day"     // день
	ShiftEvening Shift = "evening" // вечер
	ShiftNight   Shift = "night"   // ночь и ещё день в выходные
)

// ShiftOf стабилен для одного капитана: жаворонок не становится полуночником после перезапуска.
func ShiftOf(id string) Shift {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	switch h.Sum32() % 4 {
	case 0:
		return ShiftLark
	case 1:
		return ShiftDay
	case 2:
		return ShiftEvening
	default:
		return ShiftNight
	}
}

func gameClock(now time.Time) time.Time {
	loc, err := time.LoadLocation("Asia/Yekaterinburg")
	if err != nil {
		loc = time.FixedZone("YEKT", 5*60*60)
	}
	return now.In(loc)
}

// OnShift — пора ли этому капитану быть в мире.
// Окно смены чуть шире общего, и у каждого свой сдвиг: не все входят и выходят в одну минуту.
func OnShift(id string, now time.Time) bool {
	local := gameClock(now).Add(-time.Duration(slideMin(id)) * time.Minute)
	return windowOpen(ShiftOf(id), local)
}

// slideMin сдвигает личные часы на −45…+45 минут. Один и тот же капитан всегда с тем же сдвигом.
func slideMin(id string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte("slide:"))
	_, _ = h.Write([]byte(id))
	return int(h.Sum32()%91) - 45
}

func windowOpen(shift Shift, local time.Time) bool {
	mins := local.Hour()*60 + local.Minute()
	weekend := local.Weekday() == time.Saturday || local.Weekday() == time.Sunday
	switch shift {
	case ShiftLark:
		return between(mins, 6*60+40, 12*60+40)
	case ShiftDay:
		return between(mins, 10*60+40, 18*60+40)
	case ShiftEvening:
		return between(mins, 16*60+40, 24*60)
	default:
		if mins >= 21*60+30 || mins < 7*60+40 {
			return true
		}
		return weekend && between(mins, 10*60+40, 16*60+40)
	}
}

func between(mins, from, to int) bool {
	return mins >= from && mins < to
}

// PickDue берёт не больше n капитанов, у которых сейчас своё игровое время.
// Состав не прыгает между тиками: порядок фиксирован, пока окно не кончилось.
func PickDue(in []aiagent.Profile, n int, now time.Time) []aiagent.Profile {
	due := make([]aiagent.Profile, 0, len(in))
	for _, p := range in {
		if p.ID == "" || p.Name == "" {
			continue
		}
		if !OnShift(p.ID, now) {
			continue
		}
		due = append(due, p)
	}
	sort.Slice(due, func(i, j int) bool { return due[i].ID < due[j].ID })
	if len(due) == 0 {
		return []aiagent.Profile{}
	}
	return PickCast(due, n)
}
