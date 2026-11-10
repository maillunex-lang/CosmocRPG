package corp

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s *PGStore) ListClans(ctx context.Context) ([]Seat, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.name, c.tag, c.owner_id, COALESCE(p.name, ''), COUNT(m.player_id)
		FROM corporations c
		LEFT JOIN players p ON p.public_id = c.owner_id
		LEFT JOIN corporation_members m ON m.corp_id = c.id
		GROUP BY c.id, p.name
		ORDER BY c.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Seat
	for rows.Next() {
		var seat Seat
		if err := rows.Scan(&seat.CorpID, &seat.Name, &seat.Tag, &seat.Owner, &seat.OwnerName, &seat.Members); err != nil {
			return nil, err
		}
		out = append(out, seat)
	}
	return out, rows.Err()
}

func (s *PGStore) AllianceByCorp(ctx context.Context, corpID string) (Alliance, []Seat, bool, error) {
	var al Alliance
	err := s.pool.QueryRow(ctx, `
		SELECT a.id, a.name, COALESCE(a.governor_id, ''), COALESCE(g.name, '')
		FROM alliance_clans ac
		JOIN alliances a ON a.id = ac.alliance_id
		LEFT JOIN players g ON g.public_id = a.governor_id
		WHERE ac.corp_id = $1`, corpID).Scan(&al.ID, &al.Name, &al.Governor, &al.GovernorName)
	if errors.Is(err, pgx.ErrNoRows) {
		return Alliance{}, nil, false, nil
	}
	if err != nil {
		return Alliance{}, nil, false, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.name, c.tag, c.owner_id, COALESCE(p.name, ''), COUNT(m.player_id)
		FROM alliance_clans ac
		JOIN corporations c ON c.id = ac.corp_id
		LEFT JOIN players p ON p.public_id = c.owner_id
		LEFT JOIN corporation_members m ON m.corp_id = c.id
		WHERE ac.alliance_id = $1
		GROUP BY c.id, p.name
		ORDER BY c.name`, al.ID)
	if err != nil {
		return Alliance{}, nil, false, err
	}
	defer rows.Close()
	var seats []Seat
	for rows.Next() {
		var seat Seat
		if err := rows.Scan(&seat.CorpID, &seat.Name, &seat.Tag, &seat.Owner, &seat.OwnerName, &seat.Members); err != nil {
			return Alliance{}, nil, false, err
		}
		seats = append(seats, seat)
	}
	al.Clans = len(seats)
	return al, seats, true, rows.Err()
}

func (s *PGStore) FoundAlliance(ctx context.Context, id, name, corpID, governor string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO alliances (id, name, governor_id) VALUES ($1,$2,$3)`, id, name, governor); err != nil {
		return ErrTaken
	}
	if _, err := tx.Exec(ctx, `INSERT INTO alliance_clans (alliance_id, corp_id) VALUES ($1,$2)`, id, corpID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PGStore) AdmitClan(ctx context.Context, allianceID, corpID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var exists int
	if err := tx.QueryRow(ctx, `SELECT 1 FROM alliances WHERE id = $1 FOR UPDATE`, allianceID).Scan(&exists); err != nil {
		return ErrNoAlliance
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM alliance_clans WHERE alliance_id = $1`, allianceID).Scan(&n); err != nil {
		return err
	}
	if n >= MaxAllianceClans {
		return ErrAllianceFull
	}
	var taken int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM alliance_clans WHERE corp_id = $1`, corpID).Scan(&taken); err != nil {
		return err
	}
	if taken > 0 {
		return ErrHasAlliance
	}
	if _, err := tx.Exec(ctx, `INSERT INTO alliance_clans (alliance_id, corp_id) VALUES ($1,$2)`, allianceID, corpID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PGStore) CastVote(ctx context.Context, allianceID, corpID, choice string) (map[string]string, []string, error) {
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO alliance_votes (alliance_id, corp_id, choice_id) VALUES ($1,$2,$3)
		ON CONFLICT (alliance_id, corp_id) DO UPDATE SET choice_id = EXCLUDED.choice_id`, allianceID, corpID, choice); err != nil {
		return nil, nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT corp_id, choice_id FROM alliance_votes WHERE alliance_id = $1`, allianceID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	votes := map[string]string{}
	for rows.Next() {
		var corp, choiceID string
		if err := rows.Scan(&corp, &choiceID); err != nil {
			return nil, nil, err
		}
		votes[corp] = choiceID
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	own, err := s.pool.Query(ctx, `
		SELECT c.owner_id FROM alliance_clans ac JOIN corporations c ON c.id = ac.corp_id WHERE ac.alliance_id = $1`, allianceID)
	if err != nil {
		return nil, nil, err
	}
	defer own.Close()
	var owners []string
	for own.Next() {
		var id string
		if err := own.Scan(&id); err != nil {
			return nil, nil, err
		}
		owners = append(owners, id)
	}
	return votes, owners, own.Err()
}

func (s *PGStore) SetGovernor(ctx context.Context, allianceID, governor string) error {
	_, err := s.pool.Exec(ctx, `UPDATE alliances SET governor_id = $2 WHERE id = $1`, allianceID, governor)
	return err
}

func (s *PGStore) Task(ctx context.Context, corpID string) (Task, bool, error) {
	var t Task
	err := s.pool.QueryRow(ctx, `SELECT corp_id, kind, title, goal, progress FROM clan_tasks WHERE corp_id = $1`, corpID).
		Scan(&t.CorpID, &t.Kind, &t.Title, &t.Goal, &t.Progress)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, false, nil
	}
	if err != nil {
		return Task{}, false, err
	}
	return t, true, nil
}

func (s *PGStore) SaveTask(ctx context.Context, task Task) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO clan_tasks (corp_id, kind, title, goal, progress) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (corp_id) DO UPDATE SET kind = EXCLUDED.kind, title = EXCLUDED.title, goal = EXCLUDED.goal, progress = EXCLUDED.progress, updated_at = now()`,
		task.CorpID, task.Kind, task.Title, task.Goal, task.Progress)
	return err
}

func (s *PGStore) AddProgress(ctx context.Context, corpID string, n int) (Task, error) {
	var t Task
	err := s.pool.QueryRow(ctx, `
		UPDATE clan_tasks SET progress = progress + $2, updated_at = now()
		WHERE corp_id = $1
		RETURNING corp_id, kind, title, goal, progress`, corpID, n).Scan(&t.CorpID, &t.Kind, &t.Title, &t.Goal, &t.Progress)
	return t, err
}

func (s *PGStore) AddTreasury(ctx context.Context, corpID string, amount int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE corporations SET treasury = treasury + $2 WHERE id = $1`, corpID, amount)
	return err
}
