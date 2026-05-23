-- name: UpsertDaemon :one
INSERT INTO daemon (user_id, daemon_id, device_name, status, cli_version, last_heartbeat_at)
VALUES ($1, $2, $3, 'online', $4, now())
ON CONFLICT (user_id, daemon_id)
DO UPDATE SET
    device_name = EXCLUDED.device_name,
    status = 'online',
    cli_version = EXCLUDED.cli_version,
    last_heartbeat_at = now(),
    updated_at = now()
RETURNING *;

-- name: GetDaemonByID :one
SELECT * FROM daemon
WHERE id = $1;

-- name: GetDaemonByDaemonID :one
SELECT * FROM daemon
WHERE user_id = $1 AND daemon_id = $2;

-- name: ListDaemonsByUser :many
SELECT * FROM daemon
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: UpdateDaemonStatus :exec
UPDATE daemon
SET status = $2, updated_at = now()
WHERE id = $1;

-- name: UpdateDaemonHeartbeat :exec
UPDATE daemon
SET last_heartbeat_at = now(), updated_at = now()
WHERE id = $1;

-- name: ListOfflineDaemons :many
SELECT * FROM daemon
WHERE status = 'online'
  AND last_heartbeat_at < now() - make_interval(secs => @stale_seconds::double precision);
