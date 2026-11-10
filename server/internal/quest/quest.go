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
	Kind    string  // reach | kill | mine | sell | jump | deliver | visit | talk
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

// Catalog загружается из data/quests.json (см. data.go).

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

// ErrNoDialogue — сейчас не с кем говорить или разговор не в этой системе.
var ErrNoDialogue = errors.New("no_dialogue")

// ErrBadChoice — такого ответа в текущей реплике нет.
var ErrBadChoice = errors.New("bad_choice")

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
	Kind     string `json:"kind,omitempty"`
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
	line string // текущая реплика разговора; пусто — начало сцены
}

// NewService создаёт сервис. pos возвращает положение объекта мира (станции).
func NewService(log *slog.Logger, store Store, pos func(id string) (world.Vec, bool)) *Service {
	return &Service{log: log, store: store, defs: Catalog(), pos: pos, players: map[string]*playerState{}}
}

// Init записывает определения в хранилище; вызывается при запуске сервера.
func (s *Service) Init(ctx context.Context) error { return s.store.Sync(ctx, s.defs) }
