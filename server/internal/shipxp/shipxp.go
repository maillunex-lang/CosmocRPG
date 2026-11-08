// Package shipxp — опыт и уровень экземпляров кораблей (RPG-2). Уровень корабля (1–30) вычисляется из опыта по кривой
// rpg.DefaultShipCurve; начисление идемпотентно по ключу события (общий журнал xp_grants).
package shipxp

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maillunex-lang/CosmocRPG/server/internal/rpg"
)

// ErrNoPlayer — игрока не существует.
var ErrNoPlayer = errors.New("игрок не найден")

// Store — хранилище опыта кораблей.
type Store interface {
	// Grant начисляет опыт кораблю один раз на ключ события (экземпляр создаётся при необходимости); возвращает суммарный опыт корабля.
	Grant(ctx context.Context, pid, ship, key, source string, amount int64) (granted bool, xp int64, err error)
	// All возвращает опыт всех существующих экземпляров игрока: тип корабля -> опыт.
	All(ctx context.Context, pid string) (map[string]int64, error)
}

// State — прогресс одного корабля для клиента.
type State struct {
	Ship      string `json:"ship"`
	Level     int    `json:"level"`
	MaxLevel  int    `json:"max_level"`
	XP        int64  `json:"xp"`
	IntoLevel int64  `json:"into_level"`
	ToNext    int64  `json:"to_next"` // 0 на максимальном уровне
	LevelXP   int64  `json:"level_xp"`
}

// Gain — результат начисления.
type Gain struct {
	Granted bool
	LevelUp bool
	State   State
}

// Service — сервис опыта кораблей.
type Service struct {
	store Store
	curve rpg.Curve
}

// NewService создаёт сервис со стандартной кривой.
func NewService(store Store) *Service { return &Service{store: store, curve: rpg.DefaultShipCurve} }

func (s *Service) build(ship string, xp int64) State {
	level, into, next := s.curve.LevelFor(xp)
	return State{Ship: ship, Level: level, MaxLevel: s.curve.MaxLevel, XP: xp, IntoLevel: into, ToNext: next, LevelXP: s.curve.XPForNext(level)}
}

// All возвращает прогресс всех кораблей игрока, у которых есть экземпляр.
func (s *Service) All(ctx context.Context, pid string) (map[string]State, error) {
	xps, err := s.store.All(ctx, pid)
	if err != nil {
		return nil, err
	}
	out := make(map[string]State, len(xps))
	for ship, xp := range xps {
		out[ship] = s.build(ship, xp)
	}
	return out, nil
}

// State возвращает прогресс корабля (нет экземпляра — уровень 1).
func (s *Service) State(ctx context.Context, pid, ship string) (State, error) {
	xps, err := s.store.All(ctx, pid)
	if err != nil {
		return State{}, err
	}
	return s.build(ship, xps[ship]), nil
}

// Grant начисляет опыт кораблю один раз на ключ события и сообщает о повышении уровня.
func (s *Service) Grant(ctx context.Context, pid, ship, key, source string, amount int64) (Gain, error) {
	if amount <= 0 || ship == "" {
		return Gain{}, nil
	}
	before, err := s.store.All(ctx, pid)
	if err != nil {
		return Gain{}, err
	}
	granted, total, err := s.store.Grant(ctx, pid, ship, key, source, amount)
	if err != nil {
		return Gain{}, err
	}
	g := Gain{Granted: granted, State: s.build(ship, total)}
	if granted {
		lb, _, _ := s.curve.LevelFor(before[ship])
		g.LevelUp = g.State.Level > lb
	}
	return g, nil
}

// PGStore — хранилище в PostgreSQL.
type PGStore struct{ pool *pgxpool.Pool }

// NewPGStore создаёт хранилище.
func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func (s *PGStore) Grant(ctx context.Context, pid, ship, key, source string, amount int64) (bool, int64, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO player_ships (player_id, ship_type) VALUES ($1, $2) ON CONFLICT (player_id, ship_type) DO NOTHING`, pid, ship); err != nil {
		return false, 0, err
	}
	g, err := tx.Exec(ctx, `
		INSERT INTO xp_grants (player_id, grant_key, source, amount, ship_type) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (player_id, grant_key) DO NOTHING`, pid, key, source, amount, ship)
	if err != nil {
		return false, 0, err
	}
	granted := g.RowsAffected() == 1
	if granted {
		if _, err := tx.Exec(ctx, `UPDATE player_ships SET ship_xp = ship_xp + $3 WHERE player_id = $1 AND ship_type = $2`, pid, ship, amount); err != nil {
			return false, 0, err
		}
	}
	var xp int64
	if err := tx.QueryRow(ctx, `SELECT ship_xp FROM player_ships WHERE player_id = $1 AND ship_type = $2`, pid, ship).Scan(&xp); err != nil {
		return false, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, 0, err
	}
	return granted, xp, nil
}

func (s *PGStore) All(ctx context.Context, pid string) (map[string]int64, error) {
	rows, err := s.pool.Query(ctx, `SELECT ship_type, ship_xp FROM player_ships WHERE player_id = $1`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var t string
		var xp int64
		if err := rows.Scan(&t, &xp); err != nil {
			return nil, err
		}
		out[t] = xp
	}
	return out, rows.Err()
}
