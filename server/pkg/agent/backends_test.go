package agent

import (
	"log/slog"
	"testing"
)

// ── Backend factory tests for all 11 types ──

func TestFactory_AllSupportedTypes(t *testing.T) {
	t.Parallel()

	for _, agentType := range SupportedTypes {
		agentType := agentType
		t.Run(agentType, func(t *testing.T) {
			t.Parallel()
			backend, err := New(agentType, Config{Logger: slog.Default()})
			if err != nil {
				t.Fatalf("New(%q) returned error: %v", agentType, err)
			}
			if backend == nil {
				t.Fatalf("New(%q) returned nil backend", agentType)
			}
		})
	}
}

func TestFactory_AllElevenTypesRegistered(t *testing.T) {
	t.Parallel()

	expected := []string{
		"claude", "gemini", "opencode", "kiro", "hermes",
		"kimi", "codex", "copilot", "cursor", "pi", "openclaw",
	}
	if len(SupportedTypes) != len(expected) {
		t.Fatalf("expected %d supported types, got %d: %v", len(expected), len(SupportedTypes), SupportedTypes)
	}
	for _, e := range expected {
		found := false
		for _, s := range SupportedTypes {
			if s == e {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in SupportedTypes, not found", e)
		}
	}
}

// ── OpenCode arg building tests ──

func TestBuildOpencodeArgs(t *testing.T) {
	t.Parallel()

	args := buildOpencodeArgs("fix the bug", ExecOptions{Cwd: "/workspace"}, slog.Default())

	// Must include run, --format json, --dangerously-skip-permissions, --dir
	mustContain := []string{"run", "--format", "json", "--dangerously-skip-permissions", "--dir", "/workspace"}
	for _, want := range mustContain {
		found := false
		for _, a := range args {
			if a == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in args: %v", want, args)
		}
	}

	// Prompt should be the last arg.
	if args[len(args)-1] != "fix the bug" {
		t.Fatalf("expected prompt as last arg, got %q in: %v", args[len(args)-1], args)
	}
}

// ── Copilot arg building tests ──

func TestBuildCopilotArgs(t *testing.T) {
	t.Parallel()

	args := buildCopilotArgs("write tests", ExecOptions{Model: "gpt-4"}, slog.Default())

	mustContain := []string{"-p", "write tests", "--output-format", "json", "--allow-all", "--no-ask-user", "--model", "gpt-4"}
	for _, want := range mustContain {
		found := false
		for _, a := range args {
			if a == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in args: %v", want, args)
		}
	}
}

// ── Cursor arg building tests ──

func TestBuildCursorArgs(t *testing.T) {
	t.Parallel()

	args := buildCursorArgs("refactor code", ExecOptions{Cwd: "/project"}, slog.Default())

	mustContain := []string{"chat", "-p", "refactor code", "--output-format", "stream-json", "--yolo", "--workspace", "/project"}
	for _, want := range mustContain {
		found := false
		for _, a := range args {
			if a == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in args: %v", want, args)
		}
	}
}

func TestStripCursorPrefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input, want string
	}{
		{"stdout:{\"type\":\"text\"}", "{\"type\":\"text\"}"},
		{"stderr:warning", "warning"},
		{"{\"type\":\"text\"}", "{\"type\":\"text\"}"},
		{"", ""},
	}
	for _, tc := range cases {
		got := stripCursorPrefix(tc.input)
		if got != tc.want {
			t.Errorf("stripCursorPrefix(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ── Pi arg building tests ──

func TestBuildPiArgs(t *testing.T) {
	t.Parallel()

	args := buildPiArgs("analyze code", ExecOptions{Cwd: "/work", Model: "claude-3"}, slog.Default())

	mustContain := []string{"-p", "--mode", "json", "--session", "--model", "claude-3", "analyze code"}
	for _, want := range mustContain {
		found := false
		for _, a := range args {
			if a == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in args: %v", want, args)
		}
	}
}

// ── OpenClaw arg building tests ──

func TestBuildOpenclawArgs(t *testing.T) {
	t.Parallel()

	args := buildOpenclawArgs("deploy app", ExecOptions{Model: "gpt-4o"}, slog.Default())

	mustContain := []string{"agent", "--local", "--json", "--session-id", "--message", "deploy app", "--model", "gpt-4o"}
	for _, want := range mustContain {
		found := false
		for _, a := range args {
			if a == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in args: %v", want, args)
		}
	}
}

// ── ACP tool name normalization tests (for Kiro/Kimi) ──

func TestNormalizeACPToolName_KnownTitles(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input, want string
	}{
		{"Read file", "read_file"},
		{"Write file", "write_file"},
		{"Run command", "terminal"},
		{"List directory", "list_directory"},
		{"Search files", "search_files"},
		{"Web search", "web_search"},
		{"Read file: /path/to/file.go", "read_file"},
	}
	for _, tc := range cases {
		got := NormalizeACPToolName(tc.input)
		if got != tc.want {
			t.Errorf("NormalizeACPToolName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
