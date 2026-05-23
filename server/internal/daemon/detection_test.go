package daemon

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// fakeStat returns a fake FileInfo for testing.
func fakeStat(name string) (os.FileInfo, error) {
	return nil, os.ErrNotExist
}

func TestDetectAgents_NoAgentsFound(t *testing.T) {
	deps := &DetectionDeps{
		LookPath: func(file string) (string, error) {
			return "", errors.New("not found")
		},
		Getenv: func(key string) string {
			return ""
		},
		Stat: fakeStat,
		DetectVersion: func(ctx context.Context, path string) (string, error) {
			return "1.0.0", nil
		},
	}

	agents := DetectAgents(deps)
	if len(agents) != 0 {
		t.Fatalf("expected 0 agents, got %d", len(agents))
	}
}

func TestDetectAgents_FindsAgentOnPATH(t *testing.T) {
	deps := &DetectionDeps{
		LookPath: func(file string) (string, error) {
			if file == "claude" {
				return "/usr/local/bin/claude", nil
			}
			return "", errors.New("not found")
		},
		Getenv: func(key string) string {
			return ""
		},
		Stat: fakeStat,
		DetectVersion: func(ctx context.Context, path string) (string, error) {
			return "2.1.0", nil
		},
	}

	agents := DetectAgents(deps)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	entry, ok := agents["claude"]
	if !ok {
		t.Fatal("expected claude agent to be detected")
	}
	if entry.Name != "claude" {
		t.Errorf("expected name 'claude', got %q", entry.Name)
	}
	if entry.Path != "/usr/local/bin/claude" {
		t.Errorf("expected path '/usr/local/bin/claude', got %q", entry.Path)
	}
	if entry.Version != "2.1.0" {
		t.Errorf("expected version '2.1.0', got %q", entry.Version)
	}
	if entry.Model != "" {
		t.Errorf("expected empty model, got %q", entry.Model)
	}
}

func TestDetectAgents_CustomPathTakesPrecedence(t *testing.T) {
	deps := &DetectionDeps{
		LookPath: func(file string) (string, error) {
			// claude is also on PATH, but custom path should win
			if file == "claude" {
				return "/usr/local/bin/claude", nil
			}
			return "", errors.New("not found")
		},
		Getenv: func(key string) string {
			if key == "AF_CLAUDE_PATH" {
				return "/custom/path/claude"
			}
			return ""
		},
		Stat: func(name string) (os.FileInfo, error) {
			if name == "/custom/path/claude" {
				// Return a non-nil FileInfo (we just need err == nil and !IsDir)
				return fakeFileInfo{isDir: false}, nil
			}
			return nil, os.ErrNotExist
		},
		DetectVersion: func(ctx context.Context, path string) (string, error) {
			return "3.0.0", nil
		},
	}

	agents := DetectAgents(deps)
	entry, ok := agents["claude"]
	if !ok {
		t.Fatal("expected claude agent to be detected")
	}
	if entry.Path != "/custom/path/claude" {
		t.Errorf("expected custom path '/custom/path/claude', got %q", entry.Path)
	}
}

func TestDetectAgents_InvalidCustomPathSkipsAgent(t *testing.T) {
	deps := &DetectionDeps{
		LookPath: func(file string) (string, error) {
			// claude is on PATH, but custom path is invalid
			if file == "claude" {
				return "/usr/local/bin/claude", nil
			}
			return "", errors.New("not found")
		},
		Getenv: func(key string) string {
			if key == "AF_CLAUDE_PATH" {
				return "/nonexistent/path/claude"
			}
			return ""
		},
		Stat: func(name string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		DetectVersion: func(ctx context.Context, path string) (string, error) {
			return "1.0.0", nil
		},
	}

	agents := DetectAgents(deps)
	if _, ok := agents["claude"]; ok {
		t.Fatal("expected claude to be skipped due to invalid custom path")
	}
}

func TestDetectAgents_ModelOverrideFromEnv(t *testing.T) {
	deps := &DetectionDeps{
		LookPath: func(file string) (string, error) {
			if file == "gemini" {
				return "/usr/bin/gemini", nil
			}
			return "", errors.New("not found")
		},
		Getenv: func(key string) string {
			if key == "AF_GEMINI_MODEL" {
				return "gemini-2.5-pro"
			}
			return ""
		},
		Stat: fakeStat,
		DetectVersion: func(ctx context.Context, path string) (string, error) {
			return "1.5.0", nil
		},
	}

	agents := DetectAgents(deps)
	entry, ok := agents["gemini"]
	if !ok {
		t.Fatal("expected gemini agent to be detected")
	}
	if entry.Model != "gemini-2.5-pro" {
		t.Errorf("expected model 'gemini-2.5-pro', got %q", entry.Model)
	}
}

func TestDetectAgents_VersionFallbackToUnknown(t *testing.T) {
	deps := &DetectionDeps{
		LookPath: func(file string) (string, error) {
			if file == "codex" {
				return "/usr/local/bin/codex", nil
			}
			return "", errors.New("not found")
		},
		Getenv: func(key string) string {
			return ""
		},
		Stat: fakeStat,
		DetectVersion: func(ctx context.Context, path string) (string, error) {
			return "", errors.New("version detection failed")
		},
	}

	agents := DetectAgents(deps)
	entry, ok := agents["codex"]
	if !ok {
		t.Fatal("expected codex agent to be detected")
	}
	if entry.Version != "unknown" {
		t.Errorf("expected version 'unknown', got %q", entry.Version)
	}
}

func TestDetectAgents_MultipleAgents(t *testing.T) {
	available := map[string]string{
		"claude":  "/usr/local/bin/claude",
		"gemini":  "/usr/local/bin/gemini",
		"codex":   "/usr/local/bin/codex",
		"hermes":  "/usr/local/bin/hermes",
		"opencode": "/usr/local/bin/opencode",
	}

	deps := &DetectionDeps{
		LookPath: func(file string) (string, error) {
			if path, ok := available[file]; ok {
				return path, nil
			}
			return "", errors.New("not found")
		},
		Getenv: func(key string) string {
			return ""
		},
		Stat: fakeStat,
		DetectVersion: func(ctx context.Context, path string) (string, error) {
			return "1.0.0", nil
		},
	}

	agents := DetectAgents(deps)
	if len(agents) != 5 {
		t.Fatalf("expected 5 agents, got %d", len(agents))
	}
	for name := range available {
		if _, ok := agents[name]; !ok {
			t.Errorf("expected agent %q to be detected", name)
		}
	}
}

func TestDetectAgents_NilDepsUsesDefaults(t *testing.T) {
	// This test just verifies that passing nil deps doesn't panic.
	// The actual detection depends on the system state.
	agents := DetectAgents(nil)
	// We can't assert on the result since it depends on what's installed,
	// but it should not panic.
	_ = agents
}

func TestDetectAgents_CustomPathIsDirectory(t *testing.T) {
	deps := &DetectionDeps{
		LookPath: func(file string) (string, error) {
			return "", errors.New("not found")
		},
		Getenv: func(key string) string {
			if key == "AF_CLAUDE_PATH" {
				return "/some/directory"
			}
			return ""
		},
		Stat: func(name string) (os.FileInfo, error) {
			if name == "/some/directory" {
				return fakeFileInfo{isDir: true}, nil
			}
			return nil, os.ErrNotExist
		},
		DetectVersion: func(ctx context.Context, path string) (string, error) {
			return "1.0.0", nil
		},
	}

	agents := DetectAgents(deps)
	if _, ok := agents["claude"]; ok {
		t.Fatal("expected claude to be skipped when custom path is a directory")
	}
}

// fakeFileInfo implements os.FileInfo for testing.
type fakeFileInfo struct {
	isDir bool
}

func (f fakeFileInfo) Name() string      { return "fake" }
func (f fakeFileInfo) Size() int64       { return 0 }
func (f fakeFileInfo) Mode() os.FileMode { return 0755 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool       { return f.isDir }
func (f fakeFileInfo) Sys() interface{}  { return nil }
