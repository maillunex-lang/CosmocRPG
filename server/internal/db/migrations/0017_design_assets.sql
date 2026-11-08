-- Библиотека дизайна (R-53): замены встроенных элементов (3D-модели, иконки, фоны, паспорта кораблей), загруженные из админки.
-- Только расширение. Ключ элемента (element_id) не привязан к пути файла: «model:FTR-001:a» — модель облика «a» корабля FTR-001.
CREATE TABLE design_assets (
    id           BIGSERIAL PRIMARY KEY,
    element_id   TEXT NOT NULL CHECK (length(element_id) BETWEEN 3 AND 80),
    version      INT NOT NULL CHECK (version >= 1),
    file_name    TEXT NOT NULL CHECK (length(file_name) BETWEEN 1 AND 120),
    mime         TEXT NOT NULL,
    sha256       TEXT NOT NULL CHECK (length(sha256) = 64),
    size         BIGINT NOT NULL CHECK (size > 0),
    data         BYTEA NOT NULL,
    status       TEXT NOT NULL CHECK (status IN ('draft', 'published', 'archived')),
    report       JSONB NOT NULL DEFAULT '{}'::jsonb,
    notes        TEXT NOT NULL DEFAULT '',
    uploaded_by  TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    UNIQUE (element_id, version)
);
-- У одного элемента не больше одной опубликованной замены.
CREATE UNIQUE INDEX design_assets_one_published ON design_assets (element_id) WHERE status = 'published';
CREATE INDEX design_assets_sha_idx ON design_assets (sha256);

-- Версия манифеста ресурсов растёт при каждой публикации и снятии: клиент отклоняет откат версии.
CREATE TABLE design_manifest (
    id         INTEGER PRIMARY KEY CHECK (id = 1),
    version    BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
