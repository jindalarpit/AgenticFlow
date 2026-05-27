package api

// DaemonRegisterRequest is sent by the daemon on startup to register
// itself and its detected agent CLI runtimes with the server.
type DaemonRegisterRequest struct {
	DaemonID   string               `json:"daemon_id"`
	DeviceName string               `json:"device_name"`
	CLIVersion string               `json:"cli_version"`
	Agents     map[string]AgentInfo `json:"agents"`
}

// AgentInfo describes a detected CLI runtime on the daemon's machine.
type AgentInfo struct {
	Path    string `json:"path"`
	Model   string `json:"model"`
	Version string `json:"version"`
}

// TaskClaimResponse is returned by GET /api/daemon/tasks/poll when a
// pending task is available for the daemon to execute.
type TaskClaimResponse struct {
	ID              string         `json:"id"`
	AgentType       string         `json:"agent_type"`
	Prompt          string         `json:"prompt"`
	Status          string         `json:"status"`
	WorkspaceMode   string         `json:"workspace_mode,omitempty"`
	WorkspacePath   string         `json:"workspace_path,omitempty"`
	Agent           *TaskAgentData `json:"agent,omitempty"`
	CurrentStage    *StageInfo     `json:"current_stage,omitempty"`
	PriorStages     []StageInfo    `json:"prior_stages,omitempty"`
	DeliverableType string         `json:"deliverable_type,omitempty"`
}

// TaskAgentData holds agent configuration for task execution.
type TaskAgentData struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Instructions string            `json:"instructions"`
	CustomEnv    map[string]string `json:"custom_env,omitempty"`
	CustomArgs   []string          `json:"custom_args,omitempty"`
	Model        string            `json:"model,omitempty"`
}

// TaskCompleteRequest is sent by the daemon when a task finishes successfully.
type TaskCompleteRequest struct {
	Output    string `json:"output"`
	ExitCode  int32  `json:"exit_code"`
	SessionID string `json:"session_id,omitempty"`
	WorkDir   string `json:"work_dir,omitempty"`
}

// TaskFailRequest is sent by the daemon when a task fails.
type TaskFailRequest struct {
	ErrorMessage string `json:"error_message"`
	ExitCode     int32  `json:"exit_code"`
}
