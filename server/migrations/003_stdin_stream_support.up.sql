-- Migration: Add stdin stream support and unique constraint for idempotent inserts

-- 1. Update CHECK constraint on task_message.stream to include 'stdin'
ALTER TABLE task_message DROP CONSTRAINT IF EXISTS task_message_stream_check;
ALTER TABLE task_message ADD CONSTRAINT task_message_stream_check
    CHECK (stream IN ('stdout', 'stderr', 'stdin'));

-- 2. Drop existing non-unique index and create unique index for duplicate prevention
DROP INDEX IF EXISTS idx_task_message_task;
CREATE UNIQUE INDEX idx_task_message_task_sequence ON task_message(task_id, sequence);
