package daemon

import (
	"context"
	"log/slog"
	"strings"
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
