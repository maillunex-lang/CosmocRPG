// Command bot — тестовый клиент: подключается как второй игрок и летает по кругу вокруг центра системы.
// Нужен, чтобы проверять «игроки видят друг друга» без второго устройства. Не часть игры.
//
// Запуск: go run ./cmd/bot -url ws://127.0.0.1:8080/ws -name "Бот-Орбита" -radius 40
// Токен берётся из -token или COSMOC_BOT_TOKEN (по умолчанию фиксированный тестовый).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"time"

	"github.com/coder/websocket"
)

type msg struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func main() {
	url := flag.String("url", "ws://127.0.0.1:8080/ws", "адрес сервера")
	name := flag.String("name", "Бот-Орбита", "имя корабля")
	radius := flag.Float64("radius", 40, "радиус круга, ед.")
	token := flag.String("token", os.Getenv("COSMOC_BOT_TOKEN"), "токен игрока")
	flag.Parse()
	if *token == "" {
		*token = "bot-token-orbit-0000000001"
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	for ctx.Err() == nil {
		if err := run(ctx, *url, *name, *token, *radius); err != nil {
			log.Printf("бот: %v; повтор через 3 с", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(3 * time.Second):
		}
	}
}

func send(ctx context.Context, c *websocket.Conn, typ string, payload any) error {
	b, _ := json.Marshal(msg{Type: typ, ID: "b", Payload: mustJSON(payload)})
	return c.Write(ctx, websocket.MessageText, b)
}

func mustJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func run(ctx context.Context, url, name, token string, radius float64) error {
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return err
	}
	defer c.CloseNow()
	if err := send(ctx, c, "join", map[string]any{"token": token, "protocol": 1, "name": name}); err != nil {
		return err
	}
	fmt.Println("бот подключён:", name)
	var lastSent time.Time
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return err
		}
		var m msg
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		if m.Type == "error" {
			return fmt.Errorf("ошибка сервера: %s", m.Payload)
		}
		if m.Type != "state" || time.Since(lastSent) < 200*time.Millisecond {
			continue
		}
		var s struct {
			Ship struct{ Pos [2]float64 } `json:"ship"`
		}
		if json.Unmarshal(m.Payload, &s) != nil {
			continue
		}
		x, y := s.Ship.Pos[0], s.Ship.Pos[1]
		d := math.Hypot(x, y)
		// касательная к кругу плюс поправка к нужному радиусу
		var tx, ty float64 = 1, 0
		if d > 1e-6 {
			tx, ty = -y/d, x/d
		}
		k := math.Max(-1, math.Min(1, (radius-d)/10))
		rx, ry := 0.0, 0.0
		if d > 1e-6 {
			rx, ry = x/d*k, y/d*k
		}
		vx, vy := tx+rx, ty+ry
		l := math.Hypot(vx, vy)
		if l > 1 {
			vx, vy = vx/l, vy/l
		}
		lastSent = time.Now()
		if err := send(ctx, c, "move", map[string]float64{"x": vx, "y": vy}); err != nil {
			return err
		}
	}
}
