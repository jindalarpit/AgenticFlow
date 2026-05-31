-- name: CreateProvider :one
INSERT INTO online_provider (user_id, name, provider_type, credentials_encrypted, status, status_message, models)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetProvider :one
SELECT * FROM online_provider
WHERE id = $1 AND user_id = $2;

-- name: ListProvidersByUser :many
SELECT * FROM online_provider
WHERE user_id = $1
ORDER BY created_at ASC;

-- name: ListProvidersByUserAndStatus :many
SELECT * FROM online_provider
WHERE user_id = $1 AND status = $2
ORDER BY created_at ASC;

-- name: UpdateProvider :one
UPDATE online_provider SET
    name = $3,
    credentials_encrypted = $4,
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: UpdateProviderStatus :exec
UPDATE online_provider SET
    status = $2,
    status_message = $3,
    updated_at = now()
WHERE id = $1;

-- name: UpdateProviderModels :exec
UPDATE online_provider SET
    models = $2,
    updated_at = now()
WHERE id = $1;

-- name: DeleteProvider :exec
DELETE FROM online_provider
WHERE id = $1 AND user_id = $2;

-- name: CountAgentsByProvider :one
SELECT COUNT(*) FROM agent
WHERE provider_id = $1;
