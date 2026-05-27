package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// Mock DBTX for agent stats tests
// ---------------------------------------------------------------------------

// mockRow implements pgx.Row for returning controlled scan results.
type mockRow struct {
	scanFn func(dest ...interface{}) error
}

func (r *mockRow) Scan(dest ...interface{}) error {
	return r.scanFn(dest...)
}

// mockDBTX implements the db.DBTX interface for testing.
type mockDBTX struct {
	queryRowFn func(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

func (m *mockDBTX) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *mockDBTX) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (m *mockDBTX) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{scanFn: func(dest ...interface{}) error {
		return pgx.ErrNoRows
	}}
}

// ---------------------------------------------------------------------------
// Helper: set Chi URL param in request context
// ---------------------------------------------------------------------------

func withChiURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// ---------------------------------------------------------------------------
// Tests for GET /api/agents/{id}/stats
// ---------------------------------------------------------------------------

func TestGetAgentStats_InvalidAgentIDFormat(t *testing.T) {
	h := &AgentHandler{Queries: nil}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/not-a-uuid/stats", nil)
	req = withChiURLParam(req, "id", "not-a-uuid")
	w := httptest.NewRecorder()

	h.GetAgentStats(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "invalid agent id format" {
		t.Errorf("error = %q, want %q", resp["error"], "invalid agent id format")
	}
}

func TestGetAgentStats_EmptyAgentID(t *testing.T) {
	h := &AgentHandler{Queries: nil}

	req := httptest.NewRequest(http.MethodGet, "/api/agents//stats", nil)
	req = withChiURLParam(req, "id", "")
	w := httptest.NewRecorder()

	h.GetAgentStats(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "agent id is required" {
		t.Errorf("error = %q, want %q", resp["error"], "agent id is required")
	}
}

func TestGetAgentStats_AgentNotFound(t *testing.T) {
	// Mock: GetAgent returns pgx.ErrNoRows
	callCount := 0
	mock := &mockDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			callCount++
			// First QueryRow call is GetAgent
			return &mockRow{scanFn: func(dest ...interface{}) error {
				return pgx.ErrNoRows
			}}
		},
	}

	h := &AgentHandler{Queries: db.New(mock)}

	agentID := "550e8400-e29b-41d4-a716-446655440000"
	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID+"/stats", nil)
	req = withChiURLParam(req, "id", agentID)
	w := httptest.NewRecorder()

	h.GetAgentStats(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusNotFound, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "agent not found" {
		t.Errorf("error = %q, want %q", resp["error"], "agent not found")
	}
}

func TestGetAgentStats_NoTasks(t *testing.T) {
	// Mock: GetAgent succeeds, GetAgentStats30d returns zeros
	callCount := 0
	mock := &mockDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			callCount++
			if callCount == 1 {
				// GetAgent — scan into Agent struct fields (15 fields)
				return &mockRow{scanFn: func(dest ...interface{}) error {
					// We just need the scan to succeed; the handler only checks for ErrNoRows
					return scanMockAgent(dest)
				}}
			}
			// GetAgentStats30d — returns zeros
			return &mockRow{scanFn: func(dest ...interface{}) error {
				if len(dest) < 3 {
					return fmt.Errorf("expected 3 scan destinations, got %d", len(dest))
				}
				*dest[0].(*int64) = 0 // total_completed
				*dest[1].(*int64) = 0 // total_terminal
				*dest[2].(*int64) = 0 // avg_duration_ms
				return nil
			}}
		},
	}

	h := &AgentHandler{Queries: db.New(mock)}

	agentID := "550e8400-e29b-41d4-a716-446655440000"
	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID+"/stats", nil)
	req = withChiURLParam(req, "id", agentID)
	w := httptest.NewRecorder()

	h.GetAgentStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp AgentStatsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TotalRuns != 0 {
		t.Errorf("total_runs = %d, want 0", resp.TotalRuns)
	}
	if resp.SuccessRate != 0 {
		t.Errorf("success_rate = %d, want 0", resp.SuccessRate)
	}
	if resp.AvgDurationMs != 0 {
		t.Errorf("avg_duration_ms = %d, want 0", resp.AvgDurationMs)
	}
	if resp.TotalTerminal != 0 {
		t.Errorf("total_terminal = %d, want 0", resp.TotalTerminal)
	}
}

func TestGetAgentStats_MixedTerminalTasks(t *testing.T) {
	// Scenario: 7 completed, 3 failed, 2 cancelled = 12 terminal
	// success_rate = round(7/12 * 100) = 58
	// avg_duration_ms = 5000 (mock value)
	callCount := 0
	mock := &mockDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			callCount++
			if callCount == 1 {
				// GetAgent succeeds
				return &mockRow{scanFn: func(dest ...interface{}) error {
					return scanMockAgent(dest)
				}}
			}
			// GetAgentStats30d
			return &mockRow{scanFn: func(dest ...interface{}) error {
				if len(dest) < 3 {
					return fmt.Errorf("expected 3 scan destinations, got %d", len(dest))
				}
				*dest[0].(*int64) = 7     // total_completed
				*dest[1].(*int64) = 12    // total_terminal
				*dest[2].(*int64) = 5000  // avg_duration_ms
				return nil
			}}
		},
	}

	h := &AgentHandler{Queries: db.New(mock)}

	agentID := "550e8400-e29b-41d4-a716-446655440000"
	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID+"/stats", nil)
	req = withChiURLParam(req, "id", agentID)
	w := httptest.NewRecorder()

	h.GetAgentStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp AgentStatsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TotalRuns != 7 {
		t.Errorf("total_runs = %d, want 7", resp.TotalRuns)
	}
	if resp.SuccessRate != 58 {
		t.Errorf("success_rate = %d, want 58", resp.SuccessRate)
	}
	if resp.AvgDurationMs != 5000 {
		t.Errorf("avg_duration_ms = %d, want 5000", resp.AvgDurationMs)
	}
	if resp.TotalTerminal != 12 {
		t.Errorf("total_terminal = %d, want 12", resp.TotalTerminal)
	}
}

func TestGetAgentStats_OnlyFailedTasks(t *testing.T) {
	// Scenario: 0 completed, 5 failed, 0 cancelled = 5 terminal
	// success_rate = 0 (0/5 * 100 = 0)
	// total_runs = 0 (no completed tasks)
	callCount := 0
	mock := &mockDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			callCount++
			if callCount == 1 {
				// GetAgent succeeds
				return &mockRow{scanFn: func(dest ...interface{}) error {
					return scanMockAgent(dest)
				}}
			}
			// GetAgentStats30d
			return &mockRow{scanFn: func(dest ...interface{}) error {
				if len(dest) < 3 {
					return fmt.Errorf("expected 3 scan destinations, got %d", len(dest))
				}
				*dest[0].(*int64) = 0 // total_completed
				*dest[1].(*int64) = 5 // total_terminal
				*dest[2].(*int64) = 0 // avg_duration_ms (no completed tasks)
				return nil
			}}
		},
	}

	h := &AgentHandler{Queries: db.New(mock)}

	agentID := "550e8400-e29b-41d4-a716-446655440000"
	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID+"/stats", nil)
	req = withChiURLParam(req, "id", agentID)
	w := httptest.NewRecorder()

	h.GetAgentStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp AgentStatsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TotalRuns != 0 {
		t.Errorf("total_runs = %d, want 0", resp.TotalRuns)
	}
	if resp.SuccessRate != 0 {
		t.Errorf("success_rate = %d, want 0", resp.SuccessRate)
	}
	if resp.AvgDurationMs != 0 {
		t.Errorf("avg_duration_ms = %d, want 0", resp.AvgDurationMs)
	}
	if resp.TotalTerminal != 5 {
		t.Errorf("total_terminal = %d, want 5", resp.TotalTerminal)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// scanMockAgent fills the Agent struct scan destinations with valid dummy data.
// The Agent struct has 15 fields scanned in order:
// ID, UserID, Name, Description, Instructions, RuntimeID, Model,
// CustomEnv, CustomArgs, MaxConcurrentTasks, Visibility, AvatarUrl,
// Status, CreatedAt, UpdatedAt
func scanMockAgent(dest []interface{}) error {
	if len(dest) < 15 {
		return fmt.Errorf("expected 15 scan destinations for Agent, got %d", len(dest))
	}
	// We don't need to set all fields — just ensure the scan doesn't error.
	// The handler only checks for pgx.ErrNoRows from GetAgent.
	// pgtype fields have zero values that are valid enough for our purposes.
	return nil
}
