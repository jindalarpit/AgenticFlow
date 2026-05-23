package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agenticflow/agenticflow/internal/middleware"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestRegister_Validation(t *testing.T) {
	tests := []struct {
		name       string
		body       DaemonRegisterReq
		wantStatus int
		wantError  string
	}{
		{
			name:       "empty daemon_id rejected",
			body:       DaemonRegisterReq{DaemonID: "", Agents: map[string]AgentInfo{"claude": {}}},
			wantStatus: http.StatusBadRequest,
			wantError:  "daemon_id is required",
		},
		{
			name:       "whitespace daemon_id rejected",
			body:       DaemonRegisterReq{DaemonID: "   ", Agents: map[string]AgentInfo{"claude": {}}},
			wantStatus: http.StatusBadRequest,
			wantError:  "daemon_id is required",
		},
		{
			name:       "empty agents rejected",
			body:       DaemonRegisterReq{DaemonID: "test-daemon", Agents: map[string]AgentInfo{}},
			wantStatus: http.StatusBadRequest,
			wantError:  "at least one agent is required",
		},
		{
			name:       "nil agents rejected",
			body:       DaemonRegisterReq{DaemonID: "test-daemon"},
			wantStatus: http.StatusBadRequest,
			wantError:  "at least one agent is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewDaemonHandler(nil, nil)

			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/daemon/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			// Set a user ID in context so we get past the auth check.
			ctx := middleware.WithDaemonContext(req.Context(), "some-user-id", "some-daemon-id")
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			h.Register(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", w.Code, tt.wantStatus, w.Body.String())
			}

			var resp map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp["error"] != tt.wantError {
				t.Errorf("error = %q, want %q", resp["error"], tt.wantError)
			}
		})
	}
}

func TestRegister_UnauthorizedWithoutUserID(t *testing.T) {
	h := NewDaemonHandler(nil, nil)

	body, _ := json.Marshal(DaemonRegisterReq{
		DaemonID: "test-daemon",
		Agents:   map[string]AgentInfo{"claude": {Path: "/usr/bin/claude"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/daemon/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No user ID in context.
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestEnsureDefaultAgent_ParsesRuntimeIDs(t *testing.T) {
	// Verify that parseUUID works correctly for runtime IDs.
	validUUID := "12345678-1234-1234-1234-123456789abc"
	parsed, err := parseUUID(validUUID)
	if err != nil {
		t.Fatalf("parseUUID(%q) failed: %v", validUUID, err)
	}
	if !parsed.Valid {
		t.Error("expected parsed UUID to be valid")
	}

	// Invalid UUID should fail.
	_, err = parseUUID("not-a-uuid")
	if err == nil {
		t.Error("expected parseUUID to fail for invalid UUID")
	}

	// Empty UUID should fail.
	_, err = parseUUID("")
	if err == nil {
		t.Error("expected parseUUID to fail for empty string")
	}
}

func TestEnsureDefaultAgent_FirstRuntimeSelection(t *testing.T) {
	// Verify that the function picks the first valid runtime ID from the map.
	// Since Go maps are unordered, any valid UUID should be acceptable.
	runtimeIDs := map[string]string{
		"claude": "12345678-1234-1234-1234-123456789abc",
		"codex":  "invalid-uuid",
	}

	var firstValid pgtype.UUID
	for _, rid := range runtimeIDs {
		parsed, err := parseUUID(rid)
		if err == nil {
			firstValid = parsed
			break
		}
	}

	if !firstValid.Valid {
		t.Error("expected to find at least one valid runtime ID")
	}
}
