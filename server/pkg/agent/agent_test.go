package agent

import (
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// Feature: agentic-output-architecture, Property 1: Backend Factory Correctness
//
// For any string s, if s is in the set of supported agent types then
// agent.New(s, cfg) SHALL return a non-nil Backend and nil error; if s is NOT
// in the supported set then agent.New(s, cfg) SHALL return a nil Backend and
// an error whose message contains all supported type names.
//
// **Validates: Requirements 1.5, 1.6**

func TestProperty_BackendFactoryCorrectness_SupportedTypes(t *testing.T) {
	t.Parallel()

	cfg := Config{}

	rapid.Check(t, func(t *rapid.T) {
		// Draw a random supported type from the SupportedTypes slice.
		idx := rapid.IntRange(0, len(SupportedTypes)-1).Draw(t, "typeIndex")
		agentType := SupportedTypes[idx]

		backend, err := New(agentType, cfg)

		if err != nil {
			t.Fatalf("New(%q) returned unexpected error: %v", agentType, err)
		}
		if backend == nil {
			t.Fatalf("New(%q) returned nil Backend with nil error", agentType)
		}
	})
}

func TestProperty_BackendFactoryCorrectness_UnsupportedTypes(t *testing.T) {
	t.Parallel()

	cfg := Config{}

	// Build a set of supported types for quick lookup.
	supported := make(map[string]bool, len(SupportedTypes))
	for _, st := range SupportedTypes {
		supported[st] = true
	}

	rapid.Check(t, func(t *rapid.T) {
		// Generate a random string that is NOT in the supported set.
		agentType := rapid.StringMatching(`[a-z0-9_-]{1,30}`).
			Filter(func(s string) bool { return !supported[s] }).
			Draw(t, "unsupportedType")

		backend, err := New(agentType, cfg)

		if backend != nil {
			t.Fatalf("New(%q) returned non-nil Backend for unsupported type", agentType)
		}
		if err == nil {
			t.Fatalf("New(%q) returned nil error for unsupported type", agentType)
		}

		// The error message must list all supported types.
		errMsg := err.Error()
		for _, st := range SupportedTypes {
			if !strings.Contains(errMsg, st) {
				t.Fatalf("New(%q) error message %q does not contain supported type %q",
					agentType, errMsg, st)
			}
		}
	})
}
