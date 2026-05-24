package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestListRuntimeModels_MissingID(t *testing.T) {
	t.Parallel()

	h := &RuntimeHandler{Queries: nil}

	router := chi.NewRouter()
	router.Get("/api/runtimes/{id}/models", h.ListRuntimeModels)

	// Request with whitespace-only id segment — chi will trim to empty.
	r := httptest.NewRequest(http.MethodGet, "/api/runtimes/%20/models", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)

	// " " is not a valid UUID, so we expect a 400.
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestListRuntimeModels_InvalidUUID(t *testing.T) {
	t.Parallel()

	h := &RuntimeHandler{Queries: nil}

	router := chi.NewRouter()
	router.Get("/api/runtimes/{id}/models", h.ListRuntimeModels)

	r := httptest.NewRequest(http.MethodGet, "/api/runtimes/not-a-uuid/models", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "invalid runtime id format" {
		t.Errorf("expected error 'invalid runtime id format', got %q", body["error"])
	}
}

func TestModelsForProvider_KnownProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		provider string
		minCount int
	}{
		{"claude", 3},
		{"gemini", 3},
		{"codex", 3},
		{"opencode", 3},
		{"copilot", 3},
		{"cursor", 3},
		{"kiro", 1},
		{"hermes", 2},
		{"pi", 2},
		{"kimi", 1},
		{"openclaw", 2},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			models := modelsForProvider(tt.provider)
			if len(models) < tt.minCount {
				t.Errorf("provider %q: expected at least %d models, got %d",
					tt.provider, tt.minCount, len(models))
			}
			// Verify no empty strings.
			for i, m := range models {
				if m == "" {
					t.Errorf("provider %q: model[%d] is empty", tt.provider, i)
				}
			}
		})
	}
}

func TestModelsForProvider_UnknownProvider(t *testing.T) {
	t.Parallel()

	models := modelsForProvider("nonexistent-provider")
	if len(models) != 0 {
		t.Errorf("expected empty slice for unknown provider, got %v", models)
	}
}
