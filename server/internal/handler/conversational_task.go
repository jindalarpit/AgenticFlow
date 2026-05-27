package handler

import (
	"errors"
	"strings"
)

// ValidDeliverableTypes is the set of accepted deliverable_type values for
// conversational tasks.
var ValidDeliverableTypes = map[string]bool{
	"plan":      true,
	"design":    true,
	"tasks":     true,
	"execution": true,
}

// ValidateDeliverableType checks that the given string is a valid deliverable type.
// Returns nil if valid, or an error with a descriptive message if invalid.
func ValidateDeliverableType(s string) error {
	if !ValidDeliverableTypes[s] {
		return errors.New("invalid deliverable_type: must be one of plan, design, tasks, execution")
	}
	return nil
}

// ConversationalTaskCreateRequest is the request body for creating a conversational task.
type ConversationalTaskCreateRequest struct {
	AgentID            string   `json:"agent_id"`
	Prompt             string   `json:"prompt"`
	DeliverableType    string   `json:"deliverable_type"`
	PriorContext       []string `json:"prior_context,omitempty"`
	GitRepoURL         string   `json:"git_repo_url,omitempty"`
	LocalDirectoryPath string   `json:"local_directory_path,omitempty"`
}

// Validate checks the ConversationalTaskCreateRequest for correctness.
// Returns nil if valid, or an error with a descriptive message matching the
// design doc error table.
func (r *ConversationalTaskCreateRequest) Validate() error {
	// Empty prompt check.
	if strings.TrimSpace(r.Prompt) == "" {
		return errors.New("prompt is required")
	}

	// Deliverable type validation.
	if err := ValidateDeliverableType(r.DeliverableType); err != nil {
		return err
	}

	// Execution type requires local_directory_path.
	if r.DeliverableType == "execution" {
		if strings.TrimSpace(r.LocalDirectoryPath) == "" {
			return errors.New("local_directory_path is required for execution deliverable type")
		}
		if !strings.HasPrefix(r.LocalDirectoryPath, "/") {
			return errors.New("local_directory_path must be an absolute path")
		}
	}

	return nil
}

// FollowUpRequest is the request body for sending a follow-up message to refine
// a deliverable's output.
type FollowUpRequest struct {
	Prompt string `json:"prompt"`
}

// Validate checks the FollowUpRequest for correctness.
func (r *FollowUpRequest) Validate() error {
	if strings.TrimSpace(r.Prompt) == "" {
		return errors.New("prompt is required")
	}
	return nil
}

// TaskCompletionRequest is the enhanced completion payload from the daemon,
// including session tracking fields for conversational tasks.
type TaskCompletionRequest struct {
	Output    string `json:"output"`
	SessionID string `json:"session_id,omitempty"`
	WorkDir   string `json:"work_dir,omitempty"`
	ExitCode  int    `json:"exit_code"`
}

// PromptHistoryEntry represents one turn in a conversation, used as the
// JSON response type for the prompt history API.
type PromptHistoryEntry struct {
	ID          string  `json:"id"`
	TaskStageID string  `json:"task_stage_id"`
	TaskID      string  `json:"task_id"`
	PromptText  string  `json:"prompt_text"`
	OutputText  *string `json:"output_text,omitempty"`
	CreatedAt   string  `json:"created_at"`
}
