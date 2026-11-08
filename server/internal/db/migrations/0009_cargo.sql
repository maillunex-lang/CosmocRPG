-- Грузовой отсек игрока (ресурс -> количество). Только расширение: у существующих игроков отсек пуст.
ALTER TABLE players ADD COLUMN hold JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(hold) = 'object');
