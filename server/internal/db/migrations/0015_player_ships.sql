-- Экземпляры кораблей игрока и опыт корабля (RPG-2). Только расширение.
-- Экземпляр — пара (игрок, тип корабля): на время альфы смена бесплатна, у каждого корабля свой уровень и он сохраняется при смене.
-- Уровень корабля (1–30) вычисляется из опыта; строки создаются при первом использовании корабля.
CREATE TABLE player_ships (
    id         BIGSERIAL PRIMARY KEY,
    player_id  TEXT NOT NULL REFERENCES players (public_id) ON DELETE CASCADE,
    ship_type  TEXT NOT NULL CHECK (length(ship_type) BETWEEN 3 AND 24),
    ship_xp    BIGINT NOT NULL DEFAULT 0 CHECK (ship_xp >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (player_id, ship_type)
);

-- Начисления опыта кораблю идут в тот же журнал, что и опыт пилота (ship_type заполнен только для них).
ALTER TABLE xp_grants ADD COLUMN ship_type TEXT;

-- Заполнение из существующих данных: у каждого игрока есть экземпляр его текущего корабля (уровень 1).
INSERT INTO player_ships (player_id, ship_type)
SELECT public_id, ship_type FROM players
ON CONFLICT (player_id, ship_type) DO NOTHING;
