-- Этап С-01: фундамент контрактов. Только расширение: существующие таблицы и данные не меняются.

-- Публичный идентификатор игрока (первые 8 байт хеша токена): ключ для связанных таблиц.
ALTER TABLE players ADD COLUMN public_id TEXT;
UPDATE players SET public_id = encode(substring(token_hash from 1 for 8), 'hex') WHERE public_id IS NULL;
ALTER TABLE players ALTER COLUMN public_id SET NOT NULL;
CREATE UNIQUE INDEX players_public_id_idx ON players (public_id);

-- Кредиты игрока; меняются только сервером в одной транзакции с записью о награде.
ALTER TABLE players ADD COLUMN credits BIGINT NOT NULL DEFAULT 0 CHECK (credits >= 0);

-- Определения контрактов с версиями: активные задания держат ссылку на версию, с которой начаты.
CREATE TABLE quest_definitions (
    id         TEXT NOT NULL,
    version    INT  NOT NULL CHECK (version > 0),
    title      TEXT NOT NULL,
    body       JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, version)
);

CREATE TABLE player_quest_progress (
    player_id     TEXT NOT NULL REFERENCES players (public_id) ON DELETE CASCADE,
    quest_id      TEXT NOT NULL,
    quest_version INT  NOT NULL,
    state         TEXT NOT NULL CHECK (state IN ('active', 'completed')),
    progress      INT  NOT NULL DEFAULT 0 CHECK (progress >= 0),
    goal          INT  NOT NULL CHECK (goal > 0),
    started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at  TIMESTAMPTZ,
    PRIMARY KEY (player_id, quest_id),
    FOREIGN KEY (quest_id, quest_version) REFERENCES quest_definitions (id, version)
);

-- Журнал наград. UNIQUE (player_id, grant_key) делает выдачу идемпотентной: повтор не создаёт вторую запись.
CREATE TABLE reward_grants (
    id         BIGSERIAL PRIMARY KEY,
    player_id  TEXT NOT NULL REFERENCES players (public_id) ON DELETE CASCADE,
    grant_key  TEXT NOT NULL,
    kind       TEXT NOT NULL CHECK (kind IN ('credits')),
    amount     BIGINT NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (player_id, grant_key)
);
