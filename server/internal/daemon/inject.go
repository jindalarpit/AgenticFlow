package daemon

import (
	"fmt"
	"os"
	"path/filepath"
)

// ExecOptions holds the execution options that the injection modifies.
// The SystemPrompt field is consumed by provider-specific backends:
//   - claude/pi: passed via --append-system-prompt
//   - opencode: passed via --prompt
//   - codex: passed as developerInstructions
type ExecOptions struct {
	SystemPrompt string
}

// InjectBrief applies the Runtime_Brief to ExecOptions based on provider type.
// It routes the brief through the appropriate mechanism for each provider:
//   - claude, pi: sets SystemPrompt (backend uses --append-system-prompt)
//   - opencode: sets SystemPrompt (backend uses --prompt)
//   - codex: sets SystemPrompt (backend uses developerInstructions)
//   - openclaw, kiro, kimi: prepends brief to prompt with delimiter
//   - hermes: writes AGENTS.md file in workspace
//   - unknown: falls back to AGENTS.md file in workspace
//
// When brief is empty, no injection occurs and the prompt is returned unchanged.
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
		// Unknown provider: fallback to AGENTS.md
		if err := writeAgentsMD(workspaceDir, brief); err != nil {
			return prompt, err
		}
	}

	return prompt, nil
}

// writeAgentsMD writes the brief content to an AGENTS.md file in the given
// workspace directory. The file is created with 0644 permissions.
func writeAgentsMD(workspaceDir string, content string) error {
	path := filepath.Join(workspaceDir, "AGENTS.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write AGENTS.md: %w", err)
	}
	return nil
}
