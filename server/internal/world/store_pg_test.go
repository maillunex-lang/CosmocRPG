package world

import (
	"context"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
)

func TestPGStorePersistsPosition(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	a, err := s.Resolve(ctx, "pgtokentokentoken1", "Тест")
	if err != nil || a.Name != "Тест" {
		t.Fatalf("создание: %v %+v", err, a)
	}
	a.Pos = Vec{12.5, -7.25}
	a.Dir = Vec{0, 1}
	if err := s.Save(ctx, a); err != nil {
		t.Fatal(err)
	}
	b, err := s.Resolve(ctx, "pgtokentokentoken1", "Другое")
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != a.ID || b.Name != "Тест" || b.Pos != (Vec{12.5, -7.25}) || b.Dir != (Vec{0, 1}) {
		t.Fatalf("после перезахода: %+v", b)
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM players WHERE token_hash = $1", func() []byte { h := TokenHash("pgtokentokentoken1"); return h[:] }()).Scan(&n); err != nil || n != 1 {
		t.Fatalf("в базе должна быть одна запись, найдено %d (%v)", n, err)
	}
	var raw int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM players WHERE encode(token_hash,'escape') LIKE '%pgtoken%'").Scan(&raw)
	if raw != 0 {
		t.Fatal("токен не должен храниться в открытом виде")
	}
}

func TestPGStorePersistsSkinAndSpec(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	a, err := s.Resolve(ctx, "skintokentoken00001", "Пилот")
	if err != nil || a.Skin != "a" || a.Spec != "interceptor" {
		t.Fatalf("значения по умолчанию: %v %+v", err, a)
	}
	a.Skin, a.Spec = "b", "scout"
	if err := s.Save(ctx, a); err != nil {
		t.Fatal(err)
	}
	b, _ := s.Resolve(ctx, "skintokentoken00001", "")
	if b.Skin != "b" || b.Spec != "scout" {
		t.Fatalf("после перезахода: %+v", b)
	}
	// пустые значения в записи не затирают сохранённый выбор
	a.Skin, a.Spec = "", ""
	_ = s.Save(ctx, a)
	c, _ := s.Resolve(ctx, "skintokentoken00001", "")
	if c.Skin != "b" || c.Spec != "scout" {
		t.Fatalf("выбор затёрт: %+v", c)
	}
}

func TestPGCharacterCreation(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	a, _ := s.Resolve(ctx, "charactertokenaaa0001", "")
	if a.Created {
		t.Fatal("новый игрок не должен считаться созданным")
	}
	st := Starter{Ship: "TMP-MINE", Spec: "miner", Skin: "a"}
	r, err := s.CreateCharacter(ctx, a.ID, "Крот_77", "f", "miner", st)
	if err != nil || !r.Created || r.Name != "Крот_77" || r.Gender != "f" || r.PilotSpec != "miner" || r.ShipType != "TMP-MINE" || r.Spec != "miner" {
		t.Fatalf("создание: %v %+v", err, r)
	}
	if _, err := s.CreateCharacter(ctx, a.ID, "Другое", "m", "scout", st); err != ErrCharacterSet {
		t.Fatalf("повторное создание: %v", err)
	}
	b, _ := s.Resolve(ctx, "charactertokenbbb0002", "")
	if _, err := s.CreateCharacter(ctx, b.ID, "крот_77", "m", "scout", st); err != ErrNickTaken {
		t.Fatalf("ник занят без учёта регистра: %v", err)
	}
	if _, err := s.CreateCharacter(ctx, b.ID, "Свободный", "m", "scout", st); err != nil {
		t.Fatalf("свободный ник: %v", err)
	}
	// сохранение позиции не затирает тип корабля и не сбрасывает персонажа
	again, _ := s.Resolve(ctx, "charactertokenaaa0001", "")
	again.Pos = Vec{X: 9, Y: 9}
	_ = s.Save(ctx, again)
	c, _ := s.Resolve(ctx, "charactertokenaaa0001", "")
	if !c.Created || c.ShipType != "TMP-MINE" || c.Pos != (Vec{X: 9, Y: 9}) {
		t.Fatalf("после сохранения: %+v", c)
	}
}

func TestPGStorePersistsHold(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	a, err := s.Resolve(ctx, "pgholdtokentoken01", "Груз")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Hold) != 0 {
		t.Fatalf("новый игрок с грузом: %v", a.Hold)
	}
	a.Hold = map[string]float64{"iron_ore": 12.5, "crystals": 3}
	if err := s.Save(ctx, a); err != nil {
		t.Fatal(err)
	}
	b, err := s.Resolve(ctx, "pgholdtokentoken01", "")
	if err != nil || b.Hold["iron_ore"] != 12.5 || b.Hold["crystals"] != 3 {
		t.Fatalf("груз после перезахода: %v %v", b.Hold, err)
	}
}
