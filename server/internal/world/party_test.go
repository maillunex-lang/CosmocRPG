package world

import (
	"strings"
	"testing"
	"time"
)

func partyHome(t *testing.T) (*Router, *World) {
	t.Helper()
	home := New()
	r := NewRouter(home)
	r.Add(home)
	return r, home
}

func TestPartySkirmishWithAIShip(t *testing.T) {
	r, home := partyHome(t)
	const ship = "cap-wing"
	if !home.SpawnAI(AISpawn{ID: ship, Name: "Капитан", Profession: "patrol", Behavior: AIPatrol, ShipType: DefaultShip, Seed: 7}) {
		t.Fatal("spawn")
	}
	homePos := Vec{}
	if _, ok := r.Join(Record{ID: "human", Name: "Пилот", Created: true, Pos: homePos, Dir: Vec{X: 1}, ShipType: DefaultShip, PilotSpec: "interceptor"}); !ok {
		t.Fatal("join")
	}
	if err := r.PartyInvite("human", ship); err != nil {
		t.Fatal(err)
	}
	view, ok := r.PartySnapshot("human")
	if !ok || len(view.Members) != 2 {
		t.Fatalf("пати %+v", view)
	}
	accepted := false
	for _, m := range view.Members {
		if m.AI && !m.Pending {
			accepted = true
		}
	}
	if !accepted {
		t.Fatal("ИИ-капитан не принял приглашение")
	}
	if err := r.PartyStart("human"); err != nil {
		t.Fatal(err)
	}
	if w := r.For("human"); w != nil {
		lay, _ := w.Layout()
		if lay.Radius < 200 || lay.Name == "" {
			t.Fatalf("раскладка коридора: %+v", lay.Name)
		}
	}
	if !strings.HasPrefix(r.SystemID("human"), "inst-") {
		t.Fatalf("система %s", r.SystemID("human"))
	}
	for _, s := range r.Systems() {
		if skipInstanceSystem(s.ID) {
			t.Fatalf("инстанс попал на карту: %s", s.ID)
		}
	}
	now := time.Now()
	var rewards []Reward
	back := false
	for i := 0; i < 160; i++ {
		if strings.HasPrefix(r.SystemID("human"), "inst-") {
			for _, rec := range r.Records() {
				if rec.ID == "human" && rec.Pos != homePos {
					t.Fatalf("в сохранение ушла позиция стычки %+v", rec.Pos)
				}
				if strings.HasPrefix(rec.ID, "wing-") {
					t.Fatal("поднятый капитан попал в записи")
				}
			}
			if w := r.For("human"); w != nil {
				st, ok := w.Snapshot("human")
				if ok && !st.Ship.Dead {
					for _, o := range st.Objects {
						if o.Kind == "hostile" && !o.Dead {
							w.SetTarget("human", o.ID)
							_ = w.SetAttack("human", true)
							break
						}
					}
				}
			}
		}
		r.Step(0.25, now)
		rewards = append(rewards, r.DrainRewards()...)
		if !strings.HasPrefix(r.SystemID("human"), "inst-") {
			back = true
			break
		}
		if i == 159 {
			if w := r.For("human"); w != nil {
				t.Fatalf("не вернулись из %s, врагов %d", w.System().ID, w.HostilesLeft())
			}
		}
	}
	if !back {
		t.Fatal("игрок потерян")
	}
	if !home.HasAI(ship) {
		t.Fatal("капитан не вернулся в систему")
	}
	n := 0
	for _, rw := range rewards {
		if rw.Kind != "party" {
			continue
		}
		n++
		if rw.Player != "human" || rw.Credits != instancePay {
			t.Fatalf("награда %+v", rw)
		}
	}
	if n != 1 {
		t.Fatalf("наград пати %d", n)
	}
	r.Step(0.25, now)
	for _, rw := range r.DrainRewards() {
		if rw.Kind == "party" {
			t.Fatal("награда выдана дважды")
		}
	}
	for _, rec := range r.Records() {
		if strings.HasPrefix(rec.ID, "wing-") || rec.ID == ship {
			t.Fatalf("лишняя запись %s", rec.ID)
		}
	}
}

func TestPartyRules(t *testing.T) {
	r, home := partyHome(t)
	if _, ok := r.Join(Record{ID: "human", Name: "Пилот", Created: true, Pos: Vec{X: 350, Y: 0}, Dir: Vec{X: 1}, ShipType: DefaultShip}); !ok {
		t.Fatal("join")
	}
	if !home.SpawnAI(AISpawn{ID: "far", Name: "Далеко", Behavior: AIPatrol, ShipType: DefaultShip, Seed: 1}) {
		t.Fatal("spawn")
	}
	if err := r.PartyInvite("human", "far"); err != ErrPartyFar {
		t.Fatalf("далеко: %v", err)
	}

	r, home = partyHome(t)
	if _, ok := r.Join(Record{ID: "human", Name: "Пилот", Created: true, Pos: Vec{}, Dir: Vec{X: 1}, ShipType: DefaultShip}); !ok {
		t.Fatal("join")
	}
	if _, ok := r.Join(Record{ID: "bot", Name: "Кира", Created: true, AI: true, AICaptain: "cpt-kira", Pos: Vec{X: 4, Y: 2}, Dir: Vec{X: 1}, ShipType: DefaultShip}); !ok {
		t.Fatal("ai")
	}
	if err := r.PartyInvite("human", "bot"); err != nil {
		t.Fatal(err)
	}
	if view, ok := r.PartySnapshot("bot"); !ok || view.Pending {
		t.Fatalf("ИИ-игрок должен войти сразу: %+v", view)
	}
	if _, ok := r.Join(Record{ID: "mate", Name: "Напарник", Created: true, Pos: Vec{X: 8, Y: -4}, Dir: Vec{X: 1}, ShipType: DefaultShip}); !ok {
		t.Fatal("mate")
	}
	if err := r.PartyInvite("human", "mate"); err != nil {
		t.Fatal(err)
	}
	if view, ok := r.PartySnapshot("mate"); !ok || !view.Pending {
		t.Fatalf("живой пилот должен подтвердить: %+v", view)
	}
	if err := r.PartyJoin("mate"); err != nil {
		t.Fatal(err)
	}
	if view, ok := r.PartySnapshot("mate"); !ok || view.Pending {
		t.Fatalf("после принятия всё ещё pending: %+v", view)
	}
	if !home.SpawnAI(AISpawn{ID: "cap-a", Name: "А", Behavior: AIScout, ShipType: DefaultShip, Seed: 2}) ||
		!home.SpawnAI(AISpawn{ID: "cap-b", Name: "Б", Behavior: AIScout, ShipType: DefaultShip, Seed: 3}) {
		t.Fatal("spawn pair")
	}
	if err := r.PartyInvite("human", "cap-a"); err != nil {
		t.Fatal(err)
	}
	if err := r.PartyInvite("human", "cap-b"); err != ErrPartyFull {
		t.Fatalf("пати заполнена: %v", err)
	}
	if err := r.PartyLeave("mate"); err != nil {
		t.Fatal(err)
	}
	if err := r.PartyInvite("human", "cap-b"); err != nil {
		t.Fatal(err)
	}
}

func TestPartyDisconnectExtracts(t *testing.T) {
	r, home := partyHome(t)
	if !home.SpawnAI(AISpawn{ID: "cap-1", Name: "Капитан", Behavior: AIPatrol, ShipType: DefaultShip, Seed: 4}) {
		t.Fatal("spawn")
	}
	if _, ok := r.Join(Record{ID: "human", Name: "Пилот", Created: true, Pos: Vec{}, Dir: Vec{X: 1}, ShipType: DefaultShip}); !ok {
		t.Fatal("join")
	}
	if err := r.PartyInvite("human", "cap-1"); err != nil {
		t.Fatal(err)
	}
	if err := r.PartyStart("human"); err != nil {
		t.Fatal(err)
	}
	rec, ok := r.Disconnect("human", time.Now())
	if !ok {
		t.Fatal("disconnect")
	}
	if strings.HasPrefix(r.SystemID("human"), "inst-") {
		t.Fatal("остался в стычке")
	}
	if !home.HasAI("cap-1") {
		t.Fatal("капитан не восстановлен")
	}
	for _, s := range r.Systems() {
		if skipInstanceSystem(s.ID) {
			t.Fatalf("система %s", s.ID)
		}
	}
	if rec.Pos != (Vec{}) {
		t.Fatalf("позиция %+v", rec.Pos)
	}
}

func TestCorridorConsentAndDoor(t *testing.T) {
	r, _ := partyHome(t)
	if _, ok := r.Join(Record{ID: "human", Name: "Пилот", Created: true, Pos: Vec{}, Dir: Vec{X: 1}, ShipType: DefaultShip, PilotSpec: "interceptor"}); !ok {
		t.Fatal("join")
	}
	if _, ok := r.Join(Record{ID: "mate", Name: "Напарник", Created: true, Pos: Vec{X: 6}, Dir: Vec{X: 1}, ShipType: DefaultShip}); !ok {
		t.Fatal("mate")
	}
	if err := r.PartyInvite("human", "mate"); err != nil {
		t.Fatal(err)
	}
	if err := r.PartyArm("human", "loot"); err != ErrPartyNeed {
		t.Fatalf("пока напарник не принял: %v", err)
	}
	if err := r.PartyJoin("mate"); err != nil {
		t.Fatal(err)
	}
	if err := r.PartyArm("human", "loot"); err != nil {
		t.Fatal(err)
	}
	if err := r.PartyReady("human"); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(r.SystemID("human"), "inst-") {
		t.Fatal("прыжок до согласия всех")
	}
	if err := r.PartyReady("mate"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(r.SystemID("human"), "inst-") || !strings.HasPrefix(r.SystemID("mate"), "inst-") {
		t.Fatal("после согласия нет прыжка")
	}
	w := r.For("human")
	if w.System().Name != "Трюмный коридор" {
		t.Fatalf("коридор %s", w.System().Name)
	}
	w.mu.Lock()
	w.players["human"].Pos = Vec{X: 200, Y: 80}
	w.mu.Unlock()
	w.Step(0.05, time.Now())
	st, ok := w.Snapshot("human")
	if !ok || st.Ship.Pos[0] > 52.1 || st.Ship.Pos[1] > 24.1 {
		t.Fatalf("дверь не держит: %+v", st.Ship.Pos)
	}
	_ = w.SetTarget("human", "op-2")
	_ = w.SetAttack("human", true)
	hull := 0.0
	w.mu.Lock()
	for _, n := range w.npcs {
		if n.ID == "op-2" {
			n.Pos.X = 60
			hull = n.Hull
		}
	}
	w.mu.Unlock()
	w.Step(1, time.Now())
	w.mu.Lock()
	for _, n := range w.npcs {
		if n.ID == "op-2" && n.Hull < hull {
			t.Fatal("урон через закрытую дверь")
		}
	}
	w.mu.Unlock()
}
