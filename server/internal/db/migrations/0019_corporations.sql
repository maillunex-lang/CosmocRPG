-- Корпорации минимум (R-64, К-1): создание/вступление/состав/тег/казна. Только расширение.
CREATE TABLE corporations (
    id         TEXT PRIMARY KEY CHECK (length(id) BETWEEN 4 AND 24),
    name       TEXT NOT NULL UNIQUE CHECK (length(name) BETWEEN 3 AND 32),
    tag        TEXT NOT NULL UNIQUE CHECK (tag ~ '^[A-Z0-9]{2,4}$'),
    owner_id   TEXT NOT NULL REFERENCES players (public_id) ON DELETE CASCADE,
    treasury   BIGINT NOT NULL DEFAULT 0 CHECK (treasury >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE corporation_members (
    corp_id   TEXT NOT NULL REFERENCES corporations (id) ON DELETE CASCADE,
    player_id TEXT NOT NULL REFERENCES players (public_id) ON DELETE CASCADE,
    role      TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'member')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (corp_id, player_id)
);

ALTER TABLE players ADD COLUMN corporation_id TEXT REFERENCES corporations (id) ON DELETE SET NULL;

CREATE INDEX corporation_members_player ON corporation_members (player_id);
