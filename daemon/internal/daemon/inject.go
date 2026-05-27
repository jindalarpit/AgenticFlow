package daemon

import (
	"fmt"
	"os"
	"path/filepath"
)

// ExecOptions holds the execution options that the injection modifies.
type ExecOptions struct {
	SystemPrompt string
}

// InjectBrief applies the Runtime_Brief to ExecOptions based on provider type.
func InjectBrief(provider string, brief string, prompt string, workspaceDir string, opts *ExecOptions) (string, error) {
	if brief == "" {
		return prompt, nil
	}

	switch provider {
	case "claude", "pi":
		opts.SystemPrompt = brief
	case "opencode":
		opts.SystemPrompt = brief
	case "codex":
		opts.SystemPrompt = brief
	case "openclaw", "kiro", "kimi":
		prompt = brief + "\n\n---\n\n" + prompt
	case "hermes":
		if err := writeAgentsMD(workspaceDir, brief); err != nil {
			return prompt, err
		}
	default:
		if err := writeAgentsMD(workspaceDir, brief); err != nil {
			return prompt, err
		}
	}

	return prompt, nil
}

func writeAgentsMD(workspaceDir string, content string) error {
	path := filepath.Join(workspaceDir, "AGENTS.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write AGENTS.md: %w", err)
	}
	return nil
}
