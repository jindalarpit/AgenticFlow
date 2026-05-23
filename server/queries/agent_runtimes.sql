-- name: CreateRuntime :one
INSERT INTO agent_runtime (daemon_id, provider, name, version, binary_path, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetRuntimeByID :one
SELECT * FROM agent_runtime
WHERE id = $1;

-- name: ListRuntimesByDaemon :many
SELECT * FROM agent_runtime
WHERE daemon_id = $1
ORDER BY created_at ASC;

-- name: ListRuntimesByProvider :many
SELECT * FROM agent_runtime
WHERE provider = $1
ORDER BY created_at ASC;

-- name: UpdateRuntimeStatus :exec
UPDATE agent_runtime
SET status = $2, updated_at = now()
WHERE id = $1;

-- name: DeleteRuntimesByDaemon :exec
DELETE FROM agent_runtime
WHERE daemon_id = $1;

-- name: UpsertRuntime :one
INSERT INTO agent_runtime (daemon_id, provider, name, version, binary_path, status)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (daemon_id, provider)
DO UPDATE SET
    name = EXCLUDED.name,
    version = EXCLUDED.version,
    binary_path = EXCLUDED.binary_path,
    status = EXCLUDED.status,
    updated_at = now()
RETURNING *;
