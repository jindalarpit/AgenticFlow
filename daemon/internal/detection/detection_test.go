package detection

import (
	"testing"
)

func TestKnownAgentsNotEmpty(t *testing.T) {
	if len(KnownAgents) == 0 {
		t.Fatal("KnownAgents should not be empty")
	}
}

func TestKnownAgentsContainsExpected(t *testing.T) {
	expected := []string{"claude", "gemini", "codex", "copilot", "kiro"}
	for _, name := range expected {
		found := false
		for _, known := range KnownAgents {
			if known == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in KnownAgents", name)
		}
	}
}

func TestDefaultModelForAgent(t *testing.T) {
	tests := []struct {
		agent string
		want  string
	}{
		{"claude", "claude-sonnet-4-20250514"},
		{"gemini", "gemini-2.5-pro"},
		{"codex", "o3"},
		{"copilot", "gpt-4o"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			got := defaultModelForAgent(tt.agent)
			if got != tt.want {
				t.Errorf("defaultModelForAgent(%q) = %q, want %q", tt.agent, got, tt.want)
			}
		})
	}
}

func TestAgentBinaryName(t *testing.T) {
	name := agentBinaryName("claude")
	// On non-Windows, should just be "claude"
	if name != "claude" && name != "claude.exe" {
		t.Errorf("unexpected binary name: %q", name)
	}
}

func TestDetectAgents_ReturnsMap(t *testing.T) {
	// DetectAgents should always return a non-nil map, even if no agents are found
	agents := DetectAgents()
	if agents == nil {
		t.Fatal("DetectAgents() should return non-nil map")
	}
	// We can't assert specific agents are found since it depends on the test environment
}
