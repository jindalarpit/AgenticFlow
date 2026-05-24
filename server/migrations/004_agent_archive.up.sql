-- Migration: Add archived_at column to agent table for soft-delete support
ALTER TABLE agent ADD COLUMN archived_at TIMESTAMPTZ;
