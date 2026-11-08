-- Версии конфигурации мира («живые локации»): системы, враги, ресурсы, визуальные параметры. Публикация и откат из админки.
CREATE TABLE world_config_versions (
    id         BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    admin      TEXT NOT NULL DEFAULT '',
    note       TEXT NOT NULL DEFAULT '',
    source     TEXT NOT NULL CHECK (source IN ('seed', 'edit', 'rollback')),
    is_active  BOOLEAN NOT NULL DEFAULT false,
    snapshot   JSONB NOT NULL
);
CREATE UNIQUE INDEX world_config_active_idx ON world_config_versions (is_active) WHERE is_active;
