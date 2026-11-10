package aiciv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aiagent"
	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/pilot"
	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
	"github.com/maillunex-lang/CosmocRPG/server/internal/ws"
)

type gatewayProfiles struct{}

func (gatewayProfiles) Profiles(context.Context) ([]aiagent.Profile, error) {
	return []aiagent.Profile{{ID: "ai-gateway-test", Name: "КапитанАльфа"}}, nil
}

func TestBootstrapAtomicAndSameCharacterRules(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Pool(t)
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	w := world.New()
	b := Bootstrapper{Source: gatewayProfiles{}, Store: store, Catalog: w}
	request := CharacterRequest{"ai-gateway-test", "admin", "m", "miner"}
	if _, err := b.EnsurePlayer(ctx, request); !errors.Is(err, world.ErrNickReserved) {
		t.Fatal("bypassed human nick policy", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM players`).Scan(&count); err != nil || count != 0 {
		t.Fatal("orphan player after failed bootstrap", count, err)
	}
	request.Nick = "КапитанАльфа"
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := b.EnsurePlayer(ctx, request)
			if err != nil {
				t.Error(err)
				return
			}
			ids <- r.ID
		}()
	}
	wg.Wait()
	close(ids)
	id := ""
	for actual := range ids {
		if id != "" && id != actual {
			t.Fatal("duplicate identity")
		}
		id = actual
	}
	if id == "" {
		t.Fatal("no player")
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM players`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	var credits, xp int64
	if err := pool.QueryRow(ctx, `SELECT credits,pilot_xp FROM players WHERE public_id=$1`, id).Scan(&credits, &xp); err != nil || credits != 0 || xp != 0 {
		t.Fatal("fake starter rewards", credits, xp, err)
	}
	request.Nick = "Изменённый"
	r, err := b.EnsurePlayer(ctx, request)
	if err != nil || r.ID != id || r.Name != "КапитанАльфа" || !r.AI || r.AICaptain != request.CaptainID {
		t.Fatal("reset identity", r, err)
	}
	request.CaptainID = "invented"
	if _, err = b.EnsurePlayer(ctx, request); !errors.Is(err, ErrIdentity) {
		t.Fatal(err)
	}
}

func TestSessionRotationExpiryAndDisabledGate(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Pool(t)
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := NewStore(pool)
	b := Bootstrapper{gatewayProfiles{}, s, world.New()}
	r, err := b.EnsurePlayer(ctx, CharacterRequest{"ai-gateway-test", "КапитанАльфа", "m", "miner"})
	if err != nil {
		t.Fatal(err)
	}
	marked := &MarkedPlayers{Store: world.NewPGStore(pool), AI: s, Enabled: true}
	first, err := s.IssueSession(ctx, "ai-gateway-test", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := marked.Resolve(ctx, first, "")
	if err != nil || !rec.AI || rec.ID != r.ID {
		t.Fatal(rec, err)
	}
	second, err := s.IssueSession(ctx, "ai-gateway-test", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = marked.Resolve(ctx, first, ""); !errors.Is(err, ErrSession) {
		t.Fatal("old token valid", err)
	}
	if err = s.AuthorizeSession(ctx, r.ID, world.TokenHash(first)); !errors.Is(err, ErrSession) {
		t.Fatal(err)
	}
	if err = s.AuthorizeSession(ctx, r.ID, world.TokenHash(second)); err != nil {
		t.Fatal(err)
	}
	marked.Enabled = false
	if _, err = marked.Resolve(ctx, second, ""); !errors.Is(err, ErrDisabled) {
		t.Fatal(err)
	}
	marked.Enabled = true
	if _, err = pool.Exec(ctx, `UPDATE ai_gateway_sessions SET expires_at=now()-interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if _, err = marked.Resolve(ctx, second, ""); !errors.Is(err, ErrSession) {
		t.Fatal("expired token valid", err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM players`).Scan(&count); err != nil || count != 1 {
		t.Fatal("expired token created guest", count, err)
	}
	human, err := marked.Resolve(ctx, "human-gateway-control-token", "")
	if err != nil || human.AI {
		t.Fatal(human, err)
	}
}

func TestGatewayUsesLiveValidatorsAndDurableReceipts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool := testdb.Pool(t)
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	players := world.NewPGStore(pool)
	cfg := worldgen.DefaultConfig()
	lay := worldgen.Generate(cfg.Systems[0], cfg)
	w := world.New()
	if err := w.ApplyLayout(lay, cfg); err != nil {
		t.Fatal(err)
	}
	router := world.NewRouter(w)
	router.Add(w)
	b := Bootstrapper{gatewayProfiles{}, store, router}
	r, err := b.EnsurePlayer(ctx, CharacterRequest{"ai-gateway-test", "КапитанАльфа", "m", "miner"})
	if err != nil {
		t.Fatal(err)
	}
	var deposit string
	// Размещение — только fixture, не API/возможность gateway.
	for _, d := range lay.Deposits {
		deposit = d.ID
		r.Pos = world.Vec{X: d.Pos[0], Y: d.Pos[1]}
		break
	}
	if deposit == "" {
		t.Fatal("no deposit fixture")
	}
	if err = players.Save(ctx, r); err != nil {
		t.Fatal(err)
	}
	human, err := players.Resolve(ctx, "human-live-gateway-token", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := router.Join(human); !ok {
		t.Fatal("human join")
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	quests := quest.NewService(log, quest.NewPGStore(pool), w.ObjectPos)
	if err = quests.Init(ctx); err != nil {
		t.Fatal(err)
	}
	marked := &MarkedPlayers{Store: players, AI: store, Enabled: true}
	h := ws.NewHandler(log, ws.Options{MaxMessage: 8192, RatePerSec: 200, World: router, Store: marked, RequireCharacter: true, AIActorGuard: store.AuthorizeSession, Quests: quests, Pilot: pilot.NewService(pilot.NewPGStore(pool))})
	srv := httptest.NewServer(h)
	defer srv.Close()
	run, cancelRun := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); h.Run(run) }()
	defer func() { cancelRun(); <-done }()
	endpoint := "ws" + strings.TrimPrefix(srv.URL, "http")
	g, err := OpenGateway(ctx, store, "ai-gateway-test", endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	if w.PlayerCount() != 1 {
		t.Fatal("AI counted against humans", w.PlayerCount())
	}
	snapshot, ok := router.Snapshot(human.ID)
	if !ok {
		t.Fatal("no human snapshot")
	}
	seen := false
	for _, o := range snapshot.Objects {
		if o.ID == r.ID {
			seen = o.AI
		}
	}
	if !seen {
		t.Fatal("AI marker missing")
	}
	bad, err := g.Execute(ctx, "bad-mine", "mine", json.RawMessage(`{"id":"missing-deposit"}`))
	if err != nil || bad.Type != "error" {
		t.Fatal(bad, err)
	}
	replay, err := g.Execute(ctx, "bad-mine", "mine", json.RawMessage(`{"id":"missing-deposit"}`))
	if err != nil || errorCode(replay) != errorCode(bad) {
		t.Fatal("failed action replay changed", replay, err)
	}
	if _, err = g.Execute(ctx, "bad-mine", "mine", json.RawMessage(fmt.Sprintf(`{"id":%q}`, deposit))); !errors.Is(err, ErrReplay) {
		t.Fatal("action id reused", err)
	}
	if _, err = g.Execute(ctx, "buy", "buy", json.RawMessage(`{"resource":"iron_ore","amount":1}`)); !errors.Is(err, ErrUnavailable) {
		t.Fatal("economy gate bypassed", err)
	}
	// Прямой протокол тоже не должен обходить gate контроллера.
	if err = g.write(ctx, ProtocolMessage{Type: "buy", ID: "bypass-buy", Payload: json.RawMessage(`{"resource":"iron_ore","amount":1}`)}); err != nil {
		t.Fatal(err)
	}
	for {
		var response ProtocolMessage
		if err = g.read(ctx, &response); err != nil {
			t.Fatal(err)
		}
		if response.ID == "bypass-buy" {
			if response.Type != "error" || errorCode(response) != "capability_unavailable" {
				t.Fatal("server gate bypass", response)
			}
			break
		}
	}
	good, err := g.Execute(ctx, "mine-1", "mine", json.RawMessage(fmt.Sprintf(`{"id":%q}`, deposit)))
	if err != nil || good.Type != "ok" {
		t.Fatal(good, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		state, ok := router.Snapshot(r.ID)
		if ok && state.Ship.CargoUsed > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("real mining produced no cargo")
		}
		time.Sleep(20 * time.Millisecond)
	}
	_, err = g.Execute(ctx, "stop-1", "mine_stop", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Новая сессия заменяет lease, но сохранённый receipt не исполняется повторно.
	next, err := OpenGateway(ctx, store, "ai-gateway-test", endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	replay, err = next.Execute(ctx, "mine-1", "mine", json.RawMessage(fmt.Sprintf(`{"id":%q}`, deposit)))
	if err != nil || replay.Type != "ok" {
		t.Fatal(replay, err)
	}
	state, _ := router.Snapshot(r.ID)
	if state.Ship.Mining != "" {
		t.Fatal("replay restarted mining")
	}
	g.Close() // устаревшая сессия не должна отозвать lease новой.
	res, err := next.Execute(ctx, "pilot-1", "pilot", nil)
	if err != nil || res.Type != "pilot" {
		t.Fatal(res, err)
	}
}

func errorCode(m ProtocolMessage) string {
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(m.Payload, &p)
	return p.Code
}

func TestConcurrentReceiptClaimAndUnknownOutcome(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Pool(t)
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := NewStore(pool)
	b := Bootstrapper{gatewayProfiles{}, s, world.New()}
	if _, err := b.EnsurePlayer(ctx, CharacterRequest{"ai-gateway-test", "КапитанАльфа", "m", "miner"}); err != nil {
		t.Fatal(err)
	}
	request := []byte(`{"kind":"mine"}`)
	var wg sync.WaitGroup
	dispatches := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, dispatch, err := s.claimAction(ctx, "ai-gateway-test", "once", request)
			if err != nil && !errors.Is(err, ErrUncertain) {
				t.Error(err)
			}
			dispatches <- dispatch
		}()
	}
	wg.Wait()
	close(dispatches)
	count := 0
	for yes := range dispatches {
		if yes {
			count++
		}
	}
	if count != 1 {
		t.Fatal("more than one dispatch", count)
	}
	// После restart pending не превращается в разрешение повторить.
	s = NewStore(pool)
	if _, _, err := s.claimAction(ctx, "ai-gateway-test", "once", request); !errors.Is(err, ErrUncertain) {
		t.Fatal(err)
	}
	if err := s.finishAction(ctx, "ai-gateway-test", "once", json.RawMessage(`{"type":"ok"}`)); err != nil {
		t.Fatal(err)
	}
	result, dispatch, err := s.claimAction(ctx, "ai-gateway-test", "once", request)
	if err != nil || dispatch || len(result) == 0 {
		t.Fatal(string(result), dispatch, err)
	}
	if _, _, err = s.claimAction(ctx, "ai-gateway-test", "once", []byte(`{"kind":"attack"}`)); !errors.Is(err, ErrReplay) {
		t.Fatal(err)
	}
}

func TestGatewayFailsClosedWithoutMarkedPlayerStore(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Pool(t)
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := NewStore(pool)
	w := world.New()
	r := world.NewRouter(w)
	r.Add(w)
	b := Bootstrapper{gatewayProfiles{}, s, w}
	if _, err := b.EnsurePlayer(ctx, CharacterRequest{"ai-gateway-test", "КапитанАльфа", "m", "miner"}); err != nil {
		t.Fatal(err)
	}
	h := ws.NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), ws.Options{MaxMessage: 8192, RatePerSec: 200, World: r, Store: world.NewPGStore(pool)})
	srv := httptest.NewServer(h)
	defer srv.Close()
	if g, err := OpenGateway(ctx, s, "ai-gateway-test", "ws"+strings.TrimPrefix(srv.URL, "http")); err == nil {
		g.Close()
		t.Fatal("AI became unmarked human")
	}
	if w.PlayerCount() != 0 {
		t.Fatal("misconfigured gateway inserted human")
	}
}

func TestGatewayRejectsExternalEndpointsBeforeIssuingToken(t *testing.T) {
	for _, endpoint := range []string{"ws://example.com/ws", "wss://127.0.0.1/ws", "ws://user:password@127.0.0.1/ws", "ws://127.0.0.1/ws#fragment"} {
		if _, err := OpenGateway(context.Background(), nil, "captain", endpoint); err != ErrInput {
			t.Fatal(endpoint, err)
		}
	}
}
