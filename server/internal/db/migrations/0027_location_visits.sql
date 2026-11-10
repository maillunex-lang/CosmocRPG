-- Журнал первых прибытий (К-3). Одна строка на игрока, систему и объект.
CREATE TABLE location_visits (
    player_id  TEXT NOT NULL CHECK (length(player_id) BETWEEN 1 AND 64),
    system_id  TEXT NOT NULL CHECK (length(system_id) BETWEEN 1 AND 32),
    object_id  TEXT NOT NULL CHECK (length(object_id) BETWEEN 1 AND 64),
    name       TEXT NOT NULL DEFAULT '' CHECK (length(name) <= 120),
    kind       TEXT NOT NULL DEFAULT '' CHECK (length(kind) <= 24),
    visited_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (player_id, system_id, object_id)
);
