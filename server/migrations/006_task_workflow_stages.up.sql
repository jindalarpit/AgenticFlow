-- Migration: Add task workflow stages support
-- Enables multi-stage task workflows with deliverable selection,
-- workspace mode control, and approval gates between stages.

-- 1. Add workflow columns to task table
ALTER TABLE task ADD COLUMN deliverables JSONB NOT NULL DEFAULT '["execution"]';
ALTER TABLE task ADD COLUMN workspace_mode TEXT NOT NULL DEFAULT 'isolated'
    CHECK (workspace_mode IN ('isolated', 'existing'));
ALTER TABLE task ADD COLUMN workspace_path TEXT;

-- Add constraint: workspace_path required when mode is 'existing'
ALTER TABLE task ADD CONSTRAINT task_workspace_path_required
    CHECK (workspace_mode = 'isolated' OR workspace_path IS NOT NULL);

-- 2. Create task_stage table
CREATE TABLE task_stage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES task(id) ON DELETE CASCADE,
    stage_name TEXT NOT NULL CHECK (stage_name IN ('plan', 'design', 'tasks', 'execution')),
    stage_order INT NOT NULL CHECK (stage_order BETWEEN 1 AND 4),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'awaiting_approval', 'approved', 'rejected', 'completed', 'failed')),
    output_content TEXT,
    feedback TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(task_id, stage_name)
);

-- 3. Indexes for efficient querying
CREATE INDEX idx_task_stage_task_order ON task_stage(task_id, stage_order);
CREATE INDEX idx_task_stage_status ON task_stage(status);
