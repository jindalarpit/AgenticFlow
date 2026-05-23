package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// AgentEntry describes a single detected agent CLI.
type AgentEntry struct {
	Name    string // display name (e.g. "claude", "gemini")
	Path    string // absolute path to the CLI binary
	Version string // detected version string, or "unknown"
	Model   string // model override from AF_<NAME>_MODEL env var (optional)
}

// knownAgent defines a supported agent CLI with its binary command name
// and associated environment variable names for path and model overrides.
type knownAgent struct {
	Name     string // canonical agent name (e.g. "claude")
	Command  string // binary name to look up on PATH (e.g. "claude", "cursor-agent")
	EnvPath  string // env var for custom binary path (e.g. "AF_CLAUDE_PATH")
	EnvModel string // env var for model override (e.g. "AF_CLAUDE_MODEL")
}

// knownAgents lists all supported agent CLIs with their detection configuration.
var knownAgents = []knownAgent{
	{"claude", "claude", "AF_CLAUDE_PATH", "AF_CLAUDE_MODEL"},
	{"gemini", "gemini", "AF_GEMINI_PATH", "AF_GEMINI_MODEL"},
	{"opencode", "opencode", "AF_OPENCODE_PATH", "AF_OPENCODE_MODEL"},
	{"openclaw", "openclaw", "AF_OPENCLAW_PATH", "AF_OPENCLAW_MODEL"},
	{"codex", "codex", "AF_CODEX_PATH", "AF_CODEX_MODEL"},
	{"copilot", "copilot", "AF_COPILOT_PATH", "AF_COPILOT_MODEL"},
	{"hermes", "hermes", "AF_HERMES_PATH", "AF_HERMES_MODEL"},
	{"pi", "pi", "AF_PI_PATH", "AF_PI_MODEL"},
	{"cursor", "cursor-agent", "AF_CURSOR_PATH", "AF_CURSOR_MODEL"},
	{"kimi", "kimi", "AF_KIMI_PATH", "AF_KIMI_MODEL"},
	{"kiro", "kiro-cli", "AF_KIRO_PATH", "AF_KIRO_MODEL"},
}

// DetectionDeps holds injectable dependencies for DetectAgents, making
// the function testable without real filesystem or PATH lookups.
type DetectionDeps struct {
	// LookPath resolves a binary name to its absolute path on the system PATH.
	// Defaults to exec.LookPath if nil.
	LookPath func(file string) (string, error)

	// Getenv retrieves the value of an environment variable.
	// Defaults to os.Getenv if nil.
	Getenv func(key string) string

	// Stat checks whether a file exists at the given path.
	// Defaults to os.Stat if nil.
	Stat func(name string) (os.FileInfo, error)

	// DetectVersion runs the binary with --version and returns the output.
	// Defaults to detectAgentVersion if nil.
	DetectVersion func(ctx context.Context, binaryPath string) (string, error)

	// Logger for warnings and info messages.
	// Defaults to slog.Default() if nil.
	Logger *slog.Logger
}

// defaults fills in nil fields with production implementations.
func (d *DetectionDeps) defaults() {
	if d.LookPath == nil {
		d.LookPath = exec.LookPath
	}
	if d.Getenv == nil {
		d.Getenv = os.Getenv
	}
	if d.Stat == nil {
		d.Stat = os.Stat
	}
	if d.DetectVersion == nil {
		d.DetectVersion = detectAgentVersion
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
}

// DetectAgents scans for known agent CLI binaries and returns a map of
// detected agents keyed by agent name. For each known agent:
//
//  1. Check AF_<NAME>_PATH env var for a custom binary path (takes precedence).
//  2. If custom path is set, verify the binary exists; skip with warning if not.
//  3. If no custom path, use LookPath to find the binary on the system PATH.
//  4. For found agents, attempt version detection via --version (fallback "unknown").
//  5. Check AF_<NAME>_MODEL env var for model override.
//
// The deps parameter allows injecting test doubles. Pass nil for production defaults.
func DetectAgents(deps *DetectionDeps) map[string]AgentEntry {
	if deps == nil {
		deps = &DetectionDeps{}
	}
	deps.defaults()

	agents := make(map[string]AgentEntry)

	for _, ag := range knownAgents {
		entry, found := probeAgent(ag, deps)
		if !found {
			continue
		}
		agents[ag.Name] = entry
	}

	if len(agents) == 0 {
		deps.Logger.Warn("no agent runtimes detected on PATH or via environment variables")
	}

	return agents
}

// probeAgent attempts to locate a single agent binary and build an AgentEntry.
// Returns the entry and true if the agent was found, or a zero entry and false otherwise.
func probeAgent(ag knownAgent, deps *DetectionDeps) (AgentEntry, bool) {
	var binaryPath string

	// Check for custom path via environment variable (takes precedence).
	customPath := strings.TrimSpace(deps.Getenv(ag.EnvPath))
	if customPath != "" {
		// Custom path specified — verify the binary exists.
		info, err := deps.Stat(customPath)
		if err != nil || info.IsDir() {
			deps.Logger.Warn("custom agent path is invalid; skipping agent",
				"agent", ag.Name,
				"path", customPath,
				"env_var", ag.EnvPath,
			)
			return AgentEntry{}, false
		}
		binaryPath = customPath
	} else {
		// No custom path — look up on system PATH.
		path, err := deps.LookPath(ag.Command)
		if err != nil {
			return AgentEntry{}, false
		}
		binaryPath = path
	}

	// Attempt version detection.
	version := detectVersion(deps, binaryPath)

	// Check for model override.
	model := strings.TrimSpace(deps.Getenv(ag.EnvModel))

	return AgentEntry{
		Name:    ag.Name,
		Path:    binaryPath,
		Version: version,
		Model:   model,
	}, true
}

// detectVersion runs the binary with --version and extracts a version string.
// Returns "unknown" if version detection fails.
func detectVersion(deps *DetectionDeps, binaryPath string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	version, err := deps.DetectVersion(ctx, binaryPath)
	if err != nil {
		return "unknown"
	}

	version = strings.TrimSpace(version)
	if version == "" {
		return "unknown"
	}
	return version
}

// detectAgentVersion is the default production implementation that runs
// the binary with --version and returns the first non-empty line of output.
func detectAgentVersion(ctx context.Context, binaryPath string) (string, error) {
	cmd := exec.CommandContext(ctx, binaryPath, "--version")
	// Prevent the child process from inheriting stdin to avoid hangs.
	cmd.Stdin = nil

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("detect version for %s: %w", binaryPath, err)
	}

	// Return the first non-empty line (handles multi-line version output).
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
	}
	return "", fmt.Errorf("empty version output from %s", binaryPath)
}
