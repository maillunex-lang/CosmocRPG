-- Выбор облика и специализации корабля игрока (Этап «Стриж»). Только расширение: значения по умолчанию подходят существующим игрокам.
ALTER TABLE players ADD COLUMN ship_skin TEXT NOT NULL DEFAULT 'a' CHECK (length(ship_skin) BETWEEN 1 AND 8);
ALTER TABLE players ADD COLUMN ship_spec TEXT NOT NULL DEFAULT 'interceptor' CHECK (length(ship_spec) BETWEEN 1 AND 24);
