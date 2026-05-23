-- Migration: Create agent table, migrate custom_agent data, add agent_id to task

-- 1. Create new agent table with full configuration
CREATE TABLE agent (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (
        char_length(name) BETWEEN 1 AND 64
        AND name ~ '^[a-zA-Z0-9][a-zA-Z0-9_-]*$'
    ),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 255),
    instructions TEXT NOT NULL DEFAULT '' CHECK (char_length(instructions) <= 50000),
    runtime_id UUID NOT NULL REFERENCES agent_runtime(id),
    model TEXT CHECK (model IS NULL OR char_length(model) <= 100),
    custom_env JSONB NOT NULL DEFAULT '{}',
    custom_args JSONB NOT NULL DEFAULT '[]',
    max_concurrent_tasks INT NOT NULL DEFAULT 1
        CHECK (max_concurrent_tasks BETWEEN 1 AND 20),
    visibility TEXT NOT NULL DEFAULT 'private'
        CHECK (visibility IN ('private', 'shared')),
    avatar_url TEXT CHECK (avatar_url IS NULL OR char_length(avatar_url) <= 2048),
    status TEXT NOT NULL DEFAULT 'idle'
        CHECK (status IN ('idle', 'working', 'offline')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, name)
);

CREATE INDEX idx_agent_user ON agent(user_id);
CREATE INDEX idx_agent_runtime ON agent(runtime_id);
CREATE INDEX idx_agent_status ON agent(status);
CREATE INDEX idx_agent_visibility ON agent(visibility);

-- 2. Migrate existing custom_agent data to the new agent table
INSERT INTO agent (user_id, name, description, instructions, runtime_id, model, custom_env, custom_args, max_concurrent_tasks, visibility)
SELECT
    ca.user_id,
    ca.name,
    '',
    '',
    (SELECT ar.id FROM agent_runtime ar
     JOIN daemon d ON ar.daemon_id = d.id
     WHERE d.user_id = ca.user_id
     ORDER BY ar.created_at ASC LIMIT 1),
    COALESCE(ca.model_override, ''),
    COALESCE(ca.env_vars, '{}'),
    CASE WHEN ca.args_template != '' AND ca.args_template != '{{prompt}}'
         THEN jsonb_build_array(ca.args_template)
         ELSE '[]'::jsonb END,
    1,
    'private'
FROM custom_agent ca
WHERE EXISTS (
    SELECT 1 FROM agent_runtime ar
    JOIN daemon d ON ar.daemon_id = d.id
    WHERE d.user_id = ca.user_id
);

-- 3. Add agent_id column to task table with foreign key reference
ALTER TABLE task ADD COLUMN agent_id UUID REFERENCES agent(id) ON DELETE SET NULL;
CREATE INDEX idx_task_agent ON task(agent_id);

-- 4. Ensure token_prefix column exists on personal_access_token
ALTER TABLE personal_access_token ADD COLUMN IF NOT EXISTS token_prefix TEXT NOT NULL DEFAULT '';

-- 5. Remove custom_agent_id FK from task table before dropping custom_agent
ALTER TABLE task DROP COLUMN IF EXISTS custom_agent_id;

-- 6. Drop the old custom_agent table after migration
DROP TABLE IF EXISTS custom_agent;
