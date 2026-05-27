package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// TestToTaskMessageResponse_IncludesStreamField verifies that the response
// includes the stream field for all valid stream types (stdout, stderr, stdin).
func TestToTaskMessageResponse_IncludesStreamField(t *testing.T) {
	streams := []string{"stdout", "stderr", "stdin"}

	for _, stream := range streams {
		t.Run(stream, func(t *testing.T) {
			msg := db.TaskMessage{
				ID:       pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true},
				TaskID:   pgtype.UUID{Bytes: [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}, Valid: true},
				Sequence: 42,
				Stream:   pgtype.Text{String: stream, Valid: true},
				Content:  pgtype.Text{String: "test content for " + stream, Valid: true},
				CreatedAt: pgtype.Timestamptz{
					Time:  time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
					Valid: true,
				},
			}

			resp := toTaskMessageResponse(msg)

			if resp.Stream != stream {
				t.Errorf("stream = %q, want %q", resp.Stream, stream)
			}
			if resp.Sequence != 42 {
				t.Errorf("sequence = %d, want 42", resp.Sequence)
			}
			if resp.Content != "test content for "+stream {
				t.Errorf("content = %q, want %q", resp.Content, "test content for "+stream)
			}

			// Verify JSON serialization includes the stream field.
			jsonBytes, err := json.Marshal(resp)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}

			var decoded map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}

			streamVal, ok := decoded["stream"]
			if !ok {
				t.Fatal("JSON response missing 'stream' field")
			}
			if streamVal != stream {
				t.Errorf("JSON stream = %q, want %q", streamVal, stream)
			}
		})
	}
}

// TestToTaskMessageResponse_JSONFieldsComplete verifies that the JSON response
// contains all expected fields for a task message.
func TestToTaskMessageResponse_JSONFieldsComplete(t *testing.T) {
	msg := db.TaskMessage{
		ID:       pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true},
		TaskID:   pgtype.UUID{Bytes: [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}, Valid: true},
		Sequence: 1,
		Stream:   pgtype.Text{String: "stdin", Valid: true},
		Content:  pgtype.Text{String: "user input text", Valid: true},
		CreatedAt: pgtype.Timestamptz{
			Time:  time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC),
			Valid: true,
		},
	}

	resp := toTaskMessageResponse(msg)
	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	expectedFields := []string{"id", "task_id", "sequence", "stream", "content", "created_at"}
	for _, field := range expectedFields {
		if _, ok := decoded[field]; !ok {
			t.Errorf("JSON response missing field %q", field)
		}
	}
}

// TestTaskMessageResponse_OrderingBySequence verifies that when messages with
// different stream types are converted to responses, the sequence field is
// preserved correctly for ordering purposes.
func TestTaskMessageResponse_OrderingBySequence(t *testing.T) {
	// Simulate messages as they would come from the DB (ordered by sequence ASC).
	messages := []db.TaskMessage{
		{
			ID:        pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
			TaskID:    pgtype.UUID{Bytes: [16]byte{10}, Valid: true},
			Sequence:  1,
			Stream:    pgtype.Text{String: "stdout", Valid: true},
			Content:   pgtype.Text{String: "Starting...", Valid: true},
			CreatedAt: pgtype.Timestamptz{Time: time.Date(2025, 1, 1, 0, 0, 1, 0, time.UTC), Valid: true},
		},
		{
			ID:        pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
			TaskID:    pgtype.UUID{Bytes: [16]byte{10}, Valid: true},
			Sequence:  2,
			Stream:    pgtype.Text{String: "stderr", Valid: true},
			Content:   pgtype.Text{String: "Warning: something", Valid: true},
			CreatedAt: pgtype.Timestamptz{Time: time.Date(2025, 1, 1, 0, 0, 2, 0, time.UTC), Valid: true},
		},
		{
			ID:        pgtype.UUID{Bytes: [16]byte{3}, Valid: true},
			TaskID:    pgtype.UUID{Bytes: [16]byte{10}, Valid: true},
			Sequence:  3,
			Stream:    pgtype.Text{String: "stdout", Valid: true},
			Content:   pgtype.Text{String: "Do you want to continue? ", Valid: true},
			CreatedAt: pgtype.Timestamptz{Time: time.Date(2025, 1, 1, 0, 0, 3, 0, time.UTC), Valid: true},
		},
		{
			ID:        pgtype.UUID{Bytes: [16]byte{4}, Valid: true},
			TaskID:    pgtype.UUID{Bytes: [16]byte{10}, Valid: true},
			Sequence:  4,
			Stream:    pgtype.Text{String: "stdin", Valid: true},
			Content:   pgtype.Text{String: "yes", Valid: true},
			CreatedAt: pgtype.Timestamptz{Time: time.Date(2025, 1, 1, 0, 0, 4, 0, time.UTC), Valid: true},
		},
		{
			ID:        pgtype.UUID{Bytes: [16]byte{5}, Valid: true},
			TaskID:    pgtype.UUID{Bytes: [16]byte{10}, Valid: true},
			Sequence:  5,
			Stream:    pgtype.Text{String: "stdout", Valid: true},
			Content:   pgtype.Text{String: "Continuing...", Valid: true},
			CreatedAt: pgtype.Timestamptz{Time: time.Date(2025, 1, 1, 0, 0, 5, 0, time.UTC), Valid: true},
		},
	}

	// Convert to responses (same as the handler does).
	result := make([]taskMessageResponse, 0, len(messages))
	for _, m := range messages {
		result = append(result, toTaskMessageResponse(m))
	}

	// Verify ordering is preserved (sequence ascending).
	for i := 1; i < len(result); i++ {
		if result[i].Sequence <= result[i-1].Sequence {
			t.Errorf("messages not in sequence order: index %d has seq %d, index %d has seq %d",
				i-1, result[i-1].Sequence, i, result[i].Sequence)
		}
	}

	// Verify all stream types are present in the response.
	streamsSeen := map[string]bool{}
	for _, r := range result {
		streamsSeen[r.Stream] = true
	}
	for _, expected := range []string{"stdout", "stderr", "stdin"} {
		if !streamsSeen[expected] {
			t.Errorf("stream type %q not found in response", expected)
		}
	}

	// Verify the stdin message is at the correct position (sequence 4).
	if result[3].Stream != "stdin" {
		t.Errorf("expected stdin at index 3, got %q", result[3].Stream)
	}
	if result[3].Content != "yes" {
		t.Errorf("stdin content = %q, want %q", result[3].Content, "yes")
	}
}
