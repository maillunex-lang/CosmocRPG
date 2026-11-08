package world

import (
	"math"
	"testing"
)

func angleBetween(a, b Vec) float64 {
	d := math.Atan2(b.Y, b.X) - math.Atan2(a.Y, a.X)
	for d > math.Pi {
		d -= 2 * math.Pi
	}
	for d < -math.Pi {
		d += 2 * math.Pi
	}
	return math.Abs(d)
}

func TestTurnToward(t *testing.T) {
	if got := turnToward(Vec{1, 0}, Vec{0, 1}, math.Pi); angleBetween(got, Vec{0, 1}) > 1e-9 {
		t.Fatalf("малый угол достигается сразу: %+v", got)
	}
	got := turnToward(Vec{1, 0}, Vec{-1, 0.0001}, 0.5)
	if d := angleBetween(Vec{1, 0}, got); math.Abs(d-0.5) > 1e-9 {
		t.Fatalf("разворот ограничен скоростью: %v", d)
	}
	if got := turnToward(Vec{}, Vec{0, 1}, 0.1); got != (Vec{0, 1}) {
		t.Fatalf("без направления принимается цель: %+v", got)
	}
	if got := turnToward(Vec{1, 0}, Vec{}, 0.1); got != (Vec{1, 0}) {
		t.Fatalf("нулевая цель не меняет направление: %+v", got)
	}
}

func TestShipFacesTargetWhileAttacking(t *testing.T) {
	w := New()
	p, _ := w.Join(Record{ID: "a", Name: "a", Pos: Vec{0, 0}, Dir: Vec{0, 1}})
	var tgt *npc
	for _, n := range w.npcs {
		if n.Kind == "hostile" {
			tgt = n
			break
		}
	}
	if tgt == nil {
		t.Skip("нет враждебной цели")
	}
	p.Pos = tgt.Pos.add(Vec{0, 20}) // цель строго «внизу», нос смотрит «вверх» (кормой к цели)
	p.Dir = Vec{0, 1}
	if !w.SetTarget("a", tgt.ID) {
		t.Fatal("цель не выбрана")
	}
	stepFor(w, 0.5)
	if a := angleBetween(p.Dir, Vec{0, 1}); a > 0.1 {
		t.Fatalf("без атаки корабль не должен разворачиваться к цели (поворот %.2f)", a)
	}
	if err := w.SetAttack("a", true); err != nil {
		t.Fatal(err)
	}
	stepFor(w, 0.1)
	if a := angleBetween(p.Dir, Vec{0, 1}); a > TurnRate*0.1+0.05 {
		t.Fatalf("разворот должен быть плавным: за 0,1 с повернулся на %.2f рад", a)
	}
	stepFor(w, 1.5)
	want := tgt.Pos.sub(p.Pos).normalized()
	if a := angleBetween(p.Dir, want); a > 0.05 {
		t.Fatalf("корабль не повернулся к цели: остаток %.2f рад", a)
	}
	// остановили атаку и поехали в сторону: направление снова следует за движением
	_ = w.SetAttack("a", false)
	w.SetMove("a", Vec{1, 0})
	stepFor(w, 0.3)
	if a := angleBetween(p.Dir, Vec{1, 0}); a > 0.05 {
		t.Fatalf("без атаки направление — по движению: %.2f", a)
	}
}
