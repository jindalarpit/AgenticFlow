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
	Queries db.Querier
}

// NewRuntimeHandler creates a new RuntimeHandler.
func NewRuntimeHandler(queries db.Querier) *RuntimeHandler {
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
// Models are ordered with the most commonly available/recommended first.
func modelsForProvider(provider string) []string {
	switch provider {
	case "claude":
		return []string{
			"claude-opus-4-8",
			"claude-sonnet-4-6",
			"claude-haiku-4-5",
		}
	case "gemini":
		return []string{
			"gemini-3.1-pro-preview",
			"gemini-3-flash-preview",
			"gemini-3.1-flash-lite-preview",
			"gemini-2.5-pro",
			"gemini-2.5-flash",
			"gemini-2.5-flash-lite",
			"gemma-4-31b-it",
			"gemma-4-26b-a4b-it",
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
			"opencode/big-pickle",
			"opencode/deepseek-v4-flash-free",
			"opencode/mimo-v2.5-free",
			"opencode/nemotron-3-super-free",
			"github-copilot/claude-sonnet-4.5",
			"github-copilot/claude-sonnet-4.6",
			"github-copilot/claude-opus-4.5",
			"github-copilot/gemini-2.5-pro",
			"github-copilot/gpt-4.1",
			"github-copilot/gpt-4o",
			"kiro/claude-sonnet-4",
			"kiro/claude-sonnet-4.5",
			"kiro/claude-opus-4.5",
			"kiro/auto",
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
			"claude-sonnet-4-6",
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
