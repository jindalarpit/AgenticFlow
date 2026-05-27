package execution

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync/atomic"

	"github.com/agenticflow/agenticflow/shared/api"
)

// Output truncation limits.
const (
	// maxStdoutBytes is the maximum size of stdout output reported to the server (1 MB).
	maxStdoutBytes = 1 * 1024 * 1024
	// maxStderrChars is the maximum number of characters of stderr kept (last 4096).
	maxStderrChars = 4096
)

// streamingWriter is an io.Writer that sends output chunks to the server
// as TaskMessageEntry messages while also writing to an inner buffer for
// final reporting.
type streamingWriter struct {
	inner    io.Writer
	reporter Reporter
	ctx      context.Context
	taskID   string
	stream   string // "stdout" or "stderr"
	sequence *atomic.Int32
	logger   *slog.Logger
}

func (s *streamingWriter) Write(p []byte) (n int, err error) {
	// Write to inner buffer first.
	n, err = s.inner.Write(p)

	// Send to server as a task message (best-effort, don't block on failure).
	if len(p) > 0 && s.reporter != nil {
		seq := s.sequence.Add(1)
		msg := api.TaskMessageEntry{
			Sequence: seq,
			Stream:   s.stream,
			Content:  string(p),
		}
		if reportErr := s.reporter.ReportMessages(s.ctx, s.taskID, []api.TaskMessageEntry{msg}); reportErr != nil {
			s.logger.Debug("failed to report task message", "error", reportErr)
		}
	}

	return n, err
}

// truncatingBuffer is an io.Writer that captures output up to a maximum byte limit.
// Once the limit is reached, additional writes are silently discarded.
type truncatingBuffer struct {
	buf      bytes.Buffer
	maxBytes int
}

func (tb *truncatingBuffer) Write(p []byte) (n int, err error) {
	remaining := tb.maxBytes - tb.buf.Len()
	if remaining <= 0 {
		// Already at capacity — discard but report success.
		return len(p), nil
	}
	if len(p) > remaining {
		tb.buf.Write(p[:remaining])
		return len(p), nil
	}
	tb.buf.Write(p)
	return len(p), nil
}

func (tb *truncatingBuffer) String() string {
	return tb.buf.String()
}

// tailBuffer is an io.Writer that keeps only the last N characters of output.
// This is used for stderr to capture the most recent error context.
type tailBuffer struct {
	data     []byte
	maxChars int
}

func (tb *tailBuffer) Write(p []byte) (n int, err error) {
	tb.data = append(tb.data, p...)
	// Trim to last maxChars characters.
	s := string(tb.data)
	if len([]rune(s)) > tb.maxChars {
		runes := []rune(s)
		s = string(runes[len(runes)-tb.maxChars:])
		tb.data = []byte(s)
	}
	return len(p), nil
}

func (tb *tailBuffer) String() string {
	return string(tb.data)
}
