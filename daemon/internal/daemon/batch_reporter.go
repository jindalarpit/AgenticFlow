package daemon

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/agenticflow/agenticflow/daemon/pkg/agent"
)

const maxToolResultOutput = 8192
const defaultFlushInterval = 500 * time.Millisecond

// TaskMessageData is the structured payload reported from daemon to server.
type TaskMessageData struct {
	Seq     int            `json:"seq"`
	Type    string         `json:"type"`
	Tool    string         `json:"tool,omitempty"`
	Content string         `json:"content,omitempty"`
	Input   map[string]any `json:"input,omitempty"`
	Output  string         `json:"output,omitempty"`
}

// MessageReporter is the interface for sending batched messages to the server.
type MessageReporter interface {
	ReportTaskMessages(taskID string, messages []TaskMessageData) error
}

// BatchReporter accumulates agent Messages and flushes them to the server
// on a configurable interval.
type BatchReporter struct {
	client   MessageReporter
	taskID   string
	interval time.Duration
	logger   *slog.Logger

	mu          sync.Mutex
	textBuf     strings.Builder
	thinkingBuf strings.Builder
	batch       []TaskMessageData
	seq         int

	done   chan struct{}
	closed bool
}

// NewBatchReporter creates a reporter with the given flush interval.
func NewBatchReporter(client MessageReporter, taskID string, interval time.Duration, logger *slog.Logger) *BatchReporter {
	if interval <= 0 {
		interval = defaultFlushInterval
	}
	if logger == nil {
		logger = slog.Default()
	}
	br := &BatchReporter{
		client:   client,
		taskID:   taskID,
		interval: interval,
		logger:   logger,
		done:     make(chan struct{}),
	}
	go br.flushLoop()
	return br
}

// Feed processes a single Message from the agent backend.
func (br *BatchReporter) Feed(msg agent.Message) {
	br.mu.Lock()
	defer br.mu.Unlock()

	switch msg.Type {
	case agent.MessageText:
		br.textBuf.WriteString(msg.Content)
	case agent.MessageThinking:
		br.thinkingBuf.WriteString(msg.Content)
	case agent.MessageToolUse:
		br.drainBuffersLocked()
		br.seq++
		br.batch = append(br.batch, TaskMessageData{
			Seq:   br.seq,
			Type:  "tool_use",
			Tool:  msg.Tool,
			Input: msg.Input,
		})
	case agent.MessageToolResult:
		br.drainBuffersLocked()
		output := msg.Output
		if len(output) > maxToolResultOutput {
			output = output[:maxToolResultOutput]
		}
		br.seq++
		br.batch = append(br.batch, TaskMessageData{
			Seq:    br.seq,
			Type:   "tool_result",
			Tool:   msg.Tool,
			Output: output,
		})
	case agent.MessageError:
		br.drainBuffersLocked()
		br.seq++
		br.batch = append(br.batch, TaskMessageData{
			Seq:     br.seq,
			Type:    "error",
			Content: msg.Content,
		})
	case agent.MessageStatus:
		br.drainBuffersLocked()
		br.seq++
		br.batch = append(br.batch, TaskMessageData{
			Seq:     br.seq,
			Type:    "status",
			Content: msg.Status,
		})
	}
}

// Close performs a final flush and stops the ticker.
func (br *BatchReporter) Close() {
	br.mu.Lock()
	if br.closed {
		br.mu.Unlock()
		return
	}
	br.closed = true
	br.mu.Unlock()

	close(br.done)

	br.mu.Lock()
	br.drainBuffersLocked()
	batch := br.takeBatchLocked()
	br.mu.Unlock()

	if len(batch) > 0 {
		if err := br.client.ReportTaskMessages(br.taskID, batch); err != nil {
			br.logger.Warn("batch_reporter: final flush failed", "task_id", br.taskID, "error", err)
		}
	}
}

func (br *BatchReporter) flushLoop() {
	ticker := time.NewTicker(br.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			br.flush()
		case <-br.done:
			return
		}
	}
}

func (br *BatchReporter) flush() {
	br.mu.Lock()
	br.drainBuffersLocked()
	batch := br.takeBatchLocked()
	br.mu.Unlock()

	if len(batch) == 0 {
		return
	}

	if err := br.client.ReportTaskMessages(br.taskID, batch); err != nil {
		br.logger.Warn("batch_reporter: flush failed", "task_id", br.taskID, "error", err)
	}
}

func (br *BatchReporter) drainBuffersLocked() {
	if br.thinkingBuf.Len() > 0 {
		br.seq++
		br.batch = append(br.batch, TaskMessageData{
			Seq:     br.seq,
			Type:    "thinking",
			Content: br.thinkingBuf.String(),
		})
		br.thinkingBuf.Reset()
	}
	if br.textBuf.Len() > 0 {
		br.seq++
		br.batch = append(br.batch, TaskMessageData{
			Seq:     br.seq,
			Type:    "text",
			Content: br.textBuf.String(),
		})
		br.textBuf.Reset()
	}
}

func (br *BatchReporter) takeBatchLocked() []TaskMessageData {
	if len(br.batch) == 0 {
		return nil
	}
	batch := br.batch
	br.batch = nil
	return batch
}
