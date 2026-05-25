-- Reverse migration: Remove structured message columns from task_message table

-- 1. Restore stream CHECK constraint (non-nullable)
ALTER TABLE task_message DROP CONSTRAINT IF EXISTS task_message_stream_check;
ALTER TABLE task_message ADD CONSTRAINT task_message_stream_check
    CHECK (stream IN ('stdout', 'stderr', 'stdin'));

-- 2. Restore content column to NOT NULL (backfill NULLs first)
UPDATE task_message SET content = '' WHERE content IS NULL;
ALTER TABLE task_message ALTER COLUMN content SET NOT NULL;

-- 3. Restore stream column to NOT NULL (backfill NULLs first)
UPDATE task_message SET stream = 'stdout' WHERE stream IS NULL;
ALTER TABLE task_message ALTER COLUMN stream SET NOT NULL;
ALTER TABLE task_message ALTER COLUMN stream DROP DEFAULT;

-- 4. Drop the type index
DROP INDEX IF EXISTS idx_task_message_type;

-- 5. Remove structured columns
ALTER TABLE task_message DROP COLUMN IF EXISTS output;
ALTER TABLE task_message DROP COLUMN IF EXISTS input;
ALTER TABLE task_message DROP COLUMN IF EXISTS tool;
ALTER TABLE task_message DROP COLUMN IF EXISTS type;
