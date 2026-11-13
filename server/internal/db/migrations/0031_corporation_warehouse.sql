-- Этап 5: общий склад корпорации и журнал операций.
CREATE TABLE corporation_warehouse (
    corp_id TEXT NOT NULL REFERENCES corporations(id) ON DELETE CASCADE,
    resource TEXT NOT NULL CHECK (resource IN ('iron_ore','crystals','gas','scrap')),
    amount DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (amount >= 0),
    PRIMARY KEY (corp_id, resource)
);
CREATE TABLE corporation_warehouse_audit (
    id TEXT PRIMARY KEY,
    corp_id TEXT NOT NULL REFERENCES corporations(id) ON DELETE CASCADE,
    player_id TEXT NOT NULL REFERENCES players(public_id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('deposit','withdraw','task_contribution')),
    resource TEXT NOT NULL,
    amount DOUBLE PRECISION NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX corporation_warehouse_audit_corp_created ON corporation_warehouse_audit(corp_id,created_at DESC);
