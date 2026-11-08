package shop

import (
	"context"
	"testing"
	"time"
)

func newSvc() (*Service, *time.Time) {
	svc := NewService(NewMemStore())
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	return svc, &now
}

func TestDefaultCatalogValid(t *testing.T) {
	if _, err := Parse(defaultJSON); err != nil {
		t.Fatal(err)
	}
}

func TestParseRejectsBadCatalog(t *testing.T) {
	for _, bad := range []string{
		`{"items":[{"id":"a","kind":"x","name":"n"}]}`,
		`{"items":[{"id":"a","kind":"engine","name":"n","color":"red"}]}`,
		`{"items":[{"id":"a","kind":"badge","name":"n"},{"id":"a","kind":"badge","name":"n"}]}`,
		`{"items":[{"id":"v","kind":"vip","name":"n","level":9,"days":30}]}`,
		`{"items":[{"id":"a","kind":"badge","name":"n","price":-1}]}`,
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Fatalf("каталог принят: %s", bad)
		}
	}
}

func TestBuyIsIdempotentAndFreeModeRecordsZero(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	st, err := svc.Buy(ctx, "p", "badge_star", "req-0000001")
	if err != nil || len(st.Owned) != 1 {
		t.Fatalf("%v %+v", err, st)
	}
	// повтор того же запроса — не ошибка и не дубль
	if st2, err := svc.Buy(ctx, "p", "badge_star", "req-0000001"); err != nil || len(st2.Owned) != 1 {
		t.Fatalf("повтор запроса: %v %+v", err, st2)
	}
	// новый запрос на уже купленное — отказ
	if _, err := svc.Buy(ctx, "p", "badge_star", "req-0000002"); err != ErrOwned {
		t.Fatalf("повторная покупка: %v", err)
	}
	if _, err := svc.Buy(ctx, "p", "nope", "req-0000003"); err != ErrNoItem {
		t.Fatalf("неизвестный товар: %v", err)
	}
	if _, err := svc.Buy(ctx, "p", "badge_skull", "short"); err != ErrBadReq {
		t.Fatalf("короткий request_id: %v", err)
	}
}

func TestVipRequirementsAndExpiry(t *testing.T) {
	svc, now := newSvc()
	ctx := context.Background()
	if _, err := svc.Buy(ctx, "p", "engine_gold", "req-0000001"); err != ErrVipNeeded {
		t.Fatalf("без VIP: %v", err)
	}
	st, err := svc.Buy(ctx, "p", "vip_1", "req-0000002")
	if err != nil || st.VipLevel != 1 {
		t.Fatalf("%v %+v", err, st)
	}
	if _, err := svc.Buy(ctx, "p", "engine_gold", "req-0000003"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Equip(ctx, "p", "engine", "engine_gold"); err != nil {
		t.Fatal(err)
	}
	st, _ = svc.State(ctx, "p")
	if c := svc.Cosmetics(st); c.Vip != 1 || c.Engine != "#ffc940" {
		t.Fatalf("косметика при VIP: %+v", c)
	}
	// VIP истёк: эксклюзивные товары не показываются, но остаются в собственности
	*now = now.Add(31 * 24 * time.Hour)
	st, _ = svc.State(ctx, "p")
	if c := svc.Cosmetics(st); c.Vip != 0 || c.Engine != "" {
		t.Fatalf("косметика после истечения VIP: %+v", c)
	}
	if len(st.Owned) != 1 {
		t.Fatalf("купленное пропало: %+v", st)
	}
}

func TestVipExtendsAndKeepsHigherLevel(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	st, _ := svc.Buy(ctx, "p", "vip_3", "req-0000001")
	until := *st.VipUntil
	st, _ = svc.Buy(ctx, "p", "vip_1", "req-0000002")
	if st.VipLevel != 3 {
		t.Fatalf("уровень снизился: %d", st.VipLevel)
	}
	if !st.VipUntil.After(until) {
		t.Fatal("срок VIP не продлён")
	}
}

func TestEquipRules(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	if _, err := svc.Equip(ctx, "p", "badge", "badge_star"); err != ErrNotOwned {
		t.Fatalf("не куплено: %v", err)
	}
	_, _ = svc.Buy(ctx, "p", "badge_star", "req-0000001")
	if _, err := svc.Equip(ctx, "p", "engine", "badge_star"); err != ErrBadSlot {
		t.Fatalf("не тот слот: %v", err)
	}
	st, err := svc.Equip(ctx, "p", "badge", "badge_star")
	if err != nil || st.Equipped["badge"] != "badge_star" {
		t.Fatalf("%v %+v", err, st)
	}
	st, err = svc.Equip(ctx, "p", "badge", "")
	if err != nil || st.Equipped["badge"] != "" {
		t.Fatalf("снять: %v %+v", err, st)
	}
	if _, err := svc.Equip(ctx, "p", "wings", ""); err != ErrBadSlot {
		t.Fatalf("неизвестный слот: %v", err)
	}
}

func TestPlayersAreIsolated(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	_, _ = svc.Buy(ctx, "a", "badge_star", "req-0000001")
	st, _ := svc.State(ctx, "b")
	if len(st.Owned) != 0 {
		t.Fatal("покупка одного игрока видна другому")
	}
}
