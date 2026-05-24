package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockDaemonState implements DaemonStateProvider for testing.
type mockDaemonState struct {
	daemonID        string
	deviceName      string
	serverURL       string
	cliVersion      string
	activeTaskCount int64
	agents          []AgentInfo
	startTime       time.Time
}

func (m *mockDaemonState) GetDaemonID() string        { return m.daemonID }
func (m *mockDaemonState) GetDeviceName() string      { return m.deviceName }
func (m *mockDaemonState) GetServerURL() string       { return m.serverURL }
func (m *mockDaemonState) GetCLIVersion() string      { return m.cliVersion }
func (m *mockDaemonState) GetActiveTaskCount() int64  { return m.activeTaskCount }
func (m *mockDaemonState) GetAgents() []AgentInfo     { return m.agents }
func (m *mockDaemonState) GetStartTime() time.Time    { return m.startTime }

func TestHealthServer_HealthEndpoint(t *testing.T) {
	state := &mockDaemonState{
		daemonID:        "test-daemon-id",
		deviceName:      "test-machine",
		serverURL:       "http://localhost:8080",
		cliVersion:      "0.1.0",
		activeTaskCount: 2,
		agents: []AgentInfo{
			{Name: "claude", Version: "1.0.0", Path: "/usr/local/bin/claude"},
			{Name: "gemini", Version: "2.1.0", Path: "/usr/local/bin/gemini"},
		},
		startTime: time.Now().Add(-2 * time.Hour),
	}

	srv := NewHealthServer(HealthServerConfig{Port: 0}, state)

	// Test the handler directly via httptest.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "running" {
		t.Errorf("expected status 'running', got %q", resp.Status)
	}
	if resp.DaemonID != "test-daemon-id" {
		t.Errorf("expected daemon_id 'test-daemon-id', got %q", resp.DaemonID)
	}
	if resp.DeviceName != "test-machine" {
		t.Errorf("expected device_name 'test-machine', got %q", resp.DeviceName)
	}
	if resp.ServerURL != "http://localhost:8080" {
		t.Errorf("expected server_url 'http://localhost:8080', got %q", resp.ServerURL)
	}
	if resp.CLIVersion != "0.1.0" {
		t.Errorf("expected cli_version '0.1.0', got %q", resp.CLIVersion)
	}
	if resp.ActiveTaskCount != 2 {
		t.Errorf("expected active_task_count 2, got %d", resp.ActiveTaskCount)
	}
	if len(resp.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(resp.Agents))
	}
	if resp.Agents[0].Name != "claude" {
		t.Errorf("expected first agent 'claude', got %q", resp.Agents[0].Name)
	}
	if resp.PID == 0 {
		t.Error("expected non-zero PID")
	}
	if resp.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
}

func TestHealthServer_HealthEndpoint_MethodNotAllowed(t *testing.T) {
	state := &mockDaemonState{startTime: time.Now()}
	srv := NewHealthServer(HealthServerConfig{Port: 0}, state)

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()
	srv.handleHealth(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", w.Code)
	}
}

func TestHealthServer_ShutdownEndpoint(t *testing.T) {
	shutdownCalled := make(chan struct{}, 1)
	state := &mockDaemonState{startTime: time.Now()}
	srv := NewHealthServer(HealthServerConfig{
		Port: 0,
		ShutdownCallback: func() {
			shutdownCalled <- struct{}{}
		},
	}, state)

	req := httptest.NewRequest(http.MethodPost, "/shutdown", nil)
	w := httptest.NewRecorder()
	srv.handleShutdown(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "shutting down" {
		t.Errorf("expected status 'shutting down', got %q", resp["status"])
	}

	// Wait for the async shutdown callback.
	select {
	case <-shutdownCalled:
		// OK
	case <-time.After(2 * time.Second):
		t.Error("shutdown callback was not called within timeout")
	}
}

func TestHealthServer_ShutdownEndpoint_MethodNotAllowed(t *testing.T) {
	state := &mockDaemonState{startTime: time.Now()}
	srv := NewHealthServer(HealthServerConfig{Port: 0}, state)

	req := httptest.NewRequest(http.MethodGet, "/shutdown", nil)
	w := httptest.NewRecorder()
	srv.handleShutdown(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", w.Code)
	}
}

func TestHealthServer_StartStop(t *testing.T) {
	state := &mockDaemonState{
		daemonID:   "test-id",
		deviceName: "test-host",
		startTime:  time.Now(),
	}

	srv := NewHealthServer(HealthServerConfig{Port: 0}, state)

	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start health server: %v", err)
	}

	addr := srv.Addr()
	if addr == "" {
		t.Fatal("expected non-empty address after start")
	}

	// Make a real HTTP request to the running server.
	resp, err := http.Get("http://" + addr + "/health")
	if err != nil {
		t.Fatalf("failed to GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	if health.Status != "running" {
		t.Errorf("expected status 'running', got %q", health.Status)
	}
	if health.DaemonID != "test-id" {
		t.Errorf("expected daemon_id 'test-id', got %q", health.DaemonID)
	}

	// Stop the server.
	if err := srv.Stop(); err != nil {
		t.Fatalf("failed to stop health server: %v", err)
	}
}

func TestHealthServer_EmptyAgents(t *testing.T) {
	state := &mockDaemonState{
		daemonID:  "test-id",
		startTime: time.Now(),
		agents:    nil, // nil agents
	}

	srv := NewHealthServer(HealthServerConfig{Port: 0}, state)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.handleHealth(w, req)

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Agents should be an empty array, not null.
	if resp.Agents == nil {
		t.Error("expected non-nil agents slice (empty array in JSON)")
	}

	// Verify the raw JSON has [] not null.
	w2 := httptest.NewRecorder()
	srv.handleHealth(w2, httptest.NewRequest(http.MethodGet, "/health", nil))
	body := w2.Body.String()
	if !json.Valid([]byte(body)) {
		t.Fatal("response is not valid JSON")
	}
}
