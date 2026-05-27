-- Reverse migration: Drop skills tables and remove mcp_config from agent

DROP INDEX IF EXISTS idx_agent_skill_skill_id;
DROP INDEX IF EXISTS idx_agent_skill_agent_id;
DROP INDEX IF EXISTS idx_skill_file_skill_id;
DROP INDEX IF EXISTS idx_skill_user_id;
DROP TABLE IF EXISTS agent_skill;
DROP TABLE IF EXISTS skill_file;
DROP TABLE IF EXISTS skill;
ALTER TABLE agent DROP COLUMN IF EXISTS mcp_config;
