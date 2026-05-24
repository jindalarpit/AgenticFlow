package daemon

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agenticflow/agenticflow/internal/cli"
)

const (
	DefaultPollInterval       = 3 * time.Second
	DefaultHeartbeatInterval  = 15 * time.Second
	DefaultAgentTimeout       = 2 * time.Hour
	DefaultMaxConcurrentTasks = 5
)

// Config holds all daemon configuration resolved from CLI flags, environment
// variables, config file, and defaults (in that precedence order).
type Config struct {
	ServerURL          string
	DaemonID           string
	DeviceName         string
	CLIVersion         string
	HealthPort         int
	Agents             map[string]AgentEntry
	WorkspacesRoot     string
	PollInterval       time.Duration
	HeartbeatInterval  time.Duration
	AgentTimeout       time.Duration
	MaxConcurrentTasks int
}

// Overrides allows CLI flags to override environment variables, config file
// values, and defaults. Pointer types are used so nil means "not set" and
// the next precedence level is consulted.
type Overrides struct {
	ServerURL          *string
	CLIVersion         *string
	HealthPort         *int
	PollInterval       *time.Duration
	HeartbeatInterval  *time.Duration
	AgentTimeout       *time.Duration
	MaxConcurrentTasks *int
}

// LoadConfig builds the daemon configuration by resolving values in order:
// CLI flags (Overrides) > environment variables > config file > defaults.
func LoadConfig(overrides Overrides) (Config, error) {
	// Load the persisted config file values via cli.LoadConfig().
	fileCfg := cli.LoadConfig()

	// --- ServerURL ---
	serverURL := resolveString(
		overrides.ServerURL,
		os.Getenv("AF_SERVER_URL"),
		fileCfg.ServerURL,
		"",
	)

	// --- PollInterval ---
	pollInterval, err := resolveDuration(
		overrides.PollInterval,
		os.Getenv("AF_DAEMON_POLL_INTERVAL"),
		fileCfg.PollInterval,
		DefaultPollInterval,
	)
	if err != nil {
		return Config{}, fmt.Errorf("resolve poll_interval: %w", err)
	}

	// --- HeartbeatInterval ---
	heartbeatInterval, err := resolveDuration(
		overrides.HeartbeatInterval,
		os.Getenv("AF_DAEMON_HEARTBEAT_INTERVAL"),
		fileCfg.HeartbeatInterval,
		DefaultHeartbeatInterval,
	)
	if err != nil {
		return Config{}, fmt.Errorf("resolve heartbeat_interval: %w", err)
	}

	// --- AgentTimeout ---
	agentTimeout, err := resolveDuration(
		overrides.AgentTimeout,
		os.Getenv("AF_AGENT_TIMEOUT"),
		fileCfg.AgentTimeout,
		DefaultAgentTimeout,
	)
	if err != nil {
		return Config{}, fmt.Errorf("resolve agent_timeout: %w", err)
	}

	// --- MaxConcurrentTasks ---
	maxConcurrentTasks, err := resolveInt(
		overrides.MaxConcurrentTasks,
		os.Getenv("AF_DAEMON_MAX_CONCURRENT_TASKS"),
		fileCfg.MaxConcurrentTasks,
		DefaultMaxConcurrentTasks,
	)
	if err != nil {
		return Config{}, fmt.Errorf("resolve max_concurrent_tasks: %w", err)
	}

	// --- DaemonID ---
	daemonID := generateDaemonID()

	// --- DeviceName ---
	deviceName := resolveDeviceName()

	// --- WorkspacesRoot ---
	workspacesRoot, err := resolveWorkspacesRoot()
	if err != nil {
		return Config{}, fmt.Errorf("resolve workspaces_root: %w", err)
	}

	// --- Agents ---
	agents := DetectAgents(nil)

	// --- HealthPort ---
	healthPort, err := resolveInt(
		overrides.HealthPort,
		os.Getenv("AF_DAEMON_HEALTH_PORT"),
		0,
		DefaultHealthPort,
	)
	if err != nil {
		return Config{}, fmt.Errorf("resolve health_port: %w", err)
	}

	// --- CLIVersion ---
	cliVersion := ""
	if overrides.CLIVersion != nil {
		cliVersion = *overrides.CLIVersion
	}

	return Config{
		ServerURL:          serverURL,
		DaemonID:           daemonID,
		DeviceName:         deviceName,
		CLIVersion:         cliVersion,
		HealthPort:         healthPort,
		Agents:             agents,
		WorkspacesRoot:     workspacesRoot,
		PollInterval:       pollInterval,
		HeartbeatInterval:  heartbeatInterval,
		AgentTimeout:       agentTimeout,
		MaxConcurrentTasks: maxConcurrentTasks,
	}, nil
}

// resolveString returns the first non-empty value in precedence order:
// CLI flag (pointer) > env var > config file value > default.
func resolveString(flag *string, envVal, fileVal, defaultVal string) string {
	if flag != nil && *flag != "" {
		return *flag
	}
	if envVal = strings.TrimSpace(envVal); envVal != "" {
		return envVal
	}
	if fileVal != "" {
		return fileVal
	}
	return defaultVal
}

// resolveDuration returns the first set value in precedence order:
// CLI flag (pointer) > env var (parsed) > config file value > default.
// A zero-value config file duration is treated as "not set".
func resolveDuration(flag *time.Duration, envVal string, fileVal, defaultVal time.Duration) (time.Duration, error) {
	if flag != nil {
		return *flag, nil
	}
	if envVal = strings.TrimSpace(envVal); envVal != "" {
		d, err := time.ParseDuration(envVal)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", envVal, err)
		}
		return d, nil
	}
	if fileVal != 0 {
		return fileVal, nil
	}
	return defaultVal, nil
}

// resolveInt returns the first set value in precedence order:
// CLI flag (pointer) > env var (parsed) > config file value > default.
// A zero-value config file int is treated as "not set".
func resolveInt(flag *int, envVal string, fileVal, defaultVal int) (int, error) {
	if flag != nil {
		return *flag, nil
	}
	if envVal = strings.TrimSpace(envVal); envVal != "" {
		n, err := strconv.Atoi(envVal)
		if err != nil {
			return 0, fmt.Errorf("invalid integer %q: %w", envVal, err)
		}
		return n, nil
	}
	if fileVal != 0 {
		return fileVal, nil
	}
	return defaultVal, nil
}

// generateDaemonID produces a stable daemon identifier derived from the
// machine's hostname and current username. The ID is a truncated SHA-256
// hash to ensure stability across restarts without requiring persistent
// state files.
func generateDaemonID() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "unknown-host"
	}

	username := "unknown-user"
	if u, err := user.Current(); err == nil && u.Username != "" {
		username = u.Username
	}

	h := sha256.New()
	h.Write([]byte(hostname + ":" + username))
	sum := h.Sum(nil)
	// Use first 16 bytes (32 hex chars) for a compact but collision-resistant ID.
	return fmt.Sprintf("%x", sum[:16])
}

// resolveDeviceName returns the machine hostname for display purposes.
func resolveDeviceName() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		return "local-machine"
	}
	return hostname
}

// resolveWorkspacesRoot returns the absolute path to the workspaces directory,
// defaulting to ~/.agenticflow/workspaces/.
func resolveWorkspacesRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	root := filepath.Join(home, ".agenticflow", "workspaces")
	return root, nil
}
