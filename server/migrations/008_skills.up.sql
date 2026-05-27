-- Migration: Create skills tables and add mcp_config to agent

-- Skill table: stores reusable instruction packages
CREATE TABLE skill (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    name VARCHAR(64) NOT NULL,
    description VARCHAR(255) NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    config JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT skill_name_pattern CHECK (name ~ '^[a-z0-9][a-z0-9-]*$'),
    CONSTRAINT skill_name_length CHECK (char_length(name) BETWEEN 1 AND 64),
    CONSTRAINT skill_content_size CHECK (octet_length(content) <= 204800),
    UNIQUE (user_id, name)
);

-- Skill file table: supporting files within a skill
CREATE TABLE skill_file (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_id UUID NOT NULL REFERENCES skill(id) ON DELETE CASCADE,
    path VARCHAR(512) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT skill_file_content_size CHECK (octet_length(content) <= 1048576),
    UNIQUE (skill_id, path)
);

-- Agent-skill association (many-to-many)
CREATE TABLE agent_skill (
    agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    skill_id UUID NOT NULL REFERENCES skill(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, skill_id)
);

-- Add mcp_config column to agent table
ALTER TABLE agent ADD COLUMN mcp_config JSONB;

-- Indexes for common queries
CREATE INDEX idx_skill_user_id ON skill(user_id);
CREATE INDEX idx_skill_file_skill_id ON skill_file(skill_id);
CREATE INDEX idx_agent_skill_agent_id ON agent_skill(agent_id);
CREATE INDEX idx_agent_skill_skill_id ON agent_skill(skill_id);
