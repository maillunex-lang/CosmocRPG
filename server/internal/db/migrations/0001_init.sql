-- Служебная таблица: журнал запусков сервера (проверка сохранности данных при перезапуске).
CREATE TABLE server_boots (
    id         BIGSERIAL PRIMARY KEY,
    booted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    version    TEXT NOT NULL CHECK (length(version) > 0)
);
