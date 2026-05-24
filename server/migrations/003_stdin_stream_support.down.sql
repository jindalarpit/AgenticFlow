-- Reverse migration: Remove stdin stream support and restore non-unique index

-- 1. Drop unique index and restore original non-unique index
DROP INDEX IF EXISTS idx_task_message_task_sequence;
CREATE INDEX idx_task_message_task ON task_message(task_id, sequence);

-- 2. Restore original CHECK constraint without 'stdin'
ALTER TABLE task_message DROP CONSTRAINT IF EXISTS task_message_stream_check;
ALTER TABLE task_message ADD CONSTRAINT task_message_stream_check
    CHECK (stream IN ('stdout', 'stderr'));
