package world

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Ошибки аккаунтов (коды протокола совпадают с текстом ошибки).
var (
	ErrLoginTaken = errors.New("login_taken")
	ErrHasAccount = errors.New("already_has_account")
	ErrNoAccount  = errors.New("no_account")
)

// AccountStore — хранение аккаунтов. Токен устройства — секрет клиента; в хранилище только его SHA-256.
type AccountStore interface {
	// CreateAccount находит (или создаёт) игрока по токену и привязывает к нему аккаунт.
	CreateAccount(ctx context.Context, token, name, login, passHash string) (Record, error)
	// AccountHash возвращает хеш пароля и игрока аккаунта.
	AccountHash(ctx context.Context, login string) (hash, playerID string, ok bool, err error)
	// LinkToken привязывает токен нового устройства к игроку (последующий вход по токену ведёт к нему).
	LinkToken(ctx context.Context, token, playerID string) error
	// AccountLogin возвращает логин аккаунта игрока ("" — аккаунта нет).
	AccountLogin(ctx context.Context, playerID string) (string, error)
}

// ---- PostgreSQL ----

func (s *PGStore) CreateAccount(ctx context.Context, token, name, login, passHash string) (Record, error) {
	rec, err := s.Resolve(ctx, token, name)
	if err != nil {
		return Record{}, err
	}
	var have string
	switch err := s.pool.QueryRow(ctx, `SELECT login FROM accounts WHERE player_public_id = $1`, rec.ID).Scan(&have); {
	case err == nil:
		return Record{}, ErrHasAccount
	case err != pgx.ErrNoRows:
		return Record{}, err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO accounts (login, password_hash, player_public_id) VALUES ($1, $2, $3)`, login, passHash, rec.ID)
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			if pe.ConstraintName == "accounts_login_key" {
				return Record{}, ErrLoginTaken
			}
			return Record{}, ErrHasAccount
		}
		return Record{}, fmt.Errorf("аккаунт: %w", err)
	}
	return rec, nil
}

func (s *PGStore) AccountHash(ctx context.Context, login string) (string, string, bool, error) {
	var hash, pid string
	err := s.pool.QueryRow(ctx, `SELECT password_hash, player_public_id FROM accounts WHERE login = $1`, login).Scan(&hash, &pid)
	if err == pgx.ErrNoRows {
		return "", "", false, nil
	}
	return hash, pid, err == nil, err
}

func (s *PGStore) LinkToken(ctx context.Context, token, playerID string) error {
	h := TokenHash(token)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO player_tokens (token_hash, player_public_id) VALUES ($1, $2)
		ON CONFLICT (token_hash) DO UPDATE SET player_public_id = EXCLUDED.player_public_id`, h[:], playerID)
	if err == nil {
		_, _ = s.pool.Exec(ctx, `UPDATE accounts SET last_login_at = now() WHERE player_public_id = $1`, playerID)
	}
	return err
}

func (s *PGStore) AccountLogin(ctx context.Context, playerID string) (string, error) {
	var login string
	err := s.pool.QueryRow(ctx, `SELECT login FROM accounts WHERE player_public_id = $1`, playerID).Scan(&login)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return login, err
}

// ---- память (тесты) ----

type memAccount struct{ hash, playerID string }

type memAccounts struct {
	mu       sync.Mutex
	accounts map[string]memAccount // login -> аккаунт
	tokens   map[string]string     // публичный id токена -> игрок
}

func (m *MemStore) acc() *memAccounts {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.accs == nil {
		m.accs = &memAccounts{accounts: map[string]memAccount{}, tokens: map[string]string{}}
	}
	return m.accs
}

func (m *MemStore) CreateAccount(ctx context.Context, token, name, login, passHash string) (Record, error) {
	rec, err := m.Resolve(ctx, token, name)
	if err != nil {
		return Record{}, err
	}
	a := m.acc()
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.accounts[login]; ok {
		return Record{}, ErrLoginTaken
	}
	for _, x := range a.accounts {
		if x.playerID == rec.ID {
			return Record{}, ErrHasAccount
		}
	}
	a.accounts[login] = memAccount{passHash, rec.ID}
	return rec, nil
}

func (m *MemStore) AccountHash(_ context.Context, login string) (string, string, bool, error) {
	a := m.acc()
	a.mu.Lock()
	defer a.mu.Unlock()
	x, ok := a.accounts[login]
	return x.hash, x.playerID, ok, nil
}

func (m *MemStore) LinkToken(_ context.Context, token, playerID string) error {
	a := m.acc()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tokens[PublicID(TokenHash(token))] = playerID
	return nil
}

func (m *MemStore) AccountLogin(_ context.Context, playerID string) (string, error) {
	a := m.acc()
	a.mu.Lock()
	defer a.mu.Unlock()
	for l, x := range a.accounts {
		if x.playerID == playerID {
			return l, nil
		}
	}
	return "", nil
}
