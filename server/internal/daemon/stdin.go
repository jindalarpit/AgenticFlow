package daemon

import (
	"fmt"
	"io"
	"sync"
)

// StdinPipeManager manages stdin pipes for active tasks.
// It provides thread-safe write access with per-task serialization.
type StdinPipeManager struct {
	mu    sync.RWMutex
	pipes map[string]*taskStdinPipe // task_id -> pipe
}

type taskStdinPipe struct {
	pipe   io.WriteCloser
	mu     sync.Mutex // serializes writes to this specific pipe
	closed bool
}

// NewStdinPipeManager creates a new manager.
func NewStdinPipeManager() *StdinPipeManager {
	return &StdinPipeManager{
		pipes: make(map[string]*taskStdinPipe),
	}
}

// Register stores a stdin pipe for a task. Called when the CLI process is spawned.
func (m *StdinPipeManager) Register(taskID string, pipe io.WriteCloser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pipes[taskID] = &taskStdinPipe{pipe: pipe}
}

// Write writes text to the task's stdin pipe with newline appending.
// Returns an error if the task is not found or the pipe is closed or write fails.
func (m *StdinPipeManager) Write(taskID string, text string) error {
	m.mu.RLock()
	tp, ok := m.pipes[taskID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("stdin: no pipe for task %s", taskID)
	}

	tp.mu.Lock()
	defer tp.mu.Unlock()

	if tp.closed {
		return fmt.Errorf("stdin: pipe closed for task %s", taskID)
	}

	// Append newline if text doesn't already end with one.
	data := EnsureNewline(text)

	_, err := tp.pipe.Write([]byte(data))
	if err != nil {
		return fmt.Errorf("stdin: write failed for task %s: %w", taskID, err)
	}
	return nil
}

// Close closes the stdin pipe for a task and removes it from the map.
func (m *StdinPipeManager) Close(taskID string) {
	m.mu.Lock()
	tp, ok := m.pipes[taskID]
	if ok {
		delete(m.pipes, taskID)
	}
	m.mu.Unlock()

	if ok && !tp.closed {
		tp.mu.Lock()
		tp.closed = true
		tp.pipe.Close()
		tp.mu.Unlock()
	}
}

// EnsureNewline appends a newline to text if it doesn't already end with one.
// If text is empty, returns a single newline.
func EnsureNewline(text string) string {
	if text == "" {
		return "\n"
	}
	if text[len(text)-1] != '\n' {
		return text + "\n"
	}
	return text
}
