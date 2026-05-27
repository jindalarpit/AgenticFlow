// Package detection provides agent CLI runtime detection by scanning PATH.
package detection

import (
	"log/slog"
	"os/exec"
	"runtime"
	"strings"

	"github.com/agenticflow/agenticflow/shared/api"
)

// KnownAgents lists the CLI binary names for all supported agent runtimes.
var KnownAgents = []string{
	"claude",
	"gemini",
	"opencode",
	"kiro",
	"hermes",
	"kimi",
	"codex",
	"copilot",
	"cursor",
	"pi",
	"openclaw",
}

// DetectAgents scans the system PATH for known agent CLI binaries and returns
// a map of agent type → AgentInfo for each detected runtime.
func DetectAgents() map[string]api.AgentInfo {
	agents := make(map[string]api.AgentInfo)

	for _, name := range KnownAgents {
		info, found := detectAgent(name)
		if found {
			agents[name] = info
			slog.Debug("detected agent runtime", "agent", name, "path", info.Path)
		}
	}

	return agents
}

// detectAgent checks if a specific agent CLI is available on PATH.
func detectAgent(name string) (api.AgentInfo, bool) {
	binaryName := agentBinaryName(name)
	path, err := exec.LookPath(binaryName)
	if err != nil {
		return api.AgentInfo{}, false
	}

	info := api.AgentInfo{
		Path: path,
	}

	// Try to get version information
	version := getAgentVersion(path, name)
	if version != "" {
		info.Version = version
	}

	// Set default model for known agents
	info.Model = defaultModelForAgent(name)

	return info, true
}

// agentBinaryName returns the expected binary name for an agent type.
// On Windows, appends .exe suffix.
func agentBinaryName(agentType string) string {
	name := agentType
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// getAgentVersion attempts to get the version string from an agent CLI.
func getAgentVersion(path, agentType string) string {
	var args []string
	switch agentType {
	case "claude":
		args = []string{"--version"}
	case "gemini":
		args = []string{"--version"}
	case "codex":
		args = []string{"--version"}
	default:
		args = []string{"--version"}
	}

	out, err := exec.Command(path, args...).Output()
	if err != nil {
		return ""
	}

	version := strings.TrimSpace(string(out))
	// Take only the first line if multi-line output
	if idx := strings.IndexByte(version, '\n'); idx >= 0 {
		version = version[:idx]
	}
	return version
}

// defaultModelForAgent returns the default model name for a given agent type.
func defaultModelForAgent(agentType string) string {
	switch agentType {
	case "claude":
		return "claude-sonnet-4-20250514"
	case "gemini":
		return "gemini-2.5-pro"
	case "codex":
		return "o3"
	case "copilot":
		return "gpt-4o"
	case "kiro":
		return "claude-sonnet-4-20250514"
	case "opencode":
		return "claude-sonnet-4-20250514"
	case "hermes":
		return "claude-sonnet-4-20250514"
	case "kimi":
		return "kimi-latest"
	case "cursor":
		return "claude-sonnet-4-20250514"
	case "pi":
		return "claude-sonnet-4-20250514"
	case "openclaw":
		return "claude-sonnet-4-20250514"
	default:
		return ""
	}
}
