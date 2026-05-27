// Package agent provides a unified interface for executing prompts via
// coding agents (Claude Code, Gemini, OpenCode, Kiro, Codex, Copilot,
// Cursor, Pi, Hermes, Kimi, OpenClaw).
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Backend is the unified interface for executing prompts via coding agents.
type Backend interface {
	Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error)
}

// ExecOptions configures a single execution.
type ExecOptions struct {
	Cwd             string
	Model           string
	SystemPrompt    string
	Timeout         time.Duration
	CustomArgs      []string
	CustomEnv       map[string]string
	ResumeSessionID string
}

// Session represents a running agent execution.
type Session struct {
	Messages <-chan Message
	Result   <-chan Result
}

// MessageType identifies the kind of Message.
type MessageType string

const (
	MessageText       MessageType = "text"
	MessageThinking   MessageType = "thinking"
	MessageToolUse    MessageType = "tool_use"
	MessageToolResult MessageType = "tool_result"
	MessageError      MessageType = "error"
	MessageStatus     MessageType = "status"
	MessageLog        MessageType = "log"
)

// Message is a unified event emitted by an agent during execution.
type Message struct {
	Type      MessageType
	Content   string
	Tool      string
	CallID    string
	Input     map[string]any
	Output    string
	Status    string
	SessionID string
	Level     string
}

// TokenUsage tracks token consumption for a single model.
type TokenUsage struct {
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
}

// Result is the final outcome after an agent session completes.
type Result struct {
	Status     string
	Output     string
	Error      string
	DurationMs int64
	SessionID  string
	Usage      map[string]TokenUsage
}

// Config configures a Backend instance.
type Config struct {
	ExecutablePath string
	Env            map[string]string
	Logger         *slog.Logger
}

// AgentDef describes a supported agent CLI runtime.
type AgentDef struct {
	Type         string
	CLIName      string
	DisplayName  string
	DefaultModel string
	VersionFlag  string
}

// SupportedAgents is the registry of all known agent CLI runtimes.
var SupportedAgents = []AgentDef{
	{Type: "claude", CLIName: "claude", DisplayName: "Claude Code", DefaultModel: "claude-sonnet-4-20250514", VersionFlag: "--version"},
	{Type: "gemini", CLIName: "gemini", DisplayName: "Gemini CLI", DefaultModel: "gemini-2.5-pro", VersionFlag: "--version"},
	{Type: "codex", CLIName: "codex", DisplayName: "OpenAI Codex", DefaultModel: "o4-mini", VersionFlag: "--version"},
	{Type: "copilot", CLIName: "copilot", DisplayName: "GitHub Copilot", DefaultModel: "gpt-4o", VersionFlag: "--version"},
	{Type: "kiro", CLIName: "kiro", DisplayName: "Kiro CLI", DefaultModel: "kiro-default", VersionFlag: "--version"},
	{Type: "opencode", CLIName: "opencode", DisplayName: "OpenCode", DefaultModel: "opencode-default", VersionFlag: "--version"},
	{Type: "hermes", CLIName: "hermes", DisplayName: "Hermes", DefaultModel: "hermes-default", VersionFlag: "--version"},
	{Type: "kimi", CLIName: "kimi", DisplayName: "Kimi", DefaultModel: "kimi-default", VersionFlag: "--version"},
	{Type: "cursor", CLIName: "cursor", DisplayName: "Cursor", DefaultModel: "cursor-default", VersionFlag: "--version"},
	{Type: "pi", CLIName: "pi", DisplayName: "Pi", DefaultModel: "pi-default", VersionFlag: "--version"},
	{Type: "openclaw", CLIName: "openclaw", DisplayName: "OpenClaw", DefaultModel: "openclaw-default", VersionFlag: "--version"},
}

// SupportedTypes returns a list of all supported agent type strings.
func SupportedTypes() []string {
	types := make([]string, len(SupportedAgents))
	for i, a := range SupportedAgents {
		types[i] = a.Type
	}
	return types
}

// LookupByType returns the AgentDef for the given agent type, or nil if not found.
func LookupByType(agentType string) *AgentDef {
	for i := range SupportedAgents {
		if SupportedAgents[i].Type == agentType {
			return &SupportedAgents[i]
		}
	}
	return nil
}

// LookupByCLIName returns the AgentDef for the given CLI binary name, or nil if not found.
func LookupByCLIName(cliName string) *AgentDef {
	for i := range SupportedAgents {
		if SupportedAgents[i].CLIName == cliName {
			return &SupportedAgents[i]
		}
	}
	return nil
}

// New creates a Backend for the given agent type.
func New(agentType string, cfg Config) (Backend, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	switch agentType {
	case "opencode":
		return &opencodeBackend{cfg: cfg}, nil
	default:
		// All other agents use the generic CLI backend.
		for _, t := range SupportedTypes() {
			if t == agentType {
				return &cliBackend{cfg: cfg, agentType: agentType}, nil
			}
		}
		return nil, fmt.Errorf("unknown agent type: %q (supported: %s)", agentType, strings.Join(SupportedTypes(), ", "))
	}
}
