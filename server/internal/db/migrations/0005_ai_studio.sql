-- AI Civilization Training Studio: версии конфигурации AI-капитанов, импорты ZIP, прогоны тестов, сессии обучения.
-- Только расширение; игровые таблицы (players, quests, rewards) не затрагиваются.

CREATE TABLE ai_versions (
    id         BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    admin      TEXT NOT NULL DEFAULT '',
    note       TEXT NOT NULL DEFAULT '',
    source     TEXT NOT NULL CHECK (source IN ('generated', 'import', 'edit', 'rollback')),
    import_id  BIGINT,
    is_active  BOOLEAN NOT NULL DEFAULT false,
    captains   INT NOT NULL DEFAULT 0,
    snapshot   JSONB NOT NULL
);
-- активной может быть только одна версия
CREATE UNIQUE INDEX ai_versions_active_idx ON ai_versions (is_active) WHERE is_active;

CREATE TABLE ai_imports (
    id           BIGSERIAL PRIMARY KEY,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    admin        TEXT NOT NULL DEFAULT '',
    filename     TEXT NOT NULL DEFAULT '',
    sha256       TEXT NOT NULL,
    status       TEXT NOT NULL CHECK (status IN ('validated', 'rejected', 'published', 'discarded')),
    pack_id      TEXT NOT NULL DEFAULT '',
    kind         TEXT NOT NULL DEFAULT '',
    base_version BIGINT,
    gate_ok      BOOLEAN NOT NULL DEFAULT false,
    version_id   BIGINT,
    report       JSONB NOT NULL,
    plan         JSONB NOT NULL,
    gate         JSONB NOT NULL,
    parsed       JSONB NOT NULL
);

CREATE TABLE ai_test_runs (
    id         BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    admin      TEXT NOT NULL DEFAULT '',
    version_id BIGINT,
    sample     INT NOT NULL DEFAULT 0,
    total      INT NOT NULL,
    passed     INT NOT NULL,
    results    JSONB NOT NULL
);

CREATE TABLE ai_sessions (
    id          BIGSERIAL PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    admin       TEXT NOT NULL DEFAULT '',
    label       TEXT NOT NULL,
    batch_size  INT NOT NULL DEFAULT 0,
    captain_ids JSONB NOT NULL,
    pack_id     TEXT NOT NULL,
    sha256      TEXT NOT NULL
);
