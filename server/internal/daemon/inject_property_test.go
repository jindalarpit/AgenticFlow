package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"pgregory.net/rapid"
)

// Feature: agent-management, Property 10: Provider-Specific Runtime_Brief Injection
// For any non-empty Runtime_Brief and provider type, the injection mechanism SHALL be:
// - SystemPrompt set for "claude", "pi", "opencode", "codex" (prompt unchanged)
// - Prepended to prompt with "\n\n---\n\n" delimiter for "openclaw", "kiro", "kimi" (SystemPrompt empty)
// - Written to AGENTS.md file in workspace for "hermes" and any unknown provider (prompt unchanged, SystemPrompt empty)
// When brief is empty, no injection SHALL occur (prompt unchanged, SystemPrompt empty, no file written).
// **Validates: Requirements 13.3, 13.4, 13.5, 13.6, 13.7, 13.8, 13.9**

// nonEmptyStringGen generates a non-empty string suitable for use as a brief or prompt.
func nonEmptyStringGen() *rapid.Generator[string] {
	return rapid.Custom[string](func(t *rapid.T) string {
		// Generate a string of length 1-500 from printable ASCII.
		length := rapid.IntRange(1, 500).Draw(t, "length")
		chars := make([]byte, length)
		for i := range chars {
			chars[i] = byte(rapid.IntRange(32, 126).Draw(t, "char"))
		}
		return string(chars)
	})
}

// promptGen generates a prompt string (can be empty or non-empty).
func promptGen() *rapid.Generator[string] {
	return rapid.Custom[string](func(t *rapid.T) string {
		length := rapid.IntRange(0, 500).Draw(t, "promptLength")
		if length == 0 {
			return ""
		}
		chars := make([]byte, length)
		for i := range chars {
			chars[i] = byte(rapid.IntRange(32, 126).Draw(t, "promptChar"))
		}
		return string(chars)
	})
}

// unknownProviderGen generates a provider name that is NOT one of the known providers.
func unknownProviderGen() *rapid.Generator[string] {
	return rapid.Custom[string](func(t *rapid.T) string {
		known := map[string]bool{
			"claude": true, "pi": true, "opencode": true,
			"codex": true, "openclaw": true, "kiro": true,
			"kimi": true, "hermes": true,
		}
		// Generate a random string that doesn't match known providers.
		for {
			length := rapid.IntRange(1, 30).Draw(t, "unknownProviderLen")
			chars := make([]byte, length)
			for i := range chars {
				chars[i] = byte(rapid.IntRange(97, 122).Draw(t, "unknownProviderChar")) // lowercase a-z
			}
			name := string(chars)
			if !known[name] {
				return name
			}
		}
	})
}

// TestProperty_SystemPromptProviders_NonEmptyBrief verifies that for providers
// "claude", "pi", "opencode", and "codex", a non-empty brief is set as SystemPrompt
// and the prompt is returned unchanged.
func TestProperty_SystemPromptProviders_NonEmptyBrief(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		provider := rapid.SampledFrom([]string{"claude", "pi", "opencode", "codex"}).Draw(t, "provider")
		brief := nonEmptyStringGen().Draw(t, "brief")
		prompt := promptGen().Draw(t, "prompt")

		opts := &ExecOptions{}
		result, err := InjectBrief(provider, brief, prompt, "/tmp", opts)
		if err != nil {
			t.Fatalf("unexpected error for provider %q: %v", provider, err)
		}

		// Property: prompt SHALL be unchanged.
		if result != prompt {
			t.Fatalf("provider %q: expected prompt unchanged %q, got %q", provider, prompt, result)
		}

		// Property: SystemPrompt SHALL be set to brief.
		if opts.SystemPrompt != brief {
			t.Fatalf("provider %q: expected SystemPrompt = brief %q, got %q", provider, brief, opts.SystemPrompt)
		}
	})
}

// TestProperty_PrependProviders_NonEmptyBrief verifies that for providers
// "openclaw", "kiro", and "kimi", a non-empty brief is prepended to the prompt
// with "\n\n---\n\n" delimiter, and SystemPrompt remains empty.
func TestProperty_PrependProviders_NonEmptyBrief(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		provider := rapid.SampledFrom([]string{"openclaw", "kiro", "kimi"}).Draw(t, "provider")
		brief := nonEmptyStringGen().Draw(t, "brief")
		prompt := promptGen().Draw(t, "prompt")

		opts := &ExecOptions{}
		result, err := InjectBrief(provider, brief, prompt, "/tmp", opts)
		if err != nil {
			t.Fatalf("unexpected error for provider %q: %v", provider, err)
		}

		// Property: prompt SHALL be prepended with brief + "\n\n---\n\n".
		expected := brief + "\n\n---\n\n" + prompt
		if result != expected {
			t.Fatalf("provider %q: expected prepended prompt %q, got %q", provider, expected, result)
		}

		// Property: SystemPrompt SHALL be empty.
		if opts.SystemPrompt != "" {
			t.Fatalf("provider %q: expected empty SystemPrompt, got %q", provider, opts.SystemPrompt)
		}
	})
}

// TestProperty_FileWriteProviders_NonEmptyBrief verifies that for provider "hermes"
// and unknown providers, a non-empty brief is written to AGENTS.md in the workspace,
// the prompt is returned unchanged, and SystemPrompt remains empty.
func TestProperty_FileWriteProviders_NonEmptyBrief(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Test both "hermes" and random unknown providers.
		useHermes := rapid.Bool().Draw(t, "useHermes")
		var provider string
		if useHermes {
			provider = "hermes"
		} else {
			provider = unknownProviderGen().Draw(t, "unknownProvider")
		}

		brief := nonEmptyStringGen().Draw(t, "brief")
		prompt := promptGen().Draw(t, "prompt")

		tmpDir, err := os.MkdirTemp("", "inject_property_test_*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		opts := &ExecOptions{}
		result, err := InjectBrief(provider, brief, prompt, tmpDir, opts)
		if err != nil {
			t.Fatalf("unexpected error for provider %q: %v", provider, err)
		}

		// Property: prompt SHALL be unchanged.
		if result != prompt {
			t.Fatalf("provider %q: expected prompt unchanged %q, got %q", provider, prompt, result)
		}

		// Property: SystemPrompt SHALL be empty.
		if opts.SystemPrompt != "" {
			t.Fatalf("provider %q: expected empty SystemPrompt, got %q", provider, opts.SystemPrompt)
		}

		// Property: AGENTS.md SHALL be written with brief content.
		agentsMDPath := filepath.Join(tmpDir, "AGENTS.md")
		content, err := os.ReadFile(agentsMDPath)
		if err != nil {
			t.Fatalf("provider %q: failed to read AGENTS.md: %v", provider, err)
		}
		if string(content) != brief {
			t.Fatalf("provider %q: expected AGENTS.md content %q, got %q", provider, brief, string(content))
		}
	})
}

// TestProperty_EmptyBrief_NoInjection verifies that for ANY provider and an empty brief,
// no injection occurs: prompt is unchanged, SystemPrompt is empty, and no file is written.
func TestProperty_EmptyBrief_NoInjection(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate any provider: known or unknown.
		allProviders := []string{"claude", "pi", "opencode", "codex", "openclaw", "kiro", "kimi", "hermes"}
		useKnown := rapid.Bool().Draw(t, "useKnown")
		var provider string
		if useKnown {
			provider = rapid.SampledFrom(allProviders).Draw(t, "knownProvider")
		} else {
			provider = unknownProviderGen().Draw(t, "unknownProvider")
		}

		prompt := promptGen().Draw(t, "prompt")

		tmpDir, err := os.MkdirTemp("", "inject_property_test_empty_*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		opts := &ExecOptions{}
		result, err := InjectBrief(provider, "", prompt, tmpDir, opts)
		if err != nil {
			t.Fatalf("unexpected error for provider %q with empty brief: %v", provider, err)
		}

		// Property: prompt SHALL be unchanged.
		if result != prompt {
			t.Fatalf("provider %q: expected prompt unchanged %q, got %q", provider, prompt, result)
		}

		// Property: SystemPrompt SHALL be empty.
		if opts.SystemPrompt != "" {
			t.Fatalf("provider %q: expected empty SystemPrompt with empty brief, got %q", provider, opts.SystemPrompt)
		}

		// Property: no AGENTS.md file SHALL be written.
		agentsMDPath := filepath.Join(tmpDir, "AGENTS.md")
		if _, err := os.Stat(agentsMDPath); err == nil {
			t.Fatalf("provider %q: AGENTS.md should NOT be written when brief is empty", provider)
		}
	})
}
