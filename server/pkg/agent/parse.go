package agent

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ── Line-by-line stdout scanner helper ──

// defaultScanBufSize is the initial buffer size for the NDJSON scanner.
const defaultScanBufSize = 1024 * 1024 // 1 MB

// maxScanBufSize is the maximum buffer size the scanner will grow to.
// Agent CLI output can include large tool results (file contents, command
// output) embedded in JSON lines, so we allow up to 10 MB per line.
const maxScanBufSize = 10 * 1024 * 1024 // 10 MB

// newLineScanner creates a bufio.Scanner configured for reading NDJSON
// lines from an agent CLI's stdout pipe. The scanner uses a large buffer
// to handle lines containing embedded tool output (file contents, command
// results) that can be several megabytes.
func newLineScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, defaultScanBufSize), maxScanBufSize)
	return scanner
}

// ── Stderr tail buffer ──

// stderrTailSize is the maximum number of bytes retained in the stderr
// tail buffer. Large enough to contain typical CLI error output, small
// enough to stay sensible inside a task-level Result.Error string.
const stderrTailSize = 4096

// stderrTail captures the last N bytes written to an agent CLI's stderr.
// It forwards all writes to an inner writer (typically a log adapter) while
// retaining a bounded tail. When the CLI exits unexpectedly, consumers call
// Tail() to include diagnostic context in the error message.
//
// All backends that supervise a child CLI process should wire cmd.Stderr
// through this type, and on failure include Tail() in Result.Error via
// withStderrTail.
type stderrTail struct {
	inner io.Writer
	max   int

	mu  sync.Mutex
	buf []byte
}

// newStderrTail creates a stderr tail buffer that forwards writes to inner
// and retains at most max bytes. If max <= 0, stderrTailSize is used.
func newStderrTail(inner io.Writer, max int) *stderrTail {
	if max <= 0 {
		max = stderrTailSize
	}
	return &stderrTail{inner: inner, max: max}
}

// Write forwards p to the inner writer and appends it to the tail buffer,
// trimming from the front if the buffer exceeds the maximum size.
func (s *stderrTail) Write(p []byte) (int, error) {
	if _, err := s.inner.Write(p); err != nil {
		return 0, err
	}
	s.mu.Lock()
	s.buf = append(s.buf, p...)
	if len(s.buf) > s.max {
		s.buf = s.buf[len(s.buf)-s.max:]
	}
	s.mu.Unlock()
	return len(p), nil
}

// Tail returns the captured stderr content with leading/trailing whitespace
// trimmed. Returns empty string if nothing was written or everything was
// whitespace.
func (s *stderrTail) Tail() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.TrimSpace(string(s.buf))
}

// withStderrTail appends a stderr tail hint to an error message when the
// tail is non-empty, otherwise returns msg unchanged. The label identifies
// which backend produced the stderr (e.g., "claude", "gemini").
func withStderrTail(msg, label, tail string) string {
	if tail == "" {
		return msg
	}
	return msg + "; " + label + " stderr: " + tail
}

// ── Graceful process termination helper ──

// terminationGracePeriod is the time to wait after SIGTERM before sending
// SIGKILL. This gives the CLI process time to clean up resources.
const terminationGracePeriod = 10 * time.Second

// gracefulTerminate sends SIGTERM to the process and waits up to
// terminationGracePeriod for it to exit. If the process does not exit
// within the grace period, SIGKILL is sent to force termination.
//
// This function is safe to call even if the process has already exited.
// It returns nil if the process was successfully terminated (by either
// signal), or an error if signaling failed.
func gracefulTerminate(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	// Send SIGTERM first for graceful shutdown.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// Process may have already exited — not an error.
		if isProcessFinished(err) {
			return nil
		}
		return err
	}

	// Wait for the process to exit within the grace period.
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Process exited gracefully after SIGTERM.
		return nil
	case <-time.After(terminationGracePeriod):
		// Grace period expired — force kill.
		if err := cmd.Process.Signal(syscall.SIGKILL); err != nil {
			if isProcessFinished(err) {
				return nil
			}
			return err
		}
		// Wait for the kill to take effect.
		<-done
		return nil
	}
}

// closeOnCancel starts a goroutine that closes the given ReadCloser when
// the context is cancelled. This is used to unblock a scanner.Scan() call
// that is waiting on stdout when the execution context is cancelled or
// times out.
func closeOnCancel(ctx context.Context, rc io.ReadCloser) {
	go func() {
		<-ctx.Done()
		_ = rc.Close()
	}()
}

// isProcessFinished returns true if the error indicates the process has
// already exited (e.g., "os: process already finished").
func isProcessFinished(err error) bool {
	return err != nil && strings.Contains(err.Error(), "process already finished")
}

// ── Shared logging helper ──

// logWriter adapts a *slog.Logger to an io.Writer for capturing stderr.
// Each non-empty write is logged at Debug level with the given prefix.
type logWriter struct {
	logger *slog.Logger
	prefix string
}

// newLogWriter creates a writer that logs each write to the given logger.
func newLogWriter(logger *slog.Logger, prefix string) *logWriter {
	return &logWriter{logger: logger, prefix: prefix}
}

func (w *logWriter) Write(p []byte) (int, error) {
	text := strings.TrimSpace(string(p))
	if text != "" {
		w.logger.Debug(w.prefix + text)
	}
	return len(p), nil
}

// ── Shared environment helper ──

// buildEnv merges the current process environment with extra key-value
// pairs. Extra values override existing keys.
func buildEnv(extra map[string]string) []string {
	base := os.Environ()
	env := make([]string, 0, len(base)+len(extra))
	// Build a set of overridden keys for fast lookup.
	overridden := make(map[string]bool, len(extra))
	for k := range extra {
		overridden[k] = true
	}
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if overridden[key] {
			continue // will be replaced by extra value
		}
		env = append(env, entry)
	}
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}

// ── Shared channel helper ──

// trySend attempts to send a message on the channel without blocking.
// If the channel is full, the message is dropped. Final output is
// accumulated separately in Result.Output, so only streaming consumers
// are affected by drops.
func trySend(ch chan<- Message, msg Message) {
	select {
	case ch <- msg:
	default:
	}
}
