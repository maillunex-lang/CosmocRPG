package corp

import (
	"context"
	"errors"
	"testing"
)

func TestCreateJoinLeave(t *testing.T) {
	ctx := context.Background()
	st := NewMemStore()
	var balance int64 = 20000
	svc := NewService(st, func(_ context.Context, _ string, amount int64) (int64, error) {
		if balance < amount {
			return balance, errors.New("no_credits")
		}
		balance -= amount
		return balance, nil
	})

	c, err := svc.Create(ctx, "p1", "Стальная Лиса", "sl")
	if err != nil || c.Tag != "SL" {
		t.Fatalf("создание: %+v %v", c, err)
	}
	if balance != 10000 {
		t.Fatalf("взнос не списан: %d", balance)
	}
	// повторное создание — отказ
	if _, err := svc.Create(ctx, "p1", "Другая", "xx"); err != ErrHasCorp {
		t.Fatalf("уже в корпорации: %v", err)
	}
	// имя занято
	if _, err := svc.Create(ctx, "p2", "стальная лиса", "ab"); err != ErrTaken {
		t.Fatalf("имя: %v", err)
	}
	// тег занят
	if _, err := svc.Create(ctx, "p2", "Новая", "SL"); err != ErrTaken {
		t.Fatalf("тег: %v", err)
	}
	// вступление по имени и по тегу
	if _, err := svc.Join(ctx, "p2", "Стальная Лиса", "Второй"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Join(ctx, "p3", "sl", "Третий"); err != nil {
		t.Fatal(err)
	}
	// уже в корпорации
	if _, err := svc.Join(ctx, "p3", "sl", "Третий"); err != ErrHasCorp {
		t.Fatalf("повторное вступление: %v", err)
	}
	c, mem, err := svc.Info(ctx, "p1")
	if err != nil || c.Name != "Стальная Лиса" || len(mem) != 3 || mem[0].Role != "owner" {
		t.Fatalf("состав: %+v %+v %v", c, mem, err)
	}
	// выход
	if err := svc.Leave(ctx, "p2"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Info(ctx, "p2"); err != ErrNoCorp {
		t.Fatalf("после выхода: %v", err)
	}
	// невалидные имя/тег
	if _, err := svc.Create(ctx, "p9", "x", "ab"); err != ErrBadName {
		t.Fatalf("короткое имя: %v", err)
	}
	if _, err := svc.Create(ctx, "p9", "Норм", "bad!"); err != ErrBadTag {
		t.Fatalf("плохой тег: %v", err)
	}
	// нет такой корпорации
	if _, err := svc.Join(ctx, "p8", "Призраки", "Восьмой"); err != ErrNoCorp {
		t.Fatalf("не найдена: %v", err)
	}
}

func TestCreateNoFundsKeepsNothing(t *testing.T) {
	ctx := context.Background()
	st := NewMemStore()
	svc := NewService(st, func(context.Context, string, int64) (int64, error) { return 0, errors.New("no_credits") })
	if _, err := svc.Create(ctx, "p1", "Бедные", "bb"); err == nil {
		t.Fatal("создание без средств должно отклоняться")
	}
	if _, ok, _ := st.CorporationByPlayer(ctx, "p1"); ok {
		t.Fatal("корпорация не должна остаться")
	}
	if taken, _ := st.Exists(ctx, "Бедные", "BB"); taken {
		t.Fatal("имя не должно остаться занятым")
	}
}
