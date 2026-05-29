package execution

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/agenticflow/agenticflow/shared/api"
)

// BackpressureBuffer is a bounded buffer for streaming task output messages.
// It holds up to maxCount messages OR maxBytes total content size (whichever
// is reached first). When full, Push() drops the oldest messages to make room.
type BackpressureBuffer struct {
	mu       sync.Mutex
	messages []api.TaskMessageEntry
	maxCount int
	maxBytes int
	curBytes int
	dropped  atomic.Int64
	notify   chan struct{}
}

// NewBackpressureBuffer creates a new buffer bounded by maxCount messages
// and maxBytes total content size.
func NewBackpressureBuffer(maxCount, maxBytes int) *BackpressureBuffer {
	return &BackpressureBuffer{
		messages: make([]api.TaskMessageEntry, 0, maxCount),
		maxCount: maxCount,
		maxBytes: maxBytes,
		notify:   make(chan struct{}, 1),
	}
}

// Push adds a message to the buffer. If the buffer is full (by count or bytes),
// the oldest messages are dropped until there is room. Each drop increments the
// dropped counter.
func (b *BackpressureBuffer) Push(msg api.TaskMessageEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	msgSize := len(msg.Content)

	// Drop oldest messages while buffer is at capacity.
	for (len(b.messages) >= b.maxCount || (b.curBytes+msgSize > b.maxBytes && len(b.messages) > 0)) && len(b.messages) > 0 {
		b.curBytes -= len(b.messages[0].Content)
		b.messages = b.messages[1:]
		b.dropped.Add(1)
	}

	b.messages = append(b.messages, msg)
	b.curBytes += msgSize

	// Non-blocking notify to wake the flush goroutine.
	select {
	case b.notify <- struct{}{}:
	default:
	}
}

// Drain returns all buffered messages and resets the buffer.
func (b *BackpressureBuffer) Drain() []api.TaskMessageEntry {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.messages) == 0 {
		return nil
	}

	msgs := b.messages
	b.messages = make([]api.TaskMessageEntry, 0, b.maxCount)
	b.curBytes = 0
	return msgs
}

// Dropped returns the total number of messages dropped due to buffer overflow.
func (b *BackpressureBuffer) Dropped() int64 {
	return b.dropped.Load()
}

// Len returns the current number of messages in the buffer.
func (b *BackpressureBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.messages)
}

// Bytes returns the current total content size in the buffer.
func (b *BackpressureBuffer) Bytes() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.curBytes
}

// Flusher manages a background goroutine that drains the buffer and sends
// messages to the server asynchronously.
type Flusher struct {
	buffer   *BackpressureBuffer
	reporter Reporter
	taskID   string
	logger   *slog.Logger
	done     chan struct{}
	stop     chan struct{}
}

// NewFlusher creates a Flusher that drains the given buffer and reports
// messages using the provided reporter.
func NewFlusher(buffer *BackpressureBuffer, reporter Reporter, taskID string, logger *slog.Logger) *Flusher {
	return &Flusher{
		buffer:   buffer,
		reporter: reporter,
		taskID:   taskID,
		logger:   logger,
		done:     make(chan struct{}),
		stop:     make(chan struct{}),
	}
}

// Run starts the flush goroutine. It waits on the buffer's notify channel,
// drains messages, and sends them to the server. It exits when Stop() is called.
func (f *Flusher) Run(ctx context.Context) {
	go f.loop(ctx)
}

func (f *Flusher) loop(ctx context.Context) {
	defer close(f.done)

	for {
		select {
		case <-f.buffer.notify:
			f.flush(ctx)
		case <-f.stop:
			// Final drain before exiting.
			f.flush(ctx)
			return
		case <-ctx.Done():
			// Context cancelled — do a final drain attempt.
			f.flush(context.Background())
			return
		}
	}
}

// flush drains the buffer and sends messages to the server.
func (f *Flusher) flush(ctx context.Context) {
	msgs := f.buffer.Drain()
	if len(msgs) == 0 {
		return
	}

	if f.reporter != nil {
		if err := f.reporter.ReportMessages(ctx, f.taskID, msgs); err != nil {
			f.logger.Debug("failed to flush buffered messages", "error", err, "count", len(msgs))
		}
	}
}

// Stop signals the flush goroutine to drain remaining messages and exit.
// It blocks until the goroutine has completed its final flush.
func (f *Flusher) Stop() {
	close(f.stop)
	<-f.done
}
