package aiagent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

type fakeCaps struct{ ps []Profile }

func (f fakeCaps) Profiles(context.Context) ([]Profile, error) { return f.ps, nil }

func profiles(n int) []Profile {
	profs := []string{"шахтёр-астероидщик", "торговец", "разведчик", "наёмник", "медик", "учёный", "логист", "дипломат", "сборщик утиля", "страж сектора"}
	out := make([]Profile, n)
	for i := range out {
		out[i] = Profile{ID: fmt.Sprintf("cpt-%04d", i+1), Name: fmt.Sprintf("Ник%d", i+1), Profession: profs[i%len(profs)]}
	}
	return out
}

type rig struct {
	m   *Manager
	w   *world.World
	now time.Time
}

func newRig(t *testing.T, n int) *rig {
	t.Helper()
	w := world.New()
	m := NewManager(w, fakeCaps{profiles(n)}, &MemStore{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := &rig{m: m, w: w, now: time.Unix(1_700_000_000, 0)}
	m.now = func() time.Time { return r.now }
	return r
}

func (r *rig) tick(sec int) {
	for i := 0; i < sec; i++ {
		r.now = r.now.Add(time.Second)
		r.m.Tick(context.Background())
	}
}

func (r *rig) join(id string) { r.w.Join(world.Record{ID: id, Name: id}) }

func TestSettingsValidateAndTarget(t *testing.T) {
	s := DefaultSettings()
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func(*Settings){
		"потолок выше 200":   func(s *Settings) { s.AbsMax = 201 },
		"минимум > максимум": func(s *Settings) { s.MinOnline = 60 },
		"максимум > потолка": func(s *Settings) { s.AbsMax = 40 },
		"нет долей":          func(s *Settings) { s.Shares = map[string]int{"miner": 0} },
		"чужая доля":         func(s *Settings) { s.Shares = map[string]int{"dragon": 10} },
		"скорость 0":         func(s *Settings) { s.SpawnPerMin = 0 },
		"радиус":             func(s *Settings) { s.ViewRadius = 5 },
	} {
		c := DefaultSettings()
		mut(&c)
		if c.Validate() == nil {
			t.Errorf("не замечено: %s", name)
		}
	}
	if s.Target(0) != 0 {
		t.Fatal("без игроков ИИ нет")
	}
	if got := s.Target(1); got < 20 || got > 25 {
		t.Fatalf("один игрок: %d", got)
	}
	if s.Target(10) != 50 || s.Target(500) != 50 {
		t.Fatal("при 10 и более игроках — максимум")
	}
	off := s
	off.Enabled = false
	if off.Target(5) != 0 {
		t.Fatal("выключенные ИИ не появляются")
	}
	capped := s
	capped.AbsMax, capped.MaxOnline, capped.MinOnline = 30, 30, 20
	if capped.Target(10) != 30 {
		t.Fatal("потолок")
	}
}

func TestNoPlayersNoAI(t *testing.T) {
	r := newRig(t, 300)
	r.tick(120)
	if r.w.AICount() != 0 {
		t.Fatalf("без игроков ИИ быть не должно: %d", r.w.AICount())
	}
}

func TestGrowsGraduallyThenHoldsAndLeaves(t *testing.T) {
	r := newRig(t, 300)
	r.join("p1")
	r.tick(5)
	if n := r.w.AICount(); n == 0 || n > 5 {
		t.Fatalf("набор должен идти постепенно: за 5 с — %d", n)
	}
	r.tick(120)
	want := r.m.Settings().Target(1)
	if n := r.w.AICount(); n != want {
		t.Fatalf("через 2 минуты ожидалось %d, есть %d", want, n)
	}
	// живой игрок ушёл: после задержки ИИ уходят все
	r.w.Disconnect("p1", r.now)
	r.tick(10)
	if r.w.AICount() == 0 {
		t.Fatal("в течение задержки ИИ ещё остаются")
	}
	r.tick(40)
	if r.w.AICount() != 0 || r.m.Status().Online != 0 {
		t.Fatalf("без игроков ИИ нет онлайн: %d", r.w.AICount())
	}
}

func TestPopulationFollowsPlayers(t *testing.T) {
	r := newRig(t, 300)
	r.join("p1")
	r.tick(200)
	low := r.w.AICount()
	for i := 2; i <= 10; i++ {
		r.join(fmt.Sprintf("p%d", i))
	}
	r.tick(400)
	if high := r.w.AICount(); high != 50 || high <= low {
		t.Fatalf("при 10 игроках должно быть 50 ИИ (было %d), есть %d", low, high)
	}
	if st := r.m.Status(); st.Players != 10 || st.Online != 50 || st.Target != 50 {
		t.Fatalf("статус: %+v", st)
	}
}

func TestPausedAndDisabled(t *testing.T) {
	r := newRig(t, 300)
	r.join("p1")
	r.tick(60)
	n := r.w.AICount()
	s := r.m.Settings()
	s.Paused = true
	if err := r.m.SetSettings(context.Background(), s, "owner"); err != nil {
		t.Fatal(err)
	}
	r.join("p2")
	r.tick(120)
	if r.w.AICount() != n {
		t.Fatalf("на паузе число ИИ не меняется: было %d, стало %d", n, r.w.AICount())
	}
	s.Paused, s.Enabled = false, false
	if err := r.m.SetSettings(context.Background(), s, "owner"); err != nil {
		t.Fatal(err)
	}
	r.tick(120)
	if r.w.AICount() != 0 {
		t.Fatalf("выключенные ИИ уходят: %d", r.w.AICount())
	}
}

func TestSetSettingsRejectsInvalidAndPersists(t *testing.T) {
	r := newRig(t, 50)
	bad := DefaultSettings()
	bad.AbsMax = 999
	if r.m.SetSettings(context.Background(), bad, "owner") == nil {
		t.Fatal("некорректные настройки приняты")
	}
	good := DefaultSettings()
	good.MinOnline, good.MaxOnline, good.ViewRadius = 5, 10, 300
	if err := r.m.SetSettings(context.Background(), good, "owner"); err != nil {
		t.Fatal(err)
	}
	st, ok, _ := r.m.store.Load(context.Background())
	if !ok || st.MaxOnline != 10 {
		t.Fatalf("настройки не сохранены: %+v", st)
	}
	r.join("p1")
	r.tick(120)
	if n := r.w.AICount(); n < 5 || n > 6 {
		t.Fatalf("новые границы: ожидалось около 5, есть %d", n)
	}
}

func TestUniqueCaptainsAndBehaviorMix(t *testing.T) {
	r := newRig(t, 300)
	r.join("p1")
	for i := 2; i <= 10; i++ {
		r.join(fmt.Sprintf("p%d", i))
	}
	r.tick(600)
	ids := r.w.AIIDs()
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("капитан %s появился дважды", id)
		}
		seen[id] = true
	}
	by := r.m.Status().ByBehavior
	// доли по умолчанию 30/20/20/15/15 при 50 ИИ: допускаем отклонение
	want := map[string]int{"miner": 15, "trader": 10, "scout": 10, "patrol": 7, "support": 8}
	for k, w := range want {
		if d := by[k] - w; d < -4 || d > 4 {
			t.Errorf("поведение %s: %d, ожидалось около %d (всего %v)", k, by[k], w, by)
		}
	}
}

func TestProfilesFewerThanTarget(t *testing.T) {
	r := newRig(t, 8) // профилей меньше цели
	r.join("p1")
	r.tick(300)
	if n := r.w.AICount(); n != 8 {
		t.Fatalf("при 8 профилях онлайн должно быть 8, есть %d", n)
	}
}

func TestRotationAvoidsImmediateReturn(t *testing.T) {
	r := newRig(t, 300)
	r.join("p1")
	r.tick(200)
	before := map[string]bool{}
	for _, id := range r.w.AIIDs() {
		before[id] = true
	}
	// резко меньше цель: часть ИИ уходит, затем снова растёт — ушедшие не возвращаются сразу
	s := r.m.Settings()
	s.MinOnline, s.MaxOnline = 5, 6
	_ = r.m.SetSettings(context.Background(), s, "owner")
	r.tick(120)
	left := map[string]bool{}
	for id := range before {
		left[id] = true
	}
	for _, id := range r.w.AIIDs() {
		delete(left, id)
	}
	if len(left) == 0 {
		t.Fatal("ИИ должны были уйти")
	}
	s.MinOnline, s.MaxOnline = 30, 40
	_ = r.m.SetSettings(context.Background(), s, "owner")
	r.tick(120)
	for _, id := range r.w.AIIDs() {
		if left[id] {
			t.Fatalf("недавно ушедший %s вернулся сразу, хотя свободных профилей достаточно", id)
		}
	}
}

func TestBehaviorAndShipMapping(t *testing.T) {
	if BehaviorFor("шахтёр-астероидщик") != "miner" || BehaviorFor("торговец") != "trader" || BehaviorFor("медик") != "support" || BehaviorFor("неизвестно") != "support" {
		t.Fatal("поведение по профессии")
	}
	for _, p := range profiles(300) {
		b := BehaviorFor(p.Profession)
		if ShipFor(b, p.ID) != ShipFor(b, p.ID) || ShipFor(b, p.ID) == "" {
			t.Fatal("корабль капитана должен быть постоянным")
		}
	}
	w := world.New()
	for _, b := range behaviors {
		for _, id := range []string{"a", "b", "c", "d"} {
			typ := ShipFor(b, id)
			if _, ok := w.Catalog().Ships[typ]; !ok {
				t.Fatalf("корабль %s для поведения %s отсутствует в каталоге", typ, b)
			}
		}
	}
}
