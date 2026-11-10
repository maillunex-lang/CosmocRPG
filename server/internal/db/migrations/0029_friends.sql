CREATE TABLE IF NOT EXISTS friendships (
    owner_id   TEXT NOT NULL REFERENCES players (public_id) ON DELETE CASCADE,
    friend_id  TEXT NOT NULL REFERENCES players (public_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_id, friend_id)
);
