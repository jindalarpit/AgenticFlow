-- name: CreateTaskMessage :one
INSERT INTO task_message (task_id, sequence, stream, content)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreateTaskMessageIdempotent :one
-- Inserts a task message, ignoring duplicates (same task_id + sequence).
INSERT INTO task_message (task_id, sequence, stream, content)
VALUES ($1, $2, $3, $4)
ON CONFLICT (task_id, sequence) DO NOTHING
RETURNING *;

-- name: ListTaskMessagesByTask :many
SELECT * FROM task_message
WHERE task_id = $1
ORDER BY sequence ASC;
