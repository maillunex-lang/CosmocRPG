package social

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PGStore struct{ pool *pgxpool.Pool }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func (s *PGStore) Find(ctx context.Context, q string) (Person, bool, error) {
	var p Person
	err := s.pool.QueryRow(ctx, `SELECT public_id, name FROM players WHERE character_created AND (public_id = $1 OR lower(name) = lower($1)) LIMIT 1`, q).Scan(&p.ID, &p.Name)
	if err == pgx.ErrNoRows {
		return Person{}, false, nil
	}
	return p, err == nil, err
}

func (s *PGStore) List(ctx context.Context, owner string) ([]Friend, error) {
	rows, err := s.pool.Query(ctx, `SELECT f.friend_id, p.name FROM friendships f JOIN players p ON p.public_id = f.friend_id WHERE f.owner_id = $1 ORDER BY p.name`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Friend
	for rows.Next() {
		var f Friend
		if err := rows.Scan(&f.ID, &f.Name); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *PGStore) Add(ctx context.Context, owner string, friend Friend) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO friendships (owner_id, friend_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, owner, friend.ID)
	return err
}

func (s *PGStore) Remove(ctx context.Context, owner, friendID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM friendships WHERE owner_id = $1 AND friend_id = $2`, owner, friendID)
	return err
}
