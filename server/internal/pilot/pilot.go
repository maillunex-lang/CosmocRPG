// Package pilot — опыт, уровень и характеристики пилота (RPG-1). Уровень вычисляется из опыта по кривой (rpg.DefaultPilotCurve);
// начисление идемпотентно по ключу события (таблица xp_grants).
package pilot

import (
	"context"
	"errors"
	"fmt"

	"github.com/maillunex-lang/CosmocRPG/server/internal/rpg"
)

// MaxPerAttribute — потолок очков в одной характеристике (ориентир, не утверждённый баланс).
const MaxPerAttribute = 40

var (
	ErrBadAttr  = errors.New("неизвестная характеристика")
	ErrBadCount = errors.New("некорректное число очков")
	ErrNoPoints = errors.New("недостаточно свободных очков")
	ErrCap      = errors.New("достигнут предел характеристики")
	ErrNoPlayer = errors.New("игрок не найден")
)

// Store — хранилище опыта и характеристик.
type Store interface {
	// Grant начисляет опыт один раз на ключ события; возвращает суммарный опыт.
	Grant(ctx context.Context, pid, key, source string, amount int64) (granted bool, xp int64, err error)
	// Load возвращает суммарный опыт и распределённые очки.
	Load(ctx context.Context, pid string) (xp int64, attrs map[string]int, err error)
	// Update под блокировкой строки игрока применяет fn к опыту и очкам; ошибка fn отменяет изменение.
	Update(ctx context.Context, pid string, fn func(xp int64, attrs map[string]int) (map[string]int, error)) (xp int64, attrs map[string]int, err error)
}

// State — состояние пилота для клиента.
type State struct {
	Level        int            `json:"level"`
	MaxLevel     int            `json:"max_level"`
	XP           int64          `json:"xp"`
	IntoLevel    int64          `json:"into_level"`
	ToNext       int64          `json:"to_next"` // 0 на максимальном уровне
	LevelXP      int64          `json:"level_xp"`
	AttrFree     int            `json:"attr_free"`
	AttrSpent    int            `json:"attr_spent"`
	TalentPoints int            `json:"talent_points"` // накоплено; тратятся с RPG-3
	Attributes   map[string]int `json:"attributes"`
}

// Gain — результат начисления.
type Gain struct {
	Granted bool
	Amount  int64
	LevelUp bool
	State   State
}

// Service — сервис пилота.
type Service struct {
	store Store
	curve rpg.Curve
}

// NewService создаёт сервис со стандартной кривой опыта.
func NewService(store Store) *Service { return &Service{store: store, curve: rpg.DefaultPilotCurve} }

// XPFor — опыт за серверное событие (ориентиры, не утверждённый баланс): kind — kill | discover | sell | quest, credits — кредитная награда.
func XPFor(kind string, credits int64) int64 {
	if credits < 0 {
		credits = 0
	}
	switch kind {
	case "kill":
		return 15 + credits/4
	case "discover":
		return 20 + credits/4
	case "sell":
		return credits / 20
	case "quest":
		return 50 + credits/10
	}
	return 0
}

func (s *Service) build(xp int64, attrs map[string]int) State {
	level, into, next := s.curve.LevelFor(xp)
	spent := 0
	out := map[string]int{}
	for _, a := range rpg.AllAttributes {
		v := attrs[string(a)]
		out[string(a)] = v
		spent += v
	}
	free := rpg.AttributePoints(level) - spent
	if free < 0 {
		free = 0
	}
	return State{Level: level, MaxLevel: s.curve.MaxLevel, XP: xp, IntoLevel: into, ToNext: next, LevelXP: s.curve.XPForNext(level),
		AttrFree: free, AttrSpent: spent, TalentPoints: rpg.TalentPoints(level), Attributes: out}
}

// State возвращает состояние пилота.
func (s *Service) State(ctx context.Context, pid string) (State, error) {
	xp, attrs, err := s.store.Load(ctx, pid)
	if err != nil {
		return State{}, err
	}
	return s.build(xp, attrs), nil
}

// Grant начисляет опыт один раз на ключ события (повтор ничего не меняет) и сообщает о повышении уровня.
func (s *Service) Grant(ctx context.Context, pid, key, source string, amount int64) (Gain, error) {
	if amount <= 0 {
		st, err := s.State(ctx, pid)
		return Gain{State: st}, err
	}
	before, attrs, err := s.store.Load(ctx, pid)
	if err != nil {
		return Gain{}, err
	}
	granted, total, err := s.store.Grant(ctx, pid, key, source, amount)
	if err != nil {
		return Gain{}, err
	}
	g := Gain{Granted: granted, State: s.build(total, attrs)}
	if granted {
		g.Amount = amount
		lb, _, _ := s.curve.LevelFor(before)
		g.LevelUp = g.State.Level > lb
	}
	return g, nil
}

// Spend тратит свободные очки на характеристику пилота. Все проверки на сервере; сброс очков — позже, вместе с талантами (RPG-3).
func (s *Service) Spend(ctx context.Context, pid, attr string, points int) (State, error) {
	valid := false
	for _, a := range rpg.AllAttributes {
		if string(a) == attr {
			valid = true
		}
	}
	if !valid {
		return State{}, ErrBadAttr
	}
	if points < 1 || points > MaxPerAttribute {
		return State{}, ErrBadCount
	}
	xp, attrs, err := s.store.Update(ctx, pid, func(xp int64, cur map[string]int) (map[string]int, error) {
		st := s.build(xp, cur)
		if points > st.AttrFree {
			return nil, ErrNoPoints
		}
		if cur[attr]+points > MaxPerAttribute {
			return nil, ErrCap
		}
		next := map[string]int{}
		for k, v := range cur {
			next[k] = v
		}
		next[attr] += points
		return next, nil
	})
	if err != nil {
		return State{}, fmt.Errorf("%w", err)
	}
	return s.build(xp, attrs), nil
}
