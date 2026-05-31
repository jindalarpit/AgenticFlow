-- name: ListSkillTemplates :many
SELECT id, slug, name, description, category, version, icon
FROM skill_template
ORDER BY category, name;

-- name: ListSkillTemplatesByCategory :many
SELECT id, slug, name, description, category, version, icon
FROM skill_template
WHERE category = $1
ORDER BY name;

-- name: GetSkillTemplateBySlug :one
SELECT * FROM skill_template WHERE slug = $1;

-- name: UpsertSkillTemplate :exec
INSERT INTO skill_template (slug, name, description, content, category, version, icon)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    content = EXCLUDED.content,
    category = EXCLUDED.category,
    version = EXCLUDED.version,
    icon = EXCLUDED.icon,
    updated_at = now()
WHERE skill_template.version <> EXCLUDED.version;
