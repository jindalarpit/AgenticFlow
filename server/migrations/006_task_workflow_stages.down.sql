-- Reverse migration: Remove task workflow stages support

-- 1. Drop indexes
DROP INDEX IF EXISTS idx_task_stage_status;
DROP INDEX IF EXISTS idx_task_stage_task_order;

-- 2. Drop task_stage table
DROP TABLE IF EXISTS task_stage;

-- 3. Remove constraint from task table
ALTER TABLE task DROP CONSTRAINT IF EXISTS task_workspace_path_required;

-- 4. Remove workflow columns from task table
ALTER TABLE task DROP COLUMN IF EXISTS workspace_path;
ALTER TABLE task DROP COLUMN IF EXISTS workspace_mode;
ALTER TABLE task DROP COLUMN IF EXISTS deliverables;
