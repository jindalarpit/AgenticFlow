package execution

import (
	"bytes"
	"io"
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

// streamingWriter is an io.Writer that sends output chunks to a BackpressureBuffer
// for asynchronous delivery to the server, while also writing to an inner buffer
// for final reporting. Write() returns immediately without blocking regardless of
// server responsiveness.
type streamingWriter struct {
	inner    io.Writer
	buffer   *BackpressureBuffer
	stream   string // "stdout" or "stderr"
	sequence *atomic.Int32
	onWrite  func([]byte) // optional hook called on each write
}

func (s *streamingWriter) Write(p []byte) (n int, err error) {
	// Write to inner buffer first.
	n, err = s.inner.Write(p)

	// Push to backpressure buffer (non-blocking).
	if len(p) > 0 && s.buffer != nil {
		seq := s.sequence.Add(1)
		msg := api.TaskMessageEntry{
			Sequence: seq,
			Stream:   s.stream,
			Content:  string(p),
		}
		s.buffer.Push(msg)
	}

	// Call the output hook if provided.
	if len(p) > 0 && s.onWrite != nil {
		s.onWrite(p)
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
