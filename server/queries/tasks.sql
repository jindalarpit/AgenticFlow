-- name: CreateTask :one
INSERT INTO task (user_id, agent_type, prompt, agent_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetTaskByID :one
SELECT * FROM task
WHERE id = $1;

-- name: ListTasksByUser :many
SELECT * FROM task
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateTaskStatus :exec
UPDATE task
SET status = $2, updated_at = now()
WHERE id = $1;

-- name: ClaimPendingTask :one
UPDATE task
SET status = 'running',
    daemon_id = $1,
    agent_runtime_id = $2,
    started_at = now(),
    updated_at = now()
WHERE id = (
    SELECT t.id FROM task t
    JOIN agent_runtime ar ON ar.provider = t.agent_type AND ar.daemon_id = $1
    WHERE t.status = 'pending'
    ORDER BY t.created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: UpdateTaskStarted :exec
UPDATE task
SET status = 'running', started_at = now(), daemon_id = $1, agent_runtime_id = $2, updated_at = now()
WHERE id = $3;

-- name: UpdateTaskCompleted :exec
UPDATE task
SET status = 'completed', exit_code = $2, output_preview = $3, completed_at = now(), updated_at = now()
WHERE id = $1;

-- name: UpdateTaskFailed :exec
UPDATE task
SET status = 'failed', exit_code = $2, error_message = $3, completed_at = now(), updated_at = now()
WHERE id = $1;

-- name: ListTasksByAgent :many
SELECT * FROM task
WHERE agent_id = $1 AND user_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: CancelTask :exec
UPDATE task
SET status = 'cancelled', completed_at = now(), updated_at = now()
WHERE id = $1 AND user_id = $2 AND status IN ('pending', 'running');

-- name: GetTaskDaemonID :one
-- Returns the daemon_id for a running task.
SELECT daemon_id FROM task
WHERE id = $1 AND status = 'running';

-- name: GetAgentStats30d :one
-- Returns 30-day aggregate stats for a given agent: total completed tasks,
-- total terminal tasks (completed + failed + cancelled), and average duration
-- of completed tasks in milliseconds. Returns zeros when no matching tasks exist.
SELECT
    COALESCE(COUNT(*) FILTER (WHERE status = 'completed'), 0)::bigint AS total_completed,
    COALESCE(COUNT(*) FILTER (WHERE status IN ('completed', 'failed', 'cancelled')), 0)::bigint AS total_terminal,
    COALESCE(
        EXTRACT(EPOCH FROM AVG(completed_at - started_at) FILTER (WHERE status = 'completed' AND started_at IS NOT NULL))::bigint * 1000,
        0
    )::bigint AS avg_duration_ms
FROM task
WHERE agent_id = $1
  AND completed_at > now() - INTERVAL '30 days';
