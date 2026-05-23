package daemon

// TaskClaimResponse is the canonical response from the task claim endpoint
// (GET /api/daemon/tasks/poll). It contains the task details and optionally
// the full agent configuration for Runtime_Brief construction and execution.
//
// When Agent is nil (backward compatible), the daemon executes with prompt only:
// no brief injection, no custom env/args merging from agent config.
//
// When Agent is present, the daemon uses its fields to:
//   - Build the Runtime_Brief from Instructions (via BuildRuntimeBrief)
//   - Inject the brief via InjectBrief based on the provider
//   - Merge env vars via execenv.MergeEnv
//   - Merge args via execenv.MergeArgs
//   - Resolve model via execenv.ResolveModel
type TaskClaimResponse struct {
	ID        string         `json:"id"`
	AgentType string         `json:"agent_type"`
	Prompt    string         `json:"prompt"`
	Agent     *TaskAgentData `json:"agent,omitempty"`
}

// TaskAgentData holds agent configuration returned by the task claim endpoint.
// When present, the daemon uses these fields to construct the Runtime_Brief,
// merge custom environment variables, append custom arguments, and override
// the model. When nil (backward compat), the daemon executes with prompt only.
type TaskAgentData struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Instructions string            `json:"instructions"`
	CustomEnv    map[string]string `json:"custom_env,omitempty"`
	CustomArgs   []string          `json:"custom_args,omitempty"`
	Model        string            `json:"model,omitempty"`
}
