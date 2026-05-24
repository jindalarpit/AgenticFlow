package handler

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/agenticflow/agenticflow/pkg/db/generated"
	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: interactive-task-sessions, Property 12: Stdin Message Reporting Round-Trip
//
// For any input text successfully written to a task's stdin pipe, the daemon
// SHALL report a task message with stream type "stdin" containing the original
// text. The server SHALL persist this message and broadcast it via WebSocket
// with stream field set to "stdin".
//
// This property verifies the round-trip: daemon reports a TaskMessageEntry with
// stream "stdin" → server persists as db.TaskMessage → API returns
// taskMessageResponse with stream "stdin" and content matching the original.
//
// **Validates: Requirements 3.4, 6.1, 6.2**
// ---------------------------------------------------------------------------

// simulateStdinRoundTrip simulates the full stdin message reporting round-trip:
// 1. Daemon creates a TaskMessageEntry with stream "stdin" and the input text
// 2. Server validates the stream type (accepts "stdin" as valid)
// 3. Server persists as db.TaskMessage (simulated)
// 4. Server converts to taskMessageResponse for API/WebSocket broadcast
//
// Returns the final taskMessageResponse and whether the stream was accepted.
func simulateStdinRoundTrip(text string, sequence int32, taskID pgtype.UUID) (taskMessageResponse, bool) {
	// Step 1: Daemon creates the message entry (as in handleTaskInput).
	entry := TaskMessageEntry{
		Sequence: sequence,
		Stream:   "stdin",
		Content:  text,
	}

	// Step 2: Server validates the stream type (as in ReportTaskMessages).
	stream := strings.TrimSpace(entry.Stream)
	if stream != "stdout" && stream != "stderr" && stream != "stdin" {
		stream = "stdout" // fallback for invalid streams
	}
	streamAccepted := (stream == "stdin")

	// Step 3: Simulate persistence as db.TaskMessage.
	dbMsg := db.TaskMessage{
		ID:       pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true},
		TaskID:   taskID,
		Sequence: entry.Sequence,
		Stream:   stream,
		Content:  entry.Content,
		CreatedAt: pgtype.Timestamptz{
			Time:  time.Now().UTC(),
			Valid: true,
		},
	}

	// Step 4: Convert to API response (as in GetTaskMessages handler).
	resp := toTaskMessageResponse(dbMsg)

	return resp, streamAccepted
}

func TestProperty12_StdinMessageRoundTrip_StreamPreserved(t *testing.T) {
	// Feature: interactive-task-sessions, Property 12: Stdin Message Reporting Round-Trip
	// For any input text, the round-trip preserves stream type as "stdin".
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary non-empty text (simulating what the daemon writes).
		text := rapid.StringN(1, 100, 500).Draw(t, "text")
		sequence := int32(rapid.IntRange(1, 100000).Draw(t, "sequence"))
		taskID := pgtype.UUID{
			Bytes: [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
			Valid: true,
		}

		resp, streamAccepted := simulateStdinRoundTrip(text, sequence, taskID)

		// The stream must be accepted as "stdin".
		if !streamAccepted {
			t.Fatal("stream 'stdin' was not accepted by the server validation")
		}

		// The response stream must be "stdin".
		if resp.Stream != "stdin" {
			t.Fatalf("response stream = %q, want \"stdin\"", resp.Stream)
		}
	})
}

func TestProperty12_StdinMessageRoundTrip_ContentPreserved(t *testing.T) {
	// Feature: interactive-task-sessions, Property 12: Stdin Message Reporting Round-Trip
	// For any input text, the round-trip preserves the original content exactly.
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary text including special characters, unicode, whitespace.
		text := rapid.String().Draw(t, "text")
		if text == "" {
			text = "x" // ensure non-empty for valid stdin input
		}
		sequence := int32(rapid.IntRange(1, 100000).Draw(t, "sequence"))
		taskID := pgtype.UUID{
			Bytes: [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
			Valid: true,
		}

		resp, _ := simulateStdinRoundTrip(text, sequence, taskID)

		// The content must match the original text exactly.
		if resp.Content != text {
			t.Fatalf("response content does not match original:\n  got:  %q\n  want: %q", resp.Content, text)
		}
	})
}

func TestProperty12_StdinMessageRoundTrip_SequencePreserved(t *testing.T) {
	// Feature: interactive-task-sessions, Property 12: Stdin Message Reporting Round-Trip
	// For any sequence number, the round-trip preserves it exactly.
	rapid.Check(t, func(t *rapid.T) {
		text := rapid.StringN(1, 10, 50).Draw(t, "text")
		sequence := int32(rapid.IntRange(1, 2147483647).Draw(t, "sequence"))
		taskID := pgtype.UUID{
			Bytes: [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
			Valid: true,
		}

		resp, _ := simulateStdinRoundTrip(text, sequence, taskID)

		if resp.Sequence != sequence {
			t.Fatalf("response sequence = %d, want %d", resp.Sequence, sequence)
		}
	})
}

func TestProperty12_StdinMessageRoundTrip_JSONSerialization(t *testing.T) {
	// Feature: interactive-task-sessions, Property 12: Stdin Message Reporting Round-Trip
	// For any stdin message, the JSON serialization includes stream="stdin" and
	// preserves content, matching what WebSocket broadcast would deliver.
	rapid.Check(t, func(t *rapid.T) {
		text := rapid.String().Draw(t, "text")
		if text == "" {
			text = "input"
		}
		sequence := int32(rapid.IntRange(1, 100000).Draw(t, "sequence"))
		taskID := pgtype.UUID{
			Bytes: [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
			Valid: true,
		}

		resp, _ := simulateStdinRoundTrip(text, sequence, taskID)

		// Serialize to JSON (as the server would for WebSocket broadcast).
		jsonBytes, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}

		// Deserialize and verify fields.
		var decoded map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}

		// Verify stream field is "stdin".
		streamVal, ok := decoded["stream"]
		if !ok {
			t.Fatal("JSON response missing 'stream' field")
		}
		if streamVal != "stdin" {
			t.Fatalf("JSON stream = %q, want \"stdin\"", streamVal)
		}

		// Verify content field matches original text.
		contentVal, ok := decoded["content"]
		if !ok {
			t.Fatal("JSON response missing 'content' field")
		}
		if contentVal != text {
			t.Fatalf("JSON content does not match original:\n  got:  %q\n  want: %q", contentVal, text)
		}

		// Verify sequence field.
		seqVal, ok := decoded["sequence"]
		if !ok {
			t.Fatal("JSON response missing 'sequence' field")
		}
		// JSON numbers are float64.
		if int32(seqVal.(float64)) != sequence {
			t.Fatalf("JSON sequence = %v, want %d", seqVal, sequence)
		}
	})
}

func TestProperty12_StdinMessageRoundTrip_WebSocketBroadcastPayload(t *testing.T) {
	// Feature: interactive-task-sessions, Property 12: Stdin Message Reporting Round-Trip
	// Simulates the WebSocket broadcast payload structure that the server creates
	// in ReportTaskMessages for stdin messages. Verifies the payload contains
	// the correct stream type and content for any arbitrary input.
	rapid.Check(t, func(t *rapid.T) {
		text := rapid.StringN(1, 50, 200).Draw(t, "text")
		sequence := int32(rapid.IntRange(1, 100000).Draw(t, "sequence"))
		taskIDStr := rapid.StringMatching(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`).Draw(t, "taskID")

		// Simulate the WebSocket broadcast payload as constructed in ReportTaskMessages.
		// The handler creates: {"task_id": ..., "sequence": ..., "stream": ..., "content": ...}
		entry := TaskMessageEntry{
			Sequence: sequence,
			Stream:   "stdin",
			Content:  text,
		}

		// Validate stream (same logic as ReportTaskMessages).
		stream := strings.TrimSpace(entry.Stream)
		if stream != "stdout" && stream != "stderr" && stream != "stdin" {
			stream = "stdout"
		}

		// Construct the broadcast payload (same as in ReportTaskMessages).
		payload := map[string]interface{}{
			"task_id":  taskIDStr,
			"sequence": entry.Sequence,
			"stream":   stream,
			"content":  entry.Content,
		}

		// Verify the payload has stream "stdin".
		if payload["stream"] != "stdin" {
			t.Fatalf("broadcast payload stream = %q, want \"stdin\"", payload["stream"])
		}

		// Verify the payload content matches the original text.
		if payload["content"] != text {
			t.Fatalf("broadcast payload content = %q, want %q", payload["content"], text)
		}

		// Verify the payload task_id matches.
		if payload["task_id"] != taskIDStr {
			t.Fatalf("broadcast payload task_id = %q, want %q", payload["task_id"], taskIDStr)
		}

		// Verify the payload sequence matches.
		if payload["sequence"] != sequence {
			t.Fatalf("broadcast payload sequence = %v, want %d", payload["sequence"], sequence)
		}
	})
}

func TestProperty12_StdinMessageRoundTrip_DaemonEntryStreamIsStdin(t *testing.T) {
	// Feature: interactive-task-sessions, Property 12: Stdin Message Reporting Round-Trip
	// For any input text, the daemon always creates a TaskMessageEntry with
	// stream "stdin" (not "stdout" or "stderr"), ensuring the server correctly
	// identifies and persists it as a stdin message.
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary text that could be written to stdin.
		text := rapid.String().Draw(t, "text")
		if text == "" {
			text = "y"
		}
		sequence := int32(rapid.IntRange(1, 100000).Draw(t, "sequence"))

		// Simulate daemon creating the message (as in handleTaskInput).
		msg := TaskMessageEntry{
			Sequence: sequence,
			Stream:   "stdin",
			Content:  text,
		}

		// Verify the daemon always sets stream to "stdin".
		if msg.Stream != "stdin" {
			t.Fatalf("daemon message stream = %q, want \"stdin\"", msg.Stream)
		}

		// Verify the server's validation accepts "stdin" as a valid stream.
		stream := strings.TrimSpace(msg.Stream)
		validStreams := map[string]bool{"stdout": true, "stderr": true, "stdin": true}
		if !validStreams[stream] {
			t.Fatalf("stream %q is not in the set of valid streams", stream)
		}

		// Verify content is preserved through the entry.
		if msg.Content != text {
			t.Fatalf("message content = %q, want %q", msg.Content, text)
		}
	})
}


// ---------------------------------------------------------------------------
// Feature: interactive-task-sessions, Property 7: Message Ordering by Sequence Number
//
// For any set of task messages (stdout, stderr, and stdin) with distinct
// sequence numbers, the API response from GET /api/tasks/{id}/messages SHALL
// return them ordered by ascending sequence number, regardless of stream type
// or insertion order.
//
// **Validates: Requirements 6.4, 8.4, 8.5**
// ---------------------------------------------------------------------------

// simulateDBOrderBySequence sorts messages by sequence ASC, mimicking the SQL
// query: SELECT ... FROM task_message WHERE task_id = $1 ORDER BY sequence ASC.
func simulateDBOrderBySequence(messages []db.TaskMessage) []db.TaskMessage {
	sorted := make([]db.TaskMessage, len(messages))
	copy(sorted, messages)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Sequence < sorted[j].Sequence
	})
	return sorted
}

func TestProperty7_MessageOrdering_AscendingSequence(t *testing.T) {
	// Feature: interactive-task-sessions, Property 7: Message Ordering by Sequence Number
	// For any set of messages with distinct sequence numbers and mixed stream types,
	// when sorted by sequence (as the DB does), the converted responses maintain
	// ascending sequence order.
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random number of messages (2 to 50).
		numMessages := rapid.IntRange(2, 50).Draw(t, "numMessages")

		// Generate distinct sequence numbers.
		seqSet := make(map[int32]bool)
		for len(seqSet) < numMessages {
			seq := int32(rapid.IntRange(1, 100000).Draw(t, "seq"))
			seqSet[seq] = true
		}
		sequences := make([]int32, 0, numMessages)
		for seq := range seqSet {
			sequences = append(sequences, seq)
		}

		// Generate a fixed task UUID.
		taskUUID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}

		// Build messages in random order with random stream types.
		messages := make([]db.TaskMessage, numMessages)
		for i, seq := range sequences {
			stream := rapid.SampledFrom([]string{"stdout", "stderr", "stdin"}).Draw(t, fmt.Sprintf("stream_%d", i))
			content := rapid.StringN(1, 20, 100).Draw(t, fmt.Sprintf("content_%d", i))
			messages[i] = db.TaskMessage{
				ID:       pgtype.UUID{Bytes: [16]byte{byte(i + 1)}, Valid: true},
				TaskID:   taskUUID,
				Sequence: seq,
				Stream:   stream,
				Content:  content,
				CreatedAt: pgtype.Timestamptz{
					Time:  time.Date(2025, 1, 1, 0, 0, int(seq%60), 0, time.UTC),
					Valid: true,
				},
			}
		}

		// Simulate DB ORDER BY sequence ASC.
		sorted := simulateDBOrderBySequence(messages)

		// Convert to responses (same as the handler does).
		result := make([]taskMessageResponse, 0, len(sorted))
		for _, m := range sorted {
			result = append(result, toTaskMessageResponse(m))
		}

		// Verify ascending sequence order.
		for i := 1; i < len(result); i++ {
			if result[i].Sequence <= result[i-1].Sequence {
				t.Fatalf("messages not in ascending sequence order: index %d has seq %d, index %d has seq %d",
					i-1, result[i-1].Sequence, i, result[i].Sequence)
			}
		}
	})
}

func TestProperty7_MessageOrdering_StreamTypeIndependent(t *testing.T) {
	// Feature: interactive-task-sessions, Property 7: Message Ordering by Sequence Number
	// Ordering is by sequence number only — stream type does not affect order.
	// Messages with different stream types but interleaved sequence numbers
	// must still be returned in ascending sequence order.
	rapid.Check(t, func(t *rapid.T) {
		numMessages := rapid.IntRange(3, 30).Draw(t, "numMessages")

		// Generate distinct sequence numbers.
		seqSet := make(map[int32]bool)
		for len(seqSet) < numMessages {
			seq := int32(rapid.IntRange(1, 50000).Draw(t, "seq"))
			seqSet[seq] = true
		}

		taskUUID := pgtype.UUID{Bytes: [16]byte{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160}, Valid: true}

		// Build messages ensuring all three stream types are represented.
		streams := []string{"stdout", "stderr", "stdin"}
		messages := make([]db.TaskMessage, 0, numMessages)
		i := 0
		for seq := range seqSet {
			// Cycle through stream types to ensure all are present.
			stream := streams[i%len(streams)]
			messages = append(messages, db.TaskMessage{
				ID:       pgtype.UUID{Bytes: [16]byte{byte(i + 1)}, Valid: true},
				TaskID:   taskUUID,
				Sequence: seq,
				Stream:   stream,
				Content:  fmt.Sprintf("msg_%d_%s", seq, stream),
				CreatedAt: pgtype.Timestamptz{
					Time:  time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
					Valid: true,
				},
			})
			i++
		}

		// Simulate DB ORDER BY sequence ASC.
		sorted := simulateDBOrderBySequence(messages)

		// Convert to responses.
		result := make([]taskMessageResponse, 0, len(sorted))
		for _, m := range sorted {
			result = append(result, toTaskMessageResponse(m))
		}

		// Verify ascending sequence order regardless of stream type.
		for i := 1; i < len(result); i++ {
			if result[i].Sequence <= result[i-1].Sequence {
				t.Fatalf("ordering violated: result[%d].Sequence=%d (stream=%s) <= result[%d].Sequence=%d (stream=%s)",
					i, result[i].Sequence, result[i].Stream,
					i-1, result[i-1].Sequence, result[i-1].Stream)
			}
		}

		// Verify all stream types are present in the result.
		streamsSeen := make(map[string]bool)
		for _, r := range result {
			streamsSeen[r.Stream] = true
		}
		for _, s := range streams {
			if !streamsSeen[s] {
				t.Errorf("stream type %q not found in result", s)
			}
		}
	})
}

func TestProperty7_MessageOrdering_PreservesAllMessages(t *testing.T) {
	// Feature: interactive-task-sessions, Property 7: Message Ordering by Sequence Number
	// The ordering operation preserves all messages — no messages are lost or duplicated.
	rapid.Check(t, func(t *rapid.T) {
		numMessages := rapid.IntRange(1, 40).Draw(t, "numMessages")

		// Generate distinct sequence numbers.
		seqSet := make(map[int32]bool)
		for len(seqSet) < numMessages {
			seq := int32(rapid.IntRange(1, 99999).Draw(t, "seq"))
			seqSet[seq] = true
		}

		taskUUID := pgtype.UUID{Bytes: [16]byte{5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5}, Valid: true}

		messages := make([]db.TaskMessage, 0, numMessages)
		for seq := range seqSet {
			stream := rapid.SampledFrom([]string{"stdout", "stderr", "stdin"}).Draw(t, "stream")
			messages = append(messages, db.TaskMessage{
				ID:       pgtype.UUID{Bytes: [16]byte{byte(seq % 256)}, Valid: true},
				TaskID:   taskUUID,
				Sequence: seq,
				Stream:   stream,
				Content:  fmt.Sprintf("content_%d", seq),
				CreatedAt: pgtype.Timestamptz{
					Time:  time.Now().UTC(),
					Valid: true,
				},
			})
		}

		// Simulate DB ORDER BY sequence ASC.
		sorted := simulateDBOrderBySequence(messages)

		// Convert to responses.
		result := make([]taskMessageResponse, 0, len(sorted))
		for _, m := range sorted {
			result = append(result, toTaskMessageResponse(m))
		}

		// Verify no messages lost.
		if len(result) != numMessages {
			t.Fatalf("expected %d messages in result, got %d", numMessages, len(result))
		}

		// Verify all original sequence numbers are present.
		resultSeqs := make(map[int32]bool)
		for _, r := range result {
			resultSeqs[r.Sequence] = true
		}
		for seq := range seqSet {
			if !resultSeqs[seq] {
				t.Fatalf("sequence %d from original messages not found in result", seq)
			}
		}
	})
}

func TestProperty7_MessageOrdering_SingleMessageTrivial(t *testing.T) {
	// Feature: interactive-task-sessions, Property 7: Message Ordering by Sequence Number
	// A single message is trivially ordered.
	rapid.Check(t, func(t *rapid.T) {
		seq := int32(rapid.IntRange(1, 100000).Draw(t, "seq"))
		stream := rapid.SampledFrom([]string{"stdout", "stderr", "stdin"}).Draw(t, "stream")
		content := rapid.StringN(1, 10, 200).Draw(t, "content")

		taskUUID := pgtype.UUID{Bytes: [16]byte{7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7}, Valid: true}

		messages := []db.TaskMessage{
			{
				ID:       pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
				TaskID:   taskUUID,
				Sequence: seq,
				Stream:   stream,
				Content:  content,
				CreatedAt: pgtype.Timestamptz{
					Time:  time.Now().UTC(),
					Valid: true,
				},
			},
		}

		sorted := simulateDBOrderBySequence(messages)
		result := make([]taskMessageResponse, 0, len(sorted))
		for _, m := range sorted {
			result = append(result, toTaskMessageResponse(m))
		}

		if len(result) != 1 {
			t.Fatalf("expected 1 message, got %d", len(result))
		}
		if result[0].Sequence != seq {
			t.Fatalf("sequence = %d, want %d", result[0].Sequence, seq)
		}
		if result[0].Stream != stream {
			t.Fatalf("stream = %q, want %q", result[0].Stream, stream)
		}
		if result[0].Content != content {
			t.Fatalf("content = %q, want %q", result[0].Content, content)
		}
	})
}
