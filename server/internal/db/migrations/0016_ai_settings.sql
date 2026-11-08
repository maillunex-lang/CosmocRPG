-- Настройки ИИ-игроков (R-40, R-42): единственная строка, правится из админки (этап ИИ-3). Только расширение.
CREATE TABLE ai_settings (
    id         INTEGER PRIMARY KEY CHECK (id = 1),
    data       JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by TEXT NOT NULL DEFAULT ''
);
