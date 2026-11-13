package world

import (
	"math/rand"
	"strings"
	"testing"
	"time"
)

func mustEvent(t *testing.T, id string) eventDef {
	t.Helper()
	for _, d := range auroraEvents {
		if d.id == id {
			return d
		}
	}
	t.Fatalf("нет события %s", id)
	return eventDef{}
}

func expireLive(w *World) {
	if w.live != nil {
		w.live.until = w.t
	}
}

func TestAuroraEventsRunWithoutPlayers(t *testing.T) {
	w, _ := livingWorld(t)
	if w.system.ID != "aurora" {
		t.Fatal(w.system.ID)
	}
	w.eventRNG = rand.New(rand.NewSource(3))
	w.startLiveEvent(mustEvent(t, "convoy"), eventMin)
	if w.live == nil || w.live.id != "convoy" || w.anyNPC("dyn-convoy") == nil {
		t.Fatalf("конвой не вышел сам: %+v", w.live)
	}
	start := w.anyNPC("dyn-convoy").Pos
	w.Step(25, time.Now())
	if w.live.id != "convoy" || dist(w.anyNPC("dyn-convoy").Pos, start) < 80 {
		t.Fatal("грузовик не идёт челноком без игроков")
	}
	if w.anyNPC("aur-beacon") == nil {
		t.Fatal("учебный маяк пропал из-за события")
	}
	w.startLiveEvent(mustEvent(t, "bloom"), eventMin)
	if w.anyNPC("dyn-convoy") != nil {
		t.Fatal("конвой остался после смены события")
	}
	d := w.depositByID("dyn-bloom")
	if d == nil {
		t.Fatal("жила не появилась")
	}
	before := d.Amount
	w.Step(60, time.Now())
	got := w.depositByID("dyn-bloom")
	if got == nil || got.Amount >= before || got.Amount < got.Max*0.9 {
		t.Fatalf("жила должна чуть осесть за минуту, не исчезнуть: %+v", got)
	}
}

func TestAuroraEventsDoNotRepeatAndCoverTheSet(t *testing.T) {
	w, _ := livingWorld(t)
	w.eventRNG = rand.New(rand.NewSource(11))
	prev := ""
	seen := map[string]bool{}
	for i := 0; i < 240 && len(seen) < len(auroraEvents); i++ {
		expireLive(w)
		w.Step(0.05, time.Now())
		if w.live == nil || w.live.id == "" {
			t.Fatal("пустое событие")
		}
		if w.live.id == prev {
			t.Fatalf("повтор подряд: %s", prev)
		}
		life := w.live.until - w.t
		if life < eventMin-1 || life > eventMax+0.1 {
			t.Fatalf("окно %v, нужно %v–%v", life, eventMin, eventMax)
		}
		prev = w.live.id
		seen[w.live.id] = true
		p := joinAt(w, "watcher", "FTR-001", Vec{})
		st, ok := w.Snapshot(p.ID)
		if !ok || st.Event == nil || st.Event.ID != w.live.id || st.Event.Title == "" || st.Event.Left < eventMin-1 {
			t.Fatalf("снимок: %+v", st.Event)
		}
		w.Remove(p.ID)
	}
	if len(seen) != len(auroraEvents) {
		t.Fatalf("виды %d из %d: %v", len(seen), len(auroraEvents), seen)
	}
	if w.anyNPC("aur-beacon") == nil || w.anyNPC("aur-az04") == nil {
		t.Fatal("сюжетные точки не пережили смену событий")
	}
}

func TestAuroraEventIsSharedAcrossPlayers(t *testing.T) {
	w, _ := livingWorld(t)
	w.eventRNG = rand.New(rand.NewSource(5))
	w.Step(1, time.Now())
	a := joinAt(w, "a", "FTR-001", Vec{})
	b := joinAt(w, "b", "FTR-001", Vec{40, 10})
	sa, oka := w.Snapshot(a.ID)
	sb, okb := w.Snapshot(b.ID)
	if !oka || !okb || sa.Event == nil || sb.Event == nil || sa.Event.ID != sb.Event.ID || sa.Event.Title != sb.Event.Title {
		t.Fatalf("игроки видят разное: %+v %+v", sa.Event, sb.Event)
	}
}

func TestSameSeedDrawsTheSameSchedule(t *testing.T) {
	seq := func(seed int64) []string {
		w, _ := livingWorld(t)
		w.eventRNG = rand.New(rand.NewSource(seed))
		out := make([]string, 0, 6)
		for i := 0; i < 6; i++ {
			expireLive(w)
			w.Step(0.05, time.Now())
			out = append(out, w.live.id)
		}
		return out
	}
	a, b := seq(99), seq(99)
	for i := range a {
		if a[i] != b[i] || (i > 0 && a[i] == a[i-1]) {
			t.Fatalf("расписание %v", a)
		}
	}
}

func TestDynamicEventsStayInsideAurora(t *testing.T) {
	plain := New()
	plain.Step(30, time.Now())
	if plain.live != nil {
		t.Fatal("в пустой системе есть событие")
	}
	for _, n := range plain.npcs {
		if strings.HasPrefix(n.ID, "dyn-") {
			t.Fatal(n.ID)
		}
	}
	for _, d := range plain.deposits {
		if strings.HasPrefix(d.ID, "dyn-") {
			t.Fatal(d.ID)
		}
	}
}

func TestEventMotionsAreNotTheSame(t *testing.T) {
	w, _ := livingWorld(t)
	w.eventRNG = rand.New(rand.NewSource(2))
	w.startLiveEvent(mustEvent(t, "courier"), eventMin)
	c0 := w.anyNPC("dyn-courier").Pos
	w.Step(8, time.Now())
	courier := dist(w.anyNPC("dyn-courier").Pos, c0)

	w.startLiveEvent(mustEvent(t, "debris"), eventMin)
	d0 := w.anyNPC("dyn-debris").Pos
	w.Step(8, time.Now())
	debris := dist(w.anyNPC("dyn-debris").Pos, d0)

	w.startLiveEvent(mustEvent(t, "meteor"), eventMin)
	if w.anyNPC("dyn-meteor-0") == nil || w.anyNPC("dyn-meteor-1") == nil || w.anyNPC("dyn-meteor-2") == nil {
		t.Fatal("строй не собрался")
	}
	m0 := w.anyNPC("dyn-meteor-0").Pos
	w.Step(8, time.Now())
	meteor := dist(w.anyNPC("dyn-meteor-0").Pos, m0)

	w.startLiveEvent(mustEvent(t, "crystal"), eventMin)
	crystal := w.depositByID("dyn-crystal")
	if crystal == nil || crystal.Resource != "crystals" {
		t.Fatal("карман кристаллов не тот ресурс")
	}
	cpos := crystal.Pos
	w.Step(8, time.Now())
	if dist(w.depositByID("dyn-crystal").Pos, cpos) > 1 {
		t.Fatal("кристаллы не должны улетать")
	}
	if w.anyNPC("dyn-courier") != nil || w.anyNPC("dyn-debris") != nil {
		t.Fatal("прошлые события остались в секторе")
	}
	if courier < 140 || debris < 60 || meteor < 70 {
		t.Fatalf("движения слиплись: курьер %.0f обломок %.0f строй %.0f", courier, debris, meteor)
	}
	if courier < debris*1.5 {
		t.Fatalf("курьер должен быть заметно быстрее обломка: %.0f против %.0f", courier, debris)
	}
}
