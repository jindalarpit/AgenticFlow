package mcp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// Injector handles writing MCP configuration to temporary files for agent CLI
// consumption and cleaning them up after execution completes.
type Injector struct {
	Logger *slog.Logger
}

// InjectMCPConfig writes MCP config JSON to a temporary file and returns the path.
// The caller is responsible for cleanup via CleanupMCPConfig after the agent
// process completes (regardless of success or failure).
func (inj *Injector) InjectMCPConfig(mcpConfig json.RawMessage) (string, error) {
	if len(mcpConfig) == 0 {
		return "", fmt.Errorf("mcp config is empty")
	}

	tmpFile, err := os.CreateTemp(os.TempDir(), "agenticflow-mcp-*.json")
	if err != nil {
		return "", fmt.Errorf("failed to create MCP config temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(mcpConfig); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write MCP config: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to close MCP config temp file: %w", err)
	}

	// Set permissions to 0600 (owner read/write only)
	if err := os.Chmod(tmpPath, 0600); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to set MCP config file permissions: %w", err)
	}

	if inj.Logger != nil {
		inj.Logger.Info("wrote MCP config", "path", tmpPath, "size", len(mcpConfig))
	}

	return tmpPath, nil
}

// CleanupMCPConfig removes the temporary MCP config file. It is safe to call
// with an empty path (no-op). Errors during removal are logged but not returned
// to avoid masking the original task result.
func (inj *Injector) CleanupMCPConfig(path string) error {
	if path == "" {
		return nil
	}

	// Verify the file is in the expected temp directory to avoid accidental deletion
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve MCP config path: %w", err)
	}

	if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
		if inj.Logger != nil {
			inj.Logger.Warn("failed to remove MCP config temp file", "path", absPath, "error", err)
		}
		return fmt.Errorf("failed to remove MCP config temp file: %w", err)
	}

	if inj.Logger != nil {
		inj.Logger.Info("cleaned up MCP config", "path", absPath)
	}

	return nil
}

// MCPArgs returns the CLI arguments for MCP config injection.
// The returned slice is ["--mcp-config", tempFilePath].
func MCPArgs(tempFilePath string) []string {
	return []string{"--mcp-config", tempFilePath}
}
