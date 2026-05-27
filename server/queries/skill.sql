-- name: CreateSkill :one
INSERT INTO skill (user_id, name, description, content, config)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSkillByID :one
SELECT * FROM skill WHERE id = $1;

-- name: ListSkillsByUser :many
SELECT s.*, COUNT(ags.agent_id)::int AS agent_count
FROM skill s
LEFT JOIN agent_skill ags ON ags.skill_id = s.id
WHERE s.user_id = $1
GROUP BY s.id
ORDER BY s.created_at DESC;

-- name: GetSkillByName :one
SELECT * FROM skill WHERE user_id = $1 AND name = $2;

-- name: UpdateSkill :one
UPDATE skill SET
    name = COALESCE(NULLIF($2, ''), name),
    description = COALESCE($3, description),
    content = COALESCE(NULLIF($4, ''), content),
    config = COALESCE($5, config),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteSkill :exec
DELETE FROM skill WHERE id = $1 AND user_id = $2;

-- name: CreateSkillFile :one
INSERT INTO skill_file (skill_id, path, content)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListSkillFiles :many
SELECT id, skill_id, path, created_at, updated_at FROM skill_file WHERE skill_id = $1;

-- name: GetSkillFilesWithContent :many
SELECT * FROM skill_file WHERE skill_id = $1;

-- name: DeleteSkillFilesBySkill :exec
DELETE FROM skill_file WHERE skill_id = $1;

-- name: DeleteAllAgentSkills :exec
DELETE FROM agent_skill WHERE agent_id = $1;

-- name: InsertAgentSkill :exec
INSERT INTO agent_skill (agent_id, skill_id) VALUES ($1, $2);

-- name: GetAgentSkills :many
SELECT s.id, s.name, s.description, s.content, s.config
FROM skill s
JOIN agent_skill ags ON ags.skill_id = s.id
WHERE ags.agent_id = $1
ORDER BY s.name;

-- name: GetAgentSkillsWithFiles :many
SELECT s.id, s.name, s.description, s.content
FROM skill s
JOIN agent_skill ags ON ags.skill_id = s.id
WHERE ags.agent_id = $1;

-- name: CountAgentsBySkill :one
SELECT COUNT(*) FROM agent_skill WHERE skill_id = $1;
