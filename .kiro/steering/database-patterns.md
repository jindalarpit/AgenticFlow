---
inclusion: fileMatch
fileMatchPattern: "**/migrations/**,**/db/**,**sqlc**"
---

# Database Patterns

Reference: `/Users/arpit.jindal/workspace/opensource/multica/server/migrations/`

## Migration Naming

Follow multica's convention: `NNN_description.up.sql` / `NNN_description.down.sql`

Example:
- `001_init.up.sql` / `001_init.down.sql`
- `002_custom_agents.up.sql` / `002_custom_agents.down.sql`

## Core Tables (Simplified from multica)

AgenticFlow needs ONLY these tables (no issues, projects, comments, labels, etc.):

1. **user** — Authenticated users
2. **personal_access_token** — PATs for auth (90-day expiry)
3. **daemon** — Registered daemon connections
4. **agent_runtime** — Detected CLIs per daemon (like multica's agent_runtime)
5. **agent** — User-created agents bound to runtimes (like multica's agent)
6. **task** — Task queue + execution history (like multica's agent_task_queue)
7. **task_message** — Streaming output per task (like multica's task_messages)

## Key Schema Patterns (from multica)

### UUIDs
- Use `UUID PRIMARY KEY DEFAULT gen_random_uuid()` for all IDs
- Use `pgtype.UUID` in Go code

### Timestamps
- Always use `TIMESTAMPTZ NOT NULL DEFAULT now()` for created_at/updated_at
- Use `pgtype.Timestamptz` in Go code

### Status Enums
- Use CHECK constraints, not enum types (easier to migrate)
- Example: `CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled'))`

### Foreign Keys
- Always use `ON DELETE CASCADE` for child tables
- Reference parent by UUID

### Runtime Registration (from multica's UpsertAgentRuntime)

The daemon register endpoint uses UPSERT (INSERT ... ON CONFLICT UPDATE):
- Unique constraint on `(daemon_id, provider)` for agent_runtime
- On conflict: update name, version, status, metadata

## sqlc Configuration

```yaml
# sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "queries/"
    schema: "migrations/"
    gen:
      go:
        package: "db"
        out: "pkg/db/generated"
        sql_package: "pgx/v5"
        emit_json_tags: true
```

## Query Patterns (from multica)

### Task Claiming (Critical Path)

The task poll endpoint must use `FOR UPDATE SKIP LOCKED` to prevent double-claiming:

```sql
-- name: ClaimPendingTask :one
UPDATE task
SET status = 'running',
    daemon_id = $1,
    agent_runtime_id = $2,
    started_at = now(),
    updated_at = now()
WHERE id = (
    SELECT id FROM task
    WHERE status = 'pending'
    AND agent_type = ANY($3::text[])
    ORDER BY created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;
```

### Heartbeat Update

```sql
-- name: UpdateDaemonHeartbeat :exec
UPDATE daemon
SET last_heartbeat_at = now(),
    status = 'online',
    updated_at = now()
WHERE id = $1;
```

### Offline Detection

```sql
-- name: MarkOfflineDaemons :exec
UPDATE daemon
SET status = 'offline',
    updated_at = now()
WHERE status = 'online'
AND last_heartbeat_at < now() - interval '45 seconds';
```
