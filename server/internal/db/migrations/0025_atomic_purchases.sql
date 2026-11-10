-- Ревизия не даёт старому autosave затереть уже оплаченную покупку.
ALTER TABLE players ADD COLUMN cargo_revision BIGINT NOT NULL DEFAULT 0 CHECK(cargo_revision>=0);
CREATE TABLE resource_purchase_receipts (
 player_id TEXT NOT NULL REFERENCES players(public_id) ON DELETE CASCADE,
 operation_key TEXT NOT NULL CHECK(length(operation_key) BETWEEN 1 AND 128),
 intent_hash BYTEA NOT NULL CHECK(length(intent_hash)=32),
 spent BIGINT NOT NULL CHECK(spent>0),
 cargo_revision BIGINT NOT NULL CHECK(cargo_revision>0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(player_id,operation_key)
);
