-- Reverse migration: restore custom_agent table, drop agent table

-- 1. Recreate custom_agent table with original schema
CREATE TABLE IF NOT EXISTS custom_agent (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (name ~ '^[a-zA-Z0-9_-]{1,64}$'),
    command TEXT NOT NULL,
    args_template TEXT NOT NULL DEFAULT '{{prompt}}',
    model_override TEXT,
    env_vars JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_custom_agent_user ON custom_agent(user_id);

-- 2. Migrate data back from agent table (best effort)
INSERT INTO custom_agent (user_id, name, command, args_template, model_override, env_vars)
SELECT user_id, name, 'agent', '', model, custom_env
FROM agent
ON CONFLICT (user_id, name) DO NOTHING;

-- 3. Remove agent_id from task table
ALTER TABLE task DROP COLUMN IF EXISTS agent_id;

-- 4. Drop agent table and its indexes
DROP INDEX IF EXISTS idx_agent_user;
DROP INDEX IF EXISTS idx_agent_runtime;
DROP INDEX IF EXISTS idx_agent_status;
DROP INDEX IF EXISTS idx_agent_visibility;
DROP TABLE IF EXISTS agent;

-- 5. Remove token_prefix column from personal_access_token
ALTER TABLE personal_access_token DROP COLUMN IF EXISTS token_prefix;
