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

// TestProperty11_AgentOverridesDaemon verifies that agent-level env vars
// override daemon vars for the same non-blocked key.
func TestProperty11_AgentOverridesDaemon(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a shared non-blocked key.
		sharedKey := genSafeEnvKey().Draw(t, "sharedKey")
		daemonVal := genEnvValue().Draw(t, "daemonVal")
		agentVal := genEnvValue().Draw(t, "agentVal")

		daemonEnv := map[string]string{sharedKey: daemonVal}
		agentEnv := map[string]string{sharedKey: agentVal}
		taskEnv := map[string]string{}

		result := MergeEnv(daemonEnv, agentEnv, taskEnv, discardLogger())

		// Property: Agent value overrides daemon value for non-blocked keys.
		if result[sharedKey] != agentVal {
			t.Fatalf("key %q: result=%q, want agent value=%q (daemon was %q)",
				sharedKey, result[sharedKey], agentVal, daemonVal)
		}
	})
}

// TestProperty11_TaskOverridesAgent verifies that task-level env vars
// override agent vars for the same non-blocked key.
func TestProperty11_TaskOverridesAgent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a shared non-blocked key.
		sharedKey := genSafeEnvKey().Draw(t, "sharedKey")
		daemonVal := genEnvValue().Draw(t, "daemonVal")
		agentVal := genEnvValue().Draw(t, "agentVal")
		taskVal := genEnvValue().Draw(t, "taskVal")

		daemonEnv := map[string]string{sharedKey: daemonVal}
		agentEnv := map[string]string{sharedKey: agentVal}
		taskEnv := map[string]string{sharedKey: taskVal}

		result := MergeEnv(daemonEnv, agentEnv, taskEnv, discardLogger())

		// Property: Task value overrides both agent and daemon for non-blocked keys.
		if result[sharedKey] != taskVal {
			t.Fatalf("key %q: result=%q, want task value=%q (agent was %q, daemon was %q)",
				sharedKey, result[sharedKey], taskVal, agentVal, daemonVal)
		}
	})
}

// TestProperty11_BlockedKeysFromAgentNeverAppear verifies that blocked keys
// (HOME, PATH, USER, SHELL, TERM, AF_*) from agent-level never appear in the
// result (unless the daemon also set them).
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

// TestProperty11_DaemonBlockedKeysDoAppear verifies that daemon-level blocked
// keys DO appear in the result (daemon is trusted).
func TestProperty11_DaemonBlockedKeysDoAppear(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		blockedKey := genBlockedEnvKey().Draw(t, "blockedKey")
		daemonVal := genEnvValue().Draw(t, "daemonVal")

		daemonEnv := map[string]string{blockedKey: daemonVal}
		agentEnv := map[string]string{}
		taskEnv := map[string]string{}

		result := MergeEnv(daemonEnv, agentEnv, taskEnv, discardLogger())

		// Property: Daemon-level blocked keys MUST appear in result.
		resultVal, exists := result[blockedKey]
		if !exists {
			t.Fatalf("daemon blocked key %q missing from result", blockedKey)
		}
		if resultVal != daemonVal {
			t.Fatalf("daemon blocked key %q: result=%q, want=%q", blockedKey, resultVal, daemonVal)
		}
	})
}

// TestProperty11_DaemonBlockedKeysNotOverriddenByAgentOrTask verifies that
// when the daemon sets a blocked key, agent/task attempts to override it are
// silently ignored, preserving the daemon's value.
func TestProperty11_DaemonBlockedKeysNotOverriddenByAgentOrTask(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		blockedKey := genBlockedEnvKey().Draw(t, "blockedKey")
		daemonVal := genEnvValue().Draw(t, "daemonVal")
		agentVal := genEnvValue().Draw(t, "agentVal")
		taskVal := genEnvValue().Draw(t, "taskVal")

		daemonEnv := map[string]string{blockedKey: daemonVal}
		agentEnv := map[string]string{blockedKey: agentVal}
		taskEnv := map[string]string{blockedKey: taskVal}

		result := MergeEnv(daemonEnv, agentEnv, taskEnv, discardLogger())

		// Property: Daemon's blocked key value is preserved (agent/task can't override).
		if result[blockedKey] != daemonVal {
			t.Fatalf("blocked key %q: result=%q, want daemon value=%q (agent=%q, task=%q)",
				blockedKey, result[blockedKey], daemonVal, agentVal, taskVal)
		}
	})
}

// TestProperty11_NonBlockedKeysFromAllLevelsAppear verifies that non-blocked
// keys from all three levels appear correctly in the merged result with proper
// precedence (task > agent > daemon).
func TestProperty11_NonBlockedKeysFromAllLevelsAppear(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate non-blocked keys for each level (may collide — that's fine,
		// we verify using precedence rules).
		daemonKey := genSafeEnvKey().Draw(t, "daemonKey")
		agentKey := genSafeEnvKey().Draw(t, "agentKey")
		taskKey := genSafeEnvKey().Draw(t, "taskKey")

		daemonVal := genEnvValue().Draw(t, "daemonVal")
		agentVal := genEnvValue().Draw(t, "agentVal")
		taskVal := genEnvValue().Draw(t, "taskVal")

		daemonEnv := map[string]string{daemonKey: daemonVal}
		agentEnv := map[string]string{agentKey: agentVal}
		taskEnv := map[string]string{taskKey: taskVal}

		result := MergeEnv(daemonEnv, agentEnv, taskEnv, discardLogger())

		// Property: Each key appears in result with the highest-precedence value.
		// For daemonKey: if task or agent also has it, their value wins.
		expectedDaemon := daemonVal
		if agentKey == daemonKey {
			expectedDaemon = agentVal
		}
		if taskKey == daemonKey {
			expectedDaemon = taskVal
		}
		if result[daemonKey] != expectedDaemon {
			t.Fatalf("key %q: result=%q, want=%q", daemonKey, result[daemonKey], expectedDaemon)
		}

		// For agentKey: if task also has it, task value wins.
		expectedAgent := agentVal
		if taskKey == agentKey {
			expectedAgent = taskVal
		}
		if result[agentKey] != expectedAgent {
			t.Fatalf("key %q: result=%q, want=%q", agentKey, result[agentKey], expectedAgent)
		}

		// For taskKey: task always wins (highest precedence).
		if result[taskKey] != taskVal {
			t.Fatalf("key %q: result=%q, want=%q", taskKey, result[taskKey], taskVal)
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

		// Test non-blocked keys: generate keys that don't match any blocked pattern.
		// Use a key that starts with a letter other than the blocked set and doesn't start with AF_.
		safeKey := rapid.StringMatching(`[BCDFGIJKLMNOPQRSTVWXYZ][A-Z0-9_]{0,10}`).Draw(t, "safeKey")
		// Ensure it's not one of the exact blocked keys and doesn't start with AF_.
		if safeKey != "HOME" && safeKey != "PATH" && safeKey != "USER" &&
			safeKey != "SHELL" && safeKey != "TERM" && !strings.HasPrefix(safeKey, "AF_") {
			if IsBlockedEnvKey(safeKey) {
				t.Fatalf("IsBlockedEnvKey(%q) = true, want false (key should not be blocked)", safeKey)
			}
		}
	})
}

// TestProperty11_MergePrecedenceFullStack is a comprehensive property test that
// verifies the full merge precedence stack with mixed blocked and non-blocked keys.
func TestProperty11_MergePrecedenceFullStack(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate environment maps with a mix of safe and blocked keys.
		numDaemon := rapid.IntRange(1, 8).Draw(t, "numDaemon")
		numAgent := rapid.IntRange(1, 8).Draw(t, "numAgent")
		numTask := rapid.IntRange(1, 8).Draw(t, "numTask")

		daemonEnv := make(map[string]string, numDaemon)
		agentEnv := make(map[string]string, numAgent)
		taskEnv := make(map[string]string, numTask)

		for i := 0; i < numDaemon; i++ {
			// Daemon can have any key (including blocked ones).
			useBlocked := rapid.Bool().Draw(t, "daemonUseBlocked")
			var key string
			if useBlocked {
				key = genBlockedEnvKey().Draw(t, "daemonKey")
			} else {
				key = genSafeEnvKey().Draw(t, "daemonKey")
			}
			daemonEnv[key] = genEnvValue().Draw(t, "daemonVal")
		}

		for i := 0; i < numAgent; i++ {
			// Agent may attempt to set blocked keys (they should be filtered).
			useBlocked := rapid.Bool().Draw(t, "agentUseBlocked")
			var key string
			if useBlocked {
				key = genBlockedEnvKey().Draw(t, "agentKey")
			} else {
				key = genSafeEnvKey().Draw(t, "agentKey")
			}
			agentEnv[key] = genEnvValue().Draw(t, "agentVal")
		}

		for i := 0; i < numTask; i++ {
			// Task may attempt to set blocked keys (they should be filtered).
			useBlocked := rapid.Bool().Draw(t, "taskUseBlocked")
			var key string
			if useBlocked {
				key = genBlockedEnvKey().Draw(t, "taskKey")
			} else {
				key = genSafeEnvKey().Draw(t, "taskKey")
			}
			taskEnv[key] = genEnvValue().Draw(t, "taskVal")
		}

		result := MergeEnv(daemonEnv, agentEnv, taskEnv, discardLogger())

		// Verify properties for each key in the result.
		for k, v := range result {
			if IsBlockedEnvKey(k) {
				// Blocked keys in result must come from daemon only.
				daemonVal, inDaemon := daemonEnv[k]
				if !inDaemon {
					t.Fatalf("blocked key %q in result but not in daemon env", k)
				}
				if v != daemonVal {
					t.Fatalf("blocked key %q: result=%q, want daemon value=%q", k, v, daemonVal)
				}
			} else {
				// Non-blocked keys follow precedence: task > agent > daemon.
				taskVal, inTask := taskEnv[k]
				agentVal, inAgent := agentEnv[k]
				daemonVal, inDaemon := daemonEnv[k]

				if inTask && !IsBlockedEnvKey(k) {
					if v != taskVal {
						t.Fatalf("non-blocked key %q: result=%q, want task value=%q", k, v, taskVal)
					}
				} else if inAgent && !IsBlockedEnvKey(k) {
					if v != agentVal {
						t.Fatalf("non-blocked key %q: result=%q, want agent value=%q", k, v, agentVal)
					}
				} else if inDaemon {
					if v != daemonVal {
						t.Fatalf("non-blocked key %q: result=%q, want daemon value=%q", k, v, daemonVal)
					}
				} else {
					t.Fatalf("key %q in result but not in any input map", k)
				}
			}
		}

		// Verify that no blocked keys from agent/task leaked into result.
		for k := range agentEnv {
			if IsBlockedEnvKey(k) {
				// If daemon also has this key, it should be daemon's value.
				if _, inDaemon := daemonEnv[k]; inDaemon {
					if result[k] != daemonEnv[k] {
						t.Fatalf("blocked key %q from agent: result=%q, should be daemon value=%q",
							k, result[k], daemonEnv[k])
					}
				} else {
					// If daemon doesn't have it, it should not be in result at all.
					if _, exists := result[k]; exists {
						t.Fatalf("blocked key %q from agent appeared in result without daemon setting it", k)
					}
				}
			}
		}

		for k := range taskEnv {
			if IsBlockedEnvKey(k) {
				if _, inDaemon := daemonEnv[k]; inDaemon {
					if result[k] != daemonEnv[k] {
						t.Fatalf("blocked key %q from task: result=%q, should be daemon value=%q",
							k, result[k], daemonEnv[k])
					}
				} else {
					if _, exists := result[k]; exists {
						t.Fatalf("blocked key %q from task appeared in result without daemon setting it", k)
					}
				}
			}
		}

		// Verify all daemon keys are present in result.
		for k, v := range daemonEnv {
			resultVal, exists := result[k]
			if !exists {
				t.Fatalf("daemon key %q missing from result", k)
			}
			if IsBlockedEnvKey(k) {
				// Blocked daemon keys can't be overridden.
				if resultVal != v {
					t.Fatalf("daemon blocked key %q: result=%q, want=%q", k, resultVal, v)
				}
			}
		}

		// Verify all non-blocked agent keys are present in result.
		for k := range agentEnv {
			if !IsBlockedEnvKey(k) {
				if _, exists := result[k]; !exists {
					t.Fatalf("non-blocked agent key %q missing from result", k)
				}
			}
		}

		// Verify all non-blocked task keys are present in result.
		for k := range taskEnv {
			if !IsBlockedEnvKey(k) {
				if _, exists := result[k]; !exists {
					t.Fatalf("non-blocked task key %q missing from result", k)
				}
			}
		}
	})
}
