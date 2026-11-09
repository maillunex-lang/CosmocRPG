-- General inventory keeps the existing item IDs and canonical slot rows.
-- ship_type on an item is its grant/purchase origin; native grants stay bound.
ALTER TABLE player_module_items
    ADD COLUMN engineering_rank SMALLINT NOT NULL DEFAULT 0
    CHECK (engineering_rank BETWEEN 0 AND 5);
ALTER TABLE player_module_items
    ADD CONSTRAINT module_instance_owner_unique UNIQUE (instance_id, owner_id);
ALTER TABLE player_ship_loadout_slots
    DROP CONSTRAINT player_ship_loadout_slots_instance_id_owner_id_ship_type_fkey;
ALTER TABLE player_ship_loadout_slots
    ADD CONSTRAINT loadout_item_owner_fk FOREIGN KEY (instance_id, owner_id)
    REFERENCES player_module_items(instance_id, owner_id)
    DEFERRABLE INITIALLY DEFERRED;

-- Shared purchases can move after unequipping, but bound stock cannot cross
-- hulls. Check both slot writes and changes to an item's binding metadata.
CREATE FUNCTION check_native_module_binding() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE checked_id TEXT;
BEGIN
    checked_id := NEW.instance_id;
    IF EXISTS (
        SELECT 1 FROM player_module_items i
        JOIN player_ship_loadout_slots s ON s.instance_id=i.instance_id
        WHERE i.instance_id=checked_id
          AND (i.grant_key='native-engine:v1' OR i.grant_key LIKE 'native-kit:v1:%')
          AND i.ship_type<>s.ship_type
    ) THEN
        RAISE EXCEPTION 'native module is bound to its origin hull' USING ERRCODE='23503';
    END IF;
    RETURN NULL;
END;
$$;
CREATE CONSTRAINT TRIGGER loadout_native_binding
    AFTER INSERT OR UPDATE ON player_ship_loadout_slots
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_native_module_binding();
CREATE CONSTRAINT TRIGGER item_native_binding
    AFTER UPDATE ON player_module_items
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_native_module_binding();

-- Pilot learning is global, preventing a fresh hull from providing a second
-- pool of talent/research points. Old pilot/ship XP and attributes are untouched.
CREATE TABLE player_rpg_pilots (
    owner_id TEXT PRIMARY KEY REFERENCES players(public_id) ON DELETE CASCADE,
    revision BIGINT NOT NULL DEFAULT 0 CHECK (revision>=0),
    data JSONB NOT NULL CHECK (jsonb_typeof(data)='object')
);
CREATE TABLE player_rpg_profiles (
    owner_id TEXT NOT NULL,
    ship_type TEXT NOT NULL,
    data JSONB NOT NULL CHECK (jsonb_typeof(data)='object'),
    PRIMARY KEY (owner_id,ship_type),
    FOREIGN KEY (owner_id,ship_type) REFERENCES player_ship_loadouts(owner_id,ship_type) ON DELETE CASCADE
);
CREATE TABLE player_specialization_grants (
    owner_id TEXT NOT NULL REFERENCES players(public_id) ON DELETE CASCADE,
    grant_key TEXT NOT NULL CHECK (length(grant_key) BETWEEN 1 AND 128),
    ship_type TEXT NOT NULL,
    specialization_id TEXT NOT NULL,
    amount BIGINT NOT NULL CHECK (amount>0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_id,grant_key)
);
