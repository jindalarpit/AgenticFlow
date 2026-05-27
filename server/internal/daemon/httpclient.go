package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RealHTTPClient implements HTTPClient using real HTTP calls to the server.
type RealHTTPClient struct {
	baseURL    string
	token      string
	daemonID   string
	httpClient *http.Client
}

// NewRealHTTPClient creates a new HTTP client for daemon-to-server communication.
func NewRealHTTPClient(baseURL, token, daemonID string) *RealHTTPClient {
	return &RealHTTPClient{
		baseURL:  baseURL,
		token:    token,
		daemonID: daemonID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *RealHTTPClient) doJSON(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-Daemon-ID", c.daemonID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	// 204 No Content means no data available (e.g., no tasks to poll).
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *RealHTTPClient) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	// The handler expects agents as map[string]AgentInfo with path/model/version fields.
	// The daemon sends agents as map[string]string (provider → version).
	// Transform to the handler's expected format.
	type agentInfo struct {
		Path    string `json:"path"`
		Model   string `json:"model"`
		Version string `json:"version"`
	}
	body := struct {
		DaemonID   string               `json:"daemon_id"`
		DeviceName string               `json:"device_name"`
		CLIVersion string               `json:"cli_version"`
		Agents     map[string]agentInfo `json:"agents"`
	}{
		DaemonID:   req.DaemonID,
		DeviceName: req.DeviceName,
		Agents:     make(map[string]agentInfo, len(req.Agents)),
	}
	for provider, version := range req.Agents {
		body.Agents[provider] = agentInfo{Version: version}
	}

	data, err := c.doJSON(ctx, "POST", "/api/daemon/register", body)
	if err != nil {
		return nil, err
	}
	var resp struct {
		RuntimeIDs map[string]string `json:"runtime_ids"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode register response: %w", err)
	}
	return &RegisterResponse{RuntimeIDs: resp.RuntimeIDs}, nil
}

func (c *RealHTTPClient) Deregister(ctx context.Context, req DeregisterRequest) error {
	_, err := c.doJSON(ctx, "POST", "/api/daemon/deregister", req)
	return err
}

func (c *RealHTTPClient) Heartbeat(ctx context.Context, req HeartbeatRequest) error {
	_, err := c.doJSON(ctx, "POST", "/api/daemon/heartbeat", req)
	return err
}

func (c *RealHTTPClient) PollTasks(ctx context.Context, req PollRequest) (*PollResponse, error) {
	data, err := c.doJSON(ctx, "GET", "/api/daemon/tasks/poll", nil)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return &PollResponse{}, nil
	}
	// Decode the full poll response including optional stage and conversational fields.
	var raw struct {
		ID            string            `json:"id"`
		AgentType     string            `json:"agent_type"`
		Prompt        string            `json:"prompt"`
		Status        string            `json:"status"`
		Model         string            `json:"model"`
		ArgsTemplate  string            `json:"args_template"`
		EnvVars       map[string]string `json:"env_vars"`
		Agent         *TaskAgentData    `json:"agent"`
		CurrentStage  *StageInfo        `json:"current_stage"`
		PriorStages   []PriorStage      `json:"prior_stages"`
		WorkspaceMode string            `json:"workspace_mode"`
		WorkspacePath string            `json:"workspace_path"`
		StageFeedback string            `json:"stage_feedback"`
		// Conversational task fields.
		DeliverableType string           `json:"deliverable_type"`
		PriorSessionID  string           `json:"prior_session_id"`
		PriorContext    []string         `json:"prior_context"`
		PriorWorkDir    string           `json:"prior_work_dir"`
		WorkspaceConfig *WorkspaceConfig `json:"workspace_config"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode poll response: %w", err)
	}
	return &PollResponse{
		TaskID:          raw.ID,
		AgentType:       raw.AgentType,
		Prompt:          raw.Prompt,
		Model:           raw.Model,
		ArgsTemplate:    raw.ArgsTemplate,
		EnvVars:         raw.EnvVars,
		Agent:           raw.Agent,
		CurrentStage:    raw.CurrentStage,
		PriorStages:     raw.PriorStages,
		WorkspaceMode:   raw.WorkspaceMode,
		WorkspacePath:   raw.WorkspacePath,
		StageFeedback:   raw.StageFeedback,
		DeliverableType: raw.DeliverableType,
		PriorSessionID:  raw.PriorSessionID,
		PriorContext:    raw.PriorContext,
		PriorWorkDir:    raw.PriorWorkDir,
		WorkspaceConfig: raw.WorkspaceConfig,
	}, nil
}

func (c *RealHTTPClient) StartTask(ctx context.Context, taskID string) error {
	_, err := c.doJSON(ctx, "POST", "/api/daemon/tasks/"+taskID+"/start", nil)
	return err
}

func (c *RealHTTPClient) CompleteTask(ctx context.Context, taskID string, output string, exitCode int) error {
	body := map[string]interface{}{
		"output":    output,
		"exit_code": exitCode,
	}
	_, err := c.doJSON(ctx, "POST", "/api/daemon/tasks/"+taskID+"/complete", body)
	return err
}

func (c *RealHTTPClient) FailTask(ctx context.Context, taskID string, errMsg string, exitCode int) error {
	body := map[string]interface{}{
		"error_message": errMsg,
		"exit_code":     exitCode,
	}
	_, err := c.doJSON(ctx, "POST", "/api/daemon/tasks/"+taskID+"/fail", body)
	return err
}

func (c *RealHTTPClient) ReportMessages(ctx context.Context, taskID string, messages []TaskMessage) error {
	body := map[string]interface{}{
		"messages": messages,
	}
	_, err := c.doJSON(ctx, "POST", "/api/daemon/tasks/"+taskID+"/messages", body)
	return err
}

// ReportTaskMessages sends structured TaskMessageData payloads to the server.
// This implements the MessageReporter interface used by BatchReporter.
func (c *RealHTTPClient) ReportTaskMessages(taskID string, messages []TaskMessageData) error {
	body := map[string]interface{}{
		"messages": messages,
	}
	_, err := c.doJSON(context.Background(), "POST", "/api/daemon/tasks/"+taskID+"/messages", body)
	return err
}

func (c *RealHTTPClient) ReportInputState(ctx context.Context, taskID string, state string) error {
	body := map[string]interface{}{
		"state": state,
	}
	_, err := c.doJSON(ctx, "POST", "/api/daemon/tasks/"+taskID+"/input-state", body)
	return err
}

// ReportStageCompletion reports a workflow stage completion to the server.
// POST /api/daemon/tasks/{taskId}/stages/{stageName}/complete
func (c *RealHTTPClient) ReportStageCompletion(ctx context.Context, taskID, stageName, outputContent string) error {
	body := map[string]interface{}{
		"output_content": outputContent,
	}
	_, err := c.doJSON(ctx, "POST", "/api/daemon/tasks/"+taskID+"/stages/"+stageName+"/complete", body)
	return err
}

// CompleteTaskConversational reports conversational task completion with session tracking.
// POST /api/daemon/tasks/{id}/complete with session_id and work_dir fields.
func (c *RealHTTPClient) CompleteTaskConversational(ctx context.Context, taskID, output, sessionID, workDir string) error {
	body := map[string]interface{}{
		"output":    output,
		"exit_code": 0,
	}
	if sessionID != "" {
		body["session_id"] = sessionID
	}
	if workDir != "" {
		body["work_dir"] = workDir
	}
	_, err := c.doJSON(ctx, "POST", "/api/daemon/tasks/"+taskID+"/complete", body)
	return err
}
