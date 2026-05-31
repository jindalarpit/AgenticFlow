-- Reverse migration 010: Remove online providers, deliverable types, and agent/task extensions

-- 1. Revert agent status check to original values (remove 'error')
ALTER TABLE agent DROP CONSTRAINT IF EXISTS agent_status_check;
ALTER TABLE agent ADD CONSTRAINT agent_status_check
    CHECK (status IN ('idle', 'working', 'offline'));

-- 2. Remove task table extensions
DROP INDEX IF EXISTS idx_task_provider;
ALTER TABLE task DROP COLUMN IF EXISTS provider_id;
ALTER TABLE task DROP COLUMN IF EXISTS token_usage;

-- 3. Remove agent table extensions (reverse order of addition)
DROP INDEX IF EXISTS idx_agent_deliverable_type;
DROP INDEX IF EXISTS idx_agent_runtime_mode;
DROP INDEX IF EXISTS idx_agent_provider;

ALTER TABLE agent DROP CONSTRAINT IF EXISTS agent_runtime_mode_check;

-- Restore runtime_id NOT NULL (must clear online agents first)
UPDATE agent SET runtime_id = (
    SELECT ar.id FROM agent_runtime ar
    JOIN daemon d ON ar.daemon_id = d.id
    WHERE d.user_id = agent.user_id
    ORDER BY ar.created_at ASC LIMIT 1
) WHERE runtime_id IS NULL;
ALTER TABLE agent ALTER COLUMN runtime_id SET NOT NULL;

ALTER TABLE agent DROP COLUMN IF EXISTS deliverable_type_id;
ALTER TABLE agent DROP COLUMN IF EXISTS provider_id;
ALTER TABLE agent DROP COLUMN IF EXISTS runtime_mode;

-- 4. Drop online_provider table and indexes
DROP INDEX IF EXISTS idx_online_provider_type;
DROP INDEX IF EXISTS idx_online_provider_status;
DROP INDEX IF EXISTS idx_online_provider_user;
DROP TABLE IF EXISTS online_provider;

-- 5. Drop deliverable_type table and indexes
DROP INDEX IF EXISTS idx_deliverable_type_system;
DROP INDEX IF EXISTS idx_deliverable_type_user;
DROP INDEX IF EXISTS idx_deliverable_type_user_name;
DROP TABLE IF EXISTS deliverable_type;
