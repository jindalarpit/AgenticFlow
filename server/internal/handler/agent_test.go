package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
)

// TestCreateAgent_Validation tests the validation logic of the CreateAgent handler.
// These tests exercise the handler's request parsing and field validation without
// requiring a database connection (they fail before any DB call is made).
func TestCreateAgent_Validation(t *testing.T) {
	h := &AgentHandler{} // no DB needed for validation-only tests

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
		wantError  string
	}{
		{
			name:       "empty name rejected",
			body:       CreateAgentRequest{RuntimeID: "550e8400-e29b-41d4-a716-446655440000"},
			wantStatus: http.StatusBadRequest,
			wantError:  "name is required",
		},
		{
			name:       "name starting with hyphen rejected",
			body:       CreateAgentRequest{Name: "-invalid", RuntimeID: "550e8400-e29b-41d4-a716-446655440000"},
			wantStatus: http.StatusBadRequest,
			wantError:  "name must start with an alphanumeric",
		},
		{
			name:       "name starting with underscore rejected",
			body:       CreateAgentRequest{Name: "_invalid", RuntimeID: "550e8400-e29b-41d4-a716-446655440000"},
			wantStatus: http.StatusBadRequest,
			wantError:  "name must start with an alphanumeric",
		},
		{
			name:       "name with spaces rejected",
			body:       CreateAgentRequest{Name: "has space", RuntimeID: "550e8400-e29b-41d4-a716-446655440000"},
			wantStatus: http.StatusBadRequest,
			wantError:  "name must start with an alphanumeric",
		},
		{
			name:       "name too long rejected",
			body:       CreateAgentRequest{Name: "a" + strings.Repeat("b", 64), RuntimeID: "550e8400-e29b-41d4-a716-446655440000"},
			wantStatus: http.StatusBadRequest,
			wantError:  "name must start with an alphanumeric",
		},
		{
			name: "description too long rejected",
			body: CreateAgentRequest{
				Name:        "valid-agent",
				Description: strings.Repeat("x", 256),
				RuntimeID:   "550e8400-e29b-41d4-a716-446655440000",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "description must be 255 characters or fewer",
		},
		{
			name: "instructions too long rejected",
			body: CreateAgentRequest{
				Name:         "valid-agent",
				Instructions: strings.Repeat("x", 50001),
				RuntimeID:    "550e8400-e29b-41d4-a716-446655440000",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "instructions must be 50000 characters or fewer",
		},
		{
			name: "model too long rejected",
			body: CreateAgentRequest{
				Name:      "valid-agent",
				Model:     strings.Repeat("x", 101),
				RuntimeID: "550e8400-e29b-41d4-a716-446655440000",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "model must be 100 characters or fewer",
		},
		{
			name: "custom_env too many pairs rejected",
			body: CreateAgentRequest{
				Name:      "valid-agent",
				RuntimeID: "550e8400-e29b-41d4-a716-446655440000",
				CustomEnv: makeEnvPairs(21),
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "custom_env must have 20 or fewer key-value pairs",
		},
		{
			name: "custom_env empty key rejected",
			body: CreateAgentRequest{
				Name:      "valid-agent",
				RuntimeID: "550e8400-e29b-41d4-a716-446655440000",
				CustomEnv: map[string]string{"": "value"},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "custom_env key must be between 1 and 64 characters",
		},
		{
			name: "custom_env key too long rejected",
			body: CreateAgentRequest{
				Name:      "valid-agent",
				RuntimeID: "550e8400-e29b-41d4-a716-446655440000",
				CustomEnv: map[string]string{strings.Repeat("k", 65): "value"},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "custom_env key must be between 1 and 64 characters",
		},
		{
			name: "custom_env empty value rejected",
			body: CreateAgentRequest{
				Name:      "valid-agent",
				RuntimeID: "550e8400-e29b-41d4-a716-446655440000",
				CustomEnv: map[string]string{"KEY": ""},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "custom_env value must be between 1 and 1024 characters",
		},
		{
			name: "custom_env value too long rejected",
			body: CreateAgentRequest{
				Name:      "valid-agent",
				RuntimeID: "550e8400-e29b-41d4-a716-446655440000",
				CustomEnv: map[string]string{"KEY": strings.Repeat("v", 1025)},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "custom_env value must be between 1 and 1024 characters",
		},
		{
			name: "custom_args too many items rejected",
			body: CreateAgentRequest{
				Name:       "valid-agent",
				RuntimeID:  "550e8400-e29b-41d4-a716-446655440000",
				CustomArgs: makeArgs(21),
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "custom_args must have 20 or fewer items",
		},
		{
			name: "custom_args item too long rejected",
			body: CreateAgentRequest{
				Name:       "valid-agent",
				RuntimeID:  "550e8400-e29b-41d4-a716-446655440000",
				CustomArgs: []string{strings.Repeat("a", 257)},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "each custom_args item must be 256 characters or fewer",
		},
		{
			name: "max_concurrent_tasks below minimum rejected",
			body: CreateAgentRequest{
				Name:               "valid-agent",
				RuntimeID:          "550e8400-e29b-41d4-a716-446655440000",
				MaxConcurrentTasks: -1,
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "max_concurrent_tasks must be between 1 and 20",
		},
		{
			name: "max_concurrent_tasks above maximum rejected",
			body: CreateAgentRequest{
				Name:               "valid-agent",
				RuntimeID:          "550e8400-e29b-41d4-a716-446655440000",
				MaxConcurrentTasks: 21,
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "max_concurrent_tasks must be between 1 and 20",
		},
		{
			name: "invalid visibility rejected",
			body: CreateAgentRequest{
				Name:       "valid-agent",
				RuntimeID:  "550e8400-e29b-41d4-a716-446655440000",
				Visibility: "public",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "visibility must be",
		},
		{
			name: "empty runtime_id rejected",
			body: CreateAgentRequest{
				Name: "valid-agent",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "runtime_id is required",
		},
		{
			name: "invalid runtime_id format rejected",
			body: CreateAgentRequest{
				Name:      "valid-agent",
				RuntimeID: "not-a-uuid",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid runtime_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// Inject a fake user ID into context to pass the auth check.
			ctx := middleware.WithUserID(req.Context(), "550e8400-e29b-41d4-a716-446655440000")
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			h.CreateAgent(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantError != "" && !strings.Contains(w.Body.String(), tt.wantError) {
				t.Errorf("body = %s, want to contain %q", w.Body.String(), tt.wantError)
			}
		})
	}
}

// TestCreateAgent_ValidNamePatterns tests that valid agent names are accepted.
func TestCreateAgent_ValidNamePatterns(t *testing.T) {
	validNames := []string{
		"a",
		"A",
		"0",
		"agent1",
		"my-agent",
		"my_agent",
		"Agent-Name_123",
		"a" + strings.Repeat("b", 63), // exactly 64 chars
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			if !agentNameRegex.MatchString(name) {
				t.Errorf("expected name %q to be valid", name)
			}
		})
	}
}

// TestCreateAgent_InvalidNamePatterns tests that invalid agent names are rejected.
func TestCreateAgent_InvalidNamePatterns(t *testing.T) {
	invalidNames := []string{
		"",
		"-starts-with-hyphen",
		"_starts-with-underscore",
		"has space",
		"has.dot",
		"has@symbol",
		"a" + strings.Repeat("b", 64), // 65 chars
	}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			if agentNameRegex.MatchString(name) {
				t.Errorf("expected name %q to be invalid", name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeEnvPairs creates a map with n key-value pairs.
func makeEnvPairs(n int) map[string]string {
	m := make(map[string]string, n)
	for i := 0; i < n; i++ {
		m[strings.Repeat("K", 1)+strings.Repeat("0", i%10)] = "value"
	}
	// If we need exactly n pairs and the above deduplicates, use indexed keys.
	m = make(map[string]string, n)
	for i := 0; i < n; i++ {
		key := "KEY_" + strings.Repeat("0", i)
		m[key] = "value"
	}
	return m
}

// makeArgs creates a slice with n string items.
func makeArgs(n int) []string {
	args := make([]string, n)
	for i := range args {
		args[i] = "arg"
	}
	return args
}
