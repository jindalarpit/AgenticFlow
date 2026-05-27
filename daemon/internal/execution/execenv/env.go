package execenv

import (
	"log/slog"
	"strings"
)

// blockedEnvKeys are keys that cannot be overridden by agent or task custom_env.
// These are system-critical variables that must remain under daemon control.
var blockedEnvKeys = map[string]bool{
	"HOME":  true,
	"PATH":  true,
	"USER":  true,
	"SHELL": true,
	"TERM":  true,
}

// blockedEnvPrefixes are prefixes that cannot be overridden by agent or task custom_env.
// The AF_ prefix is reserved for daemon-internal variables.
var blockedEnvPrefixes = []string{"AF_"}

// IsBlockedEnvKey returns true if the key is blocked from custom_env override.
// A key is blocked if it exactly matches a blocked key name (HOME, PATH, USER,
// SHELL, TERM) or starts with a blocked prefix (AF_).
func IsBlockedEnvKey(key string) bool {
	if blockedEnvKeys[key] {
		return true
	}
	for _, prefix := range blockedEnvPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// MergeEnv combines daemon, agent, and task environment variables.
// Precedence: task > agent > daemon (highest wins for duplicate keys).
// Blocked keys from agent/task levels are skipped with a warning log.
//
// Daemon-level variables are always applied (they represent the base environment).
// Agent and task-level variables are filtered through the blocked key list to
// prevent accidental override of system-critical or daemon-internal variables.
func MergeEnv(daemonEnv, agentEnv, taskEnv map[string]string, logger *slog.Logger) map[string]string {
	result := make(map[string]string, len(daemonEnv)+len(agentEnv)+len(taskEnv))

	// Layer 1: daemon-level env (lowest precedence, no filtering).
	for k, v := range daemonEnv {
		result[k] = v
	}

	// Layer 2: agent-level env (overrides daemon for non-blocked keys).
	for k, v := range agentEnv {
		if IsBlockedEnvKey(k) {
			logger.Warn("blocked env key from agent custom_env, skipping",
				"key", k,
				"source", "agent",
			)
			continue
		}
		result[k] = v
	}

	// Layer 3: task-level env (highest precedence, overrides agent for non-blocked keys).
	for k, v := range taskEnv {
		if IsBlockedEnvKey(k) {
			logger.Warn("blocked env key from task env, skipping",
				"key", k,
				"source", "task",
			)
			continue
		}
		result[k] = v
	}

	return result
}
