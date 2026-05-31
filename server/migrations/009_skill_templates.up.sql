-- Migration: Create skill_template table for the Skill Template Library

-- Skill template table: stores system-level reusable skill templates
CREATE TABLE skill_template (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    description VARCHAR(512) NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    category VARCHAR(64) NOT NULL,
    version VARCHAR(32) NOT NULL,
    icon VARCHAR(16),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT skill_template_slug_pattern CHECK (slug ~ '^[a-z0-9][a-z0-9-]*$'),
    CONSTRAINT skill_template_slug_length CHECK (char_length(slug) BETWEEN 1 AND 64),
    CONSTRAINT skill_template_content_size CHECK (octet_length(content) <= 204800),
    UNIQUE (slug)
);

-- Indexes for common queries
CREATE INDEX idx_skill_template_category ON skill_template(category);
CREATE INDEX idx_skill_template_slug ON skill_template(slug);
