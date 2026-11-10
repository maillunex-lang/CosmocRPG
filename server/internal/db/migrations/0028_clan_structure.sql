-- Кланы: задания и союз до трёх кланов с голосованием за управляющего.
CREATE TABLE alliances (
    id          TEXT PRIMARY KEY CHECK (length(id) BETWEEN 4 AND 24),
    name        TEXT NOT NULL UNIQUE CHECK (length(name) BETWEEN 3 AND 40),
    governor_id TEXT REFERENCES players (public_id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE alliance_clans (
    alliance_id TEXT NOT NULL REFERENCES alliances (id) ON DELETE CASCADE,
    corp_id     TEXT NOT NULL UNIQUE REFERENCES corporations (id) ON DELETE CASCADE,
    PRIMARY KEY (alliance_id, corp_id)
);

CREATE TABLE alliance_votes (
    alliance_id TEXT NOT NULL REFERENCES alliances (id) ON DELETE CASCADE,
    corp_id     TEXT NOT NULL REFERENCES corporations (id) ON DELETE CASCADE,
    choice_id   TEXT NOT NULL,
    PRIMARY KEY (alliance_id, corp_id)
);

CREATE TABLE clan_tasks (
    corp_id    TEXT PRIMARY KEY REFERENCES corporations (id) ON DELETE CASCADE,
    kind       TEXT NOT NULL,
    title      TEXT NOT NULL,
    goal       INT NOT NULL CHECK (goal > 0),
    progress   INT NOT NULL DEFAULT 0 CHECK (progress >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
