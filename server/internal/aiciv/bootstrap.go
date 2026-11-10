package aiciv

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aiagent"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

var ErrIdentity = errors.New("ai_identity_unavailable")
var ErrSession = errors.New("ai_session_invalid")
var ErrDisabled = errors.New("ai_gateway_disabled")

// Bootstrapper доступен только серверному director, не клиентскому endpoint.
// Profile.ID проверяется по активному Studio; игровой прогресс туда не пишется.
type Bootstrapper struct {
	Source  aiagent.CaptainSource
	Store   *Store
	Catalog world.StarterSource
}
type CharacterRequest struct{ CaptainID, Nick, Gender, PilotSpec string }

func (b *Bootstrapper) EnsurePlayer(ctx context.Context, r CharacterRequest) (world.Record, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if !validID(r.CaptainID) {
		return world.Record{}, ErrInput
	}
	profiles, err := b.Source.Profiles(ctx)
	if err != nil {
		return world.Record{}, err
	}
	found := false
	for _, p := range profiles {
		if p.ID == r.CaptainID {
			found = true
			break
		}
	}
	if !found {
		return world.Record{}, ErrIdentity
	}
	tx, err := b.Store.pool.Begin(ctx)
	if err != nil {
		return world.Record{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO ai_agents(studio_captain_id) VALUES($1) ON CONFLICT DO NOTHING`, r.CaptainID); err != nil {
		return world.Record{}, err
	}
	var player string
	if err = tx.QueryRow(ctx, `SELECT COALESCE(player_id,'') FROM ai_agents WHERE studio_captain_id=$1 FOR UPDATE`, r.CaptainID).Scan(&player); err != nil {
		return world.Record{}, err
	}
	ps := world.NewTransactionalStore(tx)
	var rec world.Record
	if player != "" {
		rec, err = ps.Load(ctx, player)
	} else {
		nonce := make([]byte, 32)
		if _, err = rand.Read(nonce); err != nil {
			return world.Record{}, err
		}
		// Одноразовый начальный токен не сохраняется и не отдаётся ни клиенту, ни AI.
		rec, err = ps.Resolve(ctx, hex.EncodeToString(nonce), r.Nick)
		if err == nil {
			rec, err = world.CreatePilot(ctx, ps, b.Catalog, rec.ID, r.Nick, r.Gender, r.PilotSpec)
		}
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE ai_agents SET player_id=$2,revision=revision+1,updated_at=now() WHERE studio_captain_id=$1`, r.CaptainID, rec.ID)
		}
	}
	if err != nil {
		return world.Record{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return world.Record{}, err
	}
	rec.AI = true
	rec.AICaptain = r.CaptainID
	return rec, nil
}

// IssueSession создаёт только хеш временного токена. Одна действующая lease
// на капитана; старая немедленно перестаёт авторизовывать команды.
func (s *Store) IssueSession(ctx context.Context, id string, ttl time.Duration) (string, error) {
	if !validID(id) || ttl < time.Second || ttl > 10*time.Minute {
		return "", ErrInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var player string
	if err = tx.QueryRow(ctx, `SELECT player_id FROM ai_agents WHERE studio_captain_id=$1 AND player_id IS NOT NULL FOR UPDATE`, id).Scan(&player); err != nil {
		return "", ErrIdentity
	}
	nonce := make([]byte, 32)
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	token := world.AIServiceTokenPrefix + hex.EncodeToString(nonce)
	hash := world.TokenHash(token)
	// Удаляется только прежний токен данного AI, а не сессии людей.
	if _, err = tx.Exec(ctx, `DELETE FROM player_tokens WHERE token_hash IN (SELECT token_hash FROM ai_gateway_sessions WHERE agent_id=$1)`, id); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO player_tokens(token_hash,player_public_id) VALUES($1,$2)`, hash[:], player); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO ai_gateway_sessions(agent_id,token_hash,expires_at) VALUES($1,$2,now()+$3::bigint*interval '1 second')
 ON CONFLICT(agent_id) DO UPDATE SET token_hash=EXCLUDED.token_hash,expires_at=EXCLUDED.expires_at`, id, hash[:], int64(ttl/time.Second)); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return token, nil
}

// RenewSession продлевает уже открытый вход, не меняя токен соединения.
func (s *Store) RenewSession(ctx context.Context, agent string, hash [32]byte, ttl time.Duration) error {
	if s == nil || !validID(agent) {
		return ErrInput
	}
	if ttl < time.Second || ttl > 10*time.Minute {
		ttl = 10 * time.Minute
	}
	tag, err := s.pool.Exec(ctx, `UPDATE ai_gateway_sessions SET expires_at=now()+$2::bigint*interval '1 second'
 WHERE agent_id=$1 AND token_hash=$3 AND expires_at>now()`, agent, int64(ttl/time.Second), hash[:])
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSession
	}
	return nil
}

// AuthorizeSession использует время БД, не время клиента. AI-владелец определяется
// по собственной persistent binding, а не флагу AI в присланном сообщении.
func (s *Store) AuthorizeSession(ctx context.Context, player string, hash [32]byte) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ai_gateway_sessions s JOIN ai_agents a ON a.studio_captain_id=s.agent_id
 WHERE a.player_id=$1 AND s.token_hash=$2 AND s.expires_at>now())`, player, hash[:]).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return ErrSession
	}
	return nil
}

// MarkedPlayers оборачивает обычное хранилище. AI-метка не принимается от клиента.
type MarkedPlayers struct {
	world.Store
	AI      *Store
	Enabled bool
}

func (s *MarkedPlayers) Resolve(ctx context.Context, token, name string) (world.Record, error) {
	hash := world.TokenHash(token)
	// Пространство токенов AI зарезервировано: истёкший токен не создаёт гостя.
	if strings.HasPrefix(token, world.AIServiceTokenPrefix) {
		if !s.Enabled {
			return world.Record{}, ErrDisabled
		}
		var player string
		if err := s.AI.pool.QueryRow(ctx, `SELECT a.player_id FROM ai_gateway_sessions s JOIN ai_agents a ON a.studio_captain_id=s.agent_id
 WHERE s.token_hash=$1 AND s.expires_at>now()`, hash[:]).Scan(&player); err != nil {
			return world.Record{}, ErrSession
		}
		rec, err := s.Store.Resolve(ctx, token, name)
		if err != nil {
			return rec, err
		}
		if rec.ID != player {
			return world.Record{}, ErrIdentity
		}
		rec.AI = true
		if err = s.AI.pool.QueryRow(ctx, `SELECT studio_captain_id FROM ai_agents WHERE player_id=$1`, player).Scan(&rec.AICaptain); err != nil {
			return world.Record{}, err
		}
		return rec, nil
	}
	rec, err := s.Store.Resolve(ctx, token, name)
	if err != nil {
		return rec, err
	}
	var ai bool
	if err = s.AI.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ai_agents WHERE player_id=$1)`, rec.ID).Scan(&ai); err != nil {
		return world.Record{}, err
	}
	if ai {
		return world.Record{}, ErrSession
	}
	return rec, nil
}
func (s *MarkedPlayers) CreateCharacter(ctx context.Context, id, nick, gender, pilot string, st world.Starter) (world.Record, error) {
	// Bootstrap AI выполняется внутри доверенной транзакции, не через внешний join.
	var ai bool
	if err := s.AI.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ai_agents WHERE player_id=$1)`, id).Scan(&ai); err != nil {
		return world.Record{}, err
	}
	if ai {
		return world.Record{}, ErrIdentity
	}
	return s.Store.CreateCharacter(ctx, id, nick, gender, pilot, st)
}

func (s *Store) revokeSession(ctx context.Context, agent string, hash [32]byte) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM ai_gateway_sessions WHERE agent_id=$1 AND token_hash=$2`, agent, hash[:]); err != nil {
		return
	}
	if _, err = tx.Exec(ctx, `DELETE FROM player_tokens WHERE token_hash=$1`, hash[:]); err != nil {
		return
	}
	_ = tx.Commit(ctx)
}
