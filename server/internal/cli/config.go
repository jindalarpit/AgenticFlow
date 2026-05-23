package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const defaultConfigPath = ".agenticflow/config.json"

// Config holds persistent CLI settings stored in ~/.agenticflow/config.json.
type Config struct {
	ServerURL          string        `json:"server_url"`
	Token              string        `json:"token"`
	TokenExpiresAt     time.Time     `json:"token_expires_at"`
	UserEmail          string        `json:"user_email"`
	PollInterval       time.Duration `json:"poll_interval"`
	HeartbeatInterval  time.Duration `json:"heartbeat_interval"`
	AgentTimeout       time.Duration `json:"agent_timeout"`
	MaxConcurrentTasks int           `json:"max_concurrent_tasks"`
}

// configJSON is the JSON-serializable representation of Config.
// time.Duration fields are serialized as human-readable strings (e.g. "3s", "15s", "2h").
type configJSON struct {
	ServerURL          string `json:"server_url"`
	Token              string `json:"token"`
	TokenExpiresAt     string `json:"token_expires_at"`
	UserEmail          string `json:"user_email"`
	PollInterval       string `json:"poll_interval"`
	HeartbeatInterval  string `json:"heartbeat_interval"`
	AgentTimeout       string `json:"agent_timeout"`
	MaxConcurrentTasks int    `json:"max_concurrent_tasks"`
}

// MarshalJSON implements custom JSON marshaling for Config.
// Duration fields are serialized as strings (e.g. "3s", "15s", "2h0m0s").
func (c Config) MarshalJSON() ([]byte, error) {
	j := configJSON{
		ServerURL:          c.ServerURL,
		Token:              c.Token,
		UserEmail:          c.UserEmail,
		PollInterval:       formatDuration(c.PollInterval),
		HeartbeatInterval:  formatDuration(c.HeartbeatInterval),
		AgentTimeout:       formatDuration(c.AgentTimeout),
		MaxConcurrentTasks: c.MaxConcurrentTasks,
	}
	if !c.TokenExpiresAt.IsZero() {
		j.TokenExpiresAt = c.TokenExpiresAt.Format(time.RFC3339)
	}
	return json.Marshal(j)
}

// UnmarshalJSON implements custom JSON unmarshaling for Config.
// Duration fields are parsed from strings (e.g. "3s", "15s", "2h").
func (c *Config) UnmarshalJSON(data []byte) error {
	var j configJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}

	c.ServerURL = j.ServerURL
	c.Token = j.Token
	c.UserEmail = j.UserEmail
	c.MaxConcurrentTasks = j.MaxConcurrentTasks

	if j.TokenExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, j.TokenExpiresAt)
		if err != nil {
			return fmt.Errorf("parse token_expires_at: %w", err)
		}
		c.TokenExpiresAt = t
	}

	if j.PollInterval != "" {
		d, err := time.ParseDuration(j.PollInterval)
		if err != nil {
			return fmt.Errorf("parse poll_interval: %w", err)
		}
		c.PollInterval = d
	}

	if j.HeartbeatInterval != "" {
		d, err := time.ParseDuration(j.HeartbeatInterval)
		if err != nil {
			return fmt.Errorf("parse heartbeat_interval: %w", err)
		}
		c.HeartbeatInterval = d
	}

	if j.AgentTimeout != "" {
		d, err := time.ParseDuration(j.AgentTimeout)
		if err != nil {
			return fmt.Errorf("parse agent_timeout: %w", err)
		}
		c.AgentTimeout = d
	}

	return nil
}

// formatDuration formats a duration as a compact string.
// For durations that are exact multiples of hours, it returns "2h" instead of "2h0m0s".
func formatDuration(d time.Duration) string {
	if d == 0 {
		return ""
	}
	return d.String()
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		ServerURL:          "",
		PollInterval:       3 * time.Second,
		HeartbeatInterval:  15 * time.Second,
		AgentTimeout:       2 * time.Hour,
		MaxConcurrentTasks: 5,
	}
}

// ConfigPath returns the path to the CLI config file.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve config path: %w", err)
	}
	return filepath.Join(home, defaultConfigPath), nil
}

// LoadConfig reads the CLI config from ~/.agenticflow/config.json.
// If the file is missing or contains invalid JSON, it creates a default config
// and logs a warning.
func LoadConfig() Config {
	path, err := ConfigPath()
	if err != nil {
		slog.Warn("failed to resolve config path, using defaults", "error", err)
		return DefaultConfig()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Warn("config file not found, creating default", "path", path)
		} else {
			slog.Warn("failed to read config file, creating default", "path", path, "error", err)
		}
		cfg := DefaultConfig()
		if saveErr := SaveConfig(cfg); saveErr != nil {
			slog.Warn("failed to save default config", "error", saveErr)
		}
		return cfg
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		slog.Warn("config file contains invalid JSON, replacing with defaults", "path", path, "error", err)
		cfg = DefaultConfig()
		if saveErr := SaveConfig(cfg); saveErr != nil {
			slog.Warn("failed to save default config", "error", saveErr)
		}
		return cfg
	}

	return cfg
}

// SaveConfig writes the config as formatted JSON to ~/.agenticflow/config.json.
// It creates the directory if needed and uses atomic write (temp file + rename).
func SaveConfig(cfg Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	// Write to a temp file in the same directory, then rename for atomicity.
	tmp, err := os.CreateTemp(dir, ".config-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp config file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp config file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp config file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp config file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename config file: %w", err)
	}
	return nil
}

// Validate checks that all config values are within acceptable ranges.
// An empty ServerURL is allowed (unconfigured state).
func (c Config) Validate() error {
	if c.ServerURL != "" {
		if err := validateServerURL(c.ServerURL); err != nil {
			return fmt.Errorf("server_url: %w", err)
		}
	}

	if c.PollInterval < 1*time.Second || c.PollInterval > 300*time.Second {
		return fmt.Errorf("poll_interval: must be between 1s and 300s, got %s", c.PollInterval)
	}

	if c.HeartbeatInterval < 5*time.Second || c.HeartbeatInterval > 300*time.Second {
		return fmt.Errorf("heartbeat_interval: must be between 5s and 300s, got %s", c.HeartbeatInterval)
	}

	if c.AgentTimeout < 1*time.Minute || c.AgentTimeout > 24*time.Hour {
		return fmt.Errorf("agent_timeout: must be between 1m and 24h, got %s", c.AgentTimeout)
	}

	if c.MaxConcurrentTasks < 1 || c.MaxConcurrentTasks > 100 {
		return fmt.Errorf("max_concurrent_tasks: must be between 1 and 100, got %d", c.MaxConcurrentTasks)
	}

	return nil
}

// ValidateField validates a single config field by key and string value.
// This is used by `af config set` to validate before writing.
// Returns the parsed value as an interface{} on success, or an error on failure.
func ValidateField(key, value string) error {
	switch key {
	case "server_url":
		if value == "" {
			return nil // allow clearing
		}
		return validateServerURL(value)

	case "poll_interval":
		d, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		if d < 1*time.Second || d > 300*time.Second {
			return fmt.Errorf("must be between 1s and 300s, got %s", d)
		}
		return nil

	case "heartbeat_interval":
		d, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		if d < 5*time.Second || d > 300*time.Second {
			return fmt.Errorf("must be between 5s and 300s, got %s", d)
		}
		return nil

	case "agent_timeout":
		d, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		if d < 1*time.Minute || d > 24*time.Hour {
			return fmt.Errorf("must be between 1m and 24h, got %s", d)
		}
		return nil

	case "max_concurrent_tasks":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("must be an integer: %w", err)
		}
		if n < 1 || n > 100 {
			return fmt.Errorf("must be between 1 and 100, got %d", n)
		}
		return nil

	default:
		return fmt.Errorf("unrecognized config key: %q", key)
	}
}

// ApplyField applies a validated key-value pair to the config.
// Call ValidateField first to ensure the value is valid.
func (c *Config) ApplyField(key, value string) error {
	switch key {
	case "server_url":
		c.ServerURL = value
	case "poll_interval":
		d, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		c.PollInterval = d
	case "heartbeat_interval":
		d, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		c.HeartbeatInterval = d
	case "agent_timeout":
		d, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		c.AgentTimeout = d
	case "max_concurrent_tasks":
		n, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		c.MaxConcurrentTasks = n
	default:
		return fmt.Errorf("unrecognized config key: %q", key)
	}
	return nil
}

// validateServerURL checks that a URL has a valid http or https scheme.
func validateServerURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("must use http or https scheme, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("URL must have a host")
	}
	return nil
}
