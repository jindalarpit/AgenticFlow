-- Migration: Conversational workflow support
-- Adds session tracking to task_stage, creates prompt_history table,
-- simplifies task_stage status constraint (removes approval gates),
-- and adds git_repo_url to task table for execution workspace config.

-- 1. Add session tracking columns to task_stage
ALTER TABLE task_stage ADD COLUMN session_id TEXT;
ALTER TABLE task_stage ADD COLUMN work_dir TEXT;

-- 2. Modify task_stage status constraint: remove awaiting_approval, approved, rejected
ALTER TABLE task_stage DROP CONSTRAINT task_stage_status_check;
ALTER TABLE task_stage ADD CONSTRAINT task_stage_status_check
    CHECK (status IN ('pending', 'running', 'completed', 'failed'));

-- 3. Create prompt_history table
CREATE TABLE prompt_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_stage_id UUID NOT NULL REFERENCES task_stage(id) ON DELETE CASCADE,
    task_id UUID NOT NULL REFERENCES task(id) ON DELETE CASCADE,
    prompt_text TEXT NOT NULL,
    output_text TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 4. Index for efficient chronological retrieval by stage
CREATE INDEX idx_prompt_history_stage_time ON prompt_history(task_stage_id, created_at);

-- 5. Add git_repo_url to task table for execution workspace config
ALTER TABLE task ADD COLUMN git_repo_url TEXT;
