// Package quest — серверная система контрактов (Этап С-01). Сервер авторитетен: прогресс и награды
// определяет только он, клиент не может ни выполнить контракт, ни получить награду. Выдача идемпотентна:
// запись о награде уникальна по (игрок, ключ), а смена состояния и начисление идут в одной транзакции.
package quest

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// Def — определение контракта. Версия растёт при изменении опубликованного контента: задания,
// начатые по старой версии, дорабатываются по ней же.
type Def struct {
	ID      string
	Version int
	Title   string
	Text    string
	Kind    string  // reach | kill | mine | sell | jump | deliver | visit
	Target  string  // идентификатор объекта/ресурса/вида точки
	System  string  // система, если задание привязано к ней; пусто — любая
	Marker  string  // подсказка карты: station | anomaly | derelict | signal
	Radius  float64 // дистанция выполнения, ед. (reach)
	Goal    int
	Reward  Reward
	After   string // цепочка: выдаётся только после завершения этого контракта (сюжет, Э-3)
}

// Reward — награда (пока только кредиты; остальные виды добавятся расширением).
type Reward struct{ Credits int64 }

// Catalog — цепочка начала кампании (Акт I, глава 1 «Сигнал из пустоты» — вертикальный срез, Э-3).
func Catalog() []Def {
	return []Def{
		{ID: "first_flight", Version: 1, Title: "Первый полёт",
			Text: "Долетите до станции «Рубеж» в Авроре.",
			Kind: "reach", Target: world.StationID, Radius: 12, Goal: 1,
			Reward: Reward{Credits: 500}},
		{ID: "first_blood", Version: 1, Title: "Сигнал тревоги",
			Text: "Победите пирата в зоне опасности Авроры.",
			Kind: "kill", Goal: 1, After: "first_flight",
			Reward: Reward{Credits: 700}},
		{ID: "first_ore", Version: 1, Title: "Руда из пустоты",
			Text: "Добудьте 5 единиц железной руды из астероидов.",
			Kind: "mine", Target: "iron_ore", Goal: 5, After: "first_blood",
			Reward: Reward{Credits: 800}},
		{ID: "first_deal", Version: 1, Title: "Первый контракт",
			Text: "Продайте добытый груз на любой станции Авроры.",
			Kind: "sell", Goal: 1, After: "first_ore",
			Reward: Reward{Credits: 600}},
		{ID: "first_jump", Version: 1, Title: "За границей",
			Text: "Совершите гиперпереход из Авроры в Меридиан.",
			Kind: "jump", Target: "meridian", Goal: 1, After: "first_deal",
			Reward: Reward{Credits: 1000}},
		{ID: "meridian_delivery", Version: 1, Title: "Груз для Меридиана",
			Text: "Привезите 5 единиц железной руды на станцию Меридиана.",
			Kind: "deliver", Target: "iron_ore", System: "meridian", Marker: "station", Goal: 5, After: "first_jump",
			Reward: Reward{Credits: 1200}},
		{ID: "meridian_trace", Version: 1, Title: "След в пустоте",
			Text: "Найдите гравитационную аномалию в Меридиане.",
			Kind: "visit", Target: "anomaly", System: "meridian", Marker: "anomaly", Goal: 1, After: "meridian_delivery",
			Reward: Reward{Credits: 900}},
		{ID: "meridian_report", Version: 1, Title: "Доклад на станции",
			Text: "Вернитесь на станцию Меридиана и сдайте доклад.",
			Kind: "visit", Target: "station", System: "meridian", Marker: "station", Goal: 1, After: "meridian_trace",
			Reward: Reward{Credits: 700}},
	}
}

// Progress — состояние контракта у игрока.
type Progress struct {
	QuestID  string
	Version  int
	State    string // active | completed
	Progress int
	Goal     int
}

// PlayerData — всё, что система знает об игроке.
type PlayerData struct {
	Credits int64
	Quests  []Progress
}

// Store — хранилище. Все изменения атомарны.
type Store interface {
	// Sync записывает определения в хранилище (идемпотентно).
	Sync(ctx context.Context, defs []Def) error
	Load(ctx context.Context, pid string) (PlayerData, error)
	// Assign выдаёт контракт, если у игрока его ещё не было. true: выдан сейчас.
	Assign(ctx context.Context, pid string, d Def) (bool, error)
	// Advance атомарно сохраняет прирост активного задания с ограничением по goal.
	Advance(ctx context.Context, pid string, d Def, n int) (int, error)
	AdvanceOnce(ctx context.Context, pid, key string, hash [32]byte, defs []Def, n int) (map[string]int, error)
	// Complete отмечает контракт выполненным и начисляет награду одной транзакцией.
	// granted=false, если контракт уже был выполнен (повтор): ничего не меняется.
	Complete(ctx context.Context, pid string, d Def) (granted bool, credits int64, err error)
	// GrantCredits начисляет кредиты один раз на ключ события (убийство, исследование, продажа).
	GrantCredits(ctx context.Context, pid, key string, amount int64) (granted bool, credits int64, err error)
	// SpendCredits списывает кредиты, если хватает (атомарно, баланс не уходит в минус). ok=false — недостаточно средств.
	SpendCredits(ctx context.Context, pid string, amount int64) (ok bool, credits int64, err error)
}

// ErrNotEnough — недостаточно кредитов (ремонт, покупка; Э-2).
var ErrNotEnough = errors.New("no_credits")

// View — то, что видит клиент.
type View struct {
	Credits int64       `json:"credits"`
	Quests  []QuestView `json:"quests"`
}

// QuestView — активный контракт для клиента.
type QuestView struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Text     string `json:"text"`
	Progress int    `json:"progress"`
	Goal     int    `json:"goal"`
	System   string `json:"system,omitempty"`
	Marker   string `json:"marker,omitempty"`
}

// Completion — событие выполнения контракта.
// Completion — завершённый контракт и начисленная награда.
type Completion struct {
	QuestID string
	Title   string
	Credits int64  // начислено сейчас
	Total   int64  // кредитов всего
	Next    string // название следующего контракта цепочки (если выдан)
}

// Service — оперативное состояние контрактов подключённых игроков (запись сквозная в хранилище).
type Service struct {
	log   *slog.Logger
	store Store
	defs  []Def
	pos   func(id string) (world.Vec, bool)

	mu      sync.Mutex
	players map[string]*playerState
}

type playerState struct {
	data PlayerData
}

// NewService создаёт сервис. pos возвращает положение объекта мира (станции).
func NewService(log *slog.Logger, store Store, pos func(id string) (world.Vec, bool)) *Service {
	return &Service{log: log, store: store, defs: Catalog(), pos: pos, players: map[string]*playerState{}}
}

// Init записывает определения в хранилище; вызывается при запуске сервера.
func (s *Service) Init(ctx context.Context) error { return s.store.Sync(ctx, s.defs) }

func (s *Service) def(id string, version int) (Def, bool) {
	for _, d := range s.defs {
		if d.ID == id && d.Version == version {
			return d, true
		}
	}
	return Def{}, false
}

// Join загружает состояние игрока и выдаёт доступные новые контракты (на этом этапе — «Первый полёт»).
func (s *Service) Join(ctx context.Context, pid string) error {
	data, err := s.store.Load(ctx, pid)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	done := map[string]bool{}
	for _, q := range data.Quests {
		have[q.QuestID] = true
		if q.State == "completed" {
			done[q.QuestID] = true
		}
	}
	for _, d := range s.defs {
		if d.Version != latestVersion(s.defs, d.ID) || have[d.ID] || (d.After != "" && !done[d.After]) {
			continue
		}
		created, err := s.store.Assign(ctx, pid, d)
		if err != nil {
			return err
		}
		if created {
			s.log.Info("контракт выдан", "player", pid, "quest", d.ID, "version", d.Version)
			data.Quests = append(data.Quests, Progress{QuestID: d.ID, Version: d.Version, State: "active", Goal: d.Goal})
		}
	}
	s.mu.Lock()
	s.players[pid] = &playerState{data: data}
	s.mu.Unlock()
	// Восстановление после сбоя между сохранением последнего события и Complete.
	for _, q := range data.Quests {
		if q.State == "active" && q.Goal > 0 && q.Progress >= q.Goal {
			if d, ok := s.def(q.QuestID, q.Version); ok {
				if _, done := s.completeOne(ctx, pid, d); done {
					s.activateFollowers(ctx, pid, d.ID)
				}
			}
		}
	}
	return nil
}

func latestVersion(defs []Def, id string) int {
	v := 0
	for _, d := range defs {
		if d.ID == id && d.Version > v {
			v = d.Version
		}
	}
	return v
}

// Leave забывает игрока (данные уже в хранилище).
func (s *Service) Leave(pid string) {
	s.mu.Lock()
	delete(s.players, pid)
	s.mu.Unlock()
}

// View возвращает состояние для клиента.
func (s *Service) View(pid string) (View, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps, ok := s.players[pid]
	if !ok {
		return View{}, false
	}
	v := View{Credits: ps.data.Credits, Quests: []QuestView{}}
	for _, q := range ps.data.Quests {
		if q.State != "active" {
			continue
		}
		d, ok := s.def(q.QuestID, q.Version)
		if !ok {
			continue
		}
		v.Quests = append(v.Quests, QuestView{ID: d.ID, Title: d.Title, Text: d.Text, Progress: q.Progress, Goal: q.Goal, System: d.System, Marker: d.Marker})
	}
	return v, true
}

// Evaluate проверяет активные контракты игрока по его положению. Возвращает события выполнения.
// Награда выдаётся хранилищем атомарно и однократно, даже при повторном вызове или гонке сессий.
// completeOne завершает контракт и активирует следующие звенья цепочки. Возвращает Completion (без Next).
func (s *Service) completeOne(ctx context.Context, pid string, d Def) (Completion, bool) {
	granted, credits, err := s.store.Complete(ctx, pid, d)
	if err != nil {
		s.log.Error("выполнение контракта", "player", pid, "quest", d.ID, "err", err)
		return Completion{}, false
	}
	if !granted {
		return Completion{}, false
	}
	s.mu.Lock()
	if ps, ok := s.players[pid]; ok {
		for i := range ps.data.Quests {
			if ps.data.Quests[i].QuestID == d.ID {
				ps.data.Quests[i].State = "completed"
				ps.data.Quests[i].Progress = ps.data.Quests[i].Goal
			}
		}
		ps.data.Credits = credits
	}
	s.mu.Unlock()
	s.log.Info("контракт выполнен, награда выдана", "player", pid, "quest", d.ID, "credits", d.Reward.Credits, "total", credits)
	return Completion{QuestID: d.ID, Title: d.Title, Credits: d.Reward.Credits, Total: credits}, true
}

// activateFollowers выдаёт контракты, ожидающие завершения doneID; возвращает название первого выданного.
func (s *Service) activateFollowers(ctx context.Context, pid, doneID string) string {
	next := ""
	for _, d := range s.defs {
		if d.After != doneID || d.Version != latestVersion(s.defs, d.ID) {
			continue
		}
		created, err := s.store.Assign(ctx, pid, d)
		if err != nil {
			s.log.Error("выдача следующего контракта", "player", pid, "quest", d.ID, "err", err)
			continue
		}
		if created {
			s.mu.Lock()
			if ps, ok := s.players[pid]; ok {
				ps.data.Quests = append(ps.data.Quests, Progress{QuestID: d.ID, Version: d.Version, State: "active", Goal: d.Goal})
			}
			s.mu.Unlock()
			s.log.Info("контракт выдан", "player", pid, "quest", d.ID, "после", doneID)
			if next == "" {
				next = d.Title
			}
		}
	}
	return next
}

// Event учитывает серверное событие (kill/mine/sell/jump) в активных контрактах игрока.
// Возвращает завершённые контракты (с Next — названием следующего).
func (s *Service) Event(ctx context.Context, pid, kind, ref string, n int) []Completion {
	return s.event(ctx, pid, "", "", kind, ref, n)
}

// EventAt — то же событие, но только для заданий указанной системы.
// Доставка и посещение не засчитываются без системы: иначе груз можно было бы сдать не там.
func (s *Service) EventAt(ctx context.Context, pid, system, kind, ref string, n int) []Completion {
	return s.event(ctx, pid, "", system, kind, ref, n)
}

// EventOnce — только для подтверждённых событий сервера с постоянным ключом.
// Receipt и изменения частичного прогресса сохраняются в одной транзакции.
func (s *Service) EventOnce(ctx context.Context, pid, key, kind, ref string, n int) []Completion {
	if key == "" || len(key) > 128 {
		return nil
	}
	return s.event(ctx, pid, key, "", kind, ref, n)
}

// Pending возвращает активное задание этого вида в системе, если прогресс ещё не добран.
func (s *Service) Pending(pid, kind, system string) (Def, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := s.players[pid]
	if ps == nil {
		return Def{}, false
	}
	for _, q := range ps.data.Quests {
		if q.State != "active" || (q.Goal > 0 && q.Progress >= q.Goal) {
			continue
		}
		d, ok := s.def(q.QuestID, q.Version)
		if !ok || d.Kind != kind {
			continue
		}
		if d.System != "" && d.System != system {
			continue
		}
		return d, true
	}
	return Def{}, false
}

func (s *Service) event(ctx context.Context, pid, key, system, kind, ref string, n int) []Completion {
	if n <= 0 {
		return nil
	}
	s.mu.Lock()
	ps, ok := s.players[pid]
	if !ok {
		s.mu.Unlock()
		return nil
	}
	var matching []Def
	for _, q := range ps.data.Quests {
		if q.State != "active" {
			continue
		}
		d, ok := s.def(q.QuestID, q.Version)
		if ok && d.Kind == kind && (d.System == "" || d.System == system) && (d.Target == "" || d.Target == ref) {
			matching = append(matching, d)
		}
	}
	s.mu.Unlock()
	var once map[string]int
	if key != "" {
		hash := sha256.Sum256([]byte(fmt.Sprintf("%q:%q:%d", kind, ref, n)))
		var err error
		once, err = s.store.AdvanceOnce(ctx, pid, key, hash, matching, n)
		if err != nil {
			s.log.Error("учёт события задания", "player", pid, "event", key, "err", err)
			return nil
		}
	}
	var bumped []Def
	for _, d := range matching {
		var progress int
		var err error
		if key != "" {
			progress = once[d.ID]
		} else {
			progress, err = s.store.Advance(ctx, pid, d, n)
		}
		if err != nil {
			s.log.Error("сохранение прогресса задания", "player", pid, "quest", d.ID, "err", err)
			continue
		}
		s.mu.Lock()
		if current := s.players[pid]; current != nil {
			for i := range current.data.Quests {
				q := &current.data.Quests[i]
				if q.QuestID == d.ID && q.Version == d.Version && q.Progress < progress {
					q.Progress = progress
				}
			}
		}
		s.mu.Unlock()
		if progress >= d.Goal {
			bumped = append(bumped, d)
		}
	}
	var out []Completion
	for _, d := range bumped {
		c, done := s.completeOne(ctx, pid, d)
		if done {
			c.Next = s.activateFollowers(ctx, pid, d.ID)
			out = append(out, c)
		}
	}
	return out
}

func (s *Service) Evaluate(ctx context.Context, pid string, pos world.Vec) []Completion {
	s.mu.Lock()
	ps, ok := s.players[pid]
	var todo []Def
	if ok {
		for _, q := range ps.data.Quests {
			if q.State != "active" {
				continue
			}
			if d, ok := s.def(q.QuestID, q.Version); ok && s.satisfied(d, pos) {
				todo = append(todo, d)
			}
		}
	}
	s.mu.Unlock()
	var out []Completion
	for _, d := range todo {
		if c, done := s.completeOne(ctx, pid, d); done {
			c.Next = s.activateFollowers(ctx, pid, d.ID)
			out = append(out, c)
		}
	}
	return out
}

// Spend списывает кредиты за покупку/ремонт (Э-2). Отказ без изменений, если не хватает.
func (s *Service) Spend(ctx context.Context, pid string, amount int64) (int64, error) {
	if amount <= 0 {
		return 0, nil
	}
	ok, credits, err := s.store.SpendCredits(ctx, pid, amount)
	if err != nil {
		return 0, err
	}
	if !ok {
		return credits, ErrNotEnough
	}
	s.mu.Lock()
	if ps, has := s.players[pid]; has {
		ps.data.Credits = credits
	}
	s.mu.Unlock()
	return credits, nil
}

// Grant выдаёт награду за серверное событие идемпотентно (повтор с тем же ключом ничего не начисляет).
// Возвращает, начислено ли сейчас, и новый баланс.
func (s *Service) Grant(ctx context.Context, pid, key string, amount int64) (bool, int64, error) {
	granted, credits, err := s.store.GrantCredits(ctx, pid, key, amount)
	if err != nil {
		return false, 0, err
	}
	s.mu.Lock()
	if ps, ok := s.players[pid]; ok {
		ps.data.Credits = credits
	}
	s.mu.Unlock()
	return granted, credits, nil
}

func (s *Service) satisfied(d Def, pos world.Vec) bool {
	switch d.Kind {
	case "reach":
		tp, ok := s.pos(d.Target)
		if !ok {
			return false
		}
		return world.Dist(pos, tp) <= d.Radius
	}
	return false
}
