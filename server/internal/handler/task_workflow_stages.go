package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// validDeliverables is the set of accepted deliverable values.
var validDeliverables = map[string]bool{
	"plan":      true,
	"design":    true,
	"tasks":     true,
	"execution": true,
}

// deliverableOrder defines the canonical execution order for deliverables.
var deliverableOrder = map[string]int{
	"plan":      1,
	"design":    2,
	"tasks":     3,
	"execution": 4,
}

// validateDeliverables checks that the deliverables array is non-empty and
// contains only valid values from the set {plan, design, tasks, execution}.
// Returns an error message string or empty string if valid.
func validateDeliverables(deliverables []string) string {
	if len(deliverables) == 0 {
		return "deliverables must contain at least one valid value"
	}
	for _, d := range deliverables {
		if !validDeliverables[d] {
			return fmt.Sprintf("invalid deliverable: %s. Valid values: plan, design, tasks, execution", d)
		}
	}
	return ""
}

// validateWorkspaceMode checks that workspace_mode is either "isolated" or "existing".
// Returns an error message string or empty string if valid.
func validateWorkspaceMode(mode string) string {
	if mode != "isolated" && mode != "existing" {
		return "workspace_mode must be 'isolated' or 'existing'"
	}
	return ""
}

// validateWorkspacePath checks workspace_path requirements based on the workspace_mode.
// When mode is "existing", workspace_path must be non-empty and an absolute path.
// Returns an error message string or empty string if valid.
func validateWorkspacePath(mode, path string) string {
	if mode != "existing" {
		return ""
	}
	if strings.TrimSpace(path) == "" {
		return "workspace_path is required when workspace_mode is 'existing'"
	}
	if !strings.HasPrefix(path, "/") {
		return "workspace_path must be an absolute path"
	}
	return ""
}

// orderDeliverables sorts a slice of valid deliverable names by their canonical
// execution order (plan=1, design=2, tasks=3, execution=4) and removes duplicates.
// The input must contain only valid deliverable names.
func orderDeliverables(deliverables []string) []string {
	if len(deliverables) == 0 {
		return []string{}
	}

	// Deduplicate using a set
	seen := make(map[string]bool, len(deliverables))
	unique := make([]string, 0, len(deliverables))
	for _, d := range deliverables {
		if !seen[d] {
			seen[d] = true
			unique = append(unique, d)
		}
	}

	// Sort by canonical order
	sort.Slice(unique, func(i, j int) bool {
		return deliverableOrder[unique[i]] < deliverableOrder[unique[j]]
	})

	return unique
}

// ---------------------------------------------------------------------------
// GET /api/tasks/{taskId}/stages
// ---------------------------------------------------------------------------

// stageResponse represents a single workflow stage in the API response.
type stageResponse struct {
	Name          string  `json:"name"`
	Order         int32   `json:"order"`
	Status        string  `json:"status"`
	OutputContent *string `json:"output_content,omitempty"`
	Feedback      *string `json:"feedback,omitempty"`
	StartedAt     *string `json:"started_at,omitempty"`
	CompletedAt   *string `json:"completed_at,omitempty"`
}

func toStageResponse(s db.TaskStage) stageResponse {
	resp := stageResponse{
		Name:   s.StageName,
		Order:  s.StageOrder,
		Status: s.Status,
	}
	if s.OutputContent.Valid {
		resp.OutputContent = &s.OutputContent.String
	}
	if s.Feedback.Valid {
		resp.Feedback = &s.Feedback.String
	}
	if s.StartedAt.Valid {
		ts := s.StartedAt.Time.UTC().Format(time.RFC3339)
		resp.StartedAt = &ts
	}
	if s.CompletedAt.Valid {
		ts := s.CompletedAt.Time.UTC().Format(time.RFC3339)
		resp.CompletedAt = &ts
	}
	return resp
}

// ListStages returns all workflow stages for a task ordered by stage_order.
func (h *UserHandler) ListStages(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "taskId is required")
		return
	}

	taskUUID, err := parseUUID(taskID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid task id")
		return
	}

	// Verify task exists and belongs to the requesting user.
	task, err := h.Queries.GetTaskByID(r.Context(), taskUUID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "task not found")
		return
	}
	if uuidToString(task.UserID) != userID {
		writeErrorJSON(w, http.StatusNotFound, "task not found")
		return
	}

	stages, err := h.Queries.ListStagesForTask(r.Context(), taskUUID)
	if err != nil {
		slog.Error("list stages: query failed", "task_id", taskID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to list stages")
		return
	}

	result := make([]stageResponse, 0, len(stages))
	for _, s := range stages {
		result = append(result, toStageResponse(s))
	}

	writeJSON(w, http.StatusOK, result)
}
