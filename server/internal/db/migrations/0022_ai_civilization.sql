-- AI-PLAYERS-001: профиль Studio не является кошельком/персонажем.
-- player_id заполняется только после обычного создания персонажа общим gateway.
CREATE TABLE ai_agents (
    studio_captain_id TEXT PRIMARY KEY CHECK (length(studio_captain_id) BETWEEN 1 AND 128),
    player_id TEXT UNIQUE REFERENCES players(public_id),
    mode TEXT NOT NULL DEFAULT 'DORMANT' CHECK (mode IN ('ACTIVE','BACKGROUND','DORMANT')),
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE ai_memory_events (
    agent_id TEXT NOT NULL REFERENCES ai_agents(studio_captain_id),
    event_id TEXT NOT NULL CHECK (length(event_id) BETWEEN 1 AND 128),
    event_type TEXT NOT NULL CHECK (event_type IN ('observed','confirmed','failure')),
    subject_ref TEXT NOT NULL CHECK (length(subject_ref) <= 128),
    occurred_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL CHECK (expires_at > occurred_at),
    PRIMARY KEY (agent_id, event_id)
);
CREATE INDEX ai_memory_expiry ON ai_memory_events(expires_at);
