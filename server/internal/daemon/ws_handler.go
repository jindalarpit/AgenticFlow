package daemon

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// WSEvent represents a generic WebSocket event received from the server.
type WSEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// TaskInputPayload is the payload for a "task_input" WebSocket event.
type TaskInputPayload struct {
	TaskID string `json:"task_id"`
	Text   string `json:"text"`
}

// sequenceTracker maintains per-task sequence counters for stdin messages.
// The stdout/stderr sequences are managed by the streamingReporter in executeTask,
// so stdin messages need their own sequence space to avoid conflicts. However,
// since the server uses a single sequence per task for ordering, we share the
// same counter. This tracker is used when handleTaskInput needs to report a
// stdin message outside the streamingReporter's scope.
type sequenceTracker struct {
	mu       sync.Mutex
	counters map[string]*int // task_id -> shared sequence pointer
}

func newSequenceTracker() *sequenceTracker {
	return &sequenceTracker{
		counters: make(map[string]*int),
	}
}

// Register stores a sequence pointer for a task. This should be the same
// pointer used by the streamingReporter so all streams share one counter.
func (s *sequenceTracker) Register(taskID string, seq *int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[taskID] = seq
}

// Next increments and returns the next sequence number for a task.
// Returns 0 if the task is not registered.
func (s *sequenceTracker) Next(taskID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	seq, ok := s.counters[taskID]
	if !ok {
		return 0
	}
	*seq++
	return *seq
}

// Remove removes the sequence counter for a task.
func (s *sequenceTracker) Remove(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.counters, taskID)
}

// HandleWSMessage processes incoming WebSocket messages from the server.
// It dispatches events to the appropriate handler based on the event type.
func (d *Daemon) HandleWSMessage(msg []byte) {
	var event WSEvent
	if err := json.Unmarshal(msg, &event); err != nil {
		d.logger.Warn("invalid WS message", "error", err)
		return
	}

	switch event.Type {
	case "task_input":
		var payload TaskInputPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			d.logger.Warn("invalid task_input payload", "error", err)
			return
		}
		d.handleTaskInput(payload.TaskID, payload.Text)
	default:
		d.logger.Debug("unhandled WS event type", "type", event.Type)
	}
}

// handleTaskInput writes user input to the task's stdin pipe and reports
// the stdin message to the server for persistence and broadcast.
//
// On success: reports the input as a task message with stream "stdin".
// On failure: logs a warning and reports the failure to the server.
func (d *Daemon) handleTaskInput(taskID, text string) {
	logger := d.logger.With("task_id", taskID)

	if err := d.stdinManager.Write(taskID, text); err != nil {
		logger.Warn("failed to write task input", "error", err)
		d.reportInputFailure(taskID, err.Error())
		return
	}

	// Get the next sequence number for this task's message stream.
	seq := d.sequences.Next(taskID)
	if seq == 0 {
		// Task not registered in sequence tracker — use a fallback.
		// This can happen if the task was started before the sequence tracker
		// was integrated, or if there's a race condition.
		logger.Warn("task not found in sequence tracker, using timestamp-based sequence")
		seq = int(time.Now().UnixMilli() % 1000000)
	}

	// Report the stdin message to the server for persistence and broadcast.
	msg := TaskMessage{
		Sequence: seq,
		Stream:   "stdin",
		Content:  text,
	}

	if d.client == nil {
		logger.Debug("no HTTP client configured, skipping stdin message report")
		return
	}

	if err := d.client.ReportMessages(context.Background(), taskID, []TaskMessage{msg}); err != nil {
		logger.Warn("failed to report stdin message", "error", err)
	} else {
		logger.Debug("stdin message reported", "sequence", seq)
	}
}

// reportInputFailure reports a stdin write failure to the server.
// This is a best-effort notification — failures to report are logged but not fatal.
func (d *Daemon) reportInputFailure(taskID, errMsg string) {
	logger := d.logger.With("task_id", taskID)

	if d.client == nil {
		return
	}

	// Report the failure as a stderr message so it appears in the task output.
	seq := d.sequences.Next(taskID)
	if seq == 0 {
		seq = int(time.Now().UnixMilli() % 1000000)
	}

	msg := TaskMessage{
		Sequence: seq,
		Stream:   "stderr",
		Content:  "[stdin write failed: " + errMsg + "]\n",
	}

	if err := d.client.ReportMessages(context.Background(), taskID, []TaskMessage{msg}); err != nil {
		logger.Warn("failed to report input failure", "error", err)
	}
}
