-- Только служебные leases/receipts. Деньги, груз и прогресс остаются общими.
CREATE TABLE ai_gateway_sessions (
 agent_id TEXT PRIMARY KEY REFERENCES ai_agents(studio_captain_id),
 token_hash BYTEA NOT NULL UNIQUE CHECK (length(token_hash)=32),
 expires_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE ai_action_receipts (
 agent_id TEXT NOT NULL REFERENCES ai_agents(studio_captain_id),
 action_id TEXT NOT NULL CHECK(length(action_id) BETWEEN 1 AND 128),
 request_hash BYTEA NOT NULL CHECK(length(request_hash)=32),
 state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','completed','uncertain')),
 result JSONB,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(agent_id,action_id),
 CHECK((state='completed')=(result IS NOT NULL))
);
