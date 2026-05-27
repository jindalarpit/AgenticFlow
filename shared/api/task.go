package api

// TaskMessagesRequest carries streaming output chunks from the daemon
// to the server during task execution.
type TaskMessagesRequest struct {
	Messages []TaskMessageEntry `json:"messages"`
}

// TaskMessageEntry is a single output chunk reported by the daemon.
type TaskMessageEntry struct {
	Sequence int32          `json:"sequence"`
	Stream   string         `json:"stream"`
	Content  string         `json:"content"`
	Seq      int32          `json:"seq"`
	Type     string         `json:"type,omitempty"`
	Tool     string         `json:"tool,omitempty"`
	Input    map[string]any `json:"input,omitempty"`
	Output   string         `json:"output,omitempty"`
}

// StageInfo describes a workflow stage in the task poll response.
type StageInfo struct {
	Name          string `json:"name"`
	Order         int32  `json:"order"`
	Status        string `json:"status"`
	OutputContent string `json:"output_content,omitempty"`
}
