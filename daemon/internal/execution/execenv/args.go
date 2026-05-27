package execenv

// MergeArgs combines daemon-wide default arguments with agent-specific custom
// arguments. Daemon defaults appear first in the returned slice, followed by
// the agent's custom_args. Neither input slice is modified.
//
// This satisfies Requirement 14.3: "THE Daemon SHALL append the custom arguments
// to the CLI invocation after any daemon-wide default arguments."
func MergeArgs(daemonDefaults, agentCustomArgs []string) []string {
	if len(daemonDefaults) == 0 && len(agentCustomArgs) == 0 {
		return nil
	}

	result := make([]string, 0, len(daemonDefaults)+len(agentCustomArgs))
	result = append(result, daemonDefaults...)
	result = append(result, agentCustomArgs...)
	return result
}

// ResolveModel determines which model to use for a task execution.
// If the agent has a model override (non-empty), it takes precedence.
// Otherwise, the daemon-wide default model is used.
//
// This satisfies Requirement 14.4: "THE Daemon SHALL pass the model value to
// the CLI backend using the provider-appropriate mechanism (--model flag or
// equivalent), falling back to the daemon-wide provider model if the
// agent-level model is empty."
func ResolveModel(agentModel, daemonModel string) string {
	if agentModel != "" {
		return agentModel
	}
	return daemonModel
}
