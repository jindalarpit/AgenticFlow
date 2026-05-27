package agent

import (
	"testing"
)

func TestSupportedTypes(t *testing.T) {
	types := SupportedTypes()
	if len(types) == 0 {
		t.Fatal("SupportedTypes() should not be empty")
	}

	expected := []string{"claude", "gemini", "codex", "copilot", "kiro", "opencode", "hermes", "kimi", "cursor", "pi", "openclaw"}
	if len(types) != len(expected) {
		t.Errorf("SupportedTypes() length: got %d, want %d", len(types), len(expected))
	}

	for _, e := range expected {
		found := false
		for _, typ := range types {
			if typ == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in SupportedTypes()", e)
		}
	}
}

func TestLookupByType(t *testing.T) {
	tests := []struct {
		agentType string
		wantNil   bool
		wantName  string
	}{
		{"claude", false, "Claude Code"},
		{"gemini", false, "Gemini CLI"},
		{"codex", false, "OpenAI Codex"},
		{"nonexistent", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.agentType, func(t *testing.T) {
			def := LookupByType(tt.agentType)
			if tt.wantNil {
				if def != nil {
					t.Errorf("LookupByType(%q) should return nil", tt.agentType)
				}
				return
			}
			if def == nil {
				t.Fatalf("LookupByType(%q) returned nil", tt.agentType)
			}
			if def.DisplayName != tt.wantName {
				t.Errorf("DisplayName: got %q, want %q", def.DisplayName, tt.wantName)
			}
		})
	}
}

func TestLookupByCLIName(t *testing.T) {
	def := LookupByCLIName("claude")
	if def == nil {
		t.Fatal("LookupByCLIName(\"claude\") returned nil")
	}
	if def.Type != "claude" {
		t.Errorf("Type: got %q, want %q", def.Type, "claude")
	}
	if def.DefaultModel != "claude-sonnet-4-20250514" {
		t.Errorf("DefaultModel: got %q, want %q", def.DefaultModel, "claude-sonnet-4-20250514")
	}

	// Non-existent CLI name
	def = LookupByCLIName("nonexistent")
	if def != nil {
		t.Error("LookupByCLIName(\"nonexistent\") should return nil")
	}
}

func TestAgentDefFields(t *testing.T) {
	for _, agent := range SupportedAgents {
		if agent.Type == "" {
			t.Error("agent Type should not be empty")
		}
		if agent.CLIName == "" {
			t.Errorf("agent %q CLIName should not be empty", agent.Type)
		}
		if agent.DisplayName == "" {
			t.Errorf("agent %q DisplayName should not be empty", agent.Type)
		}
		if agent.DefaultModel == "" {
			t.Errorf("agent %q DefaultModel should not be empty", agent.Type)
		}
		if agent.VersionFlag == "" {
			t.Errorf("agent %q VersionFlag should not be empty", agent.Type)
		}
	}
}
