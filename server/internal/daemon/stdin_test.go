package daemon

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
)

// mockWriteCloser is a test double for io.WriteCloser.
type mockWriteCloser struct {
	buf    bytes.Buffer
	mu     sync.Mutex
	closed bool
	err    error // if set, Write returns this error
}

func (m *mockWriteCloser) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return 0, m.err
	}
	return m.buf.Write(p)
}

func (m *mockWriteCloser) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockWriteCloser) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.String()
}

func (m *mockWriteCloser) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func TestNewStdinPipeManager(t *testing.T) {
	mgr := NewStdinPipeManager()
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
	if mgr.pipes == nil {
		t.Fatal("expected non-nil pipes map")
	}
}

func TestStdinPipeManager_Register(t *testing.T) {
	mgr := NewStdinPipeManager()
	pipe := &mockWriteCloser{}

	mgr.Register("task-1", pipe)

	mgr.mu.RLock()
	tp, ok := mgr.pipes["task-1"]
	mgr.mu.RUnlock()

	if !ok {
		t.Fatal("expected pipe to be registered")
	}
	if tp.pipe != pipe {
		t.Fatal("expected registered pipe to match")
	}
	if tp.closed {
		t.Fatal("expected pipe to not be closed")
	}
}

func TestStdinPipeManager_Write_Success(t *testing.T) {
	mgr := NewStdinPipeManager()
	pipe := &mockWriteCloser{}
	mgr.Register("task-1", pipe)

	err := mgr.Write("task-1", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := pipe.String()
	if got != "hello\n" {
		t.Fatalf("expected %q, got %q", "hello\n", got)
	}
}

func TestStdinPipeManager_Write_AlreadyHasNewline(t *testing.T) {
	mgr := NewStdinPipeManager()
	pipe := &mockWriteCloser{}
	mgr.Register("task-1", pipe)

	err := mgr.Write("task-1", "hello\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := pipe.String()
	if got != "hello\n" {
		t.Fatalf("expected %q, got %q", "hello\n", got)
	}
}

func TestStdinPipeManager_Write_EmptyText(t *testing.T) {
	mgr := NewStdinPipeManager()
	pipe := &mockWriteCloser{}
	mgr.Register("task-1", pipe)

	err := mgr.Write("task-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := pipe.String()
	if got != "\n" {
		t.Fatalf("expected %q, got %q", "\n", got)
	}
}

func TestStdinPipeManager_Write_TaskNotFound(t *testing.T) {
	mgr := NewStdinPipeManager()

	err := mgr.Write("nonexistent", "hello")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestStdinPipeManager_Write_PipeClosed(t *testing.T) {
	mgr := NewStdinPipeManager()
	pipe := &mockWriteCloser{}
	mgr.Register("task-1", pipe)

	// Close the pipe via the manager.
	mgr.Close("task-1")

	err := mgr.Write("task-1", "hello")
	if err == nil {
		t.Fatal("expected error for closed pipe")
	}
}

func TestStdinPipeManager_Write_PipeError(t *testing.T) {
	mgr := NewStdinPipeManager()
	pipe := &mockWriteCloser{err: errors.New("broken pipe")}
	mgr.Register("task-1", pipe)

	err := mgr.Write("task-1", "hello")
	if err == nil {
		t.Fatal("expected error for broken pipe")
	}
}

func TestStdinPipeManager_Close(t *testing.T) {
	mgr := NewStdinPipeManager()
	pipe := &mockWriteCloser{}
	mgr.Register("task-1", pipe)

	mgr.Close("task-1")

	// Pipe should be closed.
	if !pipe.IsClosed() {
		t.Fatal("expected pipe to be closed")
	}

	// Task should be removed from map.
	mgr.mu.RLock()
	_, ok := mgr.pipes["task-1"]
	mgr.mu.RUnlock()
	if ok {
		t.Fatal("expected task to be removed from map")
	}
}

func TestStdinPipeManager_Close_Nonexistent(t *testing.T) {
	mgr := NewStdinPipeManager()
	// Should not panic.
	mgr.Close("nonexistent")
}

func TestStdinPipeManager_Close_Idempotent(t *testing.T) {
	mgr := NewStdinPipeManager()
	pipe := &mockWriteCloser{}
	mgr.Register("task-1", pipe)

	mgr.Close("task-1")
	mgr.Close("task-1") // Should not panic.

	if !pipe.IsClosed() {
		t.Fatal("expected pipe to be closed")
	}
}

func TestStdinPipeManager_ConcurrentWrites(t *testing.T) {
	mgr := NewStdinPipeManager()
	pipe := &mockWriteCloser{}
	mgr.Register("task-1", pipe)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mgr.Write("task-1", "data")
		}()
	}
	wg.Wait()

	// All writes should have completed without interleaving.
	got := pipe.String()
	expected := ""
	for i := 0; i < 10; i++ {
		expected += "data\n"
	}
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestStdinPipeManager_IsolatedTasks(t *testing.T) {
	mgr := NewStdinPipeManager()
	pipe1 := &mockWriteCloser{}
	pipe2 := &mockWriteCloser{}
	mgr.Register("task-1", pipe1)
	mgr.Register("task-2", pipe2)

	if err := mgr.Write("task-1", "for-task-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mgr.Write("task-2", "for-task-2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pipe1.String() != "for-task-1\n" {
		t.Fatalf("task-1 pipe got wrong data: %q", pipe1.String())
	}
	if pipe2.String() != "for-task-2\n" {
		t.Fatalf("task-2 pipe got wrong data: %q", pipe2.String())
	}
}

func TestEnsureNewline(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", "\n"},
		{"no trailing newline", "hello", "hello\n"},
		{"has trailing newline", "hello\n", "hello\n"},
		{"only newline", "\n", "\n"},
		{"multiple newlines", "hello\n\n", "hello\n\n"},
		{"whitespace no newline", "  ", "  \n"},
		{"whitespace with newline", "  \n", "  \n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnsureNewline(tt.input)
			if got != tt.want {
				t.Errorf("EnsureNewline(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// Verify that io.WriteCloser interface is satisfied by our mock.
var _ io.WriteCloser = (*mockWriteCloser)(nil)
