-- Чат: сообщения (общий канал и личные), мьюты и настройки. Только расширение.
-- from_id не связан внешним ключом с players: отправителем может быть ИИ-капитан (его нет в таблице игроков).
CREATE TABLE chat_messages (
    id         BIGSERIAL PRIMARY KEY,
    channel    TEXT NOT NULL CHECK (channel IN ('general', 'dm', 'system')),
    from_id    TEXT NOT NULL,
    from_name  TEXT NOT NULL,
    from_ai    BOOLEAN NOT NULL DEFAULT false,
    to_id      TEXT,
    to_name    TEXT,
    body       TEXT NOT NULL CHECK (length(body) BETWEEN 1 AND 1000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX chat_messages_general_idx ON chat_messages (id DESC) WHERE channel = 'general';
CREATE INDEX chat_messages_dm_to_idx ON chat_messages (to_id, id DESC) WHERE channel = 'dm';
CREATE INDEX chat_messages_dm_from_idx ON chat_messages (from_id, id DESC) WHERE channel = 'dm';
CREATE INDEX chat_messages_created_idx ON chat_messages (created_at);

-- until IS NULL: бессрочный запрет писать (бан в чате).
CREATE TABLE chat_mutes (
    player_id  TEXT PRIMARY KEY REFERENCES players (public_id) ON DELETE CASCADE,
    until      TIMESTAMPTZ,
    reason     TEXT NOT NULL DEFAULT '',
    admin      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE chat_settings (
    id   INTEGER PRIMARY KEY CHECK (id = 1),
    data JSONB NOT NULL
);
