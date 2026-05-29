// Package execution handles task execution, process spawning, and output streaming.
package execution

import (
	"context"
	"io"
)

// ExecutionStrategy defines the interface for different execution backends.
// Implementations handle the actual process spawning and output collection,
// while the Executor orchestrates lifecycle, hooks, and reporting.
type ExecutionStrategy interface {
	// Execute runs the task synchronously and returns the result.
	Execute(ctx context.Context, cfg TaskConfig, hooks ExecutionHooks) Result
}

// ExecutionHooks provides optional hooks that modify execution behavior.
// All fields are optional — nil values are ignored.
type ExecutionHooks struct {
	// OnStdout is called with each chunk of stdout output.
	// Used for input detection (detecting when the CLI is waiting for input).
	OnStdout func([]byte)

	// OnStderr is called with each chunk of stderr output.
	OnStderr func([]byte)

	// BriefInjector transforms the prompt before execution by injecting
	// a runtime brief (agent identity and instructions).
	// Returns the modified prompt and an optional system prompt override.
	BriefInjector func(prompt string) (newPrompt string, systemPrompt string, err error)

	// StdinProvider returns an io.WriteCloser for writing to the process stdin.
	// If non-nil, the execution will set up a stdin pipe and provide it via this callback.
	StdinProvider func(stdin io.WriteCloser)
}
