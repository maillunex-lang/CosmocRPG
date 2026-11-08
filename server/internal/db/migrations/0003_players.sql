-- Корабли игроков Этапа 01: личность по токену устройства (хранится только SHA-256), координаты.
CREATE TABLE players (
    id         BIGSERIAL PRIMARY KEY,
    token_hash BYTEA NOT NULL UNIQUE CHECK (length(token_hash) = 32),
    name       TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 64),
    pos_x      DOUBLE PRECISION NOT NULL DEFAULT 0,
    pos_y      DOUBLE PRECISION NOT NULL DEFAULT 0,
    dir_x      DOUBLE PRECISION NOT NULL DEFAULT 1,
    dir_y      DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
