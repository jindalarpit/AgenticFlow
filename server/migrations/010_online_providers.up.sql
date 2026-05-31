-- Migration 010: Online AI Providers, Deliverable Types, Agent & Task Extensions

-- 1. Deliverable Types table
CREATE TABLE deliverable_type (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES "user"(id) ON DELETE CASCADE,  -- NULL for system types
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 64),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 255),
    output_format TEXT NOT NULL DEFAULT '' CHECK (char_length(output_format) <= 10000),
    is_system BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Unique constraint: name must be unique per user (system types use a sentinel UUID)
CREATE UNIQUE INDEX idx_deliverable_type_user_name
    ON deliverable_type(COALESCE(user_id, '00000000-0000-0000-0000-000000000000'::UUID), name);

CREATE INDEX idx_deliverable_type_user ON deliverable_type(user_id);
CREATE INDEX idx_deliverable_type_system ON deliverable_type(is_system);

-- Insert system deliverable types
INSERT INTO deliverable_type (name, description, output_format, is_system) VALUES
('Code Execution', 'Execute code in a local workspace via daemon runtime', '', true),
('Chat Completion', 'Free-form text response from the AI model', '', true),
('Specification', 'Structured requirements and specification document', E'# Specification\n\n## Overview\n[Brief description]\n\n## Requirements\n[Detailed requirements]\n\n## Acceptance Criteria\n[Testable criteria]\n\n## Constraints\n[Technical and business constraints]', true),
('Architecture Design', 'High-level or low-level design document', E'# Architecture Design\n\n## Overview\n[System overview]\n\n## Components\n[Component descriptions]\n\n## Data Flow\n[Data flow description]\n\n## Technology Choices\n[Technology decisions and rationale]\n\n## Trade-offs\n[Design trade-offs and alternatives considered]', true),
('Test Plan', 'Test strategy and test case document', E'# Test Plan\n\n## Scope\n[What is being tested]\n\n## Strategy\n[Testing approach]\n\n## Test Cases\n[Detailed test cases]\n\n## Environment\n[Test environment requirements]\n\n## Risks\n[Testing risks and mitigations]', true),
('Market Research Report', 'Research findings and analysis document', E'# Research Report\n\n## Executive Summary\n[Key findings]\n\n## Methodology\n[Research approach]\n\n## Findings\n[Detailed findings]\n\n## Analysis\n[Data analysis]\n\n## Recommendations\n[Actionable recommendations]', true),
('Code Review Summary', 'Code review findings and recommendations', E'# Code Review Summary\n\n## Overview\n[What was reviewed]\n\n## Findings\n[Issues found]\n\n## Recommendations\n[Suggested improvements]\n\n## Severity\n[Issue severity classification]', true);

-- 2. Online Providers table
CREATE TABLE online_provider (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 128),
    provider_type TEXT NOT NULL CHECK (provider_type IN ('openai', 'azure_openai', 'aws_bedrock', 'anthropic', 'litellm')),
    credentials_encrypted TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'validating' CHECK (status IN ('active', 'inactive', 'error', 'validating')),
    status_message TEXT,
    models JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, name)
);

CREATE INDEX idx_online_provider_user ON online_provider(user_id);
CREATE INDEX idx_online_provider_status ON online_provider(status);
CREATE INDEX idx_online_provider_type ON online_provider(provider_type);

-- 3. Extend agent table with online provider support

-- Add runtime_mode column
ALTER TABLE agent ADD COLUMN runtime_mode TEXT NOT NULL DEFAULT 'local'
    CONSTRAINT agent_runtime_mode_values_check CHECK (runtime_mode IN ('local', 'online'));

-- Add provider_id column (FK to online_provider)
ALTER TABLE agent ADD COLUMN provider_id UUID REFERENCES online_provider(id);

-- Add deliverable_type_id column (FK to deliverable_type)
ALTER TABLE agent ADD COLUMN deliverable_type_id UUID REFERENCES deliverable_type(id);

-- Make runtime_id nullable (online agents don't have a local runtime)
ALTER TABLE agent ALTER COLUMN runtime_id DROP NOT NULL;

-- Add check constraint: local agents need runtime_id, online agents need provider_id
ALTER TABLE agent ADD CONSTRAINT agent_runtime_mode_check CHECK (
    (runtime_mode = 'local' AND runtime_id IS NOT NULL AND provider_id IS NULL) OR
    (runtime_mode = 'online' AND provider_id IS NOT NULL AND runtime_id IS NULL)
);

CREATE INDEX idx_agent_provider ON agent(provider_id);
CREATE INDEX idx_agent_runtime_mode ON agent(runtime_mode);
CREATE INDEX idx_agent_deliverable_type ON agent(deliverable_type_id);

-- 4. Extend task table with online execution metadata
ALTER TABLE task ADD COLUMN token_usage JSONB;
ALTER TABLE task ADD COLUMN provider_id UUID REFERENCES online_provider(id);

CREATE INDEX idx_task_provider ON task(provider_id);

-- 5. Update agent status check to include 'error'
-- Drop the existing inline check constraint on status column
-- PostgreSQL names inline CHECK constraints as: {table}_{column}_check
ALTER TABLE agent DROP CONSTRAINT IF EXISTS agent_status_check;
ALTER TABLE agent ADD CONSTRAINT agent_status_check
    CHECK (status IN ('idle', 'working', 'offline', 'error'));
