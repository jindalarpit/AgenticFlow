-- Reverse migration: Remove conversational workflow support

-- 1. Remove git_repo_url from task table
ALTER TABLE task DROP COLUMN IF EXISTS git_repo_url;

-- 2. Drop prompt_history index and table
DROP INDEX IF EXISTS idx_prompt_history_stage_time;
DROP TABLE IF EXISTS prompt_history;

-- 3. Restore original task_stage status constraint (with approval gate statuses)
ALTER TABLE task_stage DROP CONSTRAINT IF EXISTS task_stage_status_check;
ALTER TABLE task_stage ADD CONSTRAINT task_stage_status_check
    CHECK (status IN ('pending', 'running', 'awaiting_approval', 'approved', 'rejected', 'completed', 'failed'));

-- 4. Remove session tracking columns from task_stage
ALTER TABLE task_stage DROP COLUMN IF EXISTS work_dir;
ALTER TABLE task_stage DROP COLUMN IF EXISTS session_id;
