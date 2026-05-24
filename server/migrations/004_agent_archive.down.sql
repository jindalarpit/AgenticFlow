-- Rollback: Remove archived_at column from agent table
ALTER TABLE agent DROP COLUMN IF EXISTS archived_at;
