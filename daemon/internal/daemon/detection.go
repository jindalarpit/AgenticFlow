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
	Name     string
	Command  string
	EnvPath  string
	EnvModel string
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

// DetectionDeps holds injectable dependencies for DetectAgents.
type DetectionDeps struct {
	LookPath      func(file string) (string, error)
	Getenv        func(key string) string
	Stat          func(name string) (os.FileInfo, error)
	DetectVersion func(ctx context.Context, binaryPath string) (string, error)
	Logger        *slog.Logger
}

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
// detected agents keyed by agent name.
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

func probeAgent(ag knownAgent, deps *DetectionDeps) (AgentEntry, bool) {
	var binaryPath string

	customPath := strings.TrimSpace(deps.Getenv(ag.EnvPath))
	if customPath != "" {
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
		path, err := deps.LookPath(ag.Command)
		if err != nil {
			return AgentEntry{}, false
		}
		binaryPath = path
	}

	version := detectVersion(deps, binaryPath)
	model := strings.TrimSpace(deps.Getenv(ag.EnvModel))

	return AgentEntry{
		Name:    ag.Name,
		Path:    binaryPath,
		Version: version,
		Model:   model,
	}, true
}

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

func detectAgentVersion(ctx context.Context, binaryPath string) (string, error) {
	cmd := exec.CommandContext(ctx, binaryPath, "--version")
	cmd.Stdin = nil
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("detect version for %s: %w", binaryPath, err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
	}
	return "", fmt.Errorf("empty version output from %s", binaryPath)
}
