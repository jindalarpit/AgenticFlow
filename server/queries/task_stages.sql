-- name: CreateTaskStage :one
INSERT INTO task_stage (task_id, stage_name, stage_order, status)
VALUES ($1, $2, $3, 'pending')
RETURNING *;

-- name: GetNextPendingStage :one
SELECT * FROM task_stage
WHERE task_id = $1 AND status = 'pending'
ORDER BY stage_order ASC
LIMIT 1;

-- name: GetCompletedStagesForTask :many
SELECT * FROM task_stage
WHERE task_id = $1 AND status IN ('completed', 'approved')
ORDER BY stage_order ASC;

-- name: UpdateStageStatus :exec
UPDATE task_stage
SET status = $2,
    started_at = CASE WHEN $2 = 'running' THEN now() ELSE started_at END,
    completed_at = CASE WHEN $2 IN ('awaiting_approval', 'approved', 'completed', 'failed') THEN now() ELSE completed_at END
WHERE id = $1;

-- name: UpdateStageOutput :exec
UPDATE task_stage
SET output_content = $2
WHERE id = $1;

-- name: UpdateStageFeedback :exec
UPDATE task_stage
SET feedback = $2
WHERE id = $1;

-- name: ListStagesForTask :many
SELECT * FROM task_stage
WHERE task_id = $1
ORDER BY stage_order ASC;

-- name: GetStageByTaskAndName :one
SELECT * FROM task_stage
WHERE task_id = $1 AND stage_name = $2;

-- name: ClaimPendingTaskWithStage :one
-- Claims the next pending task that has a pending stage ready for execution.
-- Used by the daemon to pick up staged tasks where the next stage is pending.
UPDATE task
SET status = 'running',
    daemon_id = $1,
    agent_runtime_id = $2,
    started_at = now(),
    updated_at = now()
WHERE id = (
    SELECT t.id FROM task t
    JOIN agent_runtime ar ON ar.provider = t.agent_type AND ar.daemon_id = $1
    JOIN task_stage ts ON ts.task_id = t.id AND ts.status = 'pending'
    WHERE t.status = 'pending'
    ORDER BY t.created_at ASC, ts.stage_order ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;
