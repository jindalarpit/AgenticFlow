package execenv

import (
	"testing"
)

func TestMergeArgs_DaemonDefaultsFirst(t *testing.T) {
	daemon := []string{"--verbose", "--timeout", "30"}
	agent := []string{"--custom-flag", "value"}

	result := MergeArgs(daemon, agent)

	expected := []string{"--verbose", "--timeout", "30", "--custom-flag", "value"}
	if len(result) != len(expected) {
		t.Fatalf("MergeArgs length = %d, want %d", len(result), len(expected))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("MergeArgs[%d] = %q, want %q", i, v, expected[i])
		}
	}
}

func TestMergeArgs_EmptyDaemonDefaults(t *testing.T) {
	agent := []string{"--flag1", "--flag2"}

	result := MergeArgs(nil, agent)

	if len(result) != 2 {
		t.Fatalf("MergeArgs length = %d, want 2", len(result))
	}
	if result[0] != "--flag1" || result[1] != "--flag2" {
		t.Errorf("MergeArgs = %v, want [--flag1 --flag2]", result)
	}
}

func TestMergeArgs_EmptyAgentArgs(t *testing.T) {
	daemon := []string{"--verbose"}

	result := MergeArgs(daemon, nil)

	if len(result) != 1 || result[0] != "--verbose" {
		t.Errorf("MergeArgs = %v, want [--verbose]", result)
	}
}

func TestMergeArgs_BothEmpty(t *testing.T) {
	result := MergeArgs(nil, nil)

	if result != nil {
		t.Errorf("MergeArgs(nil, nil) = %v, want nil", result)
	}
}

func TestMergeArgs_BothEmptySlices(t *testing.T) {
	result := MergeArgs([]string{}, []string{})

	if result != nil {
		t.Errorf("MergeArgs([], []) = %v, want nil", result)
	}
}

func TestMergeArgs_DoesNotMutateInputs(t *testing.T) {
	daemon := []string{"--a", "--b"}
	agent := []string{"--c"}

	daemonCopy := make([]string, len(daemon))
	copy(daemonCopy, daemon)
	agentCopy := make([]string, len(agent))
	copy(agentCopy, agent)

	_ = MergeArgs(daemon, agent)

	for i, v := range daemon {
		if v != daemonCopy[i] {
			t.Errorf("daemon slice was mutated at index %d: got %q, want %q", i, v, daemonCopy[i])
		}
	}
	for i, v := range agent {
		if v != agentCopy[i] {
			t.Errorf("agent slice was mutated at index %d: got %q, want %q", i, v, agentCopy[i])
		}
	}
}

func TestResolveModel_AgentModelTakesPrecedence(t *testing.T) {
	result := ResolveModel("claude-sonnet-4-20250514", "gpt-4")

	if result != "claude-sonnet-4-20250514" {
		t.Errorf("ResolveModel = %q, want %q", result, "claude-sonnet-4-20250514")
	}
}

func TestResolveModel_FallbackToDaemonModel(t *testing.T) {
	result := ResolveModel("", "gpt-4")

	if result != "gpt-4" {
		t.Errorf("ResolveModel = %q, want %q", result, "gpt-4")
	}
}

func TestResolveModel_BothEmpty(t *testing.T) {
	result := ResolveModel("", "")

	if result != "" {
		t.Errorf("ResolveModel = %q, want empty string", result)
	}
}

func TestResolveModel_AgentModelNonEmpty_DaemonEmpty(t *testing.T) {
	result := ResolveModel("my-model", "")

	if result != "my-model" {
		t.Errorf("ResolveModel = %q, want %q", result, "my-model")
	}
}
