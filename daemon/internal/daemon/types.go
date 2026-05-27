package daemon

import (
	"encoding/json"

	"github.com/agenticflow/agenticflow/shared/api"
)

// TaskClaimResponse is the canonical response from the task claim endpoint
// (GET /api/daemon/tasks/poll). It contains the task details and optionally
// the full agent configuration for Runtime_Brief construction and execution.
type TaskClaimResponse struct {
	ID        string         `json:"id"`
	AgentType string         `json:"agent_type"`
	Prompt    string         `json:"prompt"`
	Agent     *TaskAgentData `json:"agent,omitempty"`
}

// TaskAgentData holds agent configuration returned by the task claim endpoint.
type TaskAgentData struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Instructions string            `json:"instructions"`
	CustomEnv    map[string]string `json:"custom_env,omitempty"`
	CustomArgs   []string          `json:"custom_args,omitempty"`
	Model        string            `json:"model,omitempty"`
	Skills       []api.TaskSkill   `json:"skills,omitempty"`
	MCPConfig    json.RawMessage   `json:"mcp_config,omitempty"`
}
