package cli

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ServerURL != "" {
		t.Errorf("expected empty ServerURL, got %q", cfg.ServerURL)
	}
	if cfg.PollInterval != 3*time.Second {
		t.Errorf("expected PollInterval 3s, got %s", cfg.PollInterval)
	}
	if cfg.HeartbeatInterval != 15*time.Second {
		t.Errorf("expected HeartbeatInterval 15s, got %s", cfg.HeartbeatInterval)
	}
	if cfg.AgentTimeout != 2*time.Hour {
		t.Errorf("expected AgentTimeout 2h, got %s", cfg.AgentTimeout)
	}
	if cfg.MaxConcurrentTasks != 5 {
		t.Errorf("expected MaxConcurrentTasks 5, got %d", cfg.MaxConcurrentTasks)
	}
	if cfg.HealthPort != 8081 {
		t.Errorf("expected HealthPort 8081, got %d", cfg.HealthPort)
	}
}

func TestConfigMarshalRoundTrip(t *testing.T) {
	original := Config{
		ServerURL:          "http://localhost:8080",
		Token:              "af_test_token_123",
		TokenExpiresAt:     time.Date(2025, 9, 15, 0, 0, 0, 0, time.UTC),
		UserEmail:          "user@example.com",
		PollInterval:       3 * time.Second,
		HeartbeatInterval:  15 * time.Second,
		AgentTimeout:       2 * time.Hour,
		MaxConcurrentTasks: 5,
		HealthPort:         8081,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed Config
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed.ServerURL != original.ServerURL {
		t.Errorf("ServerURL: got %q, want %q", parsed.ServerURL, original.ServerURL)
	}
	if parsed.Token != original.Token {
		t.Errorf("Token: got %q, want %q", parsed.Token, original.Token)
	}
	if !parsed.TokenExpiresAt.Equal(original.TokenExpiresAt) {
		t.Errorf("TokenExpiresAt: got %v, want %v", parsed.TokenExpiresAt, original.TokenExpiresAt)
	}
	if parsed.UserEmail != original.UserEmail {
		t.Errorf("UserEmail: got %q, want %q", parsed.UserEmail, original.UserEmail)
	}
	if parsed.PollInterval != original.PollInterval {
		t.Errorf("PollInterval: got %s, want %s", parsed.PollInterval, original.PollInterval)
	}
	if parsed.HeartbeatInterval != original.HeartbeatInterval {
		t.Errorf("HeartbeatInterval: got %s, want %s", parsed.HeartbeatInterval, original.HeartbeatInterval)
	}
	if parsed.AgentTimeout != original.AgentTimeout {
		t.Errorf("AgentTimeout: got %s, want %s", parsed.AgentTimeout, original.AgentTimeout)
	}
	if parsed.MaxConcurrentTasks != original.MaxConcurrentTasks {
		t.Errorf("MaxConcurrentTasks: got %d, want %d", parsed.MaxConcurrentTasks, original.MaxConcurrentTasks)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "valid default config",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name: "valid with server URL",
			cfg: Config{
				ServerURL:          "https://agenticflow.example.com",
				PollInterval:       3 * time.Second,
				HeartbeatInterval:  15 * time.Second,
				AgentTimeout:       2 * time.Hour,
				MaxConcurrentTasks: 5,
			},
			wantErr: false,
		},
		{
			name: "invalid server URL scheme",
			cfg: Config{
				ServerURL:          "ftp://example.com",
				PollInterval:       3 * time.Second,
				HeartbeatInterval:  15 * time.Second,
				AgentTimeout:       2 * time.Hour,
				MaxConcurrentTasks: 5,
			},
			wantErr: true,
		},
		{
			name: "poll interval too low",
			cfg: Config{
				PollInterval:       500 * time.Millisecond,
				HeartbeatInterval:  15 * time.Second,
				AgentTimeout:       2 * time.Hour,
				MaxConcurrentTasks: 5,
			},
			wantErr: true,
		},
		{
			name: "max concurrent tasks too high",
			cfg: Config{
				PollInterval:       3 * time.Second,
				HeartbeatInterval:  15 * time.Second,
				AgentTimeout:       2 * time.Hour,
				MaxConcurrentTasks: 101,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateField(t *testing.T) {
	tests := []struct {
		key     string
		value   string
		wantErr bool
	}{
		{"server_url", "http://localhost:8080", false},
		{"server_url", "https://example.com", false},
		{"server_url", "", false},
		{"server_url", "ftp://example.com", true},
		{"poll_interval", "3s", false},
		{"poll_interval", "500ms", true},
		{"heartbeat_interval", "15s", false},
		{"heartbeat_interval", "4s", true},
		{"agent_timeout", "2h", false},
		{"agent_timeout", "30s", true},
		{"max_concurrent_tasks", "5", false},
		{"max_concurrent_tasks", "0", true},
		{"unknown_key", "value", true},
	}

	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			err := ValidateField(tt.key, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateField(%q, %q) error = %v, wantErr %v", tt.key, tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestApplyFlags(t *testing.T) {
	cfg := DefaultConfig()

	flags := FlagOverrides{
		ServerURL:          "http://flag-server:9090",
		PollInterval:       10 * time.Second,
		MaxConcurrentTasks: 3,
	}

	ApplyFlags(&cfg, flags)

	if cfg.ServerURL != "http://flag-server:9090" {
		t.Errorf("ServerURL: got %q, want %q", cfg.ServerURL, "http://flag-server:9090")
	}
	if cfg.PollInterval != 10*time.Second {
		t.Errorf("PollInterval: got %s, want 10s", cfg.PollInterval)
	}
	if cfg.MaxConcurrentTasks != 3 {
		t.Errorf("MaxConcurrentTasks: got %d, want 3", cfg.MaxConcurrentTasks)
	}
	// HeartbeatInterval should remain default since flag was zero
	if cfg.HeartbeatInterval != 15*time.Second {
		t.Errorf("HeartbeatInterval should remain default: got %s, want 15s", cfg.HeartbeatInterval)
	}
}

func TestEnvOverrides(t *testing.T) {
	cfg := DefaultConfig()

	os.Setenv(EnvServerURL, "http://env-server:7070")
	os.Setenv(EnvPollInterval, "7s")
	os.Setenv(EnvMaxConcurrent, "8")
	defer func() {
		os.Unsetenv(EnvServerURL)
		os.Unsetenv(EnvPollInterval)
		os.Unsetenv(EnvMaxConcurrent)
	}()

	applyEnvOverrides(&cfg)

	if cfg.ServerURL != "http://env-server:7070" {
		t.Errorf("ServerURL: got %q, want %q", cfg.ServerURL, "http://env-server:7070")
	}
	if cfg.PollInterval != 7*time.Second {
		t.Errorf("PollInterval: got %s, want 7s", cfg.PollInterval)
	}
	if cfg.MaxConcurrentTasks != 8 {
		t.Errorf("MaxConcurrentTasks: got %d, want 8", cfg.MaxConcurrentTasks)
	}
}

func TestConfigPrecedence_FlagsOverrideEnv(t *testing.T) {
	// Set env vars
	os.Setenv(EnvServerURL, "http://env-server:7070")
	os.Setenv(EnvPollInterval, "7s")
	defer func() {
		os.Unsetenv(EnvServerURL)
		os.Unsetenv(EnvPollInterval)
	}()

	cfg := DefaultConfig()
	applyEnvOverrides(&cfg)

	// Flags should override env
	flags := FlagOverrides{
		ServerURL:    "http://flag-server:9090",
		PollInterval: 20 * time.Second,
	}
	ApplyFlags(&cfg, flags)

	if cfg.ServerURL != "http://flag-server:9090" {
		t.Errorf("ServerURL: flags should override env, got %q", cfg.ServerURL)
	}
	if cfg.PollInterval != 20*time.Second {
		t.Errorf("PollInterval: flags should override env, got %s", cfg.PollInterval)
	}
}
