// Package agent provides a unified interface for executing prompts via
// coding agents (Claude Code, Gemini, OpenCode, Kiro, Codex, Copilot,
// Cursor, Pi, Hermes, Kimi, OpenClaw). It mirrors the multica AgentBackend
// pattern, translated to idiomatic Go.
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
	// Execute runs a prompt and returns a Session for streaming results.
	// The caller should read from Session.Messages and wait on
	// Session.Result for the final outcome.
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
	// Messages streams events as the agent works. The channel is closed
	// when the agent finishes (before Result is sent).
	Messages <-chan Message
	// Result receives exactly one value — the final outcome — then closes.
	Result <-chan Result
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
	Content   string         // text content (Text, Thinking, Error, Log)
	Tool      string         // tool name (ToolUse, ToolResult)
	CallID    string         // tool call ID (ToolUse, ToolResult)
	Input     map[string]any // tool input (ToolUse)
	Output    string         // tool output (ToolResult)
	Status    string         // agent status string (Status)
	SessionID string         // backend session id (Status)
	Level     string         // log level (Log)
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
	Status     string // "completed", "failed", "aborted", "timeout"
	Output     string
	Error      string
	DurationMs int64
	SessionID  string
	Usage      map[string]TokenUsage // keyed by model name
}

// Config configures a Backend instance.
type Config struct {
	ExecutablePath string            // path to CLI binary
	Env            map[string]string // extra environment variables
	Logger         *slog.Logger
}

// SupportedTypes lists all agent types supported by the factory.
var SupportedTypes = []string{
	"claude",
	"gemini",
	"opencode",
	"kiro",
	"hermes",
	"kimi",
	"codex",
	"copilot",
	"cursor",
	"pi",
	"openclaw",
}

// New creates a Backend for the given agent type.
func New(agentType string, cfg Config) (Backend, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	switch agentType {
	case "claude":
		return &claudeBackend{cfg: cfg}, nil
	case "gemini":
		return &geminiBackend{cfg: cfg}, nil
	case "opencode":
		return &opencodeBackend{cfg: cfg}, nil
	case "kiro":
		return &kiroBackend{cfg: cfg}, nil
	case "hermes":
		return &hermesBackend{cfg: cfg}, nil
	case "kimi":
		return &kimiBackend{cfg: cfg}, nil
	case "codex":
		return &codexBackend{cfg: cfg}, nil
	case "copilot":
		return &copilotBackend{cfg: cfg}, nil
	case "cursor":
		return &cursorBackend{cfg: cfg}, nil
	case "pi":
		return &piBackend{cfg: cfg}, nil
	case "openclaw":
		return &openclawBackend{cfg: cfg}, nil
	default:
		return nil, fmt.Errorf("unknown agent type: %q (supported: %s)", agentType, strings.Join(SupportedTypes, ", "))
	}
}
