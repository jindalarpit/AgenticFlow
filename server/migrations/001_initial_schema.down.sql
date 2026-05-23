-- Drop indexes
DROP INDEX IF EXISTS idx_task_message_task;
DROP INDEX IF EXISTS idx_task_agent_type;
DROP INDEX IF EXISTS idx_task_status;
DROP INDEX IF EXISTS idx_task_user;
DROP INDEX IF EXISTS idx_custom_agent_user;
DROP INDEX IF EXISTS idx_agent_runtime_provider;
DROP INDEX IF EXISTS idx_agent_runtime_daemon;
DROP INDEX IF EXISTS idx_daemon_status;
DROP INDEX IF EXISTS idx_daemon_user;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS task_message;
DROP TABLE IF EXISTS task;
DROP TABLE IF EXISTS custom_agent;
DROP TABLE IF EXISTS agent_runtime;
DROP TABLE IF EXISTS daemon;
DROP TABLE IF EXISTS personal_access_token;
DROP TABLE IF EXISTS "user";
