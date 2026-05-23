package daemon

import (
	"encoding/json"
	"testing"
)

func TestTaskClaimResponse_JSONRoundTrip(t *testing.T) {
	t.Run("with agent data", func(t *testing.T) {
		resp := TaskClaimResponse{
			ID:        "task-123",
			AgentType: "claude",
			Prompt:    "Fix the login bug",
			Agent: &TaskAgentData{
				ID:           "agent-456",
				Name:         "Nexus",
				Instructions: "You are a helpful coding agent.",
				CustomEnv:    map[string]string{"GITHUB_TOKEN": "ghp_xxx"},
				CustomArgs:   []string{"--dangerously-skip-permissions"},
				Model:        "claude-sonnet-4-20250514",
			},
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		var decoded TaskClaimResponse
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if decoded.ID != resp.ID {
			t.Errorf("ID: got %q, want %q", decoded.ID, resp.ID)
		}
		if decoded.AgentType != resp.AgentType {
			t.Errorf("AgentType: got %q, want %q", decoded.AgentType, resp.AgentType)
		}
		if decoded.Prompt != resp.Prompt {
			t.Errorf("Prompt: got %q, want %q", decoded.Prompt, resp.Prompt)
		}
		if decoded.Agent == nil {
			t.Fatal("Agent: got nil, want non-nil")
		}
		if decoded.Agent.ID != resp.Agent.ID {
			t.Errorf("Agent.ID: got %q, want %q", decoded.Agent.ID, resp.Agent.ID)
		}
		if decoded.Agent.Name != resp.Agent.Name {
			t.Errorf("Agent.Name: got %q, want %q", decoded.Agent.Name, resp.Agent.Name)
		}
		if decoded.Agent.Instructions != resp.Agent.Instructions {
			t.Errorf("Agent.Instructions: got %q, want %q", decoded.Agent.Instructions, resp.Agent.Instructions)
		}
		if decoded.Agent.Model != resp.Agent.Model {
			t.Errorf("Agent.Model: got %q, want %q", decoded.Agent.Model, resp.Agent.Model)
		}
		if len(decoded.Agent.CustomEnv) != len(resp.Agent.CustomEnv) {
			t.Errorf("Agent.CustomEnv length: got %d, want %d", len(decoded.Agent.CustomEnv), len(resp.Agent.CustomEnv))
		}
		if decoded.Agent.CustomEnv["GITHUB_TOKEN"] != "ghp_xxx" {
			t.Errorf("Agent.CustomEnv[GITHUB_TOKEN]: got %q, want %q", decoded.Agent.CustomEnv["GITHUB_TOKEN"], "ghp_xxx")
		}
		if len(decoded.Agent.CustomArgs) != 1 || decoded.Agent.CustomArgs[0] != "--dangerously-skip-permissions" {
			t.Errorf("Agent.CustomArgs: got %v, want %v", decoded.Agent.CustomArgs, resp.Agent.CustomArgs)
		}
	})

	t.Run("without agent data (backward compat)", func(t *testing.T) {
		// Simulate a response from an older server that doesn't include agent data.
		jsonStr := `{"id":"task-789","agent_type":"claude","prompt":"Hello world"}`

		var decoded TaskClaimResponse
		if err := json.Unmarshal([]byte(jsonStr), &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if decoded.ID != "task-789" {
			t.Errorf("ID: got %q, want %q", decoded.ID, "task-789")
		}
		if decoded.AgentType != "claude" {
			t.Errorf("AgentType: got %q, want %q", decoded.AgentType, "claude")
		}
		if decoded.Prompt != "Hello world" {
			t.Errorf("Prompt: got %q, want %q", decoded.Prompt, "Hello world")
		}
		if decoded.Agent != nil {
			t.Errorf("Agent: got %+v, want nil (backward compat)", decoded.Agent)
		}
	})

	t.Run("agent with empty optional fields", func(t *testing.T) {
		resp := TaskClaimResponse{
			ID:        "task-abc",
			AgentType: "opencode",
			Prompt:    "Refactor utils",
			Agent: &TaskAgentData{
				ID:           "agent-def",
				Name:         "Nexus",
				Instructions: "Be concise.",
				// CustomEnv, CustomArgs, Model are all zero-value (omitted in JSON)
			},
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		// Verify omitempty works: custom_env, custom_args, model should not appear.
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("unmarshal raw: %v", err)
		}

		agentRaw, ok := raw["agent"].(map[string]interface{})
		if !ok {
			t.Fatal("agent field missing from JSON")
		}
		if _, exists := agentRaw["custom_env"]; exists {
			t.Error("custom_env should be omitted when nil")
		}
		if _, exists := agentRaw["custom_args"]; exists {
			t.Error("custom_args should be omitted when nil")
		}
		if _, exists := agentRaw["model"]; exists {
			t.Error("model should be omitted when empty")
		}
	})

	t.Run("nil agent omitted from JSON", func(t *testing.T) {
		resp := TaskClaimResponse{
			ID:        "task-nil",
			AgentType: "gemini",
			Prompt:    "Test prompt",
			Agent:     nil,
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("unmarshal raw: %v", err)
		}

		if _, exists := raw["agent"]; exists {
			t.Error("agent field should be omitted when nil")
		}
	})
}

func TestTaskAgentData_JSONSerialization(t *testing.T) {
	t.Run("full agent data", func(t *testing.T) {
		agent := TaskAgentData{
			ID:           "agent-001",
			Name:         "CodeReviewer",
			Instructions: "Review code for security issues.\nBe thorough.",
			CustomEnv: map[string]string{
				"API_KEY":    "secret123",
				"DEBUG_MODE": "true",
			},
			CustomArgs: []string{"--verbose", "--format=json"},
			Model:      "gpt-4o",
		}

		data, err := json.Marshal(agent)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		var decoded TaskAgentData
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if decoded.ID != agent.ID {
			t.Errorf("ID: got %q, want %q", decoded.ID, agent.ID)
		}
		if decoded.Name != agent.Name {
			t.Errorf("Name: got %q, want %q", decoded.Name, agent.Name)
		}
		if decoded.Instructions != agent.Instructions {
			t.Errorf("Instructions: got %q, want %q", decoded.Instructions, agent.Instructions)
		}
		if decoded.Model != agent.Model {
			t.Errorf("Model: got %q, want %q", decoded.Model, agent.Model)
		}
		if len(decoded.CustomEnv) != 2 {
			t.Errorf("CustomEnv length: got %d, want 2", len(decoded.CustomEnv))
		}
		if len(decoded.CustomArgs) != 2 {
			t.Errorf("CustomArgs length: got %d, want 2", len(decoded.CustomArgs))
		}
	})
}
