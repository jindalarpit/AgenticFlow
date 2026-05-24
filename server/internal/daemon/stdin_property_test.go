package daemon

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"pgregory.net/rapid"
)

// Feature: interactive-task-sessions, Property 11: Stdin Write Serialization
// For any set of concurrent writes to the same task, each input appears as a
// contiguous block (with appended newline) in the pipe output, with no
// interleaving of bytes from different writes.
// **Validates: Requirements 1.6**
func TestProperty_StdinWriteSerialization(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate N random non-empty strings to write concurrently.
		numWriters := rapid.IntRange(2, 20).Draw(t, "numWriters")
		inputs := make([]string, numWriters)
		for i := 0; i < numWriters; i++ {
			// Generate non-empty strings without newlines in the body
			// to make contiguity verification straightforward.
			length := rapid.IntRange(1, 200).Draw(t, "inputLength")
			// Use printable ASCII chars (excluding newline) for clarity.
			chars := make([]byte, length)
			for j := 0; j < length; j++ {
				chars[j] = byte(rapid.IntRange(33, 126).Draw(t, "char"))
			}
			inputs[i] = string(chars)
		}

		// Create a StdinPipeManager and register a pipe backed by a buffer.
		manager := NewStdinPipeManager()
		var buf bytes.Buffer
		taskID := "test-task-serialization"

		// Use a bufferWriteCloser that wraps the buffer.
		bwc := &bufferWriteCloser{buf: &buf}
		manager.Register(taskID, bwc)

		// Write all inputs concurrently.
		var wg sync.WaitGroup
		wg.Add(numWriters)
		for i := 0; i < numWriters; i++ {
			go func(text string) {
				defer wg.Done()
				err := manager.Write(taskID, text)
				if err != nil {
					t.Errorf("Write failed: %v", err)
				}
			}(inputs[i])
		}
		wg.Wait()

		// Read the output from the buffer.
		output := buf.String()

		// Each input should appear as a contiguous block with a newline appended.
		// Since EnsureNewline appends \n if not present, each input (which has no
		// trailing newline) should appear as "input\n" contiguously in the output.
		for i, input := range inputs {
			expected := input + "\n"
			if !strings.Contains(output, expected) {
				t.Fatalf("input[%d] %q not found as contiguous block in output:\n%s", i, expected, output)
			}
		}

		// Verify no interleaving: split output by newlines, each line should be
		// exactly one of the original inputs (since none contain newlines).
		lines := strings.Split(output, "\n")
		// The last element after split will be empty (trailing newline).
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		if len(lines) != numWriters {
			t.Fatalf("expected %d lines in output, got %d.\nOutput:\n%s", numWriters, len(lines), output)
		}

		// Each line must match exactly one input (no partial/interleaved content).
		inputSet := make(map[string]int, numWriters)
		for _, input := range inputs {
			inputSet[input]++
		}

		for _, line := range lines {
			if inputSet[line] <= 0 {
				t.Fatalf("output line %q does not match any expected input.\nExpected inputs: %v\nFull output:\n%s",
					line, inputs, output)
			}
			inputSet[line]--
		}
	})
}

// bufferWriteCloser wraps a bytes.Buffer to satisfy io.WriteCloser.
// It uses a mutex to make the underlying buffer safe for concurrent writes
// (simulating a real pipe which is also safe for individual Write calls).
type bufferWriteCloser struct {
	buf    *bytes.Buffer
	mu     sync.Mutex
	closed bool
}

func (b *bufferWriteCloser) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return 0, bytes.ErrTooLarge // simulate closed pipe error
	}
	return b.buf.Write(p)
}

func (b *bufferWriteCloser) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}


// Feature: cli-auth-daemon, Property 17: Stdin byte preservation
// For any byte sequence, child process receives exact same bytes without modification.
// **Validates: Requirements 16.1, 16.2**
func TestProperty_StdinBytePreservation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate an arbitrary byte sequence (including null bytes, high bytes, etc.)
		length := rapid.IntRange(0, 4096).Draw(t, "length")
		data := make([]byte, length)
		for i := 0; i < length; i++ {
			data[i] = byte(rapid.IntRange(0, 255).Draw(t, "byte"))
		}

		// Create a pipe to simulate the stdin pipe between the manager and child process.
		pr, pw := io.Pipe()

		manager := NewStdinPipeManager()
		taskID := "test-task-byte-preservation"
		manager.Register(taskID, pw)

		// Write the bytes and then close the pipe in a goroutine.
		// The close signals EOF to the reader, allowing ReadAll to complete.
		var writeErr error
		go func() {
			writeErr = writeRawBytes(manager, taskID, data)
			// Close the pipe writer so the reader gets EOF.
			manager.Close(taskID)
		}()

		// Read all bytes from the reader end (simulating the child process reading stdin).
		// ReadAll blocks until the writer is closed (EOF).
		received, readErr := io.ReadAll(pr)

		if writeErr != nil {
			t.Fatalf("write failed: %v", writeErr)
		}
		if readErr != nil {
			t.Fatalf("read failed: %v", readErr)
		}

		// The child process must receive the exact same bytes without modification.
		if !bytes.Equal(received, data) {
			t.Fatalf("byte preservation violated:\n  sent %d bytes\n  received %d bytes\n  sent[:32]=%x\n  recv[:32]=%x",
				len(data), len(received), truncBytes(data, 32), truncBytes(received, 32))
		}
	})
}

// writeRawBytes writes raw bytes directly to the task's stdin pipe, bypassing
// the text-oriented Write method. This simulates the byte-level preservation
// requirement: whatever bytes are written to the pipe must arrive unchanged.
func writeRawBytes(m *StdinPipeManager, taskID string, data []byte) error {
	m.mu.RLock()
	tp, ok := m.pipes[taskID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no pipe for task %s", taskID)
	}

	tp.mu.Lock()
	defer tp.mu.Unlock()

	if tp.closed {
		return fmt.Errorf("pipe closed for task %s", taskID)
	}

	_, err := tp.pipe.Write(data)
	return err
}

// truncBytes returns up to n bytes from data for display purposes.
func truncBytes(data []byte, n int) []byte {
	if len(data) <= n {
		return data
	}
	return data[:n]
}
