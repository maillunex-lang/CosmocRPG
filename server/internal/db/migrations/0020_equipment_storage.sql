-- SR-100c storage only: additive ownership/loadout tables. No runtime feature
-- is enabled and no existing player/ship/XP/catalog row is rewritten.
CREATE TABLE player_module_items (
    instance_id TEXT PRIMARY KEY CHECK (length(instance_id) BETWEEN 3 AND 96),
    owner_id TEXT NOT NULL REFERENCES players(public_id) ON DELETE CASCADE,
    ship_type TEXT NOT NULL CHECK (length(ship_type) BETWEEN 3 AND 24),
    prototype_id TEXT NOT NULL CHECK (length(prototype_id) BETWEEN 3 AND 64),
    grant_key TEXT NOT NULL CHECK (length(grant_key) BETWEEN 1 AND 96),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner_id, ship_type, grant_key),
    UNIQUE (instance_id, owner_id, ship_type)
);

CREATE TABLE player_ship_loadouts (
    owner_id TEXT NOT NULL REFERENCES players(public_id) ON DELETE CASCADE,
    ship_type TEXT NOT NULL CHECK (length(ship_type) BETWEEN 3 AND 24),
    revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_id, ship_type)
);

CREATE TABLE player_ship_loadout_slots (
    owner_id TEXT NOT NULL,
    ship_type TEXT NOT NULL,
    slot_id TEXT NOT NULL CHECK (length(slot_id) BETWEEN 1 AND 48),
    instance_id TEXT NOT NULL UNIQUE,
    PRIMARY KEY (owner_id, ship_type, slot_id),
    FOREIGN KEY (owner_id, ship_type)
        REFERENCES player_ship_loadouts(owner_id, ship_type) ON DELETE CASCADE,
    FOREIGN KEY (instance_id, owner_id, ship_type)
        REFERENCES player_module_items(instance_id, owner_id, ship_type)
        DEFERRABLE INITIALLY DEFERRED
);
