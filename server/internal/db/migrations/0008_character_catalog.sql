-- Создание персонажа (ник, пол, стартовая специализация пилота, тип корабля) и версии каталога кораблей.
-- Только расширение: существующие игроки получают значения по умолчанию и проходят создание персонажа при следующем входе;
-- боты остаются играбельными (помечаются как созданные).
ALTER TABLE players ADD COLUMN character_created BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE players ADD COLUMN gender TEXT NOT NULL DEFAULT '' CHECK (gender IN ('', 'm', 'f'));
ALTER TABLE players ADD COLUMN pilot_spec TEXT NOT NULL DEFAULT '' CHECK (length(pilot_spec) <= 24);
ALTER TABLE players ADD COLUMN ship_type TEXT NOT NULL DEFAULT 'FTR-001' CHECK (length(ship_type) BETWEEN 3 AND 24);
UPDATE players SET character_created = true WHERE name LIKE 'Бот-%';
-- ник уникален без учёта регистра среди созданных персонажей
CREATE UNIQUE INDEX players_nick_idx ON players (lower(name)) WHERE character_created;

-- Каталог кораблей с версиями: параметры редактируются в админке и ZIP-патчами, публикация и откат.
CREATE TABLE ship_catalog_versions (
    id         BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    admin      TEXT NOT NULL DEFAULT '',
    note       TEXT NOT NULL DEFAULT '',
    source     TEXT NOT NULL CHECK (source IN ('seed', 'edit', 'patch', 'rollback')),
    is_active  BOOLEAN NOT NULL DEFAULT false,
    snapshot   JSONB NOT NULL
);
CREATE UNIQUE INDEX ship_catalog_active_idx ON ship_catalog_versions (is_active) WHERE is_active;
