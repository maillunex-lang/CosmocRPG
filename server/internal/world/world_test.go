package world

import (
	"context"
	"math"
	"testing"
	"time"
)

func join(t *testing.T, w *World, id string) *Player {
	t.Helper()
	p, ok := w.Join(Record{ID: id, Name: id, Dir: Vec{1, 0}})
	if !ok {
		t.Fatal("вход отклонён")
	}
	return p
}

func TestSpeedIsLimitedByServer(t *testing.T) {
	w := New()
	p := join(t, w, "a")
	w.SetMove("a", Vec{50, 0}) // клиент просит больше единицы
	for i := 0; i < 20; i++ {
		w.Step(0.05, time.Now())
	}
	if d := p.Pos.length(); math.Abs(d-Speed) > 0.01 {
		t.Fatalf("за секунду пройдено %.3f, ожидалось %.1f", d, Speed)
	}
	if math.Abs(p.Speed-Speed) > 0.01 {
		t.Fatalf("скорость %.3f", p.Speed)
	}
}

func TestInvalidMoveRejected(t *testing.T) {
	w := New()
	join(t, w, "a")
	if w.SetMove("a", Vec{math.NaN(), 0}) || w.SetMove("a", Vec{math.Inf(1), 0}) {
		t.Fatal("NaN и бесконечность должны отклоняться")
	}
	if w.SetMove("нет", Vec{1, 0}) {
		t.Fatal("несуществующий игрок")
	}
}

func TestWorldBoundary(t *testing.T) {
	w := New()
	p := join(t, w, "a")
	p.Pos = Vec{Radius - 1, 0}
	w.SetMove("a", Vec{1, 0})
	for i := 0; i < 100; i++ {
		w.Step(0.05, time.Now())
	}
	if p.Pos.length() > Radius+1e-6 {
		t.Fatalf("вышел за границу: %.2f", p.Pos.length())
	}
}

func TestAutopilotStopsAtDistance(t *testing.T) {
	w := New()
	p := join(t, w, "a")
	if w.SetAutopilot("a", true) {
		t.Fatal("автопилот без цели должен отклоняться")
	}
	if !w.SetTarget("a", StationID) {
		t.Fatal("станция должна выбираться")
	}
	if !w.SetAutopilot("a", true) {
		t.Fatal("автопилот должен включиться")
	}
	p.Pos = Vec{150, 0}
	for i := 0; i < 20*40; i++ {
		w.Step(0.05, time.Now())
	}
	st := Vec{-12, 14}
	d := p.Pos.sub(st).length()
	if math.Abs(d-AutopilotStop) > 0.2 {
		t.Fatalf("остановился на %.2f, ожидалось %.1f", d, AutopilotStop)
	}
	if p.Speed > 0.01 {
		t.Fatalf("после остановки скорость %.2f", p.Speed)
	}
}

func TestWaypointAutopilotStops(t *testing.T) {
	w := New()
	p := join(t, w, "a")
	if w.SetWaypoint("a", Vec{math.NaN(), 0}) {
		t.Fatal("NaN не должен приниматься")
	}
	p.Pos = Vec{40, 0}
	if !w.SetWaypoint("a", Vec{120, 0}) {
		t.Fatal("точка маршрута не принята")
	}
	if !p.Autopilot || !p.HasWaypoint || p.Target != "" {
		t.Fatalf("автопилот к точке: %q", p.Target)
	}
	w.SetMove("a", Vec{0, 1})
	if p.Autopilot || p.HasWaypoint {
		t.Fatal("ручное управление должно снимать точку маршрута")
	}
	if !w.SetWaypoint("a", Vec{120, 0}) {
		t.Fatal("повторная точка маршрута не принята")
	}
	for i := 0; i < 20*40; i++ {
		w.Step(0.05, time.Now())
	}
	d := p.Pos.sub(Vec{120, 0}).length()
	if math.Abs(d-AutopilotStop) > 0.3 || p.Autopilot || p.HasWaypoint {
		t.Fatalf("не остановился у точки: dist %.2f autopilot %v", d, p.Autopilot)
	}
}

func TestManualInputCancelsAutopilot(t *testing.T) {
	w := New()
	p := join(t, w, "a")
	w.SetTarget("a", StationID)
	w.SetAutopilot("a", true)
	w.SetMove("a", Vec{0, 1})
	if p.Autopilot {
		t.Fatal("ручное управление должно отключать автопилот")
	}
}

func TestTargetValidation(t *testing.T) {
	w := New()
	join(t, w, "a")
	join(t, w, "b")
	if w.SetTarget("a", "a") {
		t.Fatal("нельзя выбрать себя")
	}
	if w.SetTarget("a", "призрак") {
		t.Fatal("нет такой цели")
	}
	if !w.SetTarget("a", "b") || !w.SetTarget("a", "") {
		t.Fatal("игрок как цель, снятие цели")
	}
}

func TestSnapshotShowsOthersNotSelf(t *testing.T) {
	w := New()
	join(t, w, "a")
	join(t, w, "b")
	st, ok := w.Snapshot("a")
	if !ok {
		t.Fatal("нет снимка")
	}
	ids := map[string]bool{}
	for _, o := range st.Objects {
		ids[o.ID] = true
	}
	if !ids["b"] || !ids[StationID] || ids["a"] {
		t.Fatalf("объекты: %v", ids)
	}
	if _, ok := w.Snapshot("нет"); ok {
		t.Fatal("снимок несуществующего игрока")
	}
}

func TestDisconnectedShipLeavesAfterGrace(t *testing.T) {
	w := New()
	p := join(t, w, "a")
	p.Pos = Vec{5, 5}
	t0 := time.Now()
	rec, ok := w.Disconnect("a", t0)
	if !ok || rec.Pos != (Vec{5, 5}) {
		t.Fatal("запись при отключении")
	}
	if removed := w.Step(0.05, t0.Add(Grace-time.Second)); len(removed) != 0 || w.Count() != 1 {
		t.Fatal("корабль должен остаться в мире в течение Grace")
	}
	removed := w.Step(0.05, t0.Add(Grace+time.Second))
	if len(removed) != 1 || removed[0].ID != "a" || w.Count() != 0 {
		t.Fatal("после Grace корабль удаляется и возвращается запись для сохранения")
	}
}

func TestRejoinKeepsShip(t *testing.T) {
	w := New()
	p := join(t, w, "a")
	p.Pos = Vec{7, 7}
	w.Disconnect("a", time.Now())
	p2, _ := w.Join(Record{ID: "a", Name: "a", Pos: Vec{0, 0}})
	if p2.Pos != (Vec{7, 7}) {
		t.Fatalf("корабль должен остаться на месте, а не взять позицию из записи: %v", p2.Pos)
	}
}

func TestWorldFull(t *testing.T) {
	w := New()
	for i := 0; i < MaxPlayers; i++ {
		if _, ok := w.Join(Record{ID: string(rune('A'+i/26)) + string(rune('a'+i%26))}); !ok {
			t.Fatalf("игрок %d отклонён", i)
		}
	}
	if _, ok := w.Join(Record{ID: "лишний"}); ok {
		t.Fatal("мир должен быть заполнен")
	}
}

func TestJoinOutOfBoundsPositionFixed(t *testing.T) {
	w := New()
	p, _ := w.Join(Record{ID: "a", Pos: Vec{1e6, 0}})
	if p.Pos.length() > Radius {
		t.Fatal("позиция за границей должна сбрасываться")
	}
	p2, _ := w.Join(Record{ID: "b", Pos: Vec{math.NaN(), 0}})
	if !p2.Pos.finite() {
		t.Fatal("NaN в позиции")
	}
}

func TestNamesAndTokens(t *testing.T) {
	if CleanName("  \x00Орион\n  ") != "Орион" {
		t.Fatal("управляющие символы")
	}
	if n := []rune(CleanName("ааааааааааааааааааааааааааааааааааааа")); len(n) != 24 {
		t.Fatalf("длина %d", len(n))
	}
	if CleanName("   ") != "" {
		t.Fatal("пустое имя")
	}
	if !ValidToken("abcdefghijklmnop") || ValidToken("коротко") || ValidToken("abc def ghi jkl mno") || ValidToken("abc") {
		t.Fatal("проверка токена")
	}
	h := TokenHash("abcdefghijklmnop")
	if len(PublicID(h)) != 16 || DefaultName(h)[:len("Пилот-")] != "Пилот-" {
		t.Fatal("идентификатор и имя по умолчанию")
	}
}

func TestMemStoreResolve(t *testing.T) {
	s := NewMemStore()
	ctx := context.Background()
	a, _ := s.Resolve(ctx, "tokentokentoken01", "Орион")
	b, _ := s.Resolve(ctx, "tokentokentoken01", "Другое")
	if a.ID != b.ID || b.Name != "Орион" {
		t.Fatal("тот же токен — тот же игрок, имя не меняется")
	}
	a.Pos = Vec{3, 4}
	_ = s.Save(ctx, a)
	c, _ := s.Resolve(ctx, "tokentokentoken01", "")
	if c.Pos != (Vec{3, 4}) {
		t.Fatal("позиция должна сохраняться")
	}
}
