-- name: CreateAgent :one
INSERT INTO agent (user_id, name, description, instructions, runtime_id, model, custom_env, custom_args, max_concurrent_tasks, visibility, avatar_url, mcp_config)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetAgent :one
SELECT * FROM agent WHERE id = $1;

-- name: ListAgentsByUser :many
SELECT * FROM agent
WHERE user_id = $1 OR visibility = 'shared'
ORDER BY created_at DESC;

-- name: UpdateAgent :one
UPDATE agent SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    instructions = COALESCE(sqlc.narg('instructions'), instructions),
    runtime_id = COALESCE(sqlc.narg('runtime_id'), runtime_id),
    model = COALESCE(sqlc.narg('model'), model),
    custom_env = COALESCE(sqlc.narg('custom_env'), custom_env),
    custom_args = COALESCE(sqlc.narg('custom_args'), custom_args),
    max_concurrent_tasks = COALESCE(sqlc.narg('max_concurrent_tasks'), max_concurrent_tasks),
    visibility = COALESCE(sqlc.narg('visibility'), visibility),
    avatar_url = COALESCE(sqlc.narg('avatar_url'), avatar_url),
    mcp_config = CASE WHEN sqlc.arg('set_mcp_config')::boolean THEN sqlc.narg('mcp_config') ELSE mcp_config END,
    updated_at = now()
WHERE id = @id AND user_id = @user_id
RETURNING *;

-- name: DeleteAgent :exec
DELETE FROM agent WHERE id = $1 AND user_id = $2;

-- name: CountActiveTasksForAgent :one
SELECT COUNT(*) FROM task
WHERE agent_id = $1 AND status = 'running';

-- name: ArchiveAgent :one
UPDATE agent SET archived_at = now(), updated_at = now()
WHERE id = $1 AND (user_id = $2 OR sqlc.arg('is_admin')::boolean = true)
RETURNING *;

-- name: RestoreAgent :one
UPDATE agent SET archived_at = NULL, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetAgentByName :one
SELECT * FROM agent WHERE user_id = $1 AND name = $2;

-- name: CountAgentsByUser :one
SELECT COUNT(*) FROM agent WHERE user_id = $1;

-- name: ListAgentsByDaemon :many
SELECT a.* FROM agent a
JOIN agent_runtime ar ON a.runtime_id = ar.id
WHERE ar.daemon_id = $1
ORDER BY a.created_at ASC;

-- name: GetAgentByTaskID :one
SELECT a.* FROM agent a
JOIN task t ON t.agent_id = a.id
WHERE t.id = $1;

-- name: UpdateAgentStatus :exec
UPDATE agent SET status = @status, updated_at = now() WHERE id = @id;

-- name: ClaimPendingTaskForRuntime :one
UPDATE task SET
    status = 'running',
    daemon_id = $2,
    started_at = now(),
    updated_at = now()
WHERE id = (
    SELECT t.id FROM task t
    JOIN agent a ON t.agent_id = a.id
    WHERE t.status = 'pending'
      AND a.runtime_id = $1
      AND (SELECT COUNT(*) FROM task t2 WHERE t2.agent_id = a.id AND t2.status = 'running') < a.max_concurrent_tasks
    ORDER BY t.created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;
