-- Migration: Add structured message columns to task_message table
-- Supports typed events (text, thinking, tool_use, tool_result, error, status)
-- while retaining existing stream/content columns for backward compatibility.

-- 1. Add structured message columns
ALTER TABLE task_message ADD COLUMN type TEXT NOT NULL DEFAULT 'text';
ALTER TABLE task_message ADD COLUMN tool TEXT;
ALTER TABLE task_message ADD COLUMN input JSONB;
ALTER TABLE task_message ADD COLUMN output TEXT;

-- 2. Index for efficient filtering by message type
CREATE INDEX idx_task_message_type ON task_message(task_id, type);

-- 3. Make stream column nullable for new structured messages that don't use it
ALTER TABLE task_message ALTER COLUMN stream DROP NOT NULL;
ALTER TABLE task_message ALTER COLUMN stream SET DEFAULT 'stdout';

-- 4. Make content column nullable for structured messages (tool_use/tool_result don't need it)
ALTER TABLE task_message ALTER COLUMN content DROP NOT NULL;

-- 5. Relax the stream CHECK constraint to allow NULL
ALTER TABLE task_message DROP CONSTRAINT IF EXISTS task_message_stream_check;
ALTER TABLE task_message ADD CONSTRAINT task_message_stream_check
    CHECK (stream IS NULL OR stream IN ('stdout', 'stderr', 'stdin'));
