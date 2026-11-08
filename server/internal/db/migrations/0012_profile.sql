-- Профиль игрока и расширение магазина (R-34): VIP 1–7, подписка, аватарки/рамки/титулы. Только расширение, данные не затрагиваются.
ALTER TABLE players DROP CONSTRAINT IF EXISTS players_vip_level_check;
ALTER TABLE players ADD CONSTRAINT players_vip_level_check CHECK (vip_level BETWEEN 0 AND 7);
ALTER TABLE players ADD COLUMN subscription_until TIMESTAMPTZ;
