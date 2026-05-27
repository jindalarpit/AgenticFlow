package execenv

import (
	"log/slog"
	"os"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// Feature: agent-management, Property 11: Environment Variable Resolution with Blocked Keys
//
// For any set of daemon-level, agent-level, and task-level environment variables
// (potentially with overlapping keys), the resolved environment SHALL contain:
// all daemon-level vars, all non-blocked agent-level vars (with agent values
// overriding daemon for duplicates), and all non-blocked task-level vars (with
// task values overriding agent for duplicates). Keys matching HOME, PATH, USER,
// SHELL, TERM, or any AF_ prefix SHALL never appear from agent or task levels.
//
// **Validates: Requirements 14.1, 14.2, 14.5**

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
}

// genSafeEnvKey generates a non-blocked environment variable key.
func genSafeEnvKey() *rapid.Generator[string] {
	return rapid.Custom[string](func(t *rapid.T) string {
		for {
			key := rapid.StringMatching(`[A-Z][A-Z0-9_]{0,15}`).Draw(t, "safeKey")
			if !IsBlockedEnvKey(key) {
				return key
			}
		}
	})
}

// genBlockedEnvKey generates a key that is guaranteed to be blocked.
func genBlockedEnvKey() *rapid.Generator[string] {
	return rapid.Custom[string](func(t *rapid.T) string {
		choice := rapid.IntRange(0, 1).Draw(t, "blockedChoice")
		if choice == 0 {
			// Pick from the exact blocked keys.
			return rapid.SampledFrom([]string{"HOME", "PATH", "USER", "SHELL", "TERM"}).Draw(t, "exactBlocked")
		}
		// Generate an AF_ prefixed key.
		suffix := rapid.StringMatching(`[A-Z0-9_]{1,10}`).Draw(t, "afSuffix")
		return "AF_" + suffix
	})
}

// genEnvValue generates a random environment variable value.
func genEnvValue() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z0-9/._-]{1,50}`)
}

// genEnvMap generates a map of environment variables with safe (non-blocked) keys.
func genSafeEnvMap(maxSize int) *rapid.Generator[map[string]string] {
	return rapid.Custom[map[string]string](func(t *rapid.T) map[string]string {
		size := rapid.IntRange(0, maxSize).Draw(t, "mapSize")
		m := make(map[string]string, size)
		for i := 0; i < size; i++ {
			key := genSafeEnvKey().Draw(t, "key")
			val := genEnvValue().Draw(t, "val")
			m[key] = val
		}
		return m
	})
}

// TestProperty11_DaemonEnvAlwaysAppears verifies that all daemon-level
// environment variables always appear in the merged result, regardless of
// whether they are "blocked" keys or not (daemon is trusted).
func TestProperty11_DaemonEnvAlwaysAppears(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate daemon env with a mix of safe and blocked keys.
		daemonEnv := make(map[string]string)
		numSafe := rapid.IntRange(0, 5).Draw(t, "numSafeDaemon")
		for i := 0; i < numSafe; i++ {
			key := genSafeEnvKey().Draw(t, "daemonSafeKey")
			val := genEnvValue().Draw(t, "daemonSafeVal")
			daemonEnv[key] = val
		}
		numBlocked := rapid.IntRange(0, 3).Draw(t, "numBlockedDaemon")
		for i := 0; i < numBlocked; i++ {
			key := genBlockedEnvKey().Draw(t, "daemonBlockedKey")
			val := genEnvValue().Draw(t, "daemonBlockedVal")
			daemonEnv[key] = val
		}

		agentEnv := genSafeEnvMap(5).Draw(t, "agentEnv")
		taskEnv := genSafeEnvMap(5).Draw(t, "taskEnv")

		result := MergeEnv(daemonEnv, agentEnv, taskEnv, discardLogger())

		// Property: Every daemon env key-value pair appears in the result
		// (unless overridden by agent/task for non-blocked keys).
		for k, v := range daemonEnv {
			resultVal, exists := result[k]
			if !exists {
				t.Fatalf("daemon env key %q missing from result", k)
			}
			// If the key is blocked, agent/task can't override it, so daemon value must persist.
			if IsBlockedEnvKey(k) {
				if resultVal != v {
					t.Fatalf("daemon blocked key %q: result=%q, want=%q", k, resultVal, v)
				}
			}
			// If the key is not blocked, it may be overridden by agent/task — that's fine.
		}
	})
}

// TestProperty11_BlockedKeysFromAgentNeverAppear verifies that blocked keys
// from agent-level never appear in the result (unless the daemon also set them).
func TestProperty11_BlockedKeysFromAgentNeverAppear(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		blockedKey := genBlockedEnvKey().Draw(t, "blockedKey")
		agentVal := genEnvValue().Draw(t, "agentVal")

		// Daemon does NOT set this blocked key.
		daemonEnv := map[string]string{}
		agentEnv := map[string]string{blockedKey: agentVal}
		taskEnv := map[string]string{}

		result := MergeEnv(daemonEnv, agentEnv, taskEnv, discardLogger())

		// Property: Blocked key from agent level must NOT appear in result.
		if _, exists := result[blockedKey]; exists {
			t.Fatalf("blocked key %q from agent level appeared in result", blockedKey)
		}
	})
}

// TestProperty11_BlockedKeysFromTaskNeverAppear verifies that blocked keys
// from task-level never appear in the result (unless the daemon set them).
func TestProperty11_BlockedKeysFromTaskNeverAppear(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		blockedKey := genBlockedEnvKey().Draw(t, "blockedKey")
		taskVal := genEnvValue().Draw(t, "taskVal")

		// Daemon does NOT set this blocked key.
		daemonEnv := map[string]string{}
		agentEnv := map[string]string{}
		taskEnv := map[string]string{blockedKey: taskVal}

		result := MergeEnv(daemonEnv, agentEnv, taskEnv, discardLogger())

		// Property: Blocked key from task level must NOT appear in result.
		if _, exists := result[blockedKey]; exists {
			t.Fatalf("blocked key %q from task level appeared in result", blockedKey)
		}
	})
}

// TestProperty11_IsBlockedEnvKey verifies that IsBlockedEnvKey correctly
// identifies blocked keys: HOME, PATH, USER, SHELL, TERM, and any AF_* prefix.
func TestProperty11_IsBlockedEnvKey(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Test exact blocked keys.
		exactBlocked := rapid.SampledFrom([]string{"HOME", "PATH", "USER", "SHELL", "TERM"}).Draw(t, "exactBlocked")
		if !IsBlockedEnvKey(exactBlocked) {
			t.Fatalf("IsBlockedEnvKey(%q) = false, want true", exactBlocked)
		}

		// Test AF_ prefixed keys.
		afSuffix := rapid.StringMatching(`[A-Z0-9_]{0,20}`).Draw(t, "afSuffix")
		afKey := "AF_" + afSuffix
		if !IsBlockedEnvKey(afKey) {
			t.Fatalf("IsBlockedEnvKey(%q) = false, want true", afKey)
		}

		// Test non-blocked keys.
		safeKey := rapid.StringMatching(`[BCDFGIJKLMNOPQRSTVWXYZ][A-Z0-9_]{0,10}`).Draw(t, "safeKey")
		if safeKey != "HOME" && safeKey != "PATH" && safeKey != "USER" &&
			safeKey != "SHELL" && safeKey != "TERM" && !strings.HasPrefix(safeKey, "AF_") {
			if IsBlockedEnvKey(safeKey) {
				t.Fatalf("IsBlockedEnvKey(%q) = true, want false (key should not be blocked)", safeKey)
			}
		}
	})
}
