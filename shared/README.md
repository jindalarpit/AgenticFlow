# Shared API Contract

This module (`github.com/agenticflow/agenticflow/shared`) defines the communication protocol between the AgenticFlow Server and Daemon. It contains request/response types, WebSocket event definitions, and shared constants used by both components.

The shared module has **no external dependencies** (stdlib only) and must never import from `server/` or `daemon/`.

---

## HTTP Endpoints

All daemon-facing endpoints are prefixed with `/api/daemon/`. The Daemon authenticates via a Bearer token in the `Authorization` header.

### POST /api/daemon/register

Daemon registers itself with the Server on startup. Reports device info, CLI version, and detected agent runtimes.

**Request Body:**

```json
{
  "daemon_id": "d-abc123",
  "device_name": "arpit-macbook",
  "cli_version": "0.3.1",
  "agents": {
    "claude": {
      "path": "/usr/local/bin/claude",
      "model": "claude-sonnet-4-20250514",
      "version": "1.0.0"
    },
    "gemini": {
      "path": "/usr/local/bin/gemini",
      "model": "gemini-2.5-pro",
      "version": "0.5.2"
    }
  }
}
```

**Response (200 OK):**

```json
{
  "id": "daemon-uuid",
  "status": "online",
  "registered_at": "2025-01-15T10:30:00Z"
}
```

**Go Types:**

```go
type DaemonRegisterRequest struct {
    DaemonID   string               `json:"daemon_id"`
    DeviceName string               `json:"device_name"`
    CLIVersion string               `json:"cli_version"`
    Agents     map[string]AgentInfo `json:"agents"`
}

type AgentInfo struct {
    Path    string `json:"path"`
    Model   string `json:"model"`
    Version string `json:"version"`
}
```

---

### GET /api/daemon/tasks/poll

Daemon polls for a pending task to execute. The Server finds a pending task matching the daemon's registered runtimes, atomically claims it, and returns the task with full agent configuration.

**Response (200 OK) — Task available:**

```json
{
  "id": "task-uuid-123",
  "agent_type": "claude",
  "prompt": "Fix the login bug in auth.go",
  "status": "pending",
  "workspace_mode": "isolated",
  "workspace_path": "",
  "agent": {
    "id": "agent-uuid-456",
    "name": "Nexus",
    "instructions": "You are a helpful coding agent. Follow best practices.",
    "custom_env": {
      "GITHUB_TOKEN": "ghp_xxxx"
    },
    "custom_args": ["--dangerously-skip-permissions"],
    "model": "claude-sonnet-4-20250514"
  },
  "current_stage": {
    "name": "implementation",
    "order": 2,
    "status": "pending"
  },
  "prior_stages": [
    {
      "name": "planning",
      "order": 1,
      "status": "completed",
      "output_content": "Plan: refactor auth middleware..."
    }
  ],
  "deliverable_type": "code"
}
```

**Response (204 No Content) — No tasks available:**

Empty body. The Daemon should wait `DefaultPollInterval` (3s) before polling again.

**Go Types:**

```go
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

type TaskAgentData struct {
    ID           string            `json:"id"`
    Name         string            `json:"name"`
    Instructions string            `json:"instructions"`
    CustomEnv    map[string]string `json:"custom_env,omitempty"`
    CustomArgs   []string          `json:"custom_args,omitempty"`
    Model        string            `json:"model,omitempty"`
}

type StageInfo struct {
    Name          string `json:"name"`
    Order         int32  `json:"order"`
    Status        string `json:"status"`
    OutputContent string `json:"output_content,omitempty"`
}
```

---

### POST /api/daemon/tasks/{id}/start

Daemon reports that task execution has started. The Server updates the task status to `running` and broadcasts a `task_started` WebSocket event.

**Request Body:** Empty (no body required).

**Response (200 OK):**

```json
{
  "status": "ok"
}
```

---

### POST /api/daemon/tasks/{id}/messages

Daemon streams output chunks (stdout/stderr) from the running agent process. Messages are delivered in batches for efficiency. The Server persists them and broadcasts `task_output` WebSocket events to connected web clients.

**Request Body:**

```json
{
  "messages": [
    {
      "sequence": 1,
      "stream": "stdout",
      "content": "Analyzing auth.go...\n",
      "seq": 1,
      "type": "text"
    },
    {
      "sequence": 2,
      "stream": "stdout",
      "content": "Found 3 issues in login handler\n",
      "seq": 2,
      "type": "text"
    },
    {
      "sequence": 3,
      "stream": "stderr",
      "content": "Warning: deprecated API usage\n",
      "seq": 3,
      "type": "text"
    },
    {
      "sequence": 4,
      "stream": "stdout",
      "content": "",
      "seq": 4,
      "type": "tool_use",
      "tool": "edit_file",
      "input": {"path": "auth.go", "line": 42},
      "output": "Applied fix to auth.go:42"
    }
  ]
}
```

**Response (200 OK):**

```json
{
  "status": "ok"
}
```

**Go Types:**

```go
type TaskMessagesRequest struct {
    Messages []TaskMessageEntry `json:"messages"`
}

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
```

**Stream values:** `"stdout"`, `"stderr"`

**Type values:** `"text"` (default), `"tool_use"`, `"tool_result"`

---

### POST /api/daemon/tasks/{id}/complete

Daemon reports that the task finished successfully. The Server updates the task status to `completed` and broadcasts a `task_completed` WebSocket event.

**Request Body:**

```json
{
  "output": "Successfully fixed the login bug. Changes applied to auth.go and auth_test.go.",
  "exit_code": 0,
  "session_id": "session-abc123",
  "work_dir": "/home/user/.agenticflow/workspaces/task-uuid-123"
}
```

**Response (200 OK):**

```json
{
  "status": "ok"
}
```

**Go Types:**

```go
type TaskCompleteRequest struct {
    Output    string `json:"output"`
    ExitCode  int32  `json:"exit_code"`
    SessionID string `json:"session_id,omitempty"`
    WorkDir   string `json:"work_dir,omitempty"`
}
```

---

### POST /api/daemon/tasks/{id}/fail

Daemon reports that the task failed. The Server updates the task status to `failed` and broadcasts a `task_failed` WebSocket event.

**Request Body:**

```json
{
  "error_message": "Agent process exited with code 1: permission denied accessing /etc/secrets",
  "exit_code": 1
}
```

**Response (200 OK):**

```json
{
  "status": "ok"
}
```

**Go Types:**

```go
type TaskFailRequest struct {
    ErrorMessage string `json:"error_message"`
    ExitCode     int32  `json:"exit_code"`
}
```

---

## WebSocket Events

The Server broadcasts real-time events to connected Web Frontend clients via WebSocket at `ws://<server>/api/ws`. Events follow a standard envelope format:

### Event Envelope

```json
{
  "type": "<event_type>",
  "payload": { ... }
}
```

### task_created

Broadcast when a new task is created via the Web UI.

```json
{
  "type": "task_created",
  "payload": {
    "id": "task-uuid-123",
    "agent_id": "agent-uuid-456",
    "agent_name": "Nexus",
    "prompt": "Fix the login bug in auth.go",
    "status": "pending",
    "created_at": "2025-01-15T10:30:00Z"
  }
}
```

### task_started

Broadcast when the Daemon begins executing a task.

```json
{
  "type": "task_started",
  "payload": {
    "id": "task-uuid-123",
    "status": "running",
    "started_at": "2025-01-15T10:30:05Z"
  }
}
```

### task_output

Broadcast when the Daemon streams output chunks. Sent for each batch of messages received at `/api/daemon/tasks/{id}/messages`.

```json
{
  "type": "task_output",
  "payload": {
    "id": "task-uuid-123",
    "sequence": 1,
    "stream": "stdout",
    "content": "Analyzing auth.go...\n",
    "type": "text"
  }
}
```

### task_completed

Broadcast when a task finishes successfully.

```json
{
  "type": "task_completed",
  "payload": {
    "id": "task-uuid-123",
    "status": "completed",
    "output": "Successfully fixed the login bug.",
    "exit_code": 0,
    "completed_at": "2025-01-15T10:32:00Z"
  }
}
```

### task_failed

Broadcast when a task fails.

```json
{
  "type": "task_failed",
  "payload": {
    "id": "task-uuid-123",
    "status": "failed",
    "error_message": "Agent process exited with code 1",
    "exit_code": 1,
    "failed_at": "2025-01-15T10:31:30Z"
  }
}
```

---

## Shared Constants

### Default Values

| Constant | Value | Description |
|----------|-------|-------------|
| `DefaultServerPort` | `8080` | Default HTTP port for the Server |
| `DefaultDaemonHealthPort` | `8081` | Default health check port for the Daemon |
| `DefaultPollInterval` | `3s` | How often the Daemon polls for tasks |
| `DefaultHeartbeatInterval` | `15s` | How often the Daemon sends heartbeats |
| `DefaultAgentTimeout` | `2h` | Maximum execution time for a single task |
| `DefaultMaxConcurrentTasks` | `5` | Maximum tasks a Daemon can run simultaneously |

### Status Enums

#### Task Status

| Value | Description |
|-------|-------------|
| `pending` | Task created, waiting for a Daemon to claim it |
| `running` | Task is being executed by a Daemon |
| `completed` | Task finished successfully |
| `failed` | Task execution failed |

#### Agent Status

| Value | Description |
|-------|-------------|
| `idle` | No running tasks |
| `working` | At least one task is running |
| `offline` | Bound runtime's Daemon is offline |
| `error` | Last task failed |

#### Daemon Status

| Value | Description |
|-------|-------------|
| `online` | Daemon is connected and polling |
| `offline` | Daemon is not reachable |

---

## Authentication

All `/api/daemon/*` endpoints require a Bearer token in the `Authorization` header:

```
Authorization: Bearer <daemon-auth-token>
```

The token is obtained during `af auth login` or `af auth token` CLI commands and stored in the Daemon's local configuration file.

---

## Error Responses

All endpoints return standard error responses on failure:

```json
{
  "error": "descriptive error message",
  "code": "ERROR_CODE"
}
```

| HTTP Status | Meaning |
|-------------|---------|
| `400` | Bad request (malformed JSON, missing required fields) |
| `401` | Unauthorized (missing or invalid auth token) |
| `404` | Resource not found (invalid task ID) |
| `409` | Conflict (task already claimed by another daemon) |
| `500` | Internal server error |
