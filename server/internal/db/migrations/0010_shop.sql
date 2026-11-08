-- VIP и донат-магазин. Только расширение. Пока включён режим «бесплатно»: покупки записываются с оплаченной суммой 0.
ALTER TABLE players ADD COLUMN vip_level SMALLINT NOT NULL DEFAULT 0 CHECK (vip_level BETWEEN 0 AND 3);
ALTER TABLE players ADD COLUMN vip_until TIMESTAMPTZ;
ALTER TABLE players ADD COLUMN equipped JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(equipped) = 'object');

CREATE TABLE player_items (
    player_id   TEXT NOT NULL REFERENCES players (public_id) ON DELETE CASCADE,
    item_id     TEXT NOT NULL CHECK (length(item_id) BETWEEN 1 AND 48),
    acquired_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (player_id, item_id)
);

-- Журнал покупок. UNIQUE (player_id, request_id) делает повторную отправку одной покупки безопасной.
CREATE TABLE shop_purchases (
    id         BIGSERIAL PRIMARY KEY,
    player_id  TEXT NOT NULL REFERENCES players (public_id) ON DELETE CASCADE,
    item_id    TEXT NOT NULL,
    request_id TEXT NOT NULL CHECK (length(request_id) BETWEEN 8 AND 64),
    price      INTEGER NOT NULL CHECK (price >= 0),
    paid       INTEGER NOT NULL CHECK (paid >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (player_id, request_id)
);
