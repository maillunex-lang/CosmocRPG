package world

import "math"

// TurnRate — скорость разворота корабля к цели во время атаки, рад/с (разворот на 180° занимает около 0,8 с).
const TurnRate = 4.0

// turnToward поворачивает направление cur к want не больше чем на maxRad; нулевое cur принимает want сразу.
func turnToward(cur, want Vec, maxRad float64) Vec {
	want = want.normalized()
	if want == (Vec{}) {
		return cur
	}
	if cur.length() < 1e-9 {
		return want
	}
	cur = cur.normalized()
	diff := math.Atan2(want.Y, want.X) - math.Atan2(cur.Y, cur.X)
	for diff > math.Pi {
		diff -= 2 * math.Pi
	}
	for diff < -math.Pi {
		diff += 2 * math.Pi
	}
	if math.Abs(diff) <= maxRad {
		return want
	}
	a := math.Atan2(cur.Y, cur.X) + math.Copysign(maxRad, diff)
	return Vec{math.Cos(a), math.Sin(a)}
}

// faceAttackTarget разворачивает корабль носом к цели, пока идёт атака (R-49): корабль стреляет не «кормой».
// Направление движения от этого не зависит: корабль может лететь боком к цели и смотреть на неё.
func (w *World) faceAttackTarget(p *Player, dt float64) {
	if !p.Attacking || p.deadUntil > w.t {
		return
	}
	n := w.npcByID(p.Target, nil)
	if n == nil || n.deadUntil > w.t {
		return
	}
	d := n.Pos.sub(p.Pos)
	if d.length() < 1e-6 {
		return
	}
	p.Dir = turnToward(p.Dir, d, TurnRate*dt)
}
