-- name: CreateConversationalTask :one
-- Creates a task for the conversational workflow model.
-- The deliverables column stores prior_context as a JSON array of strings.
-- git_repo_url and workspace_path store workspace config for execution tasks.
INSERT INTO task (user_id, agent_type, prompt, agent_id, deliverables, workspace_mode, workspace_path, git_repo_url)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: CreateConversationalTaskStage :one
-- Creates a task_stage row for a conversational task with a specific deliverable_type.
INSERT INTO task_stage (task_id, stage_name, stage_order, status)
VALUES ($1, $2, $3, 'pending')
RETURNING *;

-- name: UpdateStageCompletion :exec
-- Updates a task_stage on completion with output content, session_id, and work_dir.
UPDATE task_stage
SET status = 'completed',
    output_content = $2,
    session_id = $3,
    work_dir = $4,
    completed_at = now()
WHERE id = $1;

-- name: GetLatestSessionForStage :one
-- Gets the most recent session_id and work_dir for a task_stage.
SELECT session_id, work_dir FROM task_stage
WHERE id = $1;

-- name: CreatePromptHistoryEntry :one
-- Inserts a prompt_history row after a conversational task turn completes.
INSERT INTO prompt_history (task_stage_id, task_id, prompt_text, output_text)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListPromptHistoryForStage :many
-- Lists all prompt_history entries for a task_stage ordered by creation time.
SELECT * FROM prompt_history
WHERE task_stage_id = $1
ORDER BY created_at ASC;

-- name: CreateFollowUpTask :one
-- Creates a new task for a follow-up message linked to an existing stage.
-- The deliverables column stores prior_context, and workspace config is preserved.
INSERT INTO task (user_id, agent_type, prompt, agent_id, deliverables, workspace_mode, workspace_path, git_repo_url)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetTaskStageByTaskAndName :one
-- Retrieves a specific task_stage by task ID and stage name.
SELECT * FROM task_stage
WHERE task_id = $1 AND stage_name = $2;
