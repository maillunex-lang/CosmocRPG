// Package location — журнал первых прибытий. Сервер записывает точку один раз на игрока,
// систему и объект. Повтор сессии не создаёт вторую строку.
package location

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Visit — одно посещение именованной точки.
type Visit struct {
	Player string    `json:"-"`
	System string    `json:"system"`
	ID     string    `json:"id"`
	Name   string    `json:"name"`
	Kind   string    `json:"kind"`
	At     time.Time `json:"at,omitempty"`
}

// Store хранит журнал. Record идемпотентен.
type Store interface {
	Record(ctx context.Context, v Visit) error
	List(ctx context.Context, player string) ([]Visit, error)
}

// PGStore — журнал в PostgreSQL.
type PGStore struct{ pool *pgxpool.Pool }

// NewPGStore создаёт хранилище.
func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func (s *PGStore) Record(ctx context.Context, v Visit) error {
	if v.Player == "" || v.System == "" || v.ID == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO location_visits (player_id, system_id, object_id, name, kind)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (player_id, system_id, object_id) DO NOTHING`,
		v.Player, v.System, v.ID, v.Name, v.Kind)
	return err
}

func (s *PGStore) List(ctx context.Context, player string) ([]Visit, error) {
	if player == "" {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT system_id, object_id, name, kind, visited_at
		FROM location_visits WHERE player_id = $1
		ORDER BY visited_at, object_id`, player)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Visit
	for rows.Next() {
		var v Visit
		v.Player = player
		if err := rows.Scan(&v.System, &v.ID, &v.Name, &v.Kind, &v.At); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ErrClosed — запись отклонена: так тесты отличают отказ хранилища.
var ErrClosed = errors.New("location store closed")
