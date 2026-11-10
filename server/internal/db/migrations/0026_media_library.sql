-- Библиотека музыки и радио (R-37, этап М-1). Файлы лежат на томе COSMOC_MEDIA_DIR, здесь только метаданные.
-- Виды: music (плеер), ad/news/sfx (библиотека для эфира М-2; клиент М-1 их не играет).
CREATE TABLE media_tracks (
    id           BIGSERIAL PRIMARY KEY,
    kind         TEXT NOT NULL CHECK (kind IN ('music', 'ad', 'news', 'sfx')),
    title        TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 120),
    artist       TEXT NOT NULL DEFAULT '' CHECK (length(artist) <= 120),
    duration_ms  INT NOT NULL CHECK (duration_ms > 0 AND duration_ms <= 600000),
    sha256       TEXT NOT NULL UNIQUE CHECK (length(sha256) = 64),
    size         BIGINT NOT NULL CHECK (size > 0 AND size <= 12582912),
    ext          TEXT NOT NULL CHECK (ext IN ('mp3', 'ogg', 'opus')),
    mime         TEXT NOT NULL,
    license      TEXT NOT NULL DEFAULT '' CHECK (length(license) <= 200),
    source       TEXT NOT NULL DEFAULT '' CHECK (length(source) <= 120),
    ai_generated BOOLEAN NOT NULL DEFAULT TRUE,
    tags         TEXT NOT NULL DEFAULT '' CHECK (length(tags) <= 200),
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    uploaded_by  TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX media_tracks_playlist ON media_tracks (kind, enabled, id);
