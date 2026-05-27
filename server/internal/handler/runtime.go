package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// RuntimeHandler holds dependencies for runtime-related HTTP handlers.
type RuntimeHandler struct {
	Queries *db.Queries
}

// NewRuntimeHandler creates a new RuntimeHandler.
func NewRuntimeHandler(queries *db.Queries) *RuntimeHandler {
	return &RuntimeHandler{Queries: queries}
}

// ListRuntimeModelsResponse is the JSON response for GET /api/runtimes/:id/models.
type ListRuntimeModelsResponse struct {
	Models []string `json:"models"`
}

// ListRuntimeModels handles GET /api/runtimes/{id}/models.
// It returns a static list of available models based on the runtime's provider type.
func (h *RuntimeHandler) ListRuntimeModels(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "id")
	if runtimeID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "runtime id is required")
		return
	}

	runtimeUUID, err := parseUUID(runtimeID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid runtime id format")
		return
	}

	runtime, err := h.Queries.GetRuntimeByID(r.Context(), runtimeUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorJSON(w, http.StatusNotFound, "runtime not found")
			return
		}
		slog.Error("list runtime models: runtime lookup failed", "runtime_id", runtimeID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to get runtime")
		return
	}

	models := modelsForProvider(runtime.Provider)

	writeJSON(w, http.StatusOK, ListRuntimeModelsResponse{Models: models})
}

// modelsForProvider returns a static list of known models for the given provider.
// This avoids the complexity of querying the daemon in real-time while still
// providing useful model suggestions to the frontend.
func modelsForProvider(provider string) []string {
	switch provider {
	case "claude":
		return []string{
			"claude-sonnet-4-20250514",
			"claude-opus-4-20250514",
			"claude-3-5-haiku-20241022",
		}
	case "gemini":
		return []string{
			"gemini-2.5-pro",
			"gemini-2.5-flash",
			"gemini-2.0-flash",
		}
	case "codex":
		return []string{
			"codex-mini",
			"o4-mini",
			"o3",
			"gpt-4.1",
		}
	case "opencode":
		return []string{
			"anthropic/claude-sonnet-4-20250514",
			"openai/gpt-4.1",
			"anthropic/claude-3-5-haiku-20241022",
		}
	case "copilot":
		return []string{
			"claude-sonnet-4",
			"gpt-4.1",
			"o4-mini",
			"gemini-2.5-pro",
		}
	case "cursor":
		return []string{
			"claude-sonnet-4-20250514",
			"gpt-4.1",
			"gemini-2.5-pro",
		}
	case "kiro":
		return []string{
			"claude-sonnet-4-20250514",
		}
	case "hermes":
		return []string{
			"claude-sonnet-4-20250514",
			"gpt-4.1",
		}
	case "pi":
		return []string{
			"claude-sonnet-4-20250514",
			"gpt-4.1",
		}
	case "kimi":
		return []string{
			"kimi-latest",
		}
	case "openclaw":
		return []string{
			"claude-sonnet-4-20250514",
			"gpt-4.1",
		}
	default:
		return []string{}
	}
}
