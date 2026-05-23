-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users
CREATE TABLE "user" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT,          -- NULL for OAuth-only users
    avatar_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Personal Access Tokens
CREATE TABLE personal_access_token (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Daemons
CREATE TABLE daemon (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    daemon_id TEXT NOT NULL,     -- machine-stable identifier
    device_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'offline'
        CHECK (status IN ('online', 'offline')),
    last_heartbeat_at TIMESTAMPTZ,
    cli_version TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, daemon_id)
);

-- Agent Runtimes (detected CLIs per daemon)
CREATE TABLE agent_runtime (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    daemon_id UUID NOT NULL REFERENCES daemon(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,      -- claude, codex, gemini, etc.
    name TEXT NOT NULL,          -- display name
    version TEXT,
    binary_path TEXT,
    status TEXT NOT NULL DEFAULT 'available'
        CHECK (status IN ('available', 'busy', 'unavailable')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(daemon_id, provider)
);

-- Custom Agents
CREATE TABLE custom_agent (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (name ~ '^[a-zA-Z0-9_-]{1,64}$'),
    command TEXT NOT NULL,
    args_template TEXT NOT NULL DEFAULT '{{prompt}}',
    model_override TEXT,
    env_vars JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, name)
);

-- Tasks
CREATE TABLE task (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    agent_type TEXT NOT NULL,        -- provider name or custom_agent name
    agent_runtime_id UUID REFERENCES agent_runtime(id),
    custom_agent_id UUID REFERENCES custom_agent(id),
    daemon_id UUID REFERENCES daemon(id),
    prompt TEXT NOT NULL CHECK (char_length(prompt) <= 32000),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled', 'timeout')),
    exit_code INT,
    error_message TEXT,
    output_preview TEXT,            -- first 1024 chars of output
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Task Messages (streaming output)
CREATE TABLE task_message (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES task(id) ON DELETE CASCADE,
    sequence INT NOT NULL,
    stream TEXT NOT NULL CHECK (stream IN ('stdout', 'stderr')),
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes
CREATE INDEX idx_daemon_user ON daemon(user_id);
CREATE INDEX idx_daemon_status ON daemon(status);
CREATE INDEX idx_agent_runtime_daemon ON agent_runtime(daemon_id);
CREATE INDEX idx_agent_runtime_provider ON agent_runtime(provider);
CREATE INDEX idx_custom_agent_user ON custom_agent(user_id);
CREATE INDEX idx_task_user ON task(user_id);
CREATE INDEX idx_task_status ON task(status);
CREATE INDEX idx_task_agent_type ON task(agent_type);
CREATE INDEX idx_task_message_task ON task_message(task_id, sequence);
