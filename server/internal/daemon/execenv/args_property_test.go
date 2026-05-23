package execenv

import (
	"testing"

	"pgregory.net/rapid"
)

// Feature: agent-management, Property 15: Custom Args Ordering
// For any set of daemon-wide default arguments and agent custom_args, the final
// CLI invocation SHALL have daemon defaults appearing before custom args in the
// argument list.
//
// **Validates: Requirements 14.3**

// argGen generates a non-empty argument string (1-256 chars, printable ASCII without whitespace).
func argGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z0-9_./-]{1,64}`)
}

// argSliceGen generates a slice of 0-20 argument strings.
func argSliceGen() *rapid.Generator[[]string] {
	return rapid.SliceOfN(argGen(), 0, 20)
}

func TestProperty15_DaemonDefaultsAppearBeforeAgentArgs(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		daemonDefaults := argSliceGen().Draw(t, "daemonDefaults")
		agentArgs := argSliceGen().Draw(t, "agentArgs")

		result := MergeArgs(daemonDefaults, agentArgs)

		// Property: Daemon defaults appear at the beginning of the merged result.
		// The first len(daemonDefaults) elements must be exactly daemonDefaults.
		if len(daemonDefaults) > 0 {
			if len(result) < len(daemonDefaults) {
				t.Fatalf("result length %d is less than daemon defaults length %d",
					len(result), len(daemonDefaults))
			}
			for i, arg := range daemonDefaults {
				if result[i] != arg {
					t.Fatalf("result[%d] = %q, want daemon default %q",
						i, result[i], arg)
				}
			}
		}

		// Property: Agent custom_args appear after daemon defaults.
		// Elements at positions [len(daemonDefaults)..] must be exactly agentArgs.
		if len(agentArgs) > 0 {
			offset := len(daemonDefaults)
			if len(result) < offset+len(agentArgs) {
				t.Fatalf("result length %d is less than expected %d (daemon %d + agent %d)",
					len(result), offset+len(agentArgs), len(daemonDefaults), len(agentArgs))
			}
			for i, arg := range agentArgs {
				if result[offset+i] != arg {
					t.Fatalf("result[%d] = %q, want agent arg %q",
						offset+i, result[offset+i], arg)
				}
			}
		}
	})
}

func TestProperty15_RelativeOrderWithinDaemonDefaultsPreserved(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		daemonDefaults := rapid.SliceOfN(argGen(), 2, 20).Draw(t, "daemonDefaults")
		agentArgs := argSliceGen().Draw(t, "agentArgs")

		result := MergeArgs(daemonDefaults, agentArgs)

		// Property: The relative order within daemon defaults is preserved.
		// For any i < j in daemonDefaults, result must have daemonDefaults[i]
		// appearing before daemonDefaults[j] at their respective positions.
		for i := 0; i < len(daemonDefaults)-1; i++ {
			if result[i] != daemonDefaults[i] {
				t.Fatalf("daemon default order violated at position %d: got %q, want %q",
					i, result[i], daemonDefaults[i])
			}
		}
	})
}

func TestProperty15_RelativeOrderWithinAgentArgsPreserved(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		daemonDefaults := argSliceGen().Draw(t, "daemonDefaults")
		agentArgs := rapid.SliceOfN(argGen(), 2, 20).Draw(t, "agentArgs")

		result := MergeArgs(daemonDefaults, agentArgs)

		// Property: The relative order within agent custom_args is preserved.
		offset := len(daemonDefaults)
		for i := 0; i < len(agentArgs); i++ {
			if result[offset+i] != agentArgs[i] {
				t.Fatalf("agent args order violated at position %d: got %q, want %q",
					offset+i, result[offset+i], agentArgs[i])
			}
		}
	})
}

func TestProperty15_TotalLengthIsSum(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		daemonDefaults := argSliceGen().Draw(t, "daemonDefaults")
		agentArgs := argSliceGen().Draw(t, "agentArgs")

		result := MergeArgs(daemonDefaults, agentArgs)

		expectedLen := len(daemonDefaults) + len(agentArgs)

		// Handle the nil case: both empty → nil result
		if expectedLen == 0 {
			if result != nil {
				t.Fatalf("expected nil result for both empty inputs, got %v", result)
			}
			return
		}

		if len(result) != expectedLen {
			t.Fatalf("result length = %d, want %d (daemon %d + agent %d)",
				len(result), expectedLen, len(daemonDefaults), len(agentArgs))
		}
	})
}

func TestProperty15_EmptyDaemonDefaultsWithAgentArgs(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		agentArgs := rapid.SliceOfN(argGen(), 1, 20).Draw(t, "agentArgs")

		result := MergeArgs(nil, agentArgs)

		// Property: Empty daemon defaults + non-empty agent args = just agent args.
		if len(result) != len(agentArgs) {
			t.Fatalf("result length = %d, want %d", len(result), len(agentArgs))
		}
		for i, arg := range agentArgs {
			if result[i] != arg {
				t.Fatalf("result[%d] = %q, want %q", i, result[i], arg)
			}
		}
	})
}

func TestProperty15_DaemonDefaultsWithEmptyAgentArgs(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		daemonDefaults := rapid.SliceOfN(argGen(), 1, 20).Draw(t, "daemonDefaults")

		result := MergeArgs(daemonDefaults, nil)

		// Property: Non-empty daemon defaults + empty agent args = just daemon defaults.
		if len(result) != len(daemonDefaults) {
			t.Fatalf("result length = %d, want %d", len(result), len(daemonDefaults))
		}
		for i, arg := range daemonDefaults {
			if result[i] != arg {
				t.Fatalf("result[%d] = %q, want %q", i, result[i], arg)
			}
		}
	})
}

func TestProperty15_BothEmpty(t *testing.T) {
	// Property: Both empty = nil result.
	result := MergeArgs(nil, nil)
	if result != nil {
		t.Fatalf("expected nil for both empty inputs, got %v", result)
	}

	result = MergeArgs([]string{}, []string{})
	if result != nil {
		t.Fatalf("expected nil for both empty slices, got %v", result)
	}

	result = MergeArgs(nil, []string{})
	if result != nil {
		t.Fatalf("expected nil for nil + empty slice, got %v", result)
	}

	result = MergeArgs([]string{}, nil)
	if result != nil {
		t.Fatalf("expected nil for empty slice + nil, got %v", result)
	}
}
