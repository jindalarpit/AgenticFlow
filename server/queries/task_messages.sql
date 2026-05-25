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

-- name: CreateStructuredTaskMessage :one
-- Inserts a structured task message with type, tool, input, output fields.
INSERT INTO task_message (task_id, sequence, stream, content, type, tool, input, output)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (task_id, sequence) DO NOTHING
RETURNING *;

-- name: ListTaskMessagesByTask :many
SELECT * FROM task_message
WHERE task_id = $1
ORDER BY sequence ASC;

-- name: CountToolUseMessages :one
SELECT COUNT(*) FROM task_message
WHERE task_id = $1 AND type = 'tool_use';
