-- name: CreateDeliverableType :one
INSERT INTO deliverable_type (user_id, name, description, output_format)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetDeliverableType :one
SELECT * FROM deliverable_type
WHERE id = $1 AND (user_id = $2 OR is_system = true);

-- name: ListDeliverableTypesByUser :many
SELECT * FROM deliverable_type
WHERE user_id = $1 OR is_system = true
ORDER BY is_system DESC, name ASC;

-- name: GetSystemDeliverableTypeByName :one
SELECT * FROM deliverable_type
WHERE is_system = true AND name = $1;

-- name: UpdateDeliverableType :one
UPDATE deliverable_type SET
    name = $3,
    description = $4,
    output_format = $5,
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteDeliverableType :exec
DELETE FROM deliverable_type
WHERE id = $1 AND user_id = $2;

-- name: CountAgentsByDeliverableType :one
SELECT COUNT(*) FROM agent
WHERE deliverable_type_id = $1;
