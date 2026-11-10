-- События без денег (добыча) тоже требуют однократного учёта прогресса.
CREATE TABLE quest_event_receipts (
 player_id TEXT NOT NULL REFERENCES players(public_id) ON DELETE CASCADE,
 event_key TEXT NOT NULL CHECK(length(event_key) BETWEEN 1 AND 128),
 request_hash BYTEA NOT NULL CHECK(length(request_hash)=32),
 progresses JSONB NOT NULL DEFAULT '{}'::jsonb CHECK(jsonb_typeof(progresses)='object'),
 PRIMARY KEY(player_id,event_key)
);
