package aiciv

import (
	"context"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/corp"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// WorldView is the minimal world surface the farm controller needs.
type WorldView interface {
	Snapshot(id string) (world.State, bool)
}

// FarmController joins reserved captains as real players and runs their goals.
type FarmController struct {
	Bootstrap  *Bootstrapper
	Store      *Store
	World      WorldView
	Speaker    *ChatSpeaker
	Chorus     *Chorus
	Board      *Board
	Live       *Live
	Res        *Reservation
	Endpoint   string
	Fallback   []string
	LowHull    float64
	Log        *slog.Logger
	Clans      *corp.Service
	Galaxy     Galaxy
	Quests     QuestMind
	censusOnce sync.Once
	census     *castCensus
}

// CastMember is one captain to put in the world and in the chat.
type CastMember struct {
	ID, Name string
	Goal     GoalKind
}

func nearestHostile(st world.State, me [2]float64) (id string, pos [2]float64) {
	best, bd := "", math.MaxFloat64
	var bp [2]float64
	for _, o := range st.Objects {
		if o.Kind != "hostile" || o.Dead || o.ID == "" {
			continue
		}
		dx, dy := o.Pos[0]-me[0], o.Pos[1]-me[1]
		d := math.Hypot(dx, dy)
		if d < bd {
			best, bd, bp = o.ID, d, o.Pos
		}
	}
	return best, bp
}

// Join ensures the captain has a player and opens a Gateway. Does not start combat.
func (f *FarmController) Join(ctx context.Context, captainID, nick string) (playerID string, g *Gateway, err error) {
	if f == nil || f.Bootstrap == nil || f.Store == nil || f.Endpoint == "" || captainID == "" {
		return "", nil, ErrInput
	}
	if f.Res != nil {
		f.Res.Reserve(captainID)
	}
	if _, nickErr := world.CleanNick(nick); nickErr != nil {
		nick = "Cap" + digitsOf(captainID)
		if _, nickErr = world.CleanNick(nick); nickErr != nil {
			nick = "Captain"
		}
	}
	rec, err := f.Bootstrap.EnsurePlayer(ctx, CharacterRequest{
		CaptainID: captainID,
		Nick:      nick,
		Gender:    "m",
		PilotSpec: "interceptor",
	})
	if err != nil {
		return "", nil, err
	}
	g, err = OpenGateway(ctx, f.Store, captainID, f.Endpoint)
	if err != nil {
		return "", nil, err
	}
	return rec.ID, g, nil
}

// RunCast joins several captains, runs each goal, and registers them as chat voices.
// Blocks until ctx is cancelled, then closes every gateway.
func (f *FarmController) RunCast(ctx context.Context, members []CastMember) {
	if f == nil || len(members) == 0 {
		return
	}
	var open []*Gateway
	for i, m := range members {
		if m.ID == "" {
			continue
		}
		if i > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
		joinCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		playerID, g, err := f.Join(joinCtx, m.ID, m.Name)
		cancel()
		if err != nil {
			if f.Log != nil {
				f.Log.Error("AI join failed", "captain", m.ID, "err", err)
			}
			continue
		}
		name := m.Name
		if name == "" {
			name = "AI-" + m.ID
		}
		goal := m.Goal
		if goal == "" {
			goal = GoalFarmCredits
		}
		arch := ArchetypeFor(m.ID, goal)
		if f.Chorus != nil {
			f.Chorus.Add(Voice{ID: m.ID, Name: name, Goal: goal, Arch: arch})
		}
		if f.Log != nil {
			f.Log.Info("AI civilization: agent joined", "captain", m.ID, "name", name, "goal", string(goal), "player", playerID)
		}
		runner := &GoalRunner{World: f.World, Store: f.Store, Board: f.Board, Live: f.Live, Log: f.Log, LowHull: f.LowHull, Name: name, Clans: f.Clans, Galaxy: f.Galaxy, Quests: f.Quests}
		go runner.RunGoal(ctx, g, m.ID, playerID, goal)
		open = append(open, g)
	}
	if f.Res != nil && len(open) > 0 {
		go f.keepReserved(ctx, members)
	}
	<-ctx.Done()
	for _, g := range open {
		g.Close()
	}
}

func (f *FarmController) keepReserved(ctx context.Context, members []CastMember) {
	t := time.NewTicker(2 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, m := range members {
				f.Res.Reserve(m.ID)
			}
		}
	}
}

// Supervise keeps the cast aligned with admin knobs.
// A failed join is remembered and retried; captains already in the world stay.
func (f *FarmController) Supervise(ctx context.Context, src func(n int) []CastMember) {
	if f == nil || src == nil {
		return
	}
	running := map[string]context.CancelFunc{}
	var mu sync.Mutex
	stopAll := func() {
		mu.Lock()
		defer mu.Unlock()
		for id, cancel := range running {
			cancel()
			delete(running, id)
		}
	}
	defer stopAll()
	select {
	case <-ctx.Done():
		return
	case <-time.After(3 * time.Second):
	}
	apply := func() {
		k := DefaultKnobs()
		if f.Live != nil {
			k = f.Live.Get()
		}
		want := 0
		if k.Enabled {
			want = k.Cast
		}
		if want > 500 {
			want = 500
		}
		f.setWant(want)
		if want == 0 {
			stopAll()
			f.noteErr("")
			return
		}
		members := src(want)
		if members == nil {
			f.noteErr("в AI Studio нет профилей капитанов")
			return
		}
		if len(members) == 0 {
			stopAll()
			f.noteErr("")
			return
		}
		keep := map[string]struct{}{}
		for _, m := range members {
			if m.ID != "" {
				keep[m.ID] = struct{}{}
			}
		}
		mu.Lock()
		defer mu.Unlock()
		for id, cancel := range running {
			if _, ok := keep[id]; !ok {
				cancel()
				delete(running, id)
			}
		}
		n := 0
		for _, m := range members {
			if m.ID == "" {
				continue
			}
			if _, ok := running[m.ID]; ok {
				continue
			}
			cctx, cancel := context.WithCancel(ctx)
			running[m.ID] = cancel
			wait := time.Duration(n) * 350 * time.Millisecond
			n++
			go func(m CastMember, wait time.Duration) {
				if wait > 0 {
					select {
					case <-cctx.Done():
						return
					case <-time.After(wait):
					}
				}
				f.runOne(cctx, m)
			}(m, wait)
		}
		if f.Log != nil {
			f.Log.Info("AI civilization cast", "captains", len(keep), "endpoint", f.Endpoint)
		}
	}
	apply()
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			apply()
		}
	}
}

func (f *FarmController) runOne(ctx context.Context, m CastMember) {
	defer f.noteLeave(m.ID)
	for {
		if ctx.Err() != nil {
			return
		}
		joinCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		playerID, g, err := f.Join(joinCtx, m.ID, m.Name)
		cancel()
		if err != nil {
			f.noteErr(m.ID + ": " + err.Error())
			if f.Log != nil {
				f.Log.Error("AI join failed", "captain", m.ID, "err", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(8 * time.Second):
			}
			continue
		}
		name := m.Name
		if name == "" {
			name = m.ID
		}
		f.noteJoin(m.ID, name)
		f.noteErr("")
		goal := m.Goal
		if goal == "" {
			goal = GoalFarmCredits
		}
		if f.Chorus != nil {
			f.Chorus.Add(Voice{ID: m.ID, Name: name, Goal: goal, Arch: ArchetypeFor(m.ID, goal)})
		}
		if f.Log != nil {
			f.Log.Info("AI civilization: agent joined", "captain", m.ID, "name", name, "player", playerID)
		}
		go func() {
			tick := time.NewTicker(3 * time.Minute)
			defer tick.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tick.C:
					c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					err := g.Touch(c)
					cancel()
					if err != nil && f.Log != nil {
						f.Log.Error("AI session renew", "captain", m.ID, "err", err)
					}
				}
			}
		}()
		runner := &GoalRunner{World: f.World, Store: f.Store, Board: f.Board, Live: f.Live, Log: f.Log, LowHull: f.LowHull, Name: name, Clans: f.Clans, Galaxy: f.Galaxy, Quests: f.Quests}
		runner.RunGoal(ctx, g, m.ID, playerID, goal)
		g.Close()
		f.noteLeave(m.ID)
		if ctx.Err() != nil {
			return
		}
	}
}

func digitsOf(id string) string {
	var b strings.Builder
	for _, r := range id {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" {
		s = "100"
	}
	if len(s) > 12 {
		s = s[len(s)-12:]
	}
	return s
}

type castCensus struct {
	mu      sync.Mutex
	want    int
	online  map[string]string
	lastErr string
}

func (f *FarmController) state() *castCensus {
	f.censusOnce.Do(func() {
		f.census = &castCensus{online: map[string]string{}}
	})
	return f.census
}

func (f *FarmController) setWant(n int) {
	st := f.state()
	st.mu.Lock()
	st.want = n
	st.mu.Unlock()
}

func (f *FarmController) noteJoin(id, name string) {
	st := f.state()
	st.mu.Lock()
	if st.online == nil {
		st.online = map[string]string{}
	}
	st.online[id] = name
	st.mu.Unlock()
}

func (f *FarmController) noteLeave(id string) {
	st := f.state()
	st.mu.Lock()
	delete(st.online, id)
	st.mu.Unlock()
}

func (f *FarmController) noteErr(text string) {
	st := f.state()
	st.mu.Lock()
	st.lastErr = text
	st.mu.Unlock()
}

// Census — сколько капитанов просили и сколько реально в мире.
func (f *FarmController) Census() (want, online int, lastErr string) {
	if f == nil {
		return 0, 0, ""
	}
	st := f.state()
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.want, len(st.online), st.lastErr
}

// OnlineSet — идентификаторы студии, которые сейчас в мире.
func (f *FarmController) OnlineSet() map[string]bool {
	out := map[string]bool{}
	if f == nil {
		return out
	}
	st := f.state()
	st.mu.Lock()
	defer st.mu.Unlock()
	for id := range st.online {
		out[id] = true
	}
	return out
}

// Roster — кто сейчас в мире: идентификатор студии и ник.
func (f *FarmController) Roster() map[string]string {
	out := map[string]string{}
	if f == nil {
		return out
	}
	st := f.state()
	st.mu.Lock()
	defer st.mu.Unlock()
	for id, name := range st.online {
		out[id] = name
	}
	return out
}
