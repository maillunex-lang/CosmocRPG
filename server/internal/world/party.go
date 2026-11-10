package world

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
)

// Групповой инстанс — короткая стычка в отдельном мире, не копия галактики.
// ИИ-капитан входит в пати и стреляет, но кредиты не получает и в базу не пишется.
// PvP здесь не включается и не выключается: игроки по-прежнему не бьют друг друга.

const (
	partyMax     = 4
	partyRange   = 90.0
	partyStart   = 120.0
	instanceLife = 12 * 60.0
	instancePay  = int64(500)
	inviteFor    = 45 * time.Second
)

var (
	ErrPartyFull    = errors.New("party_full")
	ErrPartyFar     = errors.New("party_too_far")
	ErrPartyNeed    = errors.New("party_need_mate")
	ErrPartyLeader  = errors.New("party_not_leader")
	ErrPartyNone    = errors.New("party_none")
	ErrPartyBusy    = errors.New("party_busy")
	ErrPartyNobody  = errors.New("party_no_one")
	ErrPartyAlready = errors.New("party_already")
	ErrPartyPick    = errors.New("party_pick")
)

// PartyView — пати в снимке. Pending: вас позвали, вы ещё не приняли.
type PartyView struct {
	ID        string            `json:"id"`
	Leader    string            `json:"leader"`
	Title     string            `json:"title,omitempty"`
	Instance  bool              `json:"instance"`
	Op        string            `json:"op,omitempty"`
	Goal      string            `json:"goal,omitempty"`
	Progress  int               `json:"progress"`
	Need      int               `json:"need"`
	Ready     int               `json:"ready,omitempty"`
	ReadyNeed int               `json:"ready_need,omitempty"`
	YouReady  bool              `json:"you_ready,omitempty"`
	YouFollow bool              `json:"you_follow"`
	Pending   bool              `json:"pending,omitempty"`
	Members   []PartyMemberView `json:"members"`
}

// PartyMemberView — один пилот пати.
type PartyMemberView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	AI      bool   `json:"ai"`
	Leader  bool   `json:"leader"`
	Ready   bool   `json:"ready,omitempty"`
	Pending bool   `json:"pending,omitempty"`
}

type partyMember struct {
	id, shipID, name string
	ai, wing         bool
	pending          bool
	ready            bool
	followOff        bool
	ship, skin       string
	profession       string
	behavior         string
	seed             uint32
	homeOK           bool
	homeSys          string
	homePos          Vec
}

type party struct {
	id       string
	leader   string
	inst     string
	op       string
	run      int
	need     int
	progress int
	settled  bool
	members  []*partyMember
}

type partyInvite struct {
	party string
	at    time.Time
}

func (r *Router) ensureParty() {
	if r.parties == nil {
		r.parties = map[string]*party{}
		r.byMember = map[string]string{}
		r.invites = map[string]partyInvite{}
	}
}

// PartyInvite зовёт игрока или ИИ-капитана рядом. ИИ принимает сразу.
func (r *Router) PartyInvite(from, target string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureParty()
	return r.partyInviteLocked(from, target, time.Now())
}

// PartyJoin принимает приглашение живого игрока.
func (r *Router) PartyJoin(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureParty()
	inv, ok := r.invites[id]
	if !ok || time.Since(inv.at) > inviteFor {
		delete(r.invites, id)
		return ErrPartyNone
	}
	p := r.parties[inv.party]
	if p == nil || p.inst != "" {
		delete(r.invites, id)
		return ErrPartyBusy
	}
	for _, m := range p.members {
		if m.id == id {
			m.pending = false
			r.byMember[id] = p.id
			delete(r.invites, id)
			return nil
		}
	}
	delete(r.invites, id)
	return ErrPartyNone
}

// PartyLeave выходит из пати. Ведущий распускает её и возвращает всех из инстанса.
func (r *Router) PartyLeave(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureParty()
	if inv, ok := r.invites[id]; ok {
		delete(r.invites, id)
		if p := r.parties[inv.party]; p != nil {
			r.dropMemberLocked(p, id)
		}
		if r.byMember[id] == "" {
			return nil
		}
	}
	pid := r.byMember[id]
	if pid == "" {
		return ErrPartyNone
	}
	p := r.parties[pid]
	if p == nil {
		delete(r.byMember, id)
		return ErrPartyNone
	}
	if p.leader == id {
		r.disbandLocked(p, false)
		return nil
	}
	if p.inst != "" {
		r.extractLocked(p, id)
	}
	r.dropMemberLocked(p, id)
	return nil
}

// PartyStart уводит принятую пати в коридор. Согласие не спрашивает: это прямой старт ведущего.
func (r *Router) PartyStart(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureParty()
	p := r.parties[r.byMember[id]]
	if p == nil || p.leader != id {
		return ErrPartyLeader
	}
	if p.op == "" {
		p.op = "credits"
	}
	return r.beginCorridorLocked(p, id)
}

// PartyArm выбирает коридор. Согласия сбрасываются: прыжок только после нового «да» от всех.
func (r *Router) PartyArm(id, kind string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureParty()
	if _, ok := corridorOps[kind]; !ok {
		return ErrPartyPick
	}
	p := r.parties[r.byMember[id]]
	if p == nil || p.leader != id {
		return ErrPartyLeader
	}
	if p.inst != "" {
		return ErrPartyBusy
	}
	for _, m := range p.members {
		if m.pending {
			return ErrPartyNeed
		}
	}
	p.op = kind
	for _, m := range p.members {
		m.ready = m.ai || m.wing
	}
	return nil
}

// PartyReady — согласие на прыжок. Когда согласились все принятые, коридор открывается сам.
func (r *Router) PartyReady(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureParty()
	p := r.parties[r.byMember[id]]
	if p == nil {
		return ErrPartyNone
	}
	if p.inst != "" {
		return ErrPartyBusy
	}
	if _, ok := corridorOps[p.op]; !ok {
		return ErrPartyPick
	}
	m := memberOf(p, id)
	if m == nil || m.pending {
		return ErrPartyNone
	}
	m.ready = true
	if !r.allReadyLocked(p) {
		return nil
	}
	return r.beginCorridorLocked(p, p.leader)
}

// PartyFollow включает или выключает автоследование за ведущим. По умолчанию включено.
func (r *Router) PartyFollow(id string, on bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureParty()
	p := r.parties[r.byMember[id]]
	m := memberOf(p, id)
	if p == nil || m == nil || m.pending {
		return ErrPartyNone
	}
	m.followOff = !on
	return nil
}

func (r *Router) allReadyLocked(p *party) bool {
	n := 0
	for _, m := range p.members {
		if m.pending {
			return false
		}
		if !m.ready && !m.ai && !m.wing {
			return false
		}
		n++
	}
	return n >= 2
}

func (r *Router) beginCorridorLocked(p *party, leaderID string) error {
	if p.inst != "" {
		return ErrPartyBusy
	}
	if _, ok := corridorOps[p.op]; !ok {
		p.op = "credits"
	}
	ready := 0
	for _, m := range p.members {
		if !m.pending {
			ready++
		}
	}
	if ready < 2 {
		return ErrPartyNeed
	}
	leader := r.byPlayer[leaderID]
	if leader == nil {
		return ErrNoPlayer
	}
	if leader.IsDead(leaderID) {
		return ErrDead
	}
	for _, m := range p.members {
		if m.pending || m.wing {
			continue
		}
		if err := r.canMoveLocked(m.id); err != nil {
			return err
		}
	}
	lpos, lok := leader.ActorPos(leaderID)
	if !lok {
		return ErrNoPlayer
	}
	for _, m := range p.members {
		if m.pending || m.id == leaderID {
			continue
		}
		pos, ok := r.memberPosLocked(m)
		if !ok {
			return ErrPartyNobody
		}
		if dist(lpos, pos) > partyStart {
			return ErrPartyFar
		}
	}
	instID := newPartyID("inst-")
	inst := newCorridor(instID, p.op)
	r.worlds[instID] = inst
	r.order = append(r.order, instID)
	p.inst = instID
	p.run++
	p.settled = false
	p.need = 3
	p.progress = 0
	slot := 0
	for _, m := range p.members {
		if m.pending {
			continue
		}
		spawn := Vec{X: float64(slot * 6), Y: -8}
		slot++
		if m.wing {
			if err := r.liftWingLocked(p, m, inst, spawn); err != nil {
				r.disbandLocked(p, false)
				return err
			}
			continue
		}
		if err := r.moveLocked(m, inst, spawn); err != nil {
			r.disbandLocked(p, false)
			return err
		}
		if m.ai {
			inst.SetWing(m.id, true)
		}
	}
	return nil
}

// PartySnapshot — пати игрока или висящее приглашение.
func (r *Router) PartySnapshot(id string) (PartyView, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureParty()
	return r.partyViewLocked(id, time.Now())
}

func (r *Router) partyInviteLocked(from, target string, now time.Time) error {
	if from == "" || target == "" || from == target {
		return ErrPartyNobody
	}
	leaderWorld := r.byPlayer[from]
	if leaderWorld == nil {
		return ErrNoPlayer
	}
	if leaderWorld.IsDead(from) {
		return ErrDead
	}
	actor, sys, ok := r.actorLocked(target)
	if !ok {
		return ErrPartyNobody
	}
	if r.systemIDLocked(leaderWorld) != sys {
		return ErrPartyFar
	}
	lpos, lok := leaderWorld.ActorPos(from)
	if !lok || dist(lpos, actor.pos) > partyRange {
		return ErrPartyFar
	}
	if r.byMember[target] != "" || r.invites[target].party != "" {
		return ErrPartyAlready
	}
	var p *party
	if cur := r.byMember[from]; cur != "" {
		p = r.parties[cur]
		if p == nil || p.leader != from {
			return ErrPartyLeader
		}
		if p.inst != "" {
			return ErrPartyBusy
		}
	} else {
		p = &party{id: newPartyID("pty-"), leader: from}
		me, _ := leaderWorld.Actor(from)
		p.members = []*partyMember{{id: from, name: me.name, ai: me.ai}}
		r.parties[p.id] = p
		r.byMember[from] = p.id
	}
	if len(p.members) >= partyMax {
		return ErrPartyFull
	}
	m := &partyMember{id: actor.id, shipID: actor.id, name: actor.name, ai: actor.ai || actor.kind == "ai_ship", ship: actor.ship, skin: actor.skin, profession: actor.profession, behavior: actor.behavior, seed: actor.seed}
	if actor.kind == "ai_ship" {
		m.wing = true
		m.id = actor.id
		m.ready = true
		p.members = append(p.members, m)
		r.byMember[m.id] = p.id
		return nil
	}
	if actor.ai {
		m.ready = true
		p.members = append(p.members, m)
		r.byMember[m.id] = p.id
		return nil
	}
	m.pending = true
	p.members = append(p.members, m)
	r.invites[target] = partyInvite{party: p.id, at: now}
	return nil
}

func (r *Router) partyViewLocked(id string, now time.Time) (PartyView, bool) {
	if inv, ok := r.invites[id]; ok && now.Sub(inv.at) > inviteFor {
		delete(r.invites, id)
		if p := r.parties[inv.party]; p != nil {
			r.dropMemberLocked(p, id)
		}
	}
	pid := r.byMember[id]
	pending := false
	if pid == "" {
		if inv, ok := r.invites[id]; ok {
			pid = inv.party
			pending = true
		}
	}
	p := r.parties[pid]
	if p == nil {
		return PartyView{}, false
	}
	v := PartyView{ID: p.id, Leader: p.leader, Op: p.op, Instance: p.inst != "", Progress: p.progress, Need: p.need, Pending: pending}
	readyN, needN := 0, 0
	you := false
	for _, m := range p.members {
		if m.pending {
			continue
		}
		needN++
		agreed := m.ready || m.ai || m.wing
		if agreed {
			readyN++
		}
		if m.id == id {
			you = agreed
		}
	}
	v.Ready, v.ReadyNeed, v.YouReady = readyN, needN, you
	v.YouFollow = true
	if me := memberOf(p, id); me != nil {
		v.YouFollow = !me.followOff
	}
	if spec, ok := corridorOps[p.op]; ok {
		v.Title = spec.title
		if p.inst != "" {
			v.Goal = spec.goal
		} else if needN > 0 {
			v.Goal = fmt.Sprintf("Согласие %d/%d. Прыжок, когда согласны все.", readyN, needN)
		}
	}
	if p.inst != "" && v.Title == "" {
		v.Title = "Стычка у маяка"
		v.Goal = "Уничтожьте пиратов у маяка"
	}
	for _, m := range p.members {
		v.Members = append(v.Members, PartyMemberView{ID: m.id, Name: m.name, AI: m.ai, Leader: m.id == p.leader, Ready: m.ready || m.ai || m.wing, Pending: m.pending})
	}
	return v, true
}

func (r *Router) tickPartiesLocked() {
	r.ensureParty()
	r.expireInvitesLocked(time.Now())
	var done []*party
	for _, p := range r.parties {
		if p.inst == "" {
			continue
		}
		w := r.worlds[p.inst]
		if w == nil {
			p.inst = ""
			continue
		}
		left := w.HostilesLeft()
		if p.need > 0 {
			p.progress = p.need - left
			if p.progress < 0 {
				p.progress = 0
			}
		}
		fail := w.Time() >= instanceLife
		if left == 0 && !p.settled {
			p.settled = true
			r.payLocked(p, w)
		}
		if (left == 0 && p.settled) || fail || !r.anyHumanLocked(p) {
			done = append(done, p)
		}
	}
	for _, p := range done {
		r.closeInstanceLocked(p)
	}
}

func (r *Router) payLocked(p *party, w *World) {
	spec := corridorOps[p.op]
	if spec.pay == 0 && spec.title == "" {
		spec = corridorOps["credits"]
	}
	for _, m := range p.members {
		if m.pending || m.ai || m.wing {
			continue
		}
		key := "party:" + p.id + ":" + itoa(p.run) + ":" + m.id
		kind := "party"
		ref := p.inst
		if spec.gear != "" {
			kind = "gear"
			ref = spec.gear
		} else if spec.ore > 0 || spec.crystals > 0 {
			ref = "iron_ore"
			w.grantHold(m.id, "iron_ore", spec.ore)
			w.grantHold(m.id, "crystals", spec.crystals)
		}
		w.pushReward(Reward{Player: m.id, Key: key, Kind: kind, Ref: ref, Credits: spec.pay})
	}
}

func (r *Router) anyHumanLocked(p *party) bool {
	w := r.worlds[p.inst]
	for _, m := range p.members {
		if m.pending || m.ai || m.wing {
			continue
		}
		if w != nil && r.byPlayer[m.id] == w {
			return true
		}
	}
	return false
}

func (r *Router) disbandLocked(p *party, pay bool) {
	if p.inst != "" {
		if pay && !p.settled {
			if w := r.worlds[p.inst]; w != nil && w.HostilesLeft() == 0 {
				p.settled = true
				r.payLocked(p, w)
			}
		}
		r.closeInstanceLocked(p)
		if p.inst != "" {
			return
		}
	}
	for _, m := range p.members {
		delete(r.byMember, m.id)
		delete(r.invites, m.id)
	}
	delete(r.parties, p.id)
}

func (r *Router) extractAllLocked(p *party) {
	for _, m := range p.members {
		if m.pending {
			continue
		}
		r.extractLocked(p, m.id)
	}
}

func (r *Router) extractLocked(p *party, id string) {
	m := memberOf(p, id)
	if m == nil || !m.homeOK {
		return
	}
	cur := r.byPlayer[id]
	if m.wing {
		if cur != nil {
			_, _ = cur.removePlayer(id, nil)
		}
		delete(r.byPlayer, id)
		if home := r.worlds[m.homeSys]; home != nil {
			id := m.shipID
			if id == "" {
				id = m.id
			}
			home.SpawnAI(AISpawn{ID: id, Name: m.name, Profession: m.profession, Behavior: m.behavior, ShipType: m.ship, Skin: m.skin, Seed: m.seed})
		}
		m.homeOK = false
		return
	}
	if cur == nil {
		m.homeOK = false
		return
	}
	home := r.worlds[m.homeSys]
	if home == nil {
		home = r.primary
	}
	if cur == home {
		m.homeOK = false
		return
	}
	rec, err := cur.removePlayer(id, nil)
	if err != nil {
		return
	}
	back := rec
	rec.System = m.homeSys
	rec.Pos = m.homePos
	if _, ok := home.Join(rec); !ok {
		if _, restored := cur.Join(back); restored {
			r.byPlayer[id] = cur
		}
		return
	}
	r.byPlayer[id] = home
	m.homeOK = false
}

func (r *Router) moveLocked(m *partyMember, inst *World, spawn Vec) error {
	from := r.byPlayer[m.id]
	if from == nil {
		return ErrNoPlayer
	}
	if from.IsDead(m.id) {
		return ErrDead
	}
	rec, err := from.removePlayer(m.id, nil)
	if err != nil {
		return err
	}
	m.homeOK = true
	m.homeSys = from.System().ID
	m.homePos = rec.Pos
	rec.System = inst.System().ID
	rec.Pos = spawn
	if _, ok := inst.Join(rec); !ok {
		rec.System = m.homeSys
		rec.Pos = m.homePos
		_, _ = from.Join(rec)
		m.homeOK = false
		return ErrNoSystem
	}
	r.byPlayer[m.id] = inst
	return nil
}

func (r *Router) liftWingLocked(p *party, m *partyMember, inst *World, spawn Vec) error {
	from := r.worldWithShipLocked(m.id)
	if from == nil {
		return ErrPartyNobody
	}
	pos, _, ok := from.objectPosPublic(m.id)
	if !ok {
		return ErrPartyNobody
	}
	leader := r.byPlayer[p.leader]
	if leader != nil {
		if lp, ok := leader.ActorPos(p.leader); ok && dist(lp, pos) > partyStart {
			return ErrPartyFar
		}
	}
	if !from.DespawnAI(m.id) {
		return ErrPartyNobody
	}
	m.homeOK = true
	m.homeSys = from.System().ID
	m.homePos = pos
	wingID := "wing-" + m.id
	rec := Record{ID: wingID, Name: m.name, AI: true, AICaptain: m.id, Created: true, ShipType: m.ship, Skin: m.skin, Pos: spawn, Dir: Vec{X: 0, Y: 1}, System: inst.System().ID, PilotSpec: "interceptor"}
	if _, ok := inst.Join(rec); !ok {
		from.SpawnAI(AISpawn{ID: m.id, Name: m.name, Profession: m.profession, Behavior: m.behavior, ShipType: m.ship, Skin: m.skin, Seed: m.seed})
		m.homeOK = false
		return ErrNoSystem
	}
	inst.BindWing(wingID)
	delete(r.byMember, m.id)
	m.id = wingID
	r.byMember[wingID] = p.id
	r.byPlayer[wingID] = inst
	return nil
}

func (r *Router) memberPosLocked(m *partyMember) (Vec, bool) {
	if m.wing {
		w := r.worldWithShipLocked(m.id)
		if w == nil {
			return Vec{}, false
		}
		pos, _, ok := w.objectPosPublic(m.id)
		return pos, ok
	}
	w := r.byPlayer[m.id]
	if w == nil {
		return Vec{}, false
	}
	return w.ActorPos(m.id)
}

func (r *Router) expireInvitesLocked(now time.Time) {
	if len(r.invites) == 0 {
		return
	}
	ids := make([]string, 0, len(r.invites))
	for id, inv := range r.invites {
		if now.Sub(inv.at) > inviteFor {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		inv := r.invites[id]
		delete(r.invites, id)
		if p := r.parties[inv.party]; p != nil {
			r.dropMemberLocked(p, id)
		}
	}
}

func (r *Router) canMoveLocked(id string) error {
	w := r.byPlayer[id]
	if w == nil {
		return ErrNoPlayer
	}
	return w.TransferErr(id)
}

func (r *Router) dropMemberLocked(p *party, id string) {
	next := p.members[:0]
	for _, m := range p.members {
		if m.id == id {
			continue
		}
		next = append(next, m)
	}
	p.members = next
	delete(r.byMember, id)
	if len(p.members) == 0 || p.leader == id {
		r.disbandLocked(p, false)
	}
}

func (r *Router) dropWorldLocked(id string) {
	delete(r.worlds, id)
	next := r.order[:0]
	for _, s := range r.order {
		if s != id {
			next = append(next, s)
		}
	}
	r.order = append([]string{}, next...)
}

func (r *Router) worldWithShipLocked(id string) *World {
	for _, sys := range r.order {
		w := r.worlds[sys]
		if w != nil && w.HasAI(id) {
			return w
		}
	}
	return nil
}

func (r *Router) actorLocked(id string) (actorInfo, string, bool) {
	for _, sys := range r.order {
		w := r.worlds[sys]
		if w == nil {
			continue
		}
		if a, ok := w.Actor(id); ok {
			return a, r.systemIDLocked(w), true
		}
	}
	return actorInfo{}, "", false
}

func (r *Router) systemIDLocked(w *World) string {
	id := w.System().ID
	if id == "" {
		return "aurora"
	}
	return id
}

func (r *Router) homePosLocked(id string) (Vec, bool) {
	if r.byMember == nil || r.parties == nil {
		return Vec{}, false
	}
	p := r.parties[r.byMember[id]]
	if p == nil || p.inst == "" {
		return Vec{}, false
	}
	m := memberOf(p, id)
	if m == nil || !m.homeOK || m.wing {
		return Vec{}, false
	}
	return m.homePos, true
}

func memberOf(p *party, id string) *partyMember {
	for _, m := range p.members {
		if m.id == id {
			return m
		}
	}
	return nil
}

func newPartyID(prefix string) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}

func nearestHostile(npcs []*npc, from Vec) *npc {
	var best *npc
	bestD := 0.0
	for _, n := range npcs {
		if n.Kind != "hostile" || n.deadUntil > 0 {
			continue
		}
		d := dist(from, n.Pos)
		if best == nil || d < bestD {
			best, bestD = n, d
		}
	}
	return best
}

type actorInfo struct {
	id, name, kind, ship, skin, profession, behavior string
	ai                                               bool
	pos                                              Vec
	seed                                             uint32
}

func newSkirmish(id string) *World { return newCorridor(id, "credits") }

type corridorOp struct {
	title, goal, bg string
	pay             int64
	gear            string
	ore, crystals   float64
}

type corridorLane struct {
	open  int
	doors []float64
	halfY float64
}

var corridorOps = map[string]corridorOp{
	"credits": {title: "Казначейский коридор", goal: "Три зала по очереди. В конце — кредиты.", bg: "xi", pay: instancePay},
	"gear":    {title: "Арсенальный коридор", goal: "Три зала по очереди. В конце — обшивка «Сталь».", bg: "omega", pay: 100, gear: "frame_steel"},
	"loot":    {title: "Трюмный коридор", goal: "Три зала по очереди. В конце — руда и кристаллы.", bg: "deep", pay: 100, ore: 8, crystals: 4},
}

func newCorridor(id, kind string) *World {
	spec, ok := corridorOps[kind]
	if !ok {
		spec = corridorOps["credits"]
	}
	w := New()
	w.system = System{ID: id, Name: spec.title, Background: spec.bg}
	w.radius = 260
	w.station.Name = "Вход коридора"
	w.station.Pos = [2]float64{0, 0}
	w.safe = nil
	w.npcs = nil
	w.deposits = nil
	w.lane = &corridorLane{doors: []float64{52, 118}, halfY: 24}
	w.layout = worldgen.Layout{
		System: id, Name: spec.title, Radius: 260, Hub: [2]float64{0, 0},
		Stations: []worldgen.Station{{ID: "beacon", Name: "Вход", Pos: [2]float64{0, 0}}},
		Zones: []worldgen.Zone{
			{ID: "hall-1", Type: "pve", Name: "Первый зал", Pos: [2]float64{28, 0}, Radius: 26, Danger: 1},
			{ID: "hall-2", Type: "pve", Name: "Второй зал", Pos: [2]float64{86, 0}, Radius: 26, Danger: 1},
			{ID: "hall-3", Type: "pve", Name: "Зал награды", Pos: [2]float64{168, 0}, Radius: 30, Danger: 2},
		},
	}
	cfg := worldgen.EnemyCfg{Speed: 8, ChaseSpeed: 22, Aggro: 400, Leash: 500, AttackRange: 26, Damage: 6, Period: 1.4}
	type spot struct {
		room int
		pos  Vec
		name string
		hull float64
	}
	spots := []spot{
		{0, Vec{X: 28}, "Дозорный зала", 40},
		{1, Vec{X: 86}, "Страж створа", 40},
		{2, Vec{X: 168}, "Хранитель награды", 55},
	}
	if kind == "loot" {
		spots[1].name = "Грузчик-мародёр"
	}
	if kind == "gear" {
		spots[2].name = "Смотритель арсенала"
	}
	for i, s := range spots {
		n := &npc{ID: "op-" + itoa(i+1), Name: s.name, Kind: "hostile", Stance: "hostile", Pos: s.pos, Hull: s.hull, HullMax: s.hull, respawn: 1e9, ephemeral: true, room: s.room}
		n.ai = &aiState{cfg: cfg, tier: "normal", home: s.pos, patrolR: 6}
		w.npcs = append(w.npcs, n)
	}
	return w
}

func (w *World) advanceLane() {
	if w.lane == nil {
		return
	}
	for w.lane.open < len(w.lane.doors) {
		alive := false
		for _, n := range w.npcs {
			if n.Kind == "hostile" && n.room == w.lane.open && n.deadUntil <= w.t {
				alive = true
			}
		}
		if alive {
			return
		}
		w.lane.open++
	}
}

func (w *World) clampLane(p Vec) Vec {
	if w.lane == nil {
		return p
	}
	if p.Y > w.lane.halfY {
		p.Y = w.lane.halfY
	}
	if p.Y < -w.lane.halfY {
		p.Y = -w.lane.halfY
	}
	if p.X < -12 {
		p.X = -12
	}
	if w.lane.open < len(w.lane.doors) && p.X > w.lane.doors[w.lane.open] {
		p.X = w.lane.doors[w.lane.open]
	}
	return p
}

func (w *World) grantHold(id, res string, n float64) {
	if n <= 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[id]
	if p == nil {
		return
	}
	room := p.spec.Cargo - holdUsed(p.hold)
	if room <= 0 {
		return
	}
	if n > room {
		n = room
	}
	if p.hold == nil {
		p.hold = map[string]float64{}
	}
	p.hold[res] += n
	p.cargoRevision++
}

// IsDead сообщает, уничтожен ли корабль.
func (w *World) IsDead(id string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[id]
	return p != nil && p.deadUntil > w.t
}

// TransferErr — можно ли забрать корабль в другой мир.
func (w *World) TransferErr(id string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[id]
	if p == nil {
		return ErrNoPlayer
	}
	if p.actionBusy() {
		return ErrEquipmentBusy
	}
	if p.deadUntil > w.t {
		return ErrDead
	}
	return nil
}

// SetWing включает самостоятельный бой ИИ-игрока.
func (w *World) SetWing(id string, on bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p := w.players[id]; p != nil && p.ai {
		p.wing = on
		p.Autopilot = false
	}
}

// BindWing помечает поднятого капитана: он стреляет и не сохраняется.
func (w *World) BindWing(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p := w.players[id]; p != nil {
		p.ai = true
		p.wing = true
		p.ephemeral = true
		p.Autopilot = false
	}
}

// HostilesLeft — сколько живых врагов осталось.
func (w *World) HostilesLeft() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	for _, npc := range w.npcs {
		if npc.Kind == "hostile" && npc.deadUntil <= w.t {
			n++
		}
	}
	return n
}

func (w *World) pushReward(rw Reward) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.rewards = append(w.rewards, rw)
}

func (w *World) takeRewards(from *World) {
	if w == nil || from == nil || w == from {
		return
	}
	from.mu.Lock()
	rs := from.rewards
	from.rewards = nil
	from.mu.Unlock()
	if len(rs) == 0 {
		return
	}
	w.mu.Lock()
	w.rewards = append(w.rewards, rs...)
	w.mu.Unlock()
}

func (r *Router) closeInstanceLocked(p *party) {
	if p == nil || p.inst == "" {
		return
	}
	if w := r.worlds[p.inst]; w != nil && r.primary != nil {
		r.primary.takeRewards(w)
	}
	r.extractAllLocked(p)
	for _, m := range p.members {
		if !m.pending && m.homeOK {
			return
		}
	}
	r.dropWorldLocked(p.inst)
	p.inst = ""
	p.progress = 0
	p.need = 0
}

// HasAI сообщает, есть ли декоративный капитан с таким id.
func (w *World) HasAI(id string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, ok := w.aiByID[id]
	return ok
}

// ActorPos — позиция игрока.
func (w *World) ActorPos(id string) (Vec, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[id]
	if p == nil {
		return Vec{}, false
	}
	return p.Pos, true
}

func (w *World) objectPosPublic(id string) (Vec, string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.objectPos(id)
}

// Actor описывает игрока или ИИ-корабль, которого можно позвать.
func (w *World) Actor(id string) (actorInfo, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p := w.players[id]; p != nil && !p.ephemeral {
		return actorInfo{id: p.ID, name: p.Name, kind: "player", ai: p.ai, ship: p.ShipType, skin: p.Skin, pos: p.Pos}, true
	}
	if a := w.aiByID[id]; a != nil {
		return actorInfo{id: a.ID, name: a.Name, kind: "ai_ship", ai: true, ship: a.ShipType, skin: a.Skin, profession: a.Profession, behavior: a.Behavior, seed: a.rng, pos: a.Pos}, true
	}
	return actorInfo{}, false
}

func skipInstanceSystem(id string) bool {
	return strings.HasPrefix(id, "inst-")
}
