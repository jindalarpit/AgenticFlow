-- name: CreateToken :one
INSERT INTO personal_access_token (user_id, name, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreatePersonalAccessToken :one
INSERT INTO personal_access_token (user_id, name, token_hash, token_prefix, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetTokenByHash :one
SELECT * FROM personal_access_token
WHERE token_hash = $1
  AND (expires_at IS NULL OR expires_at > now());

-- name: ListTokensByUser :many
SELECT * FROM personal_access_token
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 100;

-- name: DeleteToken :exec
DELETE FROM personal_access_token
WHERE id = $1 AND user_id = $2;

-- name: DeleteTokenReturningHash :one
DELETE FROM personal_access_token
WHERE id = $1 AND user_id = $2
RETURNING token_hash;

-- name: UpdateTokenLastUsed :exec
UPDATE personal_access_token
SET last_used_at = now()
WHERE id = $1;
