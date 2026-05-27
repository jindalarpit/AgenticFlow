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
type sequenceTracker struct {
	mu       sync.Mutex
	counters map[string]*int
}

func newSequenceTracker() *sequenceTracker {
	return &sequenceTracker{
		counters: make(map[string]*int),
	}
}

func (s *sequenceTracker) Register(taskID string, seq *int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[taskID] = seq
}

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

func (s *sequenceTracker) Remove(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.counters, taskID)
}

// HandleWSMessage processes incoming WebSocket messages from the server.
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

func (d *Daemon) handleTaskInput(taskID, text string) {
	logger := d.logger.With("task_id", taskID)

	if err := d.stdinManager.Write(taskID, text); err != nil {
		logger.Warn("failed to write task input", "error", err)
		d.reportInputFailure(taskID, err.Error())
		return
	}

	seq := d.sequences.Next(taskID)
	if seq == 0 {
		logger.Warn("task not found in sequence tracker, using timestamp-based sequence")
		seq = int(time.Now().UnixMilli() % 1000000)
	}

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

func (d *Daemon) reportInputFailure(taskID, errMsg string) {
	logger := d.logger.With("task_id", taskID)

	if d.client == nil {
		return
	}

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
