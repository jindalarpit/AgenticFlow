---
inclusion: always
---

# Agent & Task Delegation Flow

This is the CORE flow of AgenticFlow — the agent → daemon → CLI execution pipeline.

## Default Agent: "Nexus"

On first user setup, create a default agent called **"Nexus"**:

- Name: "Nexus"
- Description: "Your local AI coding agent"
- Runtime mode: "local"
- Bound to the first detected runtime on the user's daemon
- Max concurrent tasks: 1 (default, configurable)
- Instructions: empty (user can customize)
- Custom env: empty
- Custom args: empty

## Agent Data Model

An agent in AgenticFlow has these fields:

```json
{
    "id": "uuid",
    "name": "Nexus",
    "description": "Your local AI coding agent",
    "instructions": "Optional system prompt for the agent",
    "avatar_url": null,
    "runtime_mode": "local",
    "runtime_id": "uuid-of-bound-runtime",
    "custom_env": {"KEY": "value"},
    "custom_args": ["--flag", "value"],
    "model": "claude-sonnet-4-20250514",
    "visibility": "private",
    "status": "idle",
    "max_concurrent_tasks": 1,
    "created_at": "2025-01-01T00:00:00Z",
    "updated_at": "2025-01-01T00:00:00Z"
}
```

## Agent Screen Fields (Web UI)

The agent creation/edit form must have these fields:

1. **Name** — Text input, required, 1-64 chars
2. **Description** — Textarea, optional, max 255 chars
3. **Instructions** — Large textarea, optional (system prompt for the agent)
4. **Runtime** — Dropdown selecting which detected runtime to bind to
5. **Model** — Text input, optional (overrides the runtime's default model)
6. **Custom Environment Variables** — Key-value pairs editor (up to 20)
7. **Custom Arguments** — Array of strings editor
8. **Max Concurrent Tasks** — Number input (1-20)
9. **Visibility** — Toggle: private (only you) or shared

## Task Delegation Flow

```
1. User creates task via Web UI:
   - Selects target agent (e.g., "Nexus")
   - Enters prompt text
   - Clicks "Run" / "Delegate"

2. Server receives POST /api/tasks:
   - Validates prompt (non-empty, ≤32000 chars)
   - Resolves agent → finds bound runtime
   - Creates task row (status: "pending")
   - Broadcasts "task_created" via WebSocket

3. Daemon polls GET /api/daemon/tasks/poll:
   - Server finds pending task matching daemon's runtime
   - Returns task with agent config (instructions, env, args, model)

4. Daemon executes task:
   - Creates workspace dir: ~/.agenticflow/workspaces/<task-id>/
   - Reports POST /api/daemon/tasks/{id}/start
   - Spawns agent CLI with:
     - Working dir = workspace
     - Env vars = agent.custom_env merged with task env
     - Args = agent.custom_args + prompt
     - Model = agent.model (if set)
   - Streams stdout/stderr → POST /api/daemon/tasks/{id}/messages

5. Web UI shows real-time output:
   - WebSocket receives "task_output" events
   - Renders streaming terminal-like output
   - Shows task status badge (pending → running → completed/failed)

6. Task completes:
   - Daemon reports POST /api/daemon/tasks/{id}/complete
   - Server updates task status, broadcasts "task_completed"
   - Web UI updates status badge, shows final output
```

## Task Claim Response

When the daemon claims a task, the server returns:

```json
{
    "id": "task-uuid",
    "agent_type": "claude",
    "prompt": "Fix the login bug in auth.go",
    "agent": {
        "id": "agent-uuid",
        "name": "Nexus",
        "instructions": "You are a helpful coding agent...",
        "custom_env": {"GITHUB_TOKEN": "..."},
        "custom_args": ["--dangerously-skip-permissions"],
        "model": "claude-sonnet-4-20250514"
    }
}
```

## Task Messages (Streaming Output)

The daemon sends output in chunks:

```json
{
    "messages": [
        {
            "sequence": 1,
            "stream": "stdout",
            "content": "Analyzing auth.go...\n",
            "timestamp": "2025-01-01T00:00:01Z"
        },
        {
            "sequence": 2,
            "stream": "stderr",
            "content": "Warning: deprecated API usage\n",
            "timestamp": "2025-01-01T00:00:02Z"
        }
    ]
}
```

These are broadcast to the Web UI via WebSocket as `task_output` events for real-time display.

## Agent Status Derivation

Agent status is derived from its tasks:
- **idle** — No running tasks
- **working** — At least one task is running
- **offline** — Bound runtime's daemon is offline
- **error** — Last task failed

## Runtime Binding

Each agent is bound to exactly ONE runtime (detected CLI on a daemon). When the daemon registers, it creates runtime rows. Agents reference a `runtime_id` to know which daemon/CLI will execute their tasks.

If a runtime goes offline (daemon stops), the agent's status becomes "offline" and tasks remain in "pending" until the daemon comes back.
