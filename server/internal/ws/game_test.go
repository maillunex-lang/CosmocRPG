package ws

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func gameServer(t *testing.T) (*Handler, *world.MemStore, string) {
	t.Helper()
	store := world.NewMemStore()
	opts := Options{MaxMessage: 4096, RatePerSec: 200, Version: "t", World: world.New(), Store: store}
	srv, h := newServer(t, opts)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return h, store, srv.URL
}

type client struct {
	t   *testing.T
	c   *websocket.Conn
	ctx context.Context
}

func connect(t *testing.T, url string) *client {
	t.Helper()
	srv := newTestURL(url)
	c, ctx := dialURL(t, srv)
	read(t, ctx, c) // hello
	return &client{t: t, c: c, ctx: ctx}
}

func (cl *client) send(typ string, payload any) {
	cl.t.Helper()
	b, _ := json.Marshal(map[string]any{"type": typ, "id": "1", "payload": payload})
	if err := cl.c.Write(cl.ctx, websocket.MessageText, b); err != nil {
		cl.t.Fatal(err)
	}
}

// next читает сообщения, пока не придёт нужный тип (состояния могут приходить между ответами).
func (cl *client) next(typ string) Message {
	cl.t.Helper()
	for i := 0; i < 200; i++ {
		m := read(cl.t, cl.ctx, cl.c)
		if m.Type == typ {
			return m
		}
		if m.Type == "error" && typ != "error" {
			cl.t.Fatalf("неожиданная ошибка %s", m.Payload)
		}
	}
	cl.t.Fatalf("не дождались %q", typ)
	return Message{}
}

func (cl *client) join(token string) map[string]any {
	cl.t.Helper()
	cl.send("join", map[string]any{"token": token, "protocol": ProtocolVersion})
	m := cl.next("welcome")
	var p map[string]any
	_ = json.Unmarshal(m.Payload, &p)
	return p
}

type stateMsg struct {
	Time float64 `json:"time"`
	Ship struct {
		Pos   [2]float64
		Speed float64
	} `json:"ship"`
	Objects []struct {
		ID, Name, Kind string
		Pos            [2]float64
	} `json:"objects"`
	Target    string `json:"target_id"`
	Autopilot bool   `json:"autopilot"`
}

func (cl *client) state() stateMsg {
	cl.t.Helper()
	m := cl.next("state")
	var s stateMsg
	if err := json.Unmarshal(m.Payload, &s); err != nil {
		cl.t.Fatal(err)
	}
	return s
}

func (cl *client) hasObject(name string) (stateMsg, bool) {
	cl.t.Helper()
	for i := 0; i < 60; i++ {
		s := cl.state()
		for _, o := range s.Objects {
			if o.Name == name {
				return s, true
			}
		}
	}
	return stateMsg{}, false
}

func TestMessagesRequireJoin(t *testing.T) {
	_, _, url := gameServer(t)
	cl := connect(t, url)
	for _, typ := range []string{"move", "target", "autopilot"} {
		cl.send(typ, map[string]any{"x": 1, "y": 0})
		if m := cl.next("error"); errCode(t, m) != "not_joined" {
			t.Fatalf("%s: %s", typ, m.Payload)
		}
	}
}

func TestJoinValidation(t *testing.T) {
	_, _, url := gameServer(t)
	cl := connect(t, url)
	cl.send("join", map[string]any{"token": "коротко", "protocol": ProtocolVersion})
	if m := cl.next("error"); errCode(t, m) != "bad_token" {
		t.Fatalf("%s", m.Payload)
	}
	cl.send("join", map[string]any{"token": "abcdefghijklmnop", "protocol": 99})
	if m := cl.next("error"); errCode(t, m) != "bad_protocol" {
		t.Fatalf("%s", m.Payload)
	}
}

func TestTwoClientsSeeEachOther(t *testing.T) {
	_, _, url := gameServer(t)
	a := connect(t, url)
	b := connect(t, url)
	a.send("join", map[string]any{"token": "tokenaaaaaaaaaaaa", "protocol": ProtocolVersion, "name": "Альфа"})
	a.next("welcome")
	b.send("join", map[string]any{"token": "tokenbbbbbbbbbbbb", "protocol": ProtocolVersion, "name": "Бета"})
	b.next("welcome")
	if _, ok := a.hasObject("Бета"); !ok {
		t.Fatal("Альфа не видит Бету")
	}
	if _, ok := b.hasObject("Альфа"); !ok {
		t.Fatal("Бета не видит Альфу")
	}
	s := a.state()
	foundStation := false
	for _, o := range s.Objects {
		if o.Kind == "station" {
			foundStation = true
		}
		if o.Name == "Альфа" {
			t.Fatal("игрок не должен видеть себя среди объектов")
		}
	}
	if !foundStation {
		t.Fatal("станция должна быть в мире")
	}
}

func TestMoveIsSeenByOthers(t *testing.T) {
	_, _, url := gameServer(t)
	a := connect(t, url)
	b := connect(t, url)
	a.join("tokenaaaaaaaaaaaa")
	b.join("tokenbbbbbbbbbbbb")
	a.send("move", map[string]any{"x": 0, "y": 1})
	deadline := time.Now().Add(4 * time.Second)
	var moved bool
	for time.Now().Before(deadline) && !moved {
		s := b.state()
		for _, o := range s.Objects {
			if o.Kind == "player" && o.Pos[1] > 5 {
				moved = true
			}
		}
	}
	if !moved {
		t.Fatal("Бета не увидела движение Альфы")
	}
	var speed float64
	for i := 0; i < 30 && speed < 10; i++ {
		speed = a.state().Ship.Speed
	}
	if speed < 10 {
		t.Fatalf("скорость в снимке %.2f", speed)
	}
}

func TestReplacedSessionAndPositionRestored(t *testing.T) {
	h, store, url := gameServer(t)
	a := connect(t, url)
	pj := a.join("tokenaaaaaaaaaaaa")
	id := pj["player_id"].(string)
	a.send("move", map[string]any{"x": 1, "y": 0})
	time.Sleep(600 * time.Millisecond)
	// тот же токен с другого соединения: старое получает replaced
	b := connect(t, url)
	b.join("tokenaaaaaaaaaaaa")
	if m := a.next("error"); errCode(t, m) != "replaced" {
		t.Fatalf("%s", m.Payload)
	}
	if h.opts.World.Count() != 1 {
		t.Fatalf("в мире должен быть один корабль, а их %d", h.opts.World.Count())
	}
	s := b.state()
	if s.Ship.Pos[0] < 2 {
		t.Fatalf("корабль должен остаться там, где был: x=%.2f", s.Ship.Pos[0])
	}
	// отключение сохраняет позицию в хранилище
	b.send("move", map[string]any{"x": 0, "y": 0})
	b.c.CloseNow()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if r, ok := store.Get(id); ok && r.Pos.X > 2 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("позиция не сохранена при отключении")
}

func TestAutopilotThroughProtocol(t *testing.T) {
	_, _, url := gameServer(t)
	a := connect(t, url)
	a.join("tokenaaaaaaaaaaaa")
	a.send("autopilot", map[string]any{"on": true})
	if m := a.next("error"); errCode(t, m) != "no_target" {
		t.Fatalf("%s", m.Payload)
	}
	a.send("target", map[string]any{"id": "несуществует"})
	if m := a.next("error"); errCode(t, m) != "bad_target" {
		t.Fatalf("%s", m.Payload)
	}
	a.send("target", map[string]any{"id": world.StationID})
	a.send("autopilot", map[string]any{"on": true})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s := a.state()
		if s.Autopilot && s.Target == world.StationID {
			return
		}
	}
	t.Fatal("автопилот не включился")
}

func TestDoubleJoinRejected(t *testing.T) {
	_, _, url := gameServer(t)
	a := connect(t, url)
	a.join("tokenaaaaaaaaaaaa")
	a.send("join", map[string]any{"token": "tokenaaaaaaaaaaaa", "protocol": ProtocolVersion})
	if m := a.next("error"); errCode(t, m) != "already_joined" {
		t.Fatalf("%s", m.Payload)
	}
}
