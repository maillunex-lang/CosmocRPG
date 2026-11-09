-- Три системы и гиперпереходы (Э-1, Решение №30): у игрока появляется текущая система. Только расширение.
ALTER TABLE players ADD COLUMN current_system TEXT NOT NULL DEFAULT 'aurora';
