package rpg

import "math"

// Curve — кривая опыта: на переход с уровня n на n+1 нужно round(Base * Growth^(n-1)) очков.
type Curve struct {
	MaxLevel int     `json:"max_level"`
	Base     float64 `json:"base"`
	Growth   float64 `json:"growth"`
}

// Стартовые кривые (ориентиры, не утверждённый баланс): пилот 1–60, корабль 1–30 (АР-4).
var (
	DefaultPilotCurve = Curve{MaxLevel: 60, Base: 100, Growth: 1.08}
	DefaultShipCurve  = Curve{MaxLevel: 30, Base: 150, Growth: 1.12}
)

// Valid проверяет параметры кривой.
func (c Curve) Valid() bool { return c.MaxLevel >= 2 && c.Base >= 1 && c.Growth >= 1 }

// XPForNext — опыт для перехода с уровня level на следующий; 0 на максимальном уровне и вне диапазона.
func (c Curve) XPForNext(level int) int64 {
	if level < 1 || level >= c.MaxLevel {
		return 0
	}
	return int64(math.Round(c.Base * math.Pow(c.Growth, float64(level-1))))
}

// TotalXP — суммарный опыт, с которым достигается уровень level (уровень 1 — 0).
func (c Curve) TotalXP(level int) int64 {
	if level > c.MaxLevel {
		level = c.MaxLevel
	}
	var sum int64
	for l := 1; l < level; l++ {
		sum += c.XPForNext(l)
	}
	return sum
}

// LevelFor возвращает уровень по суммарному опыту, опыт внутри уровня и сколько осталось до следующего (0 на максимуме).
func (c Curve) LevelFor(xp int64) (level int, into int64, toNext int64) {
	if xp < 0 {
		xp = 0
	}
	level = 1
	for level < c.MaxLevel {
		need := c.XPForNext(level)
		if xp < need {
			return level, xp, need - xp
		}
		xp -= need
		level++
	}
	return level, xp, 0
}

// AttributePoints — суммарно очков характеристик пилота на уровне level (по 2 за уровень выше первого).
func AttributePoints(level int) int {
	if level <= 1 {
		return 0
	}
	return (level - 1) * 2
}

// TalentPoints — суммарно очков талантов на уровне level (по 1 за уровень выше первого).
func TalentPoints(level int) int {
	if level <= 1 {
		return 0
	}
	return level - 1
}
