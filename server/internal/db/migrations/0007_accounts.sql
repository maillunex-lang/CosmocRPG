-- Аккаунты игроков (вход по логину и паролю, чтобы прогресс переживал удаление приложения).
-- Только расширение: существующие игроки продолжают входить по токену устройства.

-- Токены устройств: у одного игрока может быть несколько (после входа с нового устройства).
CREATE TABLE player_tokens (
    token_hash       BYTEA PRIMARY KEY CHECK (length(token_hash) = 32),
    player_public_id TEXT NOT NULL REFERENCES players (public_id) ON DELETE CASCADE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX player_tokens_player_idx ON player_tokens (player_public_id);

CREATE TABLE accounts (
    id               BIGSERIAL PRIMARY KEY,
    login            TEXT NOT NULL UNIQUE CHECK (login = lower(login) AND length(login) BETWEEN 3 AND 32),
    password_hash    TEXT NOT NULL CHECK (password_hash LIKE '$argon2id$%'),
    player_public_id TEXT NOT NULL UNIQUE REFERENCES players (public_id) ON DELETE CASCADE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at    TIMESTAMPTZ
);
