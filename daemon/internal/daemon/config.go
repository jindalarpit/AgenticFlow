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

	"github.com/agenticflow/agenticflow/daemon/internal/cli"
	"github.com/agenticflow/agenticflow/shared/constants"
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
// values, and defaults.
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
	fileCfg := cli.LoadConfig()

	serverURL := resolveString(
		overrides.ServerURL,
		os.Getenv("AF_SERVER_URL"),
		fileCfg.ServerURL,
		"",
	)

	pollInterval, err := resolveDuration(
		overrides.PollInterval,
		os.Getenv("AF_DAEMON_POLL_INTERVAL"),
		fileCfg.PollInterval,
		constants.DefaultPollInterval,
	)
	if err != nil {
		return Config{}, fmt.Errorf("resolve poll_interval: %w", err)
	}

	heartbeatInterval, err := resolveDuration(
		overrides.HeartbeatInterval,
		os.Getenv("AF_DAEMON_HEARTBEAT_INTERVAL"),
		fileCfg.HeartbeatInterval,
		constants.DefaultHeartbeatInterval,
	)
	if err != nil {
		return Config{}, fmt.Errorf("resolve heartbeat_interval: %w", err)
	}

	agentTimeout, err := resolveDuration(
		overrides.AgentTimeout,
		os.Getenv("AF_AGENT_TIMEOUT"),
		fileCfg.AgentTimeout,
		constants.DefaultAgentTimeout,
	)
	if err != nil {
		return Config{}, fmt.Errorf("resolve agent_timeout: %w", err)
	}

	maxConcurrentTasks, err := resolveInt(
		overrides.MaxConcurrentTasks,
		os.Getenv("AF_DAEMON_MAX_CONCURRENT_TASKS"),
		fileCfg.MaxConcurrentTasks,
		constants.DefaultMaxConcurrentTasks,
	)
	if err != nil {
		return Config{}, fmt.Errorf("resolve max_concurrent_tasks: %w", err)
	}

	daemonID := generateDaemonID()
	deviceName := resolveDeviceName()

	workspacesRoot, err := resolveWorkspacesRoot()
	if err != nil {
		return Config{}, fmt.Errorf("resolve workspaces_root: %w", err)
	}

	agents := DetectAgents(nil)

	healthPort, err := resolveInt(
		overrides.HealthPort,
		os.Getenv("AF_DAEMON_HEALTH_PORT"),
		0,
		DefaultHealthPort,
	)
	if err != nil {
		return Config{}, fmt.Errorf("resolve health_port: %w", err)
	}

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
	return fmt.Sprintf("%x", sum[:16])
}

func resolveDeviceName() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		return "local-machine"
	}
	return hostname
}

func resolveWorkspacesRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".agenticflow", "workspaces"), nil
}
