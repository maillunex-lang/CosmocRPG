-- Опыт и характеристики пилота (RPG-1). Только расширение: существующие игроки получают 0 опыта (уровень 1).
-- Уровень не хранится: он вычисляется из опыта по кривой, поэтому расхождений быть не может.
ALTER TABLE players ADD COLUMN pilot_xp BIGINT NOT NULL DEFAULT 0 CHECK (pilot_xp >= 0);
-- Распределённые очки характеристик: {"piloting": 3, "tactics": 1, ...}; допустимость проверяет сервер.
ALTER TABLE players ADD COLUMN pilot_attrs JSONB NOT NULL DEFAULT '{}'::jsonb;

-- Журнал начисления опыта. UNIQUE (player_id, grant_key) делает начисление идемпотентным: повтор события не удваивает опыт.
CREATE TABLE xp_grants (
    id         BIGSERIAL PRIMARY KEY,
    player_id  TEXT NOT NULL REFERENCES players (public_id) ON DELETE CASCADE,
    grant_key  TEXT NOT NULL,
    source     TEXT NOT NULL CHECK (length(source) BETWEEN 1 AND 24),
    amount     BIGINT NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (player_id, grant_key)
);
CREATE INDEX xp_grants_player_idx ON xp_grants (player_id, id DESC);
