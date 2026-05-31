package seed

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// SeedTemplates inserts or updates built-in templates.
// Called after migrations, before HTTP listener starts.
func SeedTemplates(ctx context.Context, queries db.Querier) {
	for _, tmpl := range BuiltinTemplates {
		err := queries.UpsertSkillTemplate(ctx, db.UpsertSkillTemplateParams{
			Slug:        tmpl.Slug,
			Name:        tmpl.Name,
			Description: tmpl.Description,
			Content:     tmpl.Content,
			Category:    tmpl.Category,
			Version:     tmpl.Version,
			Icon:        pgtype.Text{String: tmpl.Icon, Valid: tmpl.Icon != ""},
		})
		if err != nil {
			slog.Error("failed to seed skill template",
				"slug", tmpl.Slug,
				"error", err,
			)
			// Continue seeding remaining templates
		}
	}
}
