-- Администраторы панели, их сессии и журнал административных действий.
CREATE TABLE admin_users (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE CHECK (username = lower(username) AND length(username) BETWEEN 3 AND 64),
    password_hash TEXT NOT NULL CHECK (password_hash LIKE '$argon2id$%'),
    disabled      BOOLEAN NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- В базе хранится только SHA-256 токена сессии: утечка таблицы не даёт войти.
CREATE TABLE admin_sessions (
    token_hash BYTEA PRIMARY KEY CHECK (length(token_hash) = 32),
    admin_id   BIGINT NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX admin_sessions_expires_idx ON admin_sessions (expires_at);

CREATE TABLE admin_audit_log (
    id        BIGSERIAL PRIMARY KEY,
    at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    admin_id  BIGINT REFERENCES admin_users(id) ON DELETE SET NULL,
    username  TEXT NOT NULL,
    action    TEXT NOT NULL CHECK (length(action) > 0),
    ip        TEXT NOT NULL DEFAULT '',
    details   JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX admin_audit_log_at_idx ON admin_audit_log (at DESC);
