package ws

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/shop"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func shopServer(t *testing.T) string {
	t.Helper()
	store := world.NewMemStore()
	opts := Options{MaxMessage: 8192, RatePerSec: 200, Version: "t", World: world.New(), Store: store, Shop: shop.NewService(shop.NewMemStore())}
	srv, h := newServer(t, opts)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return srv.URL
}

type shopMsg struct {
	Catalog struct {
		Free  bool
		Items []struct{ ID string }
	}
	State struct {
		VipLevel int `json:"vip_level"`
		Owned    []string
		Equipped map[string]string
	}
	Cosmetics struct {
		Vip    int
		Badge  string
		Engine string
	}
}

func (cl *client) shopReply() shopMsg {
	cl.t.Helper()
	m := cl.next("shop")
	var s shopMsg
	_ = json.Unmarshal(m.Payload, &s)
	return s
}

func (cl *client) expectError(code string) {
	cl.t.Helper()
	m := cl.next("error")
	var e struct{ Code string }
	_ = json.Unmarshal(m.Payload, &e)
	if e.Code != code {
		cl.t.Fatalf("код ошибки %q, ожидался %q", e.Code, code)
	}
}

func TestShopBuyEquipAndVisibleToOthers(t *testing.T) {
	url := shopServer(t)
	a := connect(t, url)
	a.join("shopaaaaaaaaaaaaaaaa")
	a.send("shop", nil)
	s := a.shopReply()
	if !s.Catalog.Free || len(s.Catalog.Items) == 0 || len(s.State.Owned) != 0 {
		t.Fatalf("каталог: %+v", s)
	}
	a.send("shop_buy", map[string]any{"item": "vip_1", "request_id": "req-ws-00000001"})
	if s = a.shopReply(); s.State.VipLevel != 1 || s.Cosmetics.Vip != 1 {
		t.Fatalf("после покупки VIP: %+v", s)
	}
	a.send("shop_buy", map[string]any{"item": "badge_star", "request_id": "req-ws-00000002"})
	a.shopReply()
	a.send("shop_equip", map[string]any{"slot": "badge", "item": "badge_star"})
	if s = a.shopReply(); s.Cosmetics.Badge != "star" {
		t.Fatalf("после экипировки: %+v", s)
	}
	// другой игрок видит значок и VIP
	b := connect(t, url)
	b.join("shopbbbbbbbbbbbbbbbb")
	for i := 0; i < 100; i++ {
		var st struct {
			Objects []struct {
				Kind, Badge string
				Vip         int
			}
		}
		_ = json.Unmarshal(b.next("state").Payload, &st)
		for _, o := range st.Objects {
			if o.Kind == "player" && o.Badge == "star" && o.Vip == 1 {
				return
			}
		}
	}
	t.Fatal("другой игрок не видит значок и VIP")
}

func TestShopErrors(t *testing.T) {
	url := shopServer(t)
	a := connect(t, url)
	a.join("shoperrtokentoken01")
	a.send("shop_buy", map[string]any{"item": "engine_gold", "request_id": "req-ws-00000001"})
	a.expectError("vip_required")
	a.send("shop_buy", map[string]any{"item": "nope", "request_id": "req-ws-00000002"})
	a.expectError("no_item")
	a.send("shop_buy", map[string]any{"item": "badge_star", "request_id": "x"})
	a.expectError("bad_request")
	a.send("shop_equip", map[string]any{"slot": "badge", "item": "badge_star"})
	a.expectError("not_owned")
	a.send("shop_buy", map[string]any{"item": "badge_star", "request_id": "req-ws-00000003"})
	a.shopReply()
	a.send("shop_buy", map[string]any{"item": "badge_star", "request_id": "req-ws-00000004"})
	a.expectError("already_owned")
	// повтор прежнего запроса идемпотентен
	a.send("shop_buy", map[string]any{"item": "badge_star", "request_id": "req-ws-00000003"})
	if s := a.shopReply(); len(s.State.Owned) != 1 {
		t.Fatalf("повтор запроса: %+v", s)
	}
}

func TestShopRequiresJoin(t *testing.T) {
	url := shopServer(t)
	a := connect(t, url)
	a.send("shop", nil)
	a.expectError("not_joined")
}

func TestShopDisabledWhenNotConfigured(t *testing.T) {
	_, _, url := gameServer(t)
	a := connect(t, url)
	a.join("noshoptokentoken001")
	a.send("shop", nil)
	a.expectError("unknown_type")
}
