-- Этап 4: неизменяемый аудит операций корпоративной казны.
CREATE TABLE corporation_treasury_audit (
    id         TEXT PRIMARY KEY,
    corp_id    TEXT NOT NULL REFERENCES corporations (id) ON DELETE CASCADE,
    player_id  TEXT NOT NULL REFERENCES players (public_id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('donation', 'task_reward', 'adjustment')),
    amount     BIGINT NOT NULL CHECK (amount > 0),
    balance    BIGINT NOT NULL CHECK (balance >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX corporation_treasury_audit_corp_created ON corporation_treasury_audit (corp_id, created_at DESC);
