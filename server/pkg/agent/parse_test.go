package agent

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// ── newLineScanner tests ──

func TestNewLineScanner_BasicLines(t *testing.T) {
	input := "line1\nline2\nline3\n"
	scanner := newLineScanner(strings.NewReader(input))

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("unexpected scanner error: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func TestNewLineScanner_LargeLine(t *testing.T) {
	// Create a line larger than the default bufio.Scanner buffer (64KB)
	// but within our configured max (10MB).
	largeLine := strings.Repeat("x", 512*1024) // 512KB
	input := largeLine + "\n"
	scanner := newLineScanner(strings.NewReader(input))

	if !scanner.Scan() {
		t.Fatalf("expected to scan large line, err: %v", scanner.Err())
	}
	if len(scanner.Text()) != 512*1024 {
		t.Fatalf("expected line length 512KB, got %d", len(scanner.Text()))
	}
}

func TestNewLineScanner_EmptyInput(t *testing.T) {
	scanner := newLineScanner(strings.NewReader(""))
	if scanner.Scan() {
		t.Fatal("expected no lines from empty input")
	}
}

// ── stderrTail tests ──

func TestStderrTail_BasicCapture(t *testing.T) {
	inner := &bytes.Buffer{}
	tail := newStderrTail(inner, 4096)

	_, err := tail.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	if got := tail.Tail(); got != "hello world" {
		t.Fatalf("expected 'hello world', got %q", got)
	}
	// Verify inner writer also received the data.
	if inner.String() != "hello world" {
		t.Fatalf("inner writer missing data: %q", inner.String())
	}
}

func TestStderrTail_BoundedAt4KB(t *testing.T) {
	inner := io.Discard
	tail := newStderrTail(inner, stderrTailSize)

	// Write more than 4KB.
	data := strings.Repeat("A", 5000)
	_, err := tail.Write([]byte(data))
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	got := tail.Tail()
	if len(got) > stderrTailSize {
		t.Fatalf("tail exceeded max size: got %d bytes, max %d", len(got), stderrTailSize)
	}
	// Should contain the last 4096 bytes.
	if len(got) != stderrTailSize {
		t.Fatalf("expected tail of %d bytes, got %d", stderrTailSize, len(got))
	}
}

func TestStderrTail_RetainsLastBytes(t *testing.T) {
	inner := io.Discard
	tail := newStderrTail(inner, 10)

	// Write in multiple chunks.
	tail.Write([]byte("12345"))
	tail.Write([]byte("67890"))
	tail.Write([]byte("ABCDE"))

	got := tail.Tail()
	// Should retain the last 10 bytes: "67890ABCDE" → after trim, same.
	// Actually: buf = "1234567890ABCDE", trimmed to last 10 = "890ABCDE" wait...
	// Let's trace: after first write: buf = "12345" (5 bytes, ≤ 10)
	// After second write: buf = "1234567890" (10 bytes, ≤ 10)
	// After third write: buf = "1234567890ABCDE" (15 bytes, > 10) → trim to last 10 = "0ABCDE" wait no...
	// buf[15-10:] = buf[5:] = "67890ABCDE"
	if got != "67890ABCDE" {
		t.Fatalf("expected '67890ABCDE', got %q", got)
	}
}

func TestStderrTail_DefaultSize(t *testing.T) {
	inner := io.Discard
	tail := newStderrTail(inner, 0) // should default to stderrTailSize

	data := strings.Repeat("B", stderrTailSize+100)
	tail.Write([]byte(data))

	got := tail.Tail()
	if len(got) != stderrTailSize {
		t.Fatalf("expected default tail size %d, got %d", stderrTailSize, len(got))
	}
}

func TestStderrTail_ConcurrentWrites(t *testing.T) {
	inner := io.Discard
	tail := newStderrTail(inner, 4096)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tail.Write([]byte("concurrent write\n"))
		}()
	}
	wg.Wait()

	got := tail.Tail()
	if got == "" {
		t.Fatal("expected non-empty tail after concurrent writes")
	}
	if len(tail.buf) > 4096 {
		t.Fatalf("tail buffer exceeded max after concurrent writes: %d", len(tail.buf))
	}
}

func TestStderrTail_EmptyTail(t *testing.T) {
	inner := io.Discard
	tail := newStderrTail(inner, 4096)

	if got := tail.Tail(); got != "" {
		t.Fatalf("expected empty tail, got %q", got)
	}
}

func TestStderrTail_WhitespaceOnlyTrimmed(t *testing.T) {
	inner := io.Discard
	tail := newStderrTail(inner, 4096)

	tail.Write([]byte("  \n\t  \n  "))

	if got := tail.Tail(); got != "" {
		t.Fatalf("expected empty tail for whitespace-only, got %q", got)
	}
}

// ── withStderrTail tests ──

func TestWithStderrTail_NonEmpty(t *testing.T) {
	got := withStderrTail("process failed", "claude", "segfault at 0x0")
	expected := "process failed; claude stderr: segfault at 0x0"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestWithStderrTail_EmptyTail(t *testing.T) {
	got := withStderrTail("process failed", "claude", "")
	if got != "process failed" {
		t.Fatalf("expected unchanged message, got %q", got)
	}
}

// ── closeOnCancel tests ──

func TestCloseOnCancel_ClosesOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pr, pw := io.Pipe()

	closeOnCancel(ctx, pr)

	// Write should work before cancel.
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	// After cancel, reading from pr should return an error.
	time.Sleep(50 * time.Millisecond)
	_, err := pr.Read(make([]byte, 1))
	if err == nil {
		t.Fatal("expected read error after context cancel")
	}
	_ = pw.Close()
}

// ── trySend tests ──

func TestTrySend_SendsWhenBuffered(t *testing.T) {
	ch := make(chan Message, 1)
	msg := Message{Type: MessageText, Content: "hello"}
	trySend(ch, msg)

	select {
	case got := <-ch:
		if got.Content != "hello" {
			t.Fatalf("expected 'hello', got %q", got.Content)
		}
	default:
		t.Fatal("expected message in channel")
	}
}

func TestTrySend_DropsWhenFull(t *testing.T) {
	ch := make(chan Message, 1)
	// Fill the channel.
	ch <- Message{Type: MessageText, Content: "first"}
	// This should not block.
	trySend(ch, Message{Type: MessageText, Content: "second"})

	got := <-ch
	if got.Content != "first" {
		t.Fatalf("expected 'first', got %q", got.Content)
	}
}

// ── buildEnv tests ──

func TestBuildEnv_MergesExtra(t *testing.T) {
	env := buildEnv(map[string]string{"MY_VAR": "hello"})
	found := false
	for _, entry := range env {
		if entry == "MY_VAR=hello" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected MY_VAR=hello in environment")
	}
}

func TestBuildEnv_NilExtra(t *testing.T) {
	env := buildEnv(nil)
	if len(env) == 0 {
		t.Fatal("expected non-empty environment from os.Environ()")
	}
}

// ── logWriter tests ──

func TestLogWriter_WritesSucceed(t *testing.T) {
	logger := newTestLogger()
	w := newLogWriter(logger, "[test] ")

	n, err := w.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Fatalf("expected 5 bytes written, got %d", n)
	}
}

// newTestLogger creates a minimal slog.Logger for testing that discards output.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
