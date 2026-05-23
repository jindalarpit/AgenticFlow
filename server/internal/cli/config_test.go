package cli

import (
	"encoding/json"
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

func TestConfigJSONKeys(t *testing.T) {
	cfg := Config{
		ServerURL:          "http://localhost:8080",
		Token:              "af_tok",
		TokenExpiresAt:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UserEmail:          "test@test.com",
		PollInterval:       5 * time.Second,
		HeartbeatInterval:  10 * time.Second,
		AgentTimeout:       1 * time.Hour,
		MaxConcurrentTasks: 3,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	expectedKeys := []string{
		"server_url", "token", "token_expires_at", "user_email",
		"poll_interval", "heartbeat_interval", "agent_timeout", "max_concurrent_tasks",
	}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q not found", key)
		}
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
			name: "poll interval too high",
			cfg: Config{
				PollInterval:       301 * time.Second,
				HeartbeatInterval:  15 * time.Second,
				AgentTimeout:       2 * time.Hour,
				MaxConcurrentTasks: 5,
			},
			wantErr: true,
		},
		{
			name: "heartbeat interval too low",
			cfg: Config{
				PollInterval:       3 * time.Second,
				HeartbeatInterval:  4 * time.Second,
				AgentTimeout:       2 * time.Hour,
				MaxConcurrentTasks: 5,
			},
			wantErr: true,
		},
		{
			name: "agent timeout too low",
			cfg: Config{
				PollInterval:       3 * time.Second,
				HeartbeatInterval:  15 * time.Second,
				AgentTimeout:       30 * time.Second,
				MaxConcurrentTasks: 5,
			},
			wantErr: true,
		},
		{
			name: "agent timeout too high",
			cfg: Config{
				PollInterval:       3 * time.Second,
				HeartbeatInterval:  15 * time.Second,
				AgentTimeout:       25 * time.Hour,
				MaxConcurrentTasks: 5,
			},
			wantErr: true,
		},
		{
			name: "max concurrent tasks too low",
			cfg: Config{
				PollInterval:       3 * time.Second,
				HeartbeatInterval:  15 * time.Second,
				AgentTimeout:       2 * time.Hour,
				MaxConcurrentTasks: 0,
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
		// server_url
		{"server_url", "http://localhost:8080", false},
		{"server_url", "https://example.com", false},
		{"server_url", "", false}, // allow clearing
		{"server_url", "ftp://example.com", true},
		{"server_url", "not-a-url", true},

		// poll_interval
		{"poll_interval", "1s", false},
		{"poll_interval", "300s", false},
		{"poll_interval", "3s", false},
		{"poll_interval", "500ms", true},
		{"poll_interval", "301s", true},
		{"poll_interval", "invalid", true},

		// heartbeat_interval
		{"heartbeat_interval", "5s", false},
		{"heartbeat_interval", "300s", false},
		{"heartbeat_interval", "4s", true},
		{"heartbeat_interval", "301s", true},

		// agent_timeout
		{"agent_timeout", "1m", false},
		{"agent_timeout", "24h", false},
		{"agent_timeout", "2h", false},
		{"agent_timeout", "30s", true},
		{"agent_timeout", "25h", true},

		// max_concurrent_tasks
		{"max_concurrent_tasks", "1", false},
		{"max_concurrent_tasks", "100", false},
		{"max_concurrent_tasks", "5", false},
		{"max_concurrent_tasks", "0", true},
		{"max_concurrent_tasks", "101", true},
		{"max_concurrent_tasks", "abc", true},

		// unknown key
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

func TestApplyField(t *testing.T) {
	cfg := DefaultConfig()

	if err := cfg.ApplyField("server_url", "http://localhost:9090"); err != nil {
		t.Fatalf("ApplyField server_url: %v", err)
	}
	if cfg.ServerURL != "http://localhost:9090" {
		t.Errorf("ServerURL: got %q, want %q", cfg.ServerURL, "http://localhost:9090")
	}

	if err := cfg.ApplyField("poll_interval", "5s"); err != nil {
		t.Fatalf("ApplyField poll_interval: %v", err)
	}
	if cfg.PollInterval != 5*time.Second {
		t.Errorf("PollInterval: got %s, want 5s", cfg.PollInterval)
	}

	if err := cfg.ApplyField("max_concurrent_tasks", "10"); err != nil {
		t.Fatalf("ApplyField max_concurrent_tasks: %v", err)
	}
	if cfg.MaxConcurrentTasks != 10 {
		t.Errorf("MaxConcurrentTasks: got %d, want 10", cfg.MaxConcurrentTasks)
	}
}

func TestConfigDurationSerialization(t *testing.T) {
	cfg := Config{
		PollInterval:       3 * time.Second,
		HeartbeatInterval:  15 * time.Second,
		AgentTimeout:       2 * time.Hour,
		MaxConcurrentTasks: 5,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	// Verify durations are serialized as strings
	if v, ok := raw["poll_interval"].(string); !ok || v != "3s" {
		t.Errorf("poll_interval: got %v, want \"3s\"", raw["poll_interval"])
	}
	if v, ok := raw["heartbeat_interval"].(string); !ok || v != "15s" {
		t.Errorf("heartbeat_interval: got %v, want \"15s\"", raw["heartbeat_interval"])
	}
	if v, ok := raw["agent_timeout"].(string); !ok || v != "2h0m0s" {
		t.Errorf("agent_timeout: got %v, want \"2h0m0s\"", raw["agent_timeout"])
	}
}
