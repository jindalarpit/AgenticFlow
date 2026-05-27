package mcp

import (
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestInjectMCPConfig_WritesValidJSON(t *testing.T) {
	inj := &Injector{Logger: slog.Default()}

	config := json.RawMessage(`{"filesystem":{"command":"npx","args":["-y","@modelcontextprotocol/server-filesystem","/tmp"]}}`)

	path, err := inj.InjectMCPConfig(config)
	if err != nil {
		t.Fatalf("InjectMCPConfig failed: %v", err)
	}
	defer os.Remove(path)

	// Verify file exists and has correct content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}

	if string(content) != string(config) {
		t.Errorf("content mismatch:\ngot:  %s\nwant: %s", string(content), string(config))
	}

	// Verify file permissions are 0600
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat temp file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestInjectMCPConfig_FileHasJSONExtension(t *testing.T) {
	inj := &Injector{Logger: slog.Default()}

	config := json.RawMessage(`{"test":{"command":"echo"}}`)

	path, err := inj.InjectMCPConfig(config)
	if err != nil {
		t.Fatalf("InjectMCPConfig failed: %v", err)
	}
	defer os.Remove(path)

	if !strings.HasSuffix(path, ".json") {
		t.Errorf("expected .json extension, got path: %s", path)
	}
}

func TestInjectMCPConfig_FileInTempDir(t *testing.T) {
	inj := &Injector{Logger: slog.Default()}

	config := json.RawMessage(`{"test":{"command":"echo"}}`)

	path, err := inj.InjectMCPConfig(config)
	if err != nil {
		t.Fatalf("InjectMCPConfig failed: %v", err)
	}
	defer os.Remove(path)

	if !strings.HasPrefix(path, os.TempDir()) {
		t.Errorf("expected file in temp dir %s, got path: %s", os.TempDir(), path)
	}
}

func TestInjectMCPConfig_FileHasPrefix(t *testing.T) {
	inj := &Injector{Logger: slog.Default()}

	config := json.RawMessage(`{"test":{"command":"echo"}}`)

	path, err := inj.InjectMCPConfig(config)
	if err != nil {
		t.Fatalf("InjectMCPConfig failed: %v", err)
	}
	defer os.Remove(path)

	// The file should contain "agenticflow-mcp-" in its name
	if !strings.Contains(path, "agenticflow-mcp-") {
		t.Errorf("expected 'agenticflow-mcp-' prefix in filename, got path: %s", path)
	}
}

func TestInjectMCPConfig_EmptyConfig(t *testing.T) {
	inj := &Injector{Logger: slog.Default()}

	_, err := inj.InjectMCPConfig(json.RawMessage{})
	if err == nil {
		t.Fatal("expected error for empty config, got nil")
	}
}

func TestInjectMCPConfig_NilConfig(t *testing.T) {
	inj := &Injector{Logger: slog.Default()}

	_, err := inj.InjectMCPConfig(nil)
	if err == nil {
		t.Fatal("expected error for nil config, got nil")
	}
}

func TestCleanupMCPConfig_RemovesFile(t *testing.T) {
	inj := &Injector{Logger: slog.Default()}

	config := json.RawMessage(`{"test":{"command":"echo"}}`)

	path, err := inj.InjectMCPConfig(config)
	if err != nil {
		t.Fatalf("InjectMCPConfig failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("temp file should exist: %v", err)
	}

	// Cleanup
	if err := inj.CleanupMCPConfig(path); err != nil {
		t.Fatalf("CleanupMCPConfig failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file to be removed, but it still exists")
	}
}

func TestCleanupMCPConfig_EmptyPath(t *testing.T) {
	inj := &Injector{Logger: slog.Default()}

	// Should be a no-op
	if err := inj.CleanupMCPConfig(""); err != nil {
		t.Fatalf("CleanupMCPConfig with empty path should not error: %v", err)
	}
}

func TestCleanupMCPConfig_NonExistentFile(t *testing.T) {
	inj := &Injector{Logger: slog.Default()}

	// Should not error for non-existent file
	err := inj.CleanupMCPConfig("/tmp/agenticflow-mcp-nonexistent-12345.json")
	if err != nil {
		t.Fatalf("CleanupMCPConfig for non-existent file should not error: %v", err)
	}
}

func TestMCPArgs(t *testing.T) {
	path := "/tmp/agenticflow-mcp-abc123.json"
	args := MCPArgs(path)

	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != "--mcp-config" {
		t.Errorf("expected first arg '--mcp-config', got %q", args[0])
	}
	if args[1] != path {
		t.Errorf("expected second arg %q, got %q", path, args[1])
	}
}

func TestInjectMCPConfig_NilLogger(t *testing.T) {
	inj := &Injector{Logger: nil}

	config := json.RawMessage(`{"test":{"command":"echo"}}`)

	path, err := inj.InjectMCPConfig(config)
	if err != nil {
		t.Fatalf("InjectMCPConfig with nil logger failed: %v", err)
	}
	defer os.Remove(path)

	// Verify it still works
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	if string(content) != string(config) {
		t.Errorf("content mismatch")
	}
}
