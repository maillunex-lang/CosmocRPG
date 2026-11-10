package aiciv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coder/websocket"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type ProtocolMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Gateway не реализует игровые правила второй раз: передаёт intent обычному WS.
// Никаких grantXP/updateCredits или обходов world validator здесь нет.
type Gateway struct {
	store     *Store
	agent     string
	conn      *websocket.Conn
	leaseHash [32]byte
	mu        sync.Mutex
}

// OpenGateway вызывается доверенным director с локальным адресом, не из клиента.
// Разрешён только loopback, чтобы временный токен не отправился внешнему хосту.
func OpenGateway(ctx context.Context, store *Store, agent, endpoint string) (*Gateway, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "ws" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") || u.User != nil || u.Fragment != "" {
		return nil, ErrInput
	}
	token, err := store.IssueSession(ctx, agent, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	c, _, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPClient: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("ai_gateway_redirect_denied") }}})
	if err != nil {
		cleanup, cancelCleanup := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelCleanup()
		store.revokeSession(cleanup, agent, world.TokenHash(token))
		return nil, err
	}
	c.SetReadLimit(1 << 20)
	g := &Gateway{store: store, agent: agent, conn: c, leaseHash: world.TokenHash(token)}
	fail := func(e error) (*Gateway, error) { g.Close(); return nil, e }
	var hello ProtocolMessage
	if err = g.read(ctx, &hello); err != nil {
		return fail(err)
	}
	if hello.Type != "hello" {
		return fail(ErrSession)
	}
	payload, _ := json.Marshal(map[string]any{"token": token, "protocol": 1})
	if err = g.write(ctx, ProtocolMessage{Type: "join", ID: "ai-bootstrap", Payload: payload}); err != nil {
		return fail(err)
	}
	for {
		var m ProtocolMessage
		if err = g.read(ctx, &m); err != nil {
			return fail(err)
		}
		if m.Type == "error" {
			return fail(fmt.Errorf("AI join rejected: %s", m.Payload))
		}
		if m.Type == "welcome" {
			var p struct {
				NeedsCharacter bool `json:"needs_character"`
			}
			if json.Unmarshal(m.Payload, &p) != nil || p.NeedsCharacter {
				return fail(ErrIdentity)
			}
			return g, nil
		}
	}
}
func (g *Gateway) Close() {
	g.conn.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	g.store.revokeSession(ctx, g.agent, g.leaseHash)
}
func (g *Gateway) read(ctx context.Context, m *ProtocolMessage) error {
	_, b, err := g.conn.Read(ctx)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, m)
}
func (g *Gateway) write(ctx context.Context, m ProtocolMessage) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return g.conn.Write(ctx, websocket.MessageText, b)
}

func canonicalCommand(kind string, payload json.RawMessage) ([]byte, error) {
	switch kind {
	case "move", "target", "autopilot", "mine", "mine_stop", "attack", "pilot", "ships", "market", "buy", "purchase_status":
	default:
		return nil, ErrUnavailable
	}
	if len(payload) > 2048 {
		return nil, ErrInput
	}
	var value any
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &value); err != nil {
			return nil, ErrInput
		}
	}
	return json.Marshal(struct {
		Kind    string
		Payload any
	}{kind, value})
}

// Execute хранит отрицательный ответ так же, как успешный. Повтор завершённого
// action_id возвращает тот же receipt; неизвестный исход не выдаёт успех.
func (g *Gateway) Execute(ctx context.Context, id, kind string, payload json.RawMessage) (ProtocolMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if len(payload) > 2048 || !validID(id) {
		return ProtocolMessage{}, ErrInput
	}
	if kind == "buy" {
		// Ключ общей покупки стабилен даже при неизвестном исходе внешнего AI receipt.
		var p struct {
			Resource  string  `json:"resource"`
			Amount    float64 `json:"amount"`
			Operation string  `json:"operation_id"`
		}
		if json.Unmarshal(payload, &p) != nil || (p.Operation != "" && p.Operation != id) {
			return ProtocolMessage{}, ErrInput
		}
		p.Operation = id
		var err error
		payload, err = json.Marshal(p)
		if err != nil {
			return ProtocolMessage{}, ErrInput
		}
	}
	request, err := canonicalCommand(kind, payload)
	if err != nil {
		return ProtocolMessage{}, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	result, dispatch, err := g.store.claimAction(ctx, g.agent, id, request)
	if err != nil {
		return ProtocolMessage{}, err
	}
	if !dispatch {
		var m ProtocolMessage
		err = json.Unmarshal(result, &m)
		return m, err
	}
	unknown := func() (ProtocolMessage, error) {
		c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		g.store.uncertainAction(c, g.agent, id)
		g.conn.CloseNow()
		return ProtocolMessage{}, ErrUncertain
	}
	if err = g.write(ctx, ProtocolMessage{Type: kind, ID: id, Payload: payload}); err != nil {
		return unknown()
	}
	for {
		var m ProtocolMessage
		if err = g.read(ctx, &m); err != nil {
			return unknown()
		}
		if m.ID != id {
			continue
		}
		if m.Type != "ok" && m.Type != "error" && m.Type != "pilot" && m.Type != "ships" && m.Type != "market" && m.Type != "purchase_status" {
			return unknown()
		}
		result, err = json.Marshal(m)
		if err != nil {
			return unknown()
		}
		if err = g.store.finishAction(ctx, g.agent, id, result); err != nil {
			return unknown()
		}
		return m, nil
	}
}
