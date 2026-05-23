-- name: CreateTaskMessage :one
INSERT INTO task_message (task_id, sequence, stream, content)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListTaskMessagesByTask :many
SELECT * FROM task_message
WHERE task_id = $1
ORDER BY sequence ASC;
