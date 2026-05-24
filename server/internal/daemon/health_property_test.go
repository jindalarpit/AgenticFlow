package daemon

import (
	"encoding/json"
	"testing"

	"pgregory.net/rapid"
)

// Feature: cli-auth-daemon, Property 14: Health response JSON round-trip
// For any valid HealthResponse struct, marshal/unmarshal produces equivalent struct.
// Validates: Requirements 7.4
func TestProperty_HealthResponseJSONRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random list of agents (0 to 10).
		numAgents := rapid.IntRange(0, 10).Draw(t, "numAgents")
		agents := make([]AgentInfo, numAgents)
		for i := 0; i < numAgents; i++ {
			agents[i] = AgentInfo{
				Name: rapid.StringMatching(`[a-z][a-z0-9\-]{0,19}`).Draw(t, "agentName"),
				Version: rapid.SampledFrom([]string{
					"1.0.0", "2.3.1", "0.1.0-beta", "unknown", "3.14.159",
				}).Draw(t, "agentVersion"),
				Path: rapid.SampledFrom([]string{
					"/usr/local/bin/claude",
					"/usr/bin/gemini",
					"/home/user/.local/bin/opencode",
					"C:\\Program Files\\codex\\codex.exe",
					"/opt/homebrew/bin/kiro",
				}).Draw(t, "agentPath"),
			}
		}

		// Generate a random HealthResponse.
		original := HealthResponse{
			Status: rapid.SampledFrom([]string{
				"running", "shutting_down",
			}).Draw(t, "status"),
			PID:    rapid.IntRange(1, 99999).Draw(t, "pid"),
			Uptime: rapid.SampledFrom([]string{
				"0s", "5m30s", "2h15m30s", "24h0m0s", "720h0m0s",
			}).Draw(t, "uptime"),
			DaemonID: rapid.StringMatching(`[a-f0-9]{8,32}`).Draw(t, "daemonID"),
			DeviceName: rapid.SampledFrom([]string{
				"macbook-pro", "dev-server", "ci-runner-01", "my-desktop",
			}).Draw(t, "deviceName"),
			ServerURL: rapid.SampledFrom([]string{
				"http://localhost:8080",
				"https://agenticflow.example.com",
				"http://192.168.1.100:3000",
			}).Draw(t, "serverURL"),
			CLIVersion: rapid.SampledFrom([]string{
				"0.1.0", "1.0.0-rc1", "2.5.3", "dev",
			}).Draw(t, "cliVersion"),
			ActiveTaskCount: int64(rapid.IntRange(0, 20).Draw(t, "activeTaskCount")),
			Agents:          agents,
		}

		// Marshal to JSON.
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal HealthResponse: %v", err)
		}

		// Unmarshal back.
		var restored HealthResponse
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("failed to unmarshal HealthResponse: %v", err)
		}

		// Assert all fields are equal.
		if restored.Status != original.Status {
			t.Fatalf("Status mismatch: got %q, want %q", restored.Status, original.Status)
		}
		if restored.PID != original.PID {
			t.Fatalf("PID mismatch: got %d, want %d", restored.PID, original.PID)
		}
		if restored.Uptime != original.Uptime {
			t.Fatalf("Uptime mismatch: got %q, want %q", restored.Uptime, original.Uptime)
		}
		if restored.DaemonID != original.DaemonID {
			t.Fatalf("DaemonID mismatch: got %q, want %q", restored.DaemonID, original.DaemonID)
		}
		if restored.DeviceName != original.DeviceName {
			t.Fatalf("DeviceName mismatch: got %q, want %q", restored.DeviceName, original.DeviceName)
		}
		if restored.ServerURL != original.ServerURL {
			t.Fatalf("ServerURL mismatch: got %q, want %q", restored.ServerURL, original.ServerURL)
		}
		if restored.CLIVersion != original.CLIVersion {
			t.Fatalf("CLIVersion mismatch: got %q, want %q", restored.CLIVersion, original.CLIVersion)
		}
		if restored.ActiveTaskCount != original.ActiveTaskCount {
			t.Fatalf("ActiveTaskCount mismatch: got %d, want %d", restored.ActiveTaskCount, original.ActiveTaskCount)
		}

		// Assert agents list.
		if len(restored.Agents) != len(original.Agents) {
			t.Fatalf("Agents length mismatch: got %d, want %d", len(restored.Agents), len(original.Agents))
		}
		for i, agent := range original.Agents {
			if restored.Agents[i].Name != agent.Name {
				t.Fatalf("Agents[%d].Name mismatch: got %q, want %q", i, restored.Agents[i].Name, agent.Name)
			}
			if restored.Agents[i].Version != agent.Version {
				t.Fatalf("Agents[%d].Version mismatch: got %q, want %q", i, restored.Agents[i].Version, agent.Version)
			}
			if restored.Agents[i].Path != agent.Path {
				t.Fatalf("Agents[%d].Path mismatch: got %q, want %q", i, restored.Agents[i].Path, agent.Path)
			}
		}
	})
}
