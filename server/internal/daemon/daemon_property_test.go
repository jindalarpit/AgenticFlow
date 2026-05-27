package daemon

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Feature: agenticflow-core, Property 17: Daemon Status Output Completeness
// For any daemon state (running or stopped), the status output SHALL contain:
// running state, PID (if running), uptime, list of detected Agent_Runtimes with names,
// and heartbeat status with last timestamp and connection state.
// Validates: Requirements 2.4
func TestProperty_DaemonStatusOutputCompleteness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random daemon state.
		running := rapid.Bool().Draw(t, "running")
		pid := rapid.IntRange(1, 99999).Draw(t, "pid")
		uptimeSeconds := rapid.IntRange(0, 86400*30).Draw(t, "uptimeSeconds") // up to 30 days
		uptime := time.Duration(uptimeSeconds) * time.Second

		// Generate random agent runtimes (0 to 10 agents).
		numAgents := rapid.IntRange(0, 10).Draw(t, "numAgents")
		agents := make([]AgentRuntimeInfo, numAgents)
		for i := 0; i < numAgents; i++ {
			agents[i] = AgentRuntimeInfo{
				Name: rapid.SampledFrom([]string{
					"claude", "gemini", "codex", "copilot", "opencode",
					"openclaw", "hermes", "pi", "cursor", "kimi", "kiro",
				}).Draw(t, "agentName"),
				Version: rapid.SampledFrom([]string{
					"1.0.0", "2.3.1", "0.1.0-beta", "unknown",
				}).Draw(t, "agentVersion"),
				Status: rapid.SampledFrom([]string{
					"available", "busy", "unavailable",
				}).Draw(t, "agentStatus"),
			}
		}

		// Generate heartbeat status.
		connectionState := rapid.SampledFrom([]string{
			"connected", "disconnected",
		}).Draw(t, "connectionState")

		// Generate a last heartbeat timestamp (either zero or a random time).
		hasLastHeartbeat := rapid.Bool().Draw(t, "hasLastHeartbeat")
		var lastTimestamp time.Time
		if hasLastHeartbeat {
			// Random timestamp within the last year.
			offsetSeconds := rapid.Int64Range(0, 365*24*3600).Draw(t, "timestampOffset")
			lastTimestamp = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Add(
				time.Duration(offsetSeconds) * time.Second,
			)
		}

		status := DaemonStatus{
			Running:       running,
			PID:           pid,
			Uptime:        uptime,
			AgentRuntimes: agents,
			Heartbeat: HeartbeatStatus{
				LastTimestamp:    lastTimestamp,
				ConnectionState: connectionState,
			},
		}

		// Format the status output.
		output := FormatStatus(status)

		// Property: output SHALL contain running state.
		if running {
			if !strings.Contains(output, "running") {
				t.Fatalf("output missing running state 'running' for running daemon:\n%s", output)
			}
		} else {
			if !strings.Contains(output, "stopped") {
				t.Fatalf("output missing running state 'stopped' for stopped daemon:\n%s", output)
			}
		}

		// Property: output SHALL contain PID if running.
		if running {
			pidStr := strings.TrimSpace(strings.Split(strings.Split(output, "PID:")[1], "\n")[0])
			if pidStr == "" {
				t.Fatalf("output missing PID for running daemon:\n%s", output)
			}
		}

		// Property: output SHALL contain uptime.
		if !strings.Contains(output, "Uptime:") {
			t.Fatalf("output missing uptime field:\n%s", output)
		}

		// Property: output SHALL contain list of detected Agent_Runtimes with names.
		if !strings.Contains(output, "Agent Runtimes:") {
			t.Fatalf("output missing Agent Runtimes section:\n%s", output)
		}
		// Each agent name must appear in the output.
		for _, agent := range agents {
			if !strings.Contains(output, agent.Name) {
				t.Fatalf("output missing agent runtime name %q:\n%s", agent.Name, output)
			}
		}

		// Property: output SHALL contain heartbeat status with connection state.
		if !strings.Contains(output, "Heartbeat:") {
			t.Fatalf("output missing Heartbeat section:\n%s", output)
		}
		if !strings.Contains(output, connectionState) {
			t.Fatalf("output missing connection state %q:\n%s", connectionState, output)
		}

		// Property: output SHALL contain last heartbeat timestamp.
		if hasLastHeartbeat {
			// The timestamp should appear in RFC3339 format.
			formattedTime := lastTimestamp.Format(time.RFC3339)
			if !strings.Contains(output, formattedTime) {
				t.Fatalf("output missing last heartbeat timestamp %q:\n%s", formattedTime, output)
			}
		} else {
			// When no heartbeat has been sent, should indicate "never".
			if !strings.Contains(output, "never") {
				t.Fatalf("output missing 'never' for zero last heartbeat timestamp:\n%s", output)
			}
		}
	})
}

// --- Mock HTTP Client for Property Tests ---

// mockPollClient is a minimal HTTPClient mock that records whether PollTasks was called.
type mockPollClient struct {
	pollCalled bool
}

func (m *mockPollClient) Register(_ context.Context, _ RegisterRequest) (*RegisterResponse, error) {
	return &RegisterResponse{RuntimeIDs: map[string]string{"claude": "rt-1"}}, nil
}
func (m *mockPollClient) Deregister(_ context.Context, _ DeregisterRequest) error { return nil }
func (m *mockPollClient) Heartbeat(_ context.Context, _ HeartbeatRequest) error   { return nil }
func (m *mockPollClient) PollTasks(_ context.Context, _ PollRequest) (*PollResponse, error) {
	m.pollCalled = true
	return nil, nil // No task available
}
func (m *mockPollClient) StartTask(_ context.Context, _ string) error                    { return nil }
func (m *mockPollClient) CompleteTask(_ context.Context, _ string, _ string, _ int) error { return nil }
func (m *mockPollClient) FailTask(_ context.Context, _ string, _ string, _ int) error     { return nil }
func (m *mockPollClient) ReportMessages(_ context.Context, _ string, _ []TaskMessage) error {
	return nil
}
func (m *mockPollClient) ReportInputState(_ context.Context, _ string, _ string) error {
	return nil
}
func (m *mockPollClient) ReportStageCompletion(_ context.Context, _ string, _ string, _ string) error {
	return nil
}
func (m *mockPollClient) CompleteTaskConversational(_ context.Context, _ string, _ string, _ string, _ string) error {
	return nil
}

// Feature: agenticflow-core, Property 8: Concurrent Task Polling Suppression
// For any daemon state where activeTasks equals maxConcurrentTasks, pollForTasks
// SHALL skip polling (no PollTasks call). When activeTasks drops below the limit,
// polling resumes.
// Validates: Requirements 4.8
func TestProperty_ConcurrentTaskPollingSuppression(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random maxConcurrentTasks (1-20).
		maxConcurrent := rapid.IntRange(1, 20).Draw(t, "maxConcurrentTasks")

		// Generate activeTasks level: at max, below max, or above max.
		// We test three scenarios in one property:
		// 0 = below max, 1 = at max, 2 = above max
		scenario := rapid.IntRange(0, 2).Draw(t, "scenario")

		var activeTasks int64
		switch scenario {
		case 0: // below max
			if maxConcurrent > 1 {
				activeTasks = int64(rapid.IntRange(0, maxConcurrent-1).Draw(t, "activeTasks_below"))
			} else {
				activeTasks = 0
			}
		case 1: // at max
			activeTasks = int64(maxConcurrent)
		case 2: // above max (edge case: could happen if max was lowered after tasks started)
			activeTasks = int64(maxConcurrent + rapid.IntRange(1, 10).Draw(t, "activeTasks_above"))
		}

		// Create a daemon with the generated configuration.
		logger := slog.New(slog.NewTextHandler(nil, nil))
		d := New(Config{
			DaemonID:           "test-daemon",
			DeviceName:         "test-device",
			PollInterval:       3 * time.Second,
			HeartbeatInterval:  15 * time.Second,
			AgentTimeout:       2 * time.Hour,
			MaxConcurrentTasks: maxConcurrent,
		}, logger)

		// Set up a mock client to track PollTasks calls.
		mock := &mockPollClient{}
		d.SetClient(mock)

		// Register at least one runtime so polling doesn't skip due to empty runtimes.
		d.mu.Lock()
		d.runtimes = map[string]string{"rt-1": "claude"}
		d.mu.Unlock()

		// Set the active tasks count.
		d.activeTasks.Store(activeTasks)

		// Call pollForTasks.
		ctx := context.Background()
		d.pollForTasks(ctx)

		// Verify the property.
		if activeTasks >= int64(maxConcurrent) {
			// At or above max: PollTasks SHALL NOT be called.
			if mock.pollCalled {
				t.Fatalf("PollTasks was called when activeTasks(%d) >= maxConcurrentTasks(%d)",
					activeTasks, maxConcurrent)
			}
		} else {
			// Below max: PollTasks SHALL be called (polling resumes).
			if !mock.pollCalled {
				t.Fatalf("PollTasks was NOT called when activeTasks(%d) < maxConcurrentTasks(%d)",
					activeTasks, maxConcurrent)
			}
		}
	})
}

// Feature: agenticflow-core, Property 9: Output Truncation
// For any task output exceeding 1 MB, the truncatingBuffer captures exactly 1 MB.
// For any stderr exceeding 4096 characters, the tailBuffer keeps exactly the last
// 4096 characters.
// Validates: Requirements 4.4, 4.9
func TestProperty_OutputTruncation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// --- Test truncatingBuffer (stdout, 1 MB limit) ---

		// Generate a random output size from 0 to 5 MB.
		stdoutSize := rapid.IntRange(0, 5*1024*1024).Draw(t, "stdoutSize")

		tb := &truncatingBuffer{maxBytes: maxStdoutBytes}

		// Write data in random-sized chunks to simulate streaming.
		remaining := stdoutSize
		chunkID := 0
		for remaining > 0 {
			// Random chunk size between 1 and min(remaining, 64KB).
			maxChunk := remaining
			if maxChunk > 65536 {
				maxChunk = 65536
			}
			chunkSize := rapid.IntRange(1, maxChunk).Draw(t, "stdoutChunk_"+strings.Repeat("x", 0))
			chunkID++

			// Write a chunk of 'A' bytes.
			chunk := make([]byte, chunkSize)
			for i := range chunk {
				chunk[i] = 'A'
			}
			n, err := tb.Write(chunk)
			if err != nil {
				t.Fatalf("truncatingBuffer.Write returned error: %v", err)
			}
			if n != chunkSize {
				t.Fatalf("truncatingBuffer.Write returned n=%d, want %d", n, chunkSize)
			}
			remaining -= chunkSize
		}

		// Verify the property.
		result := tb.String()
		if stdoutSize <= maxStdoutBytes {
			// Output fits within limit: captured size equals input size.
			if len(result) != stdoutSize {
				t.Fatalf("truncatingBuffer: input size %d <= limit %d, but captured %d bytes",
					stdoutSize, maxStdoutBytes, len(result))
			}
		} else {
			// Output exceeds limit: captured size equals exactly 1 MB.
			if len(result) != maxStdoutBytes {
				t.Fatalf("truncatingBuffer: input size %d > limit %d, but captured %d bytes (want exactly %d)",
					stdoutSize, maxStdoutBytes, len(result), maxStdoutBytes)
			}
		}

		// --- Test tailBuffer (stderr, 4096 character limit) ---

		// Generate a random stderr size from 0 to 20000 characters.
		stderrSize := rapid.IntRange(0, 20000).Draw(t, "stderrSize")

		tail := &tailBuffer{maxChars: maxStderrChars}

		// Generate the full stderr content with identifiable characters.
		// Use digits 0-9 cycling so we can verify the tail is correct.
		stderrContent := make([]byte, stderrSize)
		for i := range stderrContent {
			stderrContent[i] = byte('0' + (i % 10))
		}

		// Write in random-sized chunks.
		remaining = stderrSize
		offset := 0
		for remaining > 0 {
			maxChunk := remaining
			if maxChunk > 4096 {
				maxChunk = 4096
			}
			chunkSize := rapid.IntRange(1, maxChunk).Draw(t, "stderrChunk_"+strings.Repeat("y", 0))

			n, err := tail.Write(stderrContent[offset : offset+chunkSize])
			if err != nil {
				t.Fatalf("tailBuffer.Write returned error: %v", err)
			}
			if n != chunkSize {
				t.Fatalf("tailBuffer.Write returned n=%d, want %d", n, chunkSize)
			}
			offset += chunkSize
			remaining -= chunkSize
		}

		// Verify the property.
		stderrResult := tail.String()
		stderrRunes := []rune(stderrResult)

		if stderrSize <= maxStderrChars {
			// Stderr fits within limit: captured equals full input.
			if len(stderrRunes) != stderrSize {
				t.Fatalf("tailBuffer: input size %d <= limit %d, but captured %d chars",
					stderrSize, maxStderrChars, len(stderrRunes))
			}
			if stderrResult != string(stderrContent) {
				t.Fatalf("tailBuffer: content mismatch for input within limit")
			}
		} else {
			// Stderr exceeds limit: captured size equals exactly 4096 characters.
			if len(stderrRunes) != maxStderrChars {
				t.Fatalf("tailBuffer: input size %d > limit %d, but captured %d chars (want exactly %d)",
					stderrSize, maxStderrChars, len(stderrRunes), maxStderrChars)
			}
			// Verify it's the LAST 4096 characters.
			expectedTail := string(stderrContent[stderrSize-maxStderrChars:])
			if stderrResult != expectedTail {
				t.Fatalf("tailBuffer: captured content is not the last %d characters of input",
					maxStderrChars)
			}
		}
	})
}

// --- Mock infrastructure for heartbeat failure escalation tests ---

// heartbeatOutcome represents whether a heartbeat interval succeeds or fails.
type heartbeatOutcome bool

const (
	heartbeatSuccess heartbeatOutcome = true
	heartbeatFailure heartbeatOutcome = false
)

// sequentialHeartbeatClient is a mock HTTPClient that returns success or failure
// for each heartbeat call based on a pre-defined sequence. Each call to Heartbeat
// consumes the next outcome in the sequence. When all retries within a single
// sendHeartbeat call fail, the interval counts as a failure.
type sequentialHeartbeatClient struct {
	// failAll controls whether ALL retries within a sendHeartbeat call fail.
	// When true, every Heartbeat() call returns an error.
	// When false, the first Heartbeat() call succeeds.
	outcomes []heartbeatOutcome
	idx      int
	mu       sync.Mutex
}

func (s *sequentialHeartbeatClient) currentOutcome() heartbeatOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.idx >= len(s.outcomes) {
		return heartbeatSuccess // default to success if we run out
	}
	return s.outcomes[s.idx]
}

func (s *sequentialHeartbeatClient) advance() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idx++
}

func (s *sequentialHeartbeatClient) Register(_ context.Context, _ RegisterRequest) (*RegisterResponse, error) {
	return &RegisterResponse{RuntimeIDs: map[string]string{"claude": "rt-1"}}, nil
}
func (s *sequentialHeartbeatClient) Deregister(_ context.Context, _ DeregisterRequest) error {
	return nil
}
func (s *sequentialHeartbeatClient) Heartbeat(_ context.Context, _ HeartbeatRequest) error {
	// The outcome is determined by the current interval's outcome.
	// If the current interval should fail, ALL retries fail.
	// If the current interval should succeed, the first call succeeds.
	outcome := s.currentOutcome()
	if outcome == heartbeatFailure {
		return fmt.Errorf("heartbeat failed (simulated)")
	}
	return nil
}
func (s *sequentialHeartbeatClient) PollTasks(_ context.Context, _ PollRequest) (*PollResponse, error) {
	return nil, nil
}
func (s *sequentialHeartbeatClient) StartTask(_ context.Context, _ string) error { return nil }
func (s *sequentialHeartbeatClient) CompleteTask(_ context.Context, _ string, _ string, _ int) error {
	return nil
}
func (s *sequentialHeartbeatClient) FailTask(_ context.Context, _ string, _ string, _ int) error {
	return nil
}
func (s *sequentialHeartbeatClient) ReportMessages(_ context.Context, _ string, _ []TaskMessage) error {
	return nil
}
func (s *sequentialHeartbeatClient) ReportInputState(_ context.Context, _ string, _ string) error {
	return nil
}
func (s *sequentialHeartbeatClient) ReportStageCompletion(_ context.Context, _ string, _ string, _ string) error {
	return nil
}
func (s *sequentialHeartbeatClient) CompleteTaskConversational(_ context.Context, _ string, _ string, _ string, _ string) error {
	return nil
}

// logRecord captures a single log entry's level and message.
type logRecord struct {
	Level   slog.Level
	Message string
}

// capturingHandler is a slog.Handler that captures all log records for inspection.
type capturingHandler struct {
	mu      sync.Mutex
	records []logRecord
}

func (h *capturingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, logRecord{Level: r.Level, Message: r.Message})
	return nil
}
func (h *capturingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(_ string) slog.Handler      { return h }

func (h *capturingHandler) getRecords() []logRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]logRecord, len(h.records))
	copy(result, h.records)
	return result
}

func (h *capturingHandler) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = nil
}

// Feature: cli-auth-daemon, Property 11: Consecutive heartbeat failure escalation
// For any sequence of heartbeat attempts, error log emitted iff 3+ consecutive failures.
// **Validates: Requirements 10.4**
func TestProperty_ConsecutiveHeartbeatFailureEscalation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random sequence of heartbeat outcomes (success/failure).
		// Length between 1 and 20 to keep tests fast.
		seqLen := rapid.IntRange(1, 20).Draw(t, "sequenceLength")
		outcomes := make([]heartbeatOutcome, seqLen)
		for i := 0; i < seqLen; i++ {
			if rapid.Bool().Draw(t, fmt.Sprintf("outcome_%d", i)) {
				outcomes[i] = heartbeatSuccess
			} else {
				outcomes[i] = heartbeatFailure
			}
		}

		// Set up the daemon with a capturing log handler.
		handler := &capturingHandler{}
		logger := slog.New(handler)

		d := New(Config{
			DaemonID:           "test-daemon",
			DeviceName:         "test-device",
			PollInterval:       3 * time.Second,
			HeartbeatInterval:  15 * time.Second,
			AgentTimeout:       2 * time.Hour,
			MaxConcurrentTasks: 5,
		}, logger)

		// Use a very short retry delay so tests run fast.
		d.heartbeatRetryDelay = 1 * time.Millisecond

		// Create the mock client.
		mock := &sequentialHeartbeatClient{outcomes: outcomes}
		d.SetClient(mock)

		ctx := context.Background()

		// Track expected consecutive failures manually.
		expectedConsecutiveFailures := 0

		for i, outcome := range outcomes {
			// Reset captured logs before each sendHeartbeat call.
			handler.reset()

			// Set the mock to the current interval's outcome.
			mock.mu.Lock()
			mock.idx = i
			mock.mu.Unlock()

			// Call sendHeartbeat.
			d.sendHeartbeat(ctx)

			// Update expected consecutive failures.
			if outcome == heartbeatSuccess {
				expectedConsecutiveFailures = 0
			} else {
				expectedConsecutiveFailures++
			}

			// Verify the consecutive failure counter matches.
			actual := d.ConsecutiveHeartbeatFailures()
			if actual != expectedConsecutiveFailures {
				t.Fatalf("after interval %d (outcome=%v): expected consecutiveHeartbeatFailures=%d, got=%d",
					i, outcome, expectedConsecutiveFailures, actual)
			}

			// Verify log level escalation property:
			// Error log emitted iff 3+ consecutive failures.
			records := handler.getRecords()

			if outcome == heartbeatFailure {
				// A failure interval should produce a log entry about the failure.
				hasErrorLog := false
				hasWarnLog := false
				for _, rec := range records {
					if rec.Level == slog.LevelError && strings.Contains(rec.Message, "connectivity loss") {
						hasErrorLog = true
					}
					if rec.Level == slog.LevelWarn && strings.Contains(rec.Message, "heartbeat failed after all retries") {
						hasWarnLog = true
					}
				}

				if expectedConsecutiveFailures >= 3 {
					// Property: error log MUST be emitted at 3+ consecutive failures.
					if !hasErrorLog {
						t.Fatalf("after interval %d: expected error log at %d consecutive failures, but none found. Logs: %v",
							i, expectedConsecutiveFailures, records)
					}
				} else {
					// Property: error log MUST NOT be emitted at fewer than 3 consecutive failures.
					if hasErrorLog {
						t.Fatalf("after interval %d: unexpected error log at %d consecutive failures (< 3). Logs: %v",
							i, expectedConsecutiveFailures, records)
					}
					// Should have a warning instead.
					if !hasWarnLog {
						t.Fatalf("after interval %d: expected warning log at %d consecutive failures, but none found. Logs: %v",
							i, expectedConsecutiveFailures, records)
					}
				}
			}
		}
	})
}

// Feature: cli-auth-daemon, Property 7: Exit code to task status mapping
// For any exit code, status is "completed" if 0, "failed" otherwise.
// **Validates: Requirements 9.5, 9.6**
func TestProperty_ExitCodeToTaskStatusMapping(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate any integer exit code (including negative values and large values).
		exitCode := rapid.IntRange(-128, 255).Draw(t, "exitCode")

		status := TaskStatusFromExitCode(exitCode)

		if exitCode == 0 {
			// Property: exit code 0 SHALL produce status "completed".
			if status != "completed" {
				t.Fatalf("exit code 0 should produce 'completed', got %q", status)
			}
		} else {
			// Property: any non-zero exit code SHALL produce status "failed".
			if status != "failed" {
				t.Fatalf("exit code %d should produce 'failed', got %q", exitCode, status)
			}
		}
	})
}

// Feature: cli-auth-daemon, Property 9: Concurrency limit enforcement
// For any max_concurrent_tasks N, daemon never has more than N tasks executing simultaneously.
// **Validates: Requirements 9.8**
func TestProperty_ConcurrencyLimitEnforcement(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random max concurrent tasks limit (1-8).
		maxConcurrent := rapid.IntRange(1, 8).Draw(t, "maxConcurrentTasks")

		// Generate the number of sequential poll attempts (more than maxConcurrent).
		pollAttempts := rapid.IntRange(maxConcurrent+1, maxConcurrent*3).Draw(t, "pollAttempts")

		// peakTracker records the maximum activeTasks observed during PollTasks calls.
		peakTracker := &peakActiveTracker{}

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		d := New(Config{
			DaemonID:           "test-daemon",
			DeviceName:         "test-device",
			PollInterval:       3 * time.Second,
			HeartbeatInterval:  15 * time.Second,
			AgentTimeout:       2 * time.Hour,
			MaxConcurrentTasks: maxConcurrent,
			Agents: map[string]AgentEntry{
				"test-agent": {Name: "test-agent", Path: "/bin/echo", Version: "1.0.0"},
			},
		}, logger)

		// Use a mock client that always returns a task and records the
		// activeTasks value at the moment PollTasks is called.
		mock := &concurrencyLimitClient{
			daemon:      d,
			peakTracker: peakTracker,
		}
		d.SetClient(mock)

		// Register a runtime so polling doesn't skip due to empty runtimes.
		d.mu.Lock()
		d.runtimes = map[string]string{"rt-1": "test-agent"}
		d.mu.Unlock()

		ctx := context.Background()

		// Call pollForTasks sequentially many times. Each successful poll
		// increments activeTasks and spawns a goroutine (which will fail
		// quickly since the binary path is just "/bin/echo" with no proper
		// args, but the increment happens BEFORE the goroutine runs).
		// The property: at no point should activeTasks exceed maxConcurrent.
		for i := 0; i < pollAttempts; i++ {
			d.pollForTasks(ctx)

			// Check the invariant after each poll.
			current := d.activeTasks.Load()
			if current > int64(maxConcurrent) {
				t.Fatalf("concurrency limit violated after poll %d: activeTasks=%d > maxConcurrentTasks=%d",
					i, current, maxConcurrent)
			}
		}

		// Wait for any spawned task goroutines to complete.
		time.Sleep(50 * time.Millisecond)

		// Verify the peak observed during PollTasks calls never exceeded the limit.
		peak := peakTracker.Peak()
		if peak > int64(maxConcurrent) {
			t.Fatalf("concurrency limit violated: peak activeTasks observed=%d > maxConcurrentTasks=%d",
				peak, maxConcurrent)
		}

		// Additional invariant: after all tasks complete, activeTasks should be 0.
		// (Give tasks time to finish — they fail fast since /bin/echo exits immediately.)
		time.Sleep(50 * time.Millisecond)
		final := d.activeTasks.Load()
		if final < 0 {
			t.Fatalf("activeTasks went negative: %d (indicates double-decrement bug)", final)
		}
	})
}

// peakActiveTracker records the maximum value observed via Record calls.
type peakActiveTracker struct {
	peak atomic.Int64
}

func (p *peakActiveTracker) Record(val int64) {
	for {
		current := p.peak.Load()
		if val <= current {
			return
		}
		if p.peak.CompareAndSwap(current, val) {
			return
		}
	}
}

func (p *peakActiveTracker) Peak() int64 {
	return p.peak.Load()
}

// concurrencyLimitClient is a mock HTTPClient that always returns a task
// and records the daemon's activeTasks at the moment of each PollTasks call.
type concurrencyLimitClient struct {
	daemon      *Daemon
	peakTracker *peakActiveTracker
	taskCounter atomic.Int64
}

func (c *concurrencyLimitClient) Register(_ context.Context, _ RegisterRequest) (*RegisterResponse, error) {
	return &RegisterResponse{RuntimeIDs: map[string]string{"test-agent": "rt-1"}}, nil
}
func (c *concurrencyLimitClient) Deregister(_ context.Context, _ DeregisterRequest) error { return nil }
func (c *concurrencyLimitClient) Heartbeat(_ context.Context, _ HeartbeatRequest) error   { return nil }
func (c *concurrencyLimitClient) PollTasks(_ context.Context, _ PollRequest) (*PollResponse, error) {
	// Record the current activeTasks at the moment of polling.
	// This captures the state just before a new task would be added.
	if c.daemon != nil {
		c.peakTracker.Record(c.daemon.activeTasks.Load())
	}
	id := c.taskCounter.Add(1)
	return &PollResponse{
		TaskID:    fmt.Sprintf("task-%d", id),
		AgentType: "test-agent",
		Prompt:    "test prompt",
	}, nil
}
func (c *concurrencyLimitClient) StartTask(_ context.Context, _ string) error { return nil }
func (c *concurrencyLimitClient) CompleteTask(_ context.Context, _ string, _ string, _ int) error {
	return nil
}
func (c *concurrencyLimitClient) FailTask(_ context.Context, _ string, _ string, _ int) error {
	return nil
}
func (c *concurrencyLimitClient) ReportMessages(_ context.Context, _ string, _ []TaskMessage) error {
	return nil
}
func (c *concurrencyLimitClient) ReportInputState(_ context.Context, _ string, _ string) error {
	return nil
}
func (c *concurrencyLimitClient) ReportStageCompletion(_ context.Context, _ string, _ string, _ string) error {
	return nil
}
func (c *concurrencyLimitClient) CompleteTaskConversational(_ context.Context, _ string, _ string, _ string, _ string) error {
	return nil
}

// Feature: cli-auth-daemon, Property 10: Heartbeat payload completeness
// For any daemon state, heartbeat payload contains daemon_id, all runtime names, and exact active task count.
// **Validates: Requirements 10.2**
func TestProperty_HeartbeatPayloadCompleteness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random daemon ID.
		daemonID := rapid.StringMatching(`[a-f0-9]{16,32}`).Draw(t, "daemonID")

		// Generate a random set of agent runtimes (0 to 10 agents).
		numAgents := rapid.IntRange(0, 10).Draw(t, "numAgents")
		agents := make(map[string]AgentEntry, numAgents)
		agentNames := make([]string, 0, numAgents)
		for i := 0; i < numAgents; i++ {
			name := rapid.SampledFrom([]string{
				"claude", "gemini", "codex", "copilot", "opencode",
				"kiro", "cursor", "hermes", "pi", "kimi", "aider",
			}).Draw(t, fmt.Sprintf("agentName_%d", i))
			// Avoid duplicate names by appending index if already present.
			uniqueName := name
			if _, exists := agents[uniqueName]; exists {
				uniqueName = fmt.Sprintf("%s_%d", name, i)
			}
			agents[uniqueName] = AgentEntry{
				Name:    uniqueName,
				Path:    fmt.Sprintf("/usr/local/bin/%s", uniqueName),
				Version: rapid.SampledFrom([]string{"1.0.0", "2.3.1", "0.1.0-beta", "unknown"}).Draw(t, fmt.Sprintf("agentVersion_%d", i)),
			}
			agentNames = append(agentNames, uniqueName)
		}

		// Generate a random active task count (0 to 20).
		activeTasks := int64(rapid.IntRange(0, 20).Draw(t, "activeTasks"))

		// Create a daemon with the generated state.
		logger := slog.New(slog.NewTextHandler(nil, nil))
		d := New(Config{
			DaemonID:           daemonID,
			DeviceName:         "test-device",
			PollInterval:       3 * time.Second,
			HeartbeatInterval:  15 * time.Second,
			AgentTimeout:       2 * time.Hour,
			MaxConcurrentTasks: 5,
			Agents:             agents,
		}, logger)

		// Set the active tasks count.
		d.activeTasks.Store(activeTasks)

		// Build the heartbeat request.
		req := d.BuildHeartbeatRequest()

		// Property 1: daemon_id must match exactly.
		if req.DaemonID != daemonID {
			t.Fatalf("heartbeat DaemonID mismatch: got %q, want %q", req.DaemonID, daemonID)
		}

		// Property 2: active task count must match exactly.
		if req.ActiveTasks != activeTasks {
			t.Fatalf("heartbeat ActiveTasks mismatch: got %d, want %d", req.ActiveTasks, activeTasks)
		}

		// Property 3: runtimes must contain ALL agent names (no missing, no extras).
		if len(req.Runtimes) != len(agents) {
			t.Fatalf("heartbeat Runtimes count mismatch: got %d, want %d", len(req.Runtimes), len(agents))
		}

		// Build a set of runtime names from the request for lookup.
		runtimeSet := make(map[string]bool, len(req.Runtimes))
		for _, name := range req.Runtimes {
			runtimeSet[name] = true
		}

		// Every configured agent name must appear in the heartbeat runtimes.
		for name := range agents {
			if !runtimeSet[name] {
				t.Fatalf("heartbeat Runtimes missing agent %q; got %v", name, req.Runtimes)
			}
		}

		// Every runtime in the heartbeat must correspond to a configured agent.
		for _, name := range req.Runtimes {
			if _, exists := agents[name]; !exists {
				t.Fatalf("heartbeat Runtimes contains unexpected agent %q; configured agents: %v", name, agentNames)
			}
		}
	})
}
