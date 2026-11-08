// Package quest — серверная система контрактов (Этап С-01). Сервер авторитетен: прогресс и награды
// определяет только он, клиент не может ни выполнить контракт, ни получить награду. Выдача идемпотентна:
// запись о награде уникальна по (игрок, ключ), а смена состояния и начисление идут в одной транзакции.
package quest

import (
	"context"
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
	Kind    string  // "reach": подлететь к объекту
	Target  string  // идентификатор объекта мира
	Radius  float64 // дистанция выполнения, ед.
	Goal    int
	Reward  Reward
}

// Reward — награда (пока только кредиты; остальные виды добавятся расширением).
type Reward struct{ Credits int64 }

// Catalog — доступные контракты. Версионирование: старые версии остаются в списке, пока по ним есть активные задания.
func Catalog() []Def {
	return []Def{{
		ID: "first_flight", Version: 1, Title: "Первый полёт",
		Text: "Долетите до станции «Рубеж».",
		Kind: "reach", Target: world.StationID, Radius: 12, Goal: 1,
		Reward: Reward{Credits: 500},
	}}
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
	// Complete отмечает контракт выполненным и начисляет награду одной транзакцией.
	// granted=false, если контракт уже был выполнен (повтор): ничего не меняется.
	Complete(ctx context.Context, pid string, d Def) (granted bool, credits int64, err error)
}

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
}

// Completion — событие выполнения контракта.
type Completion struct {
	QuestID string
	Title   string
	Credits int64 // начислено сейчас
	Total   int64 // кредитов всего
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
	for _, q := range data.Quests {
		have[q.QuestID] = true
	}
	for _, d := range s.defs {
		if d.Version != latestVersion(s.defs, d.ID) || have[d.ID] {
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
		v.Quests = append(v.Quests, QuestView{ID: d.ID, Title: d.Title, Text: d.Text, Progress: q.Progress, Goal: q.Goal})
	}
	return v, true
}

// Evaluate проверяет активные контракты игрока по его положению. Возвращает события выполнения.
// Награда выдаётся хранилищем атомарно и однократно, даже при повторном вызове или гонке сессий.
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
		granted, credits, err := s.store.Complete(ctx, pid, d)
		if err != nil {
			s.log.Error("выполнение контракта", "player", pid, "quest", d.ID, "err", err)
			continue
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
		if granted {
			s.log.Info("контракт выполнен, награда выдана", "player", pid, "quest", d.ID, "credits", d.Reward.Credits, "total", credits)
			out = append(out, Completion{QuestID: d.ID, Title: d.Title, Credits: d.Reward.Credits, Total: credits})
		}
	}
	return out
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
