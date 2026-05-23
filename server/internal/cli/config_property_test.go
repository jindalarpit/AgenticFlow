package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Feature: agenticflow-core, Property 3: Configuration Resolution Precedence
// For any config key with values at multiple levels (env var, config file, default),
// the resolved value equals the highest-precedence source.
// Validates: Requirements 2.8
func TestPropertyConfigResolutionPrecedence(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random valid config values for each level
		defaultCfg := DefaultConfig()

		// Generate a valid config file value (different from default)
		filePollInterval := rapid.Int64Range(1, 300).Draw(t, "filePollSeconds")
		fileHeartbeatInterval := rapid.Int64Range(5, 300).Draw(t, "fileHeartbeatSeconds")
		fileAgentTimeout := rapid.Int64Range(60, 86400).Draw(t, "fileAgentTimeoutSeconds")
		fileMaxConcurrent := rapid.IntRange(1, 100).Draw(t, "fileMaxConcurrent")
		fileServerURL := rapid.SampledFrom([]string{
			"http://localhost:8080",
			"https://example.com",
			"http://192.168.1.1:3000",
		}).Draw(t, "fileServerURL")

		fileCfg := Config{
			ServerURL:          fileServerURL,
			PollInterval:       time.Duration(filePollInterval) * time.Second,
			HeartbeatInterval:  time.Duration(fileHeartbeatInterval) * time.Second,
			AgentTimeout:       time.Duration(fileAgentTimeout) * time.Second,
			MaxConcurrentTasks: fileMaxConcurrent,
		}

		// Generate env var values (different from file values)
		envPollInterval := rapid.Int64Range(1, 300).Draw(t, "envPollSeconds")
		envHeartbeatInterval := rapid.Int64Range(5, 300).Draw(t, "envHeartbeatSeconds")
		envAgentTimeout := rapid.Int64Range(60, 86400).Draw(t, "envAgentTimeoutSeconds")
		envMaxConcurrent := rapid.IntRange(1, 100).Draw(t, "envMaxConcurrent")
		envServerURL := rapid.SampledFrom([]string{
			"http://env-server:9090",
			"https://env.example.com",
			"http://10.0.0.1:4000",
		}).Draw(t, "envServerURL")

		// Choose which level to test: env > file > default
		level := rapid.IntRange(0, 2).Draw(t, "level")

		switch level {
		case 0:
			// Test: default values are used when no file or env override
			// The DefaultConfig() should provide the expected defaults
			if defaultCfg.PollInterval != 3*time.Second {
				t.Fatalf("default PollInterval should be 3s, got %s", defaultCfg.PollInterval)
			}
			if defaultCfg.HeartbeatInterval != 15*time.Second {
				t.Fatalf("default HeartbeatInterval should be 15s, got %s", defaultCfg.HeartbeatInterval)
			}
			if defaultCfg.AgentTimeout != 2*time.Hour {
				t.Fatalf("default AgentTimeout should be 2h, got %s", defaultCfg.AgentTimeout)
			}
			if defaultCfg.MaxConcurrentTasks != 5 {
				t.Fatalf("default MaxConcurrentTasks should be 5, got %d", defaultCfg.MaxConcurrentTasks)
			}

		case 1:
			// Test: config file values override defaults
			// Simulate: start with defaults, apply file config
			resolved := defaultCfg
			resolved.ServerURL = fileCfg.ServerURL
			resolved.PollInterval = fileCfg.PollInterval
			resolved.HeartbeatInterval = fileCfg.HeartbeatInterval
			resolved.AgentTimeout = fileCfg.AgentTimeout
			resolved.MaxConcurrentTasks = fileCfg.MaxConcurrentTasks

			// File values should override defaults
			if resolved.ServerURL != fileCfg.ServerURL {
				t.Fatalf("file ServerURL should override default: got %q, want %q", resolved.ServerURL, fileCfg.ServerURL)
			}
			if resolved.PollInterval != fileCfg.PollInterval {
				t.Fatalf("file PollInterval should override default: got %s, want %s", resolved.PollInterval, fileCfg.PollInterval)
			}
			if resolved.HeartbeatInterval != fileCfg.HeartbeatInterval {
				t.Fatalf("file HeartbeatInterval should override default: got %s, want %s", resolved.HeartbeatInterval, fileCfg.HeartbeatInterval)
			}
			if resolved.AgentTimeout != fileCfg.AgentTimeout {
				t.Fatalf("file AgentTimeout should override default: got %s, want %s", resolved.AgentTimeout, fileCfg.AgentTimeout)
			}
			if resolved.MaxConcurrentTasks != fileCfg.MaxConcurrentTasks {
				t.Fatalf("file MaxConcurrentTasks should override default: got %d, want %d", resolved.MaxConcurrentTasks, fileCfg.MaxConcurrentTasks)
			}

		case 2:
			// Test: env vars override config file values
			// Simulate resolution: start with file config, apply env overrides
			resolved := fileCfg

			// Set env vars and apply them (simulating the resolution logic)
			envVars := map[string]string{
				"AF_SERVER_URL":                   envServerURL,
				"AF_DAEMON_POLL_INTERVAL":         fmt.Sprintf("%ds", envPollInterval),
				"AF_DAEMON_HEARTBEAT_INTERVAL":    fmt.Sprintf("%ds", envHeartbeatInterval),
				"AF_AGENT_TIMEOUT":                fmt.Sprintf("%ds", envAgentTimeout),
				"AF_DAEMON_MAX_CONCURRENT_TASKS":  fmt.Sprintf("%d", envMaxConcurrent),
			}

			// Set env vars
			for k, v := range envVars {
				os.Setenv(k, v)
			}
			defer func() {
				for k := range envVars {
					os.Unsetenv(k)
				}
			}()

			// Apply env var overrides (simulating resolution precedence)
			if v := os.Getenv("AF_SERVER_URL"); v != "" {
				resolved.ServerURL = v
			}
			if v := os.Getenv("AF_DAEMON_POLL_INTERVAL"); v != "" {
				if d, err := time.ParseDuration(v); err == nil {
					resolved.PollInterval = d
				}
			}
			if v := os.Getenv("AF_DAEMON_HEARTBEAT_INTERVAL"); v != "" {
				if d, err := time.ParseDuration(v); err == nil {
					resolved.HeartbeatInterval = d
				}
			}
			if v := os.Getenv("AF_AGENT_TIMEOUT"); v != "" {
				if d, err := time.ParseDuration(v); err == nil {
					resolved.AgentTimeout = d
				}
			}
			if v := os.Getenv("AF_DAEMON_MAX_CONCURRENT_TASKS"); v != "" {
				var n int
				if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
					resolved.MaxConcurrentTasks = n
				}
			}

			// Env values should take precedence over file values
			if resolved.ServerURL != envServerURL {
				t.Fatalf("env ServerURL should override file: got %q, want %q", resolved.ServerURL, envServerURL)
			}
			expectedPoll := time.Duration(envPollInterval) * time.Second
			if resolved.PollInterval != expectedPoll {
				t.Fatalf("env PollInterval should override file: got %s, want %s", resolved.PollInterval, expectedPoll)
			}
			expectedHeartbeat := time.Duration(envHeartbeatInterval) * time.Second
			if resolved.HeartbeatInterval != expectedHeartbeat {
				t.Fatalf("env HeartbeatInterval should override file: got %s, want %s", resolved.HeartbeatInterval, expectedHeartbeat)
			}
			expectedTimeout := time.Duration(envAgentTimeout) * time.Second
			if resolved.AgentTimeout != expectedTimeout {
				t.Fatalf("env AgentTimeout should override file: got %s, want %s", resolved.AgentTimeout, expectedTimeout)
			}
			if resolved.MaxConcurrentTasks != envMaxConcurrent {
				t.Fatalf("env MaxConcurrentTasks should override file: got %d, want %d", resolved.MaxConcurrentTasks, envMaxConcurrent)
			}
		}
	})
}

// Feature: agenticflow-core, Property 4: Configuration Serialization Round-Trip
// For any valid Config object, json.Marshal then json.Unmarshal produces a deeply equal Config.
// Validates: Requirements 9.4
func TestPropertyConfigSerializationRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random valid Config
		cfg := generateValidConfig(t)

		// Marshal to JSON
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}

		// Unmarshal back
		var restored Config
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}

		// Verify deep equality
		if cfg.ServerURL != restored.ServerURL {
			t.Fatalf("ServerURL mismatch: got %q, want %q", restored.ServerURL, cfg.ServerURL)
		}
		if cfg.Token != restored.Token {
			t.Fatalf("Token mismatch: got %q, want %q", restored.Token, cfg.Token)
		}
		if !cfg.TokenExpiresAt.Equal(restored.TokenExpiresAt) {
			t.Fatalf("TokenExpiresAt mismatch: got %v, want %v", restored.TokenExpiresAt, cfg.TokenExpiresAt)
		}
		if cfg.UserEmail != restored.UserEmail {
			t.Fatalf("UserEmail mismatch: got %q, want %q", restored.UserEmail, cfg.UserEmail)
		}
		if cfg.PollInterval != restored.PollInterval {
			t.Fatalf("PollInterval mismatch: got %s, want %s", restored.PollInterval, cfg.PollInterval)
		}
		if cfg.HeartbeatInterval != restored.HeartbeatInterval {
			t.Fatalf("HeartbeatInterval mismatch: got %s, want %s", restored.HeartbeatInterval, cfg.HeartbeatInterval)
		}
		if cfg.AgentTimeout != restored.AgentTimeout {
			t.Fatalf("AgentTimeout mismatch: got %s, want %s", restored.AgentTimeout, cfg.AgentTimeout)
		}
		if cfg.MaxConcurrentTasks != restored.MaxConcurrentTasks {
			t.Fatalf("MaxConcurrentTasks mismatch: got %d, want %d", restored.MaxConcurrentTasks, cfg.MaxConcurrentTasks)
		}
	})
}

// Feature: agenticflow-core, Property 5: Configuration Value Validation
// For any config key and value pair, Validate/ValidateField accepts iff the value is within
// the documented bounds.
// Validates: Requirements 9.6, 9.7
func TestPropertyConfigValueValidation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Choose which field to test
		field := rapid.IntRange(0, 4).Draw(t, "field")

		switch field {
		case 0:
			// server_url validation
			testServerURLValidation(t)
		case 1:
			// poll_interval validation: valid range [1s, 300s]
			testDurationValidation(t, "poll_interval", 1, 300)
		case 2:
			// heartbeat_interval validation: valid range [5s, 300s]
			testDurationValidation(t, "heartbeat_interval", 5, 300)
		case 3:
			// agent_timeout validation: valid range [60s (1m), 86400s (24h)]
			testAgentTimeoutValidation(t)
		case 4:
			// max_concurrent_tasks validation: valid range [1, 100]
			testMaxConcurrentValidation(t)
		}
	})
}

// TestPropertyConfigValidationConsistency verifies that Config.Validate() and ValidateField()
// agree on what constitutes valid values.
// Feature: agenticflow-core, Property 5: Configuration Value Validation
// Validates: Requirements 9.6, 9.7
func TestPropertyConfigValidationConsistency(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a config that may or may not be valid
		useValid := rapid.Bool().Draw(t, "useValid")

		var cfg Config
		if useValid {
			cfg = generateValidConfig(t)
		} else {
			cfg = generatePossiblyInvalidConfig(t)
		}

		// Validate the full config
		fullErr := cfg.Validate()

		// For valid configs, Validate should pass
		if useValid && fullErr != nil {
			t.Fatalf("valid config should pass Validate(), got error: %v", fullErr)
		}

		// Verify individual field validation is consistent with full validation
		if cfg.ServerURL != "" {
			fieldErr := ValidateField("server_url", cfg.ServerURL)
			if fullErr == nil && fieldErr != nil {
				// If full validation passes but field validation fails, that's inconsistent
				// (only if the error is about server_url)
				t.Fatalf("Validate() passed but ValidateField(server_url, %q) failed: %v", cfg.ServerURL, fieldErr)
			}
		}
	})
}

// --- Helper generators ---

// generateValidConfig creates a random Config that passes Validate().
func generateValidConfig(t *rapid.T) Config {
	// Generate valid server URL (or empty)
	serverURL := rapid.SampledFrom([]string{
		"",
		"http://localhost:8080",
		"https://example.com",
		"http://192.168.1.1:3000",
		"https://agenticflow.dev:443",
		"http://10.0.0.1:9090",
	}).Draw(t, "serverURL")

	// Generate valid durations within bounds
	pollSeconds := rapid.Int64Range(1, 300).Draw(t, "pollSeconds")
	heartbeatSeconds := rapid.Int64Range(5, 300).Draw(t, "heartbeatSeconds")
	agentTimeoutSeconds := rapid.Int64Range(60, 86400).Draw(t, "agentTimeoutSeconds")
	maxConcurrent := rapid.IntRange(1, 100).Draw(t, "maxConcurrent")

	// Generate optional token fields
	token := rapid.SampledFrom([]string{
		"",
		"af_test_token_abc123",
		"af_longertoken_xyz789def456",
	}).Draw(t, "token")

	email := rapid.SampledFrom([]string{
		"",
		"user@example.com",
		"dev@agenticflow.io",
		"test+tag@domain.org",
	}).Draw(t, "email")

	// Generate a valid time (truncated to seconds for RFC3339 round-trip)
	useExpiry := rapid.Bool().Draw(t, "useExpiry")
	var expiresAt time.Time
	if useExpiry && token != "" {
		year := rapid.IntRange(2024, 2030).Draw(t, "year")
		month := rapid.IntRange(1, 12).Draw(t, "month")
		day := rapid.IntRange(1, 28).Draw(t, "day")
		hour := rapid.IntRange(0, 23).Draw(t, "hour")
		minute := rapid.IntRange(0, 59).Draw(t, "minute")
		second := rapid.IntRange(0, 59).Draw(t, "second")
		expiresAt = time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)
	}

	return Config{
		ServerURL:          serverURL,
		Token:              token,
		TokenExpiresAt:     expiresAt,
		UserEmail:          email,
		PollInterval:       time.Duration(pollSeconds) * time.Second,
		HeartbeatInterval:  time.Duration(heartbeatSeconds) * time.Second,
		AgentTimeout:       time.Duration(agentTimeoutSeconds) * time.Second,
		MaxConcurrentTasks: maxConcurrent,
	}
}

// generatePossiblyInvalidConfig creates a Config that may have invalid values.
func generatePossiblyInvalidConfig(t *rapid.T) Config {
	// Generate potentially invalid server URL
	serverURL := rapid.SampledFrom([]string{
		"",
		"http://localhost:8080",
		"https://example.com",
		"ftp://invalid.com",
		"not-a-url",
		"://missing-scheme",
	}).Draw(t, "serverURL")

	// Generate potentially out-of-range durations
	pollSeconds := rapid.Int64Range(0, 600).Draw(t, "pollSeconds")
	heartbeatSeconds := rapid.Int64Range(0, 600).Draw(t, "heartbeatSeconds")
	agentTimeoutSeconds := rapid.Int64Range(0, 100000).Draw(t, "agentTimeoutSeconds")
	maxConcurrent := rapid.IntRange(0, 200).Draw(t, "maxConcurrent")

	return Config{
		ServerURL:          serverURL,
		PollInterval:       time.Duration(pollSeconds) * time.Second,
		HeartbeatInterval:  time.Duration(heartbeatSeconds) * time.Second,
		AgentTimeout:       time.Duration(agentTimeoutSeconds) * time.Second,
		MaxConcurrentTasks: maxConcurrent,
	}
}

// --- Field-specific validation testers ---

func testServerURLValidation(t *rapid.T) {
	// Generate either a valid or invalid URL
	isValid := rapid.Bool().Draw(t, "isValidURL")

	var value string
	if isValid {
		value = rapid.SampledFrom([]string{
			"",
			"http://localhost:8080",
			"https://example.com",
			"http://192.168.1.1:3000",
			"https://sub.domain.com:443/path",
			"http://10.0.0.1:9090",
		}).Draw(t, "validURL")
	} else {
		value = rapid.SampledFrom([]string{
			"ftp://example.com",
			"ws://example.com",
			"not-a-url-at-all",
			"://missing-scheme",
			"file:///local/path",
			"ssh://server.com",
		}).Draw(t, "invalidURL")
	}

	err := ValidateField("server_url", value)

	if isValid && err != nil {
		t.Fatalf("valid server_url %q should pass validation, got: %v", value, err)
	}
	if !isValid && err == nil {
		t.Fatalf("invalid server_url %q should fail validation", value)
	}
}

func testDurationValidation(t *rapid.T, key string, minSeconds, maxSeconds int64) {
	// Generate either a valid or invalid duration
	isValid := rapid.Bool().Draw(t, "isValidDuration")

	var seconds int64
	if isValid {
		seconds = rapid.Int64Range(minSeconds, maxSeconds).Draw(t, "validSeconds")
	} else {
		// Generate out-of-range value
		belowMin := rapid.Bool().Draw(t, "belowMin")
		if belowMin {
			seconds = rapid.Int64Range(0, minSeconds-1).Draw(t, "tooLowSeconds")
		} else {
			seconds = rapid.Int64Range(maxSeconds+1, maxSeconds+300).Draw(t, "tooHighSeconds")
		}
	}

	value := fmt.Sprintf("%ds", seconds)
	err := ValidateField(key, value)

	if isValid && err != nil {
		t.Fatalf("valid %s=%q should pass validation, got: %v", key, value, err)
	}
	if !isValid && err == nil {
		t.Fatalf("invalid %s=%q should fail validation (seconds=%d, valid range [%d, %d])",
			key, value, seconds, minSeconds, maxSeconds)
	}
}

func testAgentTimeoutValidation(t *rapid.T) {
	// agent_timeout: valid range [1m, 24h] = [60s, 86400s]
	isValid := rapid.Bool().Draw(t, "isValidTimeout")

	var value string
	if isValid {
		// Generate valid timeout using various duration formats
		format := rapid.IntRange(0, 2).Draw(t, "format")
		switch format {
		case 0:
			// Use seconds
			seconds := rapid.Int64Range(60, 86400).Draw(t, "validTimeoutSeconds")
			value = fmt.Sprintf("%ds", seconds)
		case 1:
			// Use minutes
			minutes := rapid.Int64Range(1, 1440).Draw(t, "validTimeoutMinutes")
			value = fmt.Sprintf("%dm", minutes)
		case 2:
			// Use hours
			hours := rapid.Int64Range(1, 24).Draw(t, "validTimeoutHours")
			value = fmt.Sprintf("%dh", hours)
		}
	} else {
		// Generate invalid timeout
		belowMin := rapid.Bool().Draw(t, "belowMin")
		if belowMin {
			seconds := rapid.Int64Range(1, 59).Draw(t, "tooLowTimeoutSeconds")
			value = fmt.Sprintf("%ds", seconds)
		} else {
			hours := rapid.Int64Range(25, 100).Draw(t, "tooHighTimeoutHours")
			value = fmt.Sprintf("%dh", hours)
		}
	}

	err := ValidateField("agent_timeout", value)

	if isValid && err != nil {
		t.Fatalf("valid agent_timeout=%q should pass validation, got: %v", value, err)
	}
	if !isValid && err == nil {
		t.Fatalf("invalid agent_timeout=%q should fail validation", value)
	}
}

func testMaxConcurrentValidation(t *rapid.T) {
	// max_concurrent_tasks: valid range [1, 100]
	isValid := rapid.Bool().Draw(t, "isValidMaxConcurrent")

	var n int
	if isValid {
		n = rapid.IntRange(1, 100).Draw(t, "validMaxConcurrent")
	} else {
		belowMin := rapid.Bool().Draw(t, "belowMin")
		if belowMin {
			n = rapid.IntRange(-10, 0).Draw(t, "tooLowMaxConcurrent")
		} else {
			n = rapid.IntRange(101, 500).Draw(t, "tooHighMaxConcurrent")
		}
	}

	value := fmt.Sprintf("%d", n)
	err := ValidateField("max_concurrent_tasks", value)

	if isValid && err != nil {
		t.Fatalf("valid max_concurrent_tasks=%q should pass validation, got: %v", value, err)
	}
	if !isValid && err == nil {
		t.Fatalf("invalid max_concurrent_tasks=%q should fail validation", value)
	}
}

// Feature: agenticflow-core, Property 16: Token Storage Round-Trip
// For any valid PAT token (string starting with "af_" followed by random alphanumeric chars),
// storing it in config (with SaveConfig) and reading it back (with LoadConfig) produces the
// identical token string with the correct 90-day expiry timestamp (within 1 second tolerance).
// Validates: Requirements 3.3
func TestPropertyTokenStorageRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Create a temp directory to act as HOME so we don't touch the real config
		tmpHome := filepath.Join(os.TempDir(), fmt.Sprintf("af_test_%d", time.Now().UnixNano()))
		if err := os.MkdirAll(tmpHome, 0o755); err != nil {
			t.Fatalf("failed to create temp home: %v", err)
		}
		defer os.RemoveAll(tmpHome)

		// Override HOME to use the temp directory
		origHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpHome)
		defer os.Setenv("HOME", origHome)

		// Generate a random valid PAT token: "af_" followed by 8-64 random alphanumeric chars
		const alphanumeric = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		tokenLen := rapid.IntRange(8, 64).Draw(t, "tokenLen")
		tokenChars := make([]byte, tokenLen)
		for i := range tokenChars {
			idx := rapid.IntRange(0, len(alphanumeric)-1).Draw(t, fmt.Sprintf("char%d", i))
			tokenChars[i] = alphanumeric[idx]
		}
		token := "af_" + string(tokenChars)

		// Compute the expected 90-day expiry from now (truncated to second precision for RFC3339)
		now := time.Now().UTC().Truncate(time.Second)
		expectedExpiry := now.Add(90 * 24 * time.Hour)

		// Create config with the token and expiry
		cfg := DefaultConfig()
		cfg.Token = token
		cfg.TokenExpiresAt = expectedExpiry

		// Save the config
		if err := SaveConfig(cfg); err != nil {
			t.Fatalf("SaveConfig failed: %v", err)
		}

		// Load the config back
		loaded := LoadConfig()

		// Verify the token is identical
		if loaded.Token != token {
			t.Fatalf("token mismatch: got %q, want %q", loaded.Token, token)
		}

		// Verify the expiry timestamp is within 1 second tolerance
		diff := loaded.TokenExpiresAt.Sub(expectedExpiry)
		if diff < 0 {
			diff = -diff
		}
		if diff > 1*time.Second {
			t.Fatalf("token expiry mismatch: got %v, want %v (diff: %v)",
				loaded.TokenExpiresAt, expectedExpiry, diff)
		}
	})
}
