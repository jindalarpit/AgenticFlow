package execution

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agenticflow/agenticflow/shared/api"
	"pgregory.net/rapid"
)

func TestBackpressureBuffer_PushAndDrain(t *testing.T) {
	buf := NewBackpressureBuffer(100, 1<<20)

	msg := api.TaskMessageEntry{Sequence: 1, Stream: "stdout", Content: "hello"}
	buf.Push(msg)

	if buf.Len() != 1 {
		t.Fatalf("expected 1 message, got %d", buf.Len())
	}

	msgs := buf.Drain()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 drained message, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" {
		t.Errorf("content = %q, want %q", msgs[0].Content, "hello")
	}

	// Buffer should be empty after drain.
	if buf.Len() != 0 {
		t.Errorf("expected 0 messages after drain, got %d", buf.Len())
	}
}

func TestBackpressureBuffer_DrainEmpty(t *testing.T) {
	buf := NewBackpressureBuffer(100, 1<<20)

	msgs := buf.Drain()
	if msgs != nil {
		t.Errorf("expected nil from empty drain, got %v", msgs)
	}
}

func TestBackpressureBuffer_DropsOldestOnCountOverflow(t *testing.T) {
	buf := NewBackpressureBuffer(3, 1<<20) // max 3 messages

	for i := int32(1); i <= 5; i++ {
		buf.Push(api.TaskMessageEntry{Sequence: i, Content: "x"})
	}

	if buf.Len() != 3 {
		t.Fatalf("expected 3 messages, got %d", buf.Len())
	}
	if buf.Dropped() != 2 {
		t.Fatalf("expected 2 dropped, got %d", buf.Dropped())
	}

	msgs := buf.Drain()
	// Should have messages 3, 4, 5 (oldest 1, 2 dropped).
	if msgs[0].Sequence != 3 {
		t.Errorf("first message sequence = %d, want 3", msgs[0].Sequence)
	}
	if msgs[2].Sequence != 5 {
		t.Errorf("last message sequence = %d, want 5", msgs[2].Sequence)
	}
}

func TestBackpressureBuffer_DropsOldestOnBytesOverflow(t *testing.T) {
	// Max 10 bytes total content.
	buf := NewBackpressureBuffer(100, 10)

	// Push 3 messages of 4 bytes each = 12 bytes total, exceeds 10.
	buf.Push(api.TaskMessageEntry{Sequence: 1, Content: "aaaa"}) // 4 bytes, total=4
	buf.Push(api.TaskMessageEntry{Sequence: 2, Content: "bbbb"}) // 4 bytes, total=8
	buf.Push(api.TaskMessageEntry{Sequence: 3, Content: "cccc"}) // 4 bytes, would be 12 > 10, drop oldest

	if buf.Dropped() != 1 {
		t.Fatalf("expected 1 dropped, got %d", buf.Dropped())
	}

	msgs := buf.Drain()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Sequence != 2 {
		t.Errorf("first message sequence = %d, want 2", msgs[0].Sequence)
	}
}

func TestBackpressureBuffer_BytesTracking(t *testing.T) {
	buf := NewBackpressureBuffer(100, 1<<20)

	buf.Push(api.TaskMessageEntry{Content: "hello"}) // 5 bytes
	buf.Push(api.TaskMessageEntry{Content: "world"}) // 5 bytes

	if buf.Bytes() != 10 {
		t.Errorf("bytes = %d, want 10", buf.Bytes())
	}

	buf.Drain()
	if buf.Bytes() != 0 {
		t.Errorf("bytes after drain = %d, want 0", buf.Bytes())
	}
}

func TestBackpressureBuffer_ConcurrentPushAndDrain(t *testing.T) {
	buf := NewBackpressureBuffer(100, 1<<20)

	var wg sync.WaitGroup
	// Push from multiple goroutines.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				buf.Push(api.TaskMessageEntry{
					Sequence: int32(id*50 + j),
					Content:  "msg",
				})
			}
		}(i)
	}

	// Drain concurrently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			buf.Drain()
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	// No panic or race condition is the success criterion.
	// Buffer should be in a consistent state.
	if buf.Len() < 0 {
		t.Error("negative buffer length")
	}
}

func TestFlusher_DrainOnStop(t *testing.T) {
	buf := NewBackpressureBuffer(100, 1<<20)
	reporter := &mockReporter{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	flusher := NewFlusher(buf, reporter, "task-1", logger)
	flusher.Run(context.Background())

	// Push some messages.
	buf.Push(api.TaskMessageEntry{Sequence: 1, Content: "line1"})
	buf.Push(api.TaskMessageEntry{Sequence: 2, Content: "line2"})

	// Give the flusher a moment to process the notify.
	time.Sleep(50 * time.Millisecond)

	// Stop should drain remaining messages.
	buf.Push(api.TaskMessageEntry{Sequence: 3, Content: "line3"})
	flusher.Stop()

	// All messages should have been reported.
	if len(reporter.messages) == 0 {
		t.Fatal("expected messages to be flushed")
	}

	// Verify message 3 was flushed on stop.
	found := false
	for _, msg := range reporter.messages {
		if msg.Sequence == 3 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected message 3 to be flushed on stop")
	}
}

func TestFlusher_FlushesOnNotify(t *testing.T) {
	buf := NewBackpressureBuffer(100, 1<<20)
	reporter := &mockReporter{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	flusher := NewFlusher(buf, reporter, "task-2", logger)
	flusher.Run(context.Background())

	buf.Push(api.TaskMessageEntry{Sequence: 1, Content: "data"})

	// Wait for flush to process.
	time.Sleep(50 * time.Millisecond)

	flusher.Stop()

	if len(reporter.messages) == 0 {
		t.Fatal("expected messages to be flushed via notify")
	}
	if reporter.messages[0].Content != "data" {
		t.Errorf("content = %q, want %q", reporter.messages[0].Content, "data")
	}
}

func TestFlusher_ContextCancellation(t *testing.T) {
	buf := NewBackpressureBuffer(100, 1<<20)
	reporter := &mockReporter{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	ctx, cancel := context.WithCancel(context.Background())
	flusher := NewFlusher(buf, reporter, "task-3", logger)
	flusher.Run(ctx)

	buf.Push(api.TaskMessageEntry{Sequence: 1, Content: "before-cancel"})
	time.Sleep(50 * time.Millisecond)

	// Cancel context — flusher should do a final drain and exit.
	cancel()

	// Wait for the flusher to finish.
	select {
	case <-flusher.done:
	case <-time.After(time.Second):
		t.Fatal("flusher did not exit after context cancellation")
	}

	// Messages should have been flushed.
	if len(reporter.messages) == 0 {
		t.Fatal("expected messages to be flushed on context cancellation")
	}
}

func TestPushIsNonBlocking(t *testing.T) {
	buf := NewBackpressureBuffer(100, 1<<20)

	// Push should return immediately even without a consumer.
	start := time.Now()
	for i := 0; i < 200; i++ {
		buf.Push(api.TaskMessageEntry{Sequence: int32(i), Content: "msg"})
	}
	elapsed := time.Since(start)

	if elapsed > 10*time.Millisecond {
		t.Errorf("Push took %v, expected < 10ms for 200 pushes", elapsed)
	}
}

// **Validates: Requirements 14.1, 14.2**
// Property 4: Backpressure buffer bounds
// For any sequence of messages pushed to the BackpressureBuffer, the buffer
// never contains more than maxCount messages AND the total byte size of
// buffered message content never exceeds maxBytes.
func TestProperty_BackpressureBufferBounds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate buffer configuration with reasonable bounds.
		maxCount := rapid.IntRange(1, 50).Draw(t, "maxCount")
		maxBytes := rapid.IntRange(1, 1024).Draw(t, "maxBytes")

		buf := NewBackpressureBuffer(maxCount, maxBytes)

		// Generate a random number of messages to push.
		numMessages := rapid.IntRange(1, 200).Draw(t, "numMessages")

		for i := 0; i < numMessages; i++ {
			// Generate random content size between 1 and maxBytes (to ensure
			// individual messages can fit, and also test messages that are
			// close to or at the byte limit).
			contentLen := rapid.IntRange(1, maxBytes).Draw(t, "contentLen")
			content := strings.Repeat("x", contentLen)

			msg := api.TaskMessageEntry{
				Sequence: int32(i + 1),
				Stream:   "stdout",
				Content:  content,
			}

			buf.Push(msg)

			// After every push, verify the buffer invariants hold.
			currentLen := buf.Len()
			currentBytes := buf.Bytes()

			if currentLen > maxCount {
				t.Fatalf("buffer count %d exceeds maxCount %d after push %d",
					currentLen, maxCount, i+1)
			}

			if currentBytes > maxBytes {
				t.Fatalf("buffer bytes %d exceeds maxBytes %d after push %d",
					currentBytes, maxBytes, i+1)
			}
		}
	})
}

// **Validates: Requirements 14.4**
// Property 5: Non-blocking streaming writes
// For any call to streamingWriter.Write(p) or BackpressureBuffer.Push(msg),
// the method returns within 1 millisecond regardless of buffer fullness or
// consumer state.
func TestProperty_NonBlockingStreamingWrites(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate buffer configuration — use small buffers to force overflow.
		maxCount := rapid.IntRange(1, 20).Draw(t, "maxCount")
		maxBytes := rapid.IntRange(10, 512).Draw(t, "maxBytes")

		buf := NewBackpressureBuffer(maxCount, maxBytes)

		// Do NOT start any consumer/flusher — simulates a blocked/slow consumer.

		// Generate random message sizes to push.
		numWrites := rapid.IntRange(1, 100).Draw(t, "numWrites")

		for i := 0; i < numWrites; i++ {
			contentLen := rapid.IntRange(1, maxBytes*2).Draw(t, "contentLen")
			content := strings.Repeat("a", contentLen)

			msg := api.TaskMessageEntry{
				Sequence: int32(i + 1),
				Stream:   "stdout",
				Content:  content,
			}

			start := time.Now()
			buf.Push(msg)
			elapsed := time.Since(start)

			if elapsed > time.Millisecond {
				t.Fatalf("Push() took %v (> 1ms) on write %d with contentLen=%d, maxCount=%d, maxBytes=%d",
					elapsed, i+1, contentLen, maxCount, maxBytes)
			}
		}
	})
}

// TestProperty_NonBlockingStreamingWriterWrite verifies that streamingWriter.Write()
// returns within 1ms regardless of buffer fullness or consumer state.
//
// **Validates: Requirements 14.4**
func TestProperty_NonBlockingStreamingWriterWrite(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate buffer configuration — use small buffers to force overflow.
		maxCount := rapid.IntRange(1, 20).Draw(t, "maxCount")
		maxBytes := rapid.IntRange(10, 512).Draw(t, "maxBytes")

		buf := NewBackpressureBuffer(maxCount, maxBytes)

		// Create a streamingWriter with no consumer draining the buffer.
		var seq atomic.Int32
		inner := &bytes.Buffer{}
		sw := &streamingWriter{
			inner:    inner,
			buffer:   buf,
			stream:   "stdout",
			sequence: &seq,
		}

		// Generate random write payloads.
		numWrites := rapid.IntRange(1, 100).Draw(t, "numWrites")

		for i := 0; i < numWrites; i++ {
			contentLen := rapid.IntRange(1, maxBytes*2).Draw(t, "contentLen")
			payload := []byte(strings.Repeat("b", contentLen))

			start := time.Now()
			n, err := sw.Write(payload)
			elapsed := time.Since(start)

			if err != nil {
				t.Fatalf("Write() returned error on write %d: %v", i+1, err)
			}
			if n != len(payload) {
				t.Fatalf("Write() returned n=%d, want %d on write %d", n, len(payload), i+1)
			}
			if elapsed > time.Millisecond {
				t.Fatalf("Write() took %v (> 1ms) on write %d with contentLen=%d, maxCount=%d, maxBytes=%d",
					elapsed, i+1, contentLen, maxCount, maxBytes)
			}
		}
	})
}
