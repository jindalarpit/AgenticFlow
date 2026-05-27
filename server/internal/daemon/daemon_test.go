package daemon

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

// mockHTTPClient implements HTTPClient for testing.
type mockHTTPClient struct {
	mu             sync.Mutex
	registerCalls  int
	deregisterCalls int
	heartbeatCalls int
	pollCalls      int
	startTaskCalls int
	completeTaskCalls int
	failTaskCalls  int
	reportMsgCalls int
	reportInputStateCalls int

	registerResp *RegisterResponse
	registerErr  error
	deregisterErr error
	heartbeatErr error
	pollResp     *PollResponse
	pollErr      error
	startTaskErr error
	completeTaskErr error
	failTaskErr  error
	reportMsgErr error
	reportInputStateErr error
}

func (m *mockHTTPClient) Register(_ context.Context, _ RegisterRequest) (*RegisterResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registerCalls++
	if m.registerErr != nil {
		return nil, m.registerErr
	}
	if m.registerResp != nil {
		return m.registerResp, nil
	}
	return &RegisterResponse{RuntimeIDs: map[string]string{"claude": "rt-1"}}, nil
}

func (m *mockHTTPClient) Deregister(_ context.Context, _ DeregisterRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deregisterCalls++
	return m.deregisterErr
}

func (m *mockHTTPClient) Heartbeat(_ context.Context, _ HeartbeatRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.heartbeatCalls++
	return m.heartbeatErr
}

func (m *mockHTTPClient) PollTasks(_ context.Context, _ PollRequest) (*PollResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pollCalls++
	if m.pollErr != nil {
		return nil, m.pollErr
	}
	if m.pollResp != nil {
		return m.pollResp, nil
	}
	return &PollResponse{}, nil
}

func (m *mockHTTPClient) StartTask(_ context.Context, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startTaskCalls++
	return m.startTaskErr
}

func (m *mockHTTPClient) CompleteTask(_ context.Context, _ string, _ string, _ int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completeTaskCalls++
	return m.completeTaskErr
}

func (m *mockHTTPClient) FailTask(_ context.Context, _ string, _ string, _ int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failTaskCalls++
	return m.failTaskErr
}

func (m *mockHTTPClient) ReportMessages(_ context.Context, _ string, _ []TaskMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reportMsgCalls++
	return m.reportMsgErr
}

func (m *mockHTTPClient) ReportInputState(_ context.Context, _ string, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reportInputStateCalls++
	return m.reportInputStateErr
}

func (m *mockHTTPClient) ReportStageCompletion(_ context.Context, _ string, _ string, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil
}

func (m *mockHTTPClient) CompleteTaskConversational(_ context.Context, _ string, _ string, _ string, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil
}

func (m *mockHTTPClient) getHeartbeatCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.heartbeatCalls
}

func (m *mockHTTPClient) getRegisterCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.registerCalls
}

func (m *mockHTTPClient) getDeregisterCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.deregisterCalls
}

func testConfig() Config {
	return Config{
		ServerURL:          "http://localhost:8080",
		DaemonID:           "test-daemon-id",
		DeviceName:         "test-machine",
		Agents:             map[string]AgentEntry{"claude": {Name: "claude", Path: "/usr/bin/claude", Version: "1.0.0"}},
		WorkspacesRoot:     os.TempDir(),
		PollInterval:       50 * time.Millisecond,
		HeartbeatInterval:  50 * time.Millisecond,
		AgentTimeout:       2 * time.Hour,
		MaxConcurrentTasks: 5,
	}
}

func TestNew(t *testing.T) {
	cfg := testConfig()
	logger := testLogger()

	d := New(cfg, logger)

	if d.cfg.DaemonID != "test-daemon-id" {
		t.Errorf("expected daemon_id %q, got %q", "test-daemon-id", d.cfg.DaemonID)
	}
	if d.runtimes == nil {
		t.Error("expected runtimes map to be initialized")
	}
	if d.stopCh == nil {
		t.Error("expected stopCh to be initialized")
	}
	if d.done == nil {
		t.Error("expected done to be initialized")
	}
}

func TestDaemonRunAndStop(t *testing.T) {
	cfg := testConfig()
	logger := testLogger()
	client := &mockHTTPClient{}

	d := New(cfg, logger)
	d.SetClient(client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run daemon in background.
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait a bit for loops to start and heartbeat to fire.
	time.Sleep(200 * time.Millisecond)

	// Stop the daemon.
	d.Stop()

	// Wait for Run to return.
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return within 5s after Stop()")
	}

	// Verify registration and deregistration were called.
	if calls := client.getRegisterCalls(); calls < 1 {
		t.Errorf("expected at least 1 register call, got %d", calls)
	}
	if calls := client.getDeregisterCalls(); calls < 1 {
		t.Errorf("expected at least 1 deregister call, got %d", calls)
	}
}

func TestDaemonRunContextCancel(t *testing.T) {
	cfg := testConfig()
	logger := testLogger()
	client := &mockHTTPClient{}

	d := New(cfg, logger)
	d.SetClient(client)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for daemon to start.
	time.Sleep(100 * time.Millisecond)

	// Cancel context to trigger shutdown.
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return within 5s after context cancel")
	}

	// Verify deregistration was called.
	if calls := client.getDeregisterCalls(); calls < 1 {
		t.Errorf("expected at least 1 deregister call, got %d", calls)
	}
}

func TestHeartbeatRetry(t *testing.T) {
	cfg := testConfig()
	cfg.HeartbeatInterval = 50 * time.Millisecond
	logger := testLogger()

	client := &mockHTTPClient{
		heartbeatErr: os.ErrDeadlineExceeded,
	}

	d := New(cfg, logger)
	d.SetClient(client)
	d.heartbeatRetryDelay = 10 * time.Millisecond // Fast retries for testing.

	// Test sendHeartbeat directly to verify retry logic without timing issues.
	ctx := context.Background()
	d.sendHeartbeat(ctx)

	// With 3 retries on failure, we should see exactly 3 heartbeat attempts.
	calls := client.getHeartbeatCalls()
	if calls != 3 {
		t.Errorf("expected exactly 3 heartbeat calls (retries), got %d", calls)
	}
}

func TestPollSkipsWhenAtMaxConcurrency(t *testing.T) {
	cfg := testConfig()
	cfg.MaxConcurrentTasks = 2
	cfg.PollInterval = 50 * time.Millisecond
	logger := testLogger()
	client := &mockHTTPClient{}

	d := New(cfg, logger)
	d.SetClient(client)

	// Simulate max concurrent tasks.
	d.activeTasks.Store(2)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for a few poll cycles.
	time.Sleep(200 * time.Millisecond)
	cancel()
	<-errCh

	// Poll should not have been called since we're at max.
	client.mu.Lock()
	pollCalls := client.pollCalls
	client.mu.Unlock()

	if pollCalls > 0 {
		t.Errorf("expected 0 poll calls when at max concurrency, got %d", pollCalls)
	}
}

func TestActiveTasks(t *testing.T) {
	cfg := testConfig()
	logger := testLogger()

	d := New(cfg, logger)

	if d.ActiveTasks() != 0 {
		t.Errorf("expected 0 active tasks, got %d", d.ActiveTasks())
	}

	d.activeTasks.Store(3)
	if d.ActiveTasks() != 3 {
		t.Errorf("expected 3 active tasks, got %d", d.ActiveTasks())
	}
}

func TestRuntimeIDs(t *testing.T) {
	cfg := testConfig()
	logger := testLogger()

	d := New(cfg, logger)
	d.runtimes["rt-1"] = "claude"
	d.runtimes["rt-2"] = "gemini"

	ids := d.RuntimeIDs()
	if len(ids) != 2 {
		t.Errorf("expected 2 runtime IDs, got %d", len(ids))
	}
}

func TestPIDFileRoundTrip(t *testing.T) {
	cfg := testConfig()
	logger := testLogger()
	d := New(cfg, logger)

	// Write PID file.
	if err := d.WritePIDFile(); err != nil {
		t.Fatalf("WritePIDFile failed: %v", err)
	}

	// Read PID file.
	pid, err := ReadPIDFile()
	if err != nil {
		t.Fatalf("ReadPIDFile failed: %v", err)
	}
	if pid != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), pid)
	}

	// Remove PID file.
	if err := d.RemovePIDFile(); err != nil {
		t.Fatalf("RemovePIDFile failed: %v", err)
	}

	// Verify it's gone.
	pid, err = ReadPIDFile()
	if err != nil {
		t.Fatalf("ReadPIDFile after remove failed: %v", err)
	}
	if pid != 0 {
		t.Errorf("expected PID 0 after removal, got %d", pid)
	}
}

func TestCleanStalePIDFile(t *testing.T) {
	cfg := testConfig()
	logger := testLogger()
	d := New(cfg, logger)

	// Write a PID file with a non-existent PID.
	path, err := pidFilePath()
	if err != nil {
		t.Fatal(err)
	}
	dir := path[:len(path)-len("daemon.pid")]
	os.MkdirAll(dir, 0o755)
	// Use a very high PID that's unlikely to exist.
	os.WriteFile(path, []byte("999999999"), 0o644)

	// CleanStalePIDFile should remove it since the process doesn't exist.
	err = d.CleanStalePIDFile()
	if err != nil {
		t.Fatalf("CleanStalePIDFile failed: %v", err)
	}

	// Verify it's gone.
	pid, err := ReadPIDFile()
	if err != nil {
		t.Fatalf("ReadPIDFile failed: %v", err)
	}
	if pid != 0 {
		t.Errorf("expected PID 0 after cleaning stale file, got %d", pid)
	}
}

func TestStopIdempotent(t *testing.T) {
	cfg := testConfig()
	logger := testLogger()
	client := &mockHTTPClient{}

	d := New(cfg, logger)
	d.SetClient(client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Call Stop multiple times — should not panic.
	d.Stop()
	d.Stop()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}
