# Requirements Document

## Introduction

AgenticFlow is a lightweight, self-hostable platform for detecting local AI CLI agent runtimes, managing a background daemon, and delegating tasks to detected agents via a minimal web interface. Unlike full project management tools, AgenticFlow focuses exclusively on runtime detection, daemon lifecycle, shared authentication, task delegation, and custom agent creation. The system consists of a Go backend (API server + daemon logic), a CLI tool (`af`), and a lightweight web UI.

## Glossary

- **Daemon**: A background process running on the user's local machine that detects agent CLIs, registers runtimes with the Server, polls for tasks, and executes them
- **Server**: The Go HTTP backend that manages authentication, agent registry, task queue, and WebSocket connections
- **CLI**: The `af` command-line tool used to authenticate, configure, and manage the Daemon
- **Agent_Runtime**: A detected AI CLI tool registered by the Daemon as available for task execution
- **Agent_Registry**: The server-side catalog of all registered agent runtimes across connected daemons
- **Task**: A unit of work delegated to a specific Agent_Runtime for execution
- **Task_Queue**: The server-side ordered list of pending tasks awaiting execution
- **Custom_Agent**: A user-defined agent configuration that wraps any CLI tool with a name, command, arguments template, model override, and environment variables
- **PAT**: Personal Access Token — a 90-day bearer token used for daemon-to-server and CLI-to-server authentication
- **Heartbeat**: A periodic signal sent by the Daemon to the Server indicating the daemon is alive and operational
- **Web_UI**: The lightweight browser-based interface for managing agents, viewing status, and delegating tasks
- **Detection_Scanner**: The Daemon subsystem that scans the system PATH for known AI CLI binaries

## Requirements

### Requirement 1: Agent Runtime Detection

**User Story:** As a developer, I want the daemon to automatically detect installed AI CLI tools on my system, so that I can use them for task execution without manual configuration.

#### Acceptance Criteria

1. WHEN the Daemon starts, THE Detection_Scanner SHALL scan the system PATH for the following CLI binaries: `claude`, `gemini`, `opencode`, `openclaw`, `codex`, `copilot`, `hermes`, `pi`, `cursor-agent`, `kimi`, `kiro-cli`
2. WHEN the Detection_Scanner finds a CLI binary on the PATH, THE Daemon SHALL register it as an available Agent_Runtime with the Server for each watched workspace
3. WHEN the Detection_Scanner does not find any CLI binary on the PATH, THE Daemon SHALL log a warning indicating no agent runtimes were detected and continue running
4. WHEN a previously detected CLI binary is removed from the PATH, THE Daemon SHALL deregister the corresponding Agent_Runtime on the next detection scan
5. THE Detection_Scanner SHALL support custom binary paths via environment variables following the pattern `AF_<AGENT_NAME>_PATH` (e.g., `AF_CLAUDE_PATH=/custom/path/claude`), where a custom path takes precedence over PATH lookup for that agent
6. WHEN the Daemon registers an Agent_Runtime, THE Daemon SHALL record the agent name, binary path, and detected version
7. IF the Detection_Scanner cannot determine the version of a detected CLI binary, THEN THE Daemon SHALL register the Agent_Runtime with the version recorded as "unknown"
8. WHEN a custom path is specified via `AF_<AGENT_NAME>_PATH` and the binary does not exist at that path, THEN THE Daemon SHALL log a warning identifying the invalid custom path and skip registration for that agent

### Requirement 2: Daemon Lifecycle Management

**User Story:** As a developer, I want to start, stop, and monitor the daemon process, so that I can control when my machine is available for task execution.

#### Acceptance Criteria

1. WHEN the user runs `af daemon start`, THE Daemon SHALL start as a background process and write its PID to `~/.agenticflow/daemon.pid`
2. WHEN the user runs `af daemon start --foreground`, THE Daemon SHALL run in the foreground and log to stdout
3. WHEN the user runs `af daemon stop`, THE Daemon SHALL gracefully terminate within 30 seconds, deregister all Agent_Runtimes from the Server, and remove the PID file; IF the Daemon does not terminate within 30 seconds, THEN THE CLI SHALL forcefully kill the process and remove the PID file
4. WHEN the user runs `af daemon status`, THE Daemon SHALL report: running state (running or stopped), PID, uptime in seconds, list of detected Agent_Runtimes with their names, and heartbeat status including the timestamp of the last successful heartbeat and connection state (connected or disconnected)
5. WHILE the Daemon is running, THE Daemon SHALL send a Heartbeat to the Server at a configurable interval with a default of 15 seconds
6. IF the Daemon crashes due to an unhandled error, THEN THE Daemon SHALL write the error message and stack trace to `~/.agenticflow/daemon.log` before exiting with a non-zero exit code
7. WHEN the Daemon starts, THE Daemon SHALL check for a stale PID file and clean it up if the referenced process is not running
8. THE Daemon SHALL support configuration via flags and environment variables for: poll interval (default 3s, minimum 1s), heartbeat interval (default 15s, minimum 5s), agent timeout (default 2h, minimum 1m), and max concurrent tasks (default 5, range 1 to 20); flags SHALL take precedence over environment variables, which SHALL take precedence over config file values
9. IF the user runs `af daemon start` while the Daemon is already running, THEN THE CLI SHALL display an error message indicating the Daemon is already running and report the existing PID
10. IF the user runs `af daemon stop` while the Daemon is not running, THEN THE CLI SHALL display a message indicating no running Daemon was found
11. IF the Daemon fails to reach the Server during a Heartbeat, THEN THE Daemon SHALL retry up to 3 times with a 5-second delay between attempts and log a warning after each failed attempt

### Requirement 3: CLI Setup and Authentication

**User Story:** As a developer, I want a simple CLI-based setup and authentication flow, so that I can quickly connect my local machine to the AgenticFlow server.

#### Acceptance Criteria

1. WHEN the user runs `af setup`, THE CLI SHALL prompt for the server URL (defaulting to `http://localhost:8080`), open the browser for authentication, and start the Daemon in sequence
2. WHEN the user runs `af login`, THE CLI SHALL open the default browser to the Server OAuth endpoint and wait for the authentication callback for a maximum of 120 seconds
3. WHEN authentication succeeds, THE CLI SHALL store the PAT in `~/.agenticflow/config.json` with a 90-day expiry timestamp
4. WHILE the PAT has fewer than 7 days remaining before expiry, THE Daemon SHALL automatically refresh the token on each heartbeat cycle and update `~/.agenticflow/config.json`
5. WHEN the user runs `af login --token <token>`, THE CLI SHALL validate the token with the Server and, if valid, store it in `~/.agenticflow/config.json` without opening a browser
6. WHEN the user runs `af auth status`, THE CLI SHALL display the current server URL, authenticated user, and token validity period
7. WHEN the user runs `af auth logout`, THE CLI SHALL remove the stored PAT from `~/.agenticflow/config.json`
8. IF the PAT is expired and auto-refresh fails, THEN THE CLI SHALL prompt the user to re-authenticate via `af login`
9. IF the authentication callback is not received within 120 seconds, THEN THE CLI SHALL cancel the login attempt and display an error message indicating the authentication timed out
10. IF the Server is unreachable during `af login` or `af setup`, THEN THE CLI SHALL display an error message indicating the server connection failed and exit without modifying the configuration file
11. IF the user runs `af login --token <token>` and the Server rejects the token as invalid, THEN THE CLI SHALL display an error message indicating the token is invalid and exit without storing it

### Requirement 4: Task Delegation and Execution

**User Story:** As a developer, I want to delegate tasks to detected agents and monitor their execution, so that I can leverage AI agents for automated work.

#### Acceptance Criteria

1. WHILE the Daemon is running, THE Daemon SHALL poll the Server for assigned tasks at the configured poll interval (default 3s)
2. WHEN the Daemon receives a task assignment, THE Daemon SHALL spawn the specified Agent_Runtime CLI in an isolated workspace directory under `~/.agenticflow/workspaces/`
3. WHILE a task is executing, THE Daemon SHALL stream the agent's stdout and stderr back to the Server via WebSocket
4. WHEN a task's agent process exits with code 0, THE Daemon SHALL report a success status and the final output (truncated to 1 MB if larger) to the Server
5. IF a task exceeds the configured agent timeout, THEN THE Daemon SHALL send SIGTERM to the agent process, wait up to 10 seconds for exit, send SIGKILL if still running, and report a timeout failure to the Server
6. WHEN the user delegates a task via the Web_UI, THE Server SHALL validate that the task prompt does not exceed 32,000 characters and enqueue the task in the Task_Queue with the specified agent type and prompt
7. THE Server SHALL assign queued tasks only to Daemons that have a matching registered Agent_Runtime; tasks with no matching online Daemon SHALL remain in the Task_Queue with a "pending" status until a matching Daemon becomes available
8. WHILE the Daemon is executing tasks at the max concurrent task limit, THE Daemon SHALL skip task polling until a running task completes, leaving queued tasks available for other Daemons
9. IF a task's agent process exits with a non-zero exit code, THEN THE Daemon SHALL report a failure status including the exit code and the last 4,096 characters of stderr to the Server
10. IF the WebSocket connection to the Server drops while a task is executing, THEN THE Daemon SHALL buffer output locally (up to 5 MB) and attempt to reconnect at 5-second intervals; upon reconnection the Daemon SHALL flush the buffered output to the Server

### Requirement 5: Custom Agent Creator

**User Story:** As a developer, I want to define custom agent configurations that wrap any CLI tool, so that I can extend AgenticFlow beyond the built-in agent types.

#### Acceptance Criteria

1. WHEN the user creates a Custom_Agent via the Web_UI, THE Server SHALL store the agent definition with: name (1–64 characters, alphanumeric, hyphens, and underscores only), CLI command, arguments template, optional model override, and up to 20 environment variables
2. IF the user submits a Custom_Agent name that already exists within their account, THEN THE Server SHALL reject the creation request and return an error indicating the name is already in use
3. WHEN the Daemon receives a task targeting a Custom_Agent, THE Daemon SHALL resolve the CLI command path and execute it with the configured arguments template and environment variables
4. THE arguments template SHALL support variable substitution for: `{{prompt}}`, `{{workspace}}`, and `{{model}}`, where `{{model}}` resolves to the configured model override value or is replaced with an empty string if no model override is set
5. WHEN the user updates a Custom_Agent definition, THE Server SHALL propagate the updated definition to connected Daemons on their next poll
6. WHEN the user deletes a Custom_Agent, THE Server SHALL prevent new task assignments to that agent, allow in-flight tasks to complete, and notify connected Daemons to deregister it
7. IF the Custom_Agent CLI command is not found on the Daemon's PATH, THEN THE Daemon SHALL report the agent as unavailable and reject tasks targeting it with an error indicating the command was not found

### Requirement 6: Lightweight Web UI

**User Story:** As a developer, I want a minimal web interface to view connected daemons, manage agents, and delegate tasks, so that I can interact with AgenticFlow without using the CLI.

#### Acceptance Criteria

1. THE Web_UI SHALL display a dashboard showing: connected daemons with their status (online or offline), detected Agent_Runtimes per daemon, and the Task_Queue displaying up to 50 pending tasks with pagination controls for additional entries
2. THE Web_UI SHALL provide a task submission form allowing the user to select a target agent type from available Agent_Runtimes and enter a task prompt of 1 to 10,000 characters
3. IF the user submits a task with an empty prompt or with no agent type selected, THEN THE Web_UI SHALL display a validation error indicating the missing field and SHALL NOT submit the task to the Server
4. WHILE a task is executing, THE Web_UI SHALL display streaming output from the agent via WebSocket, updating the displayed content within 2 seconds of receiving each message from the Server
5. THE Web_UI SHALL display task history with status, duration, agent used, and an output preview truncated to 200 characters, with pagination displaying up to 25 tasks per page
6. THE Web_UI SHALL provide a Custom_Agent creation form with fields for: name, CLI command, arguments template, model override, and environment variables
7. WHEN a Daemon connects or disconnects, THE Web_UI SHALL update the dashboard within 2 seconds via WebSocket
8. IF the WebSocket connection is lost, THEN THE Web_UI SHALL display a connection status indicator and attempt to reconnect at 5-second intervals
9. THE Web_UI SHALL require authentication before displaying any data and SHALL redirect unauthenticated users to the login page
10. IF authentication fails or the session expires, THEN THE Web_UI SHALL redirect the user to the login page and display an error message indicating the authentication failure reason

### Requirement 7: Server API and Data Persistence

**User Story:** As a developer, I want the server to provide a reliable API and persist state, so that the system remains consistent across daemon restarts and browser sessions.

#### Acceptance Criteria

1. THE Server SHALL expose a RESTful HTTP API for: authentication, agent registry, task management, custom agent CRUD, and daemon status
2. THE Server SHALL support PostgreSQL as the primary database for multi-user deployments
3. THE Server SHALL support SQLite as an alternative database for single-user self-hosted deployments
4. THE Server SHALL authenticate all API requests using the PAT in the Authorization header, and IF the PAT is missing, malformed, or expired, THEN THE Server SHALL reject the request with an HTTP 401 response and an error message indicating the authentication failure reason
5. THE Server SHALL expose WebSocket endpoints for: real-time task output streaming and daemon status updates, delivering messages within 2 seconds of the originating event
6. WHEN the Server receives a Heartbeat from a Daemon, THE Server SHALL update the daemon's last-seen timestamp
7. IF a Daemon misses 3 consecutive heartbeat intervals (default interval: 15 seconds, yielding a 45-second offline threshold), THEN THE Server SHALL mark the daemon as offline and deregister its Agent_Runtimes
8. WHEN the Server receives a SIGTERM signal, THE Server SHALL stop accepting new connections, drain in-flight requests within a maximum of 30 seconds, and then shut down
9. IF a WebSocket client connects without a valid PAT, THEN THE Server SHALL reject the connection with an appropriate close frame and an error message indicating authentication failure

### Requirement 8: Self-Hosting and Deployment

**User Story:** As a developer, I want to self-host AgenticFlow with minimal configuration, so that I can run it on my own infrastructure without depending on external services.

#### Acceptance Criteria

1. THE Server SHALL be deployable as a single Docker container with configurable environment variables for database URL, port, and authentication provider
2. THE Server SHALL provide a `docker-compose.yml` that starts the server with PostgreSQL using `docker compose up` with no additional manual steps
3. WHEN the user runs `af setup self-host`, THE CLI SHALL configure the server URL to `http://localhost:8080` and start the Daemon
4. THE Server SHALL support configurable OAuth providers (GitHub, Google) via environment variables for client ID and client secret per provider
5. IF no OAuth provider environment variables are configured, THEN THE Server SHALL enable email/password authentication with passwords requiring a minimum of 8 characters
6. WHILE using SQLite mode, WHEN started on a machine with at least 2 CPU cores and 4 GB RAM, THE Server SHALL be ready to accept requests within 5 seconds
7. WHEN deployed with Docker, THE Server SHALL expose a single port (default 8080) serving both the API and the Web_UI static assets
8. WHEN the Server starts, THE Server SHALL automatically run pending database migrations before accepting requests
9. IF the configured database is unreachable on startup, THEN THE Server SHALL log an error message indicating the connection failure and exit with a non-zero status code within 10 seconds

### Requirement 9: Configuration Parsing and Persistence

**User Story:** As a developer, I want the CLI configuration to be stored in a well-defined JSON format, so that I can inspect and modify settings manually if needed.

#### Acceptance Criteria

1. THE CLI SHALL store all configuration in `~/.agenticflow/config.json`
2. WHEN the user runs `af config show`, THE CLI SHALL display the current configuration as formatted JSON including: server_url, authenticated user email, poll_interval, heartbeat_interval, agent_timeout, and max_concurrent_tasks
3. WHEN the user runs `af config set <key> <value>`, THE CLI SHALL update the specified key in `~/.agenticflow/config.json` where key is one of: server_url, poll_interval, heartbeat_interval, agent_timeout, max_concurrent_tasks
4. THE CLI SHALL guarantee that parsing a valid configuration file, serializing it back to JSON, and parsing the result produces a deeply equal configuration object
5. IF the configuration file is missing or contains invalid JSON or fails schema validation, THEN THE CLI SHALL create a default configuration file with server_url set to empty string, poll_interval set to 3s, heartbeat_interval set to 15s, agent_timeout set to 2h, max_concurrent_tasks set to 5, and log a warning indicating the original file was replaced
6. THE CLI SHALL validate configuration values on write: server_url must be a valid URL with http or https scheme, poll_interval must be a duration between 1s and 300s, heartbeat_interval must be a duration between 5s and 300s, agent_timeout must be a duration between 1m and 24h, and max_concurrent_tasks must be an integer between 1 and 100
7. IF the user runs `af config set` with an unrecognized key or a value that fails validation, THEN THE CLI SHALL reject the write, preserve the existing configuration file unchanged, and display an error message indicating the reason for rejection

### Requirement 10: Task Workspace Isolation

**User Story:** As a developer, I want each task to execute in an isolated workspace directory, so that concurrent tasks do not interfere with each other.

#### Acceptance Criteria

1. WHEN the Daemon begins executing a task, THE Daemon SHALL create a unique workspace directory under `~/.agenticflow/workspaces/<task-id>/`
2. WHEN the Daemon spawns an agent process for a task, THE Daemon SHALL set the working directory of the spawned agent process to the task's workspace directory
3. WHEN a task completes or is cancelled, THE Daemon SHALL retain the workspace directory for a configurable retention period (default 24 hours, minimum 1 hour, maximum 720 hours) and then delete the workspace directory and all its contents
4. WHILE multiple tasks are executing concurrently, THE Daemon SHALL ensure each task operates in its own isolated workspace directory with no shared filesystem paths between task workspaces
5. IF workspace creation fails due to filesystem errors, THEN THE Daemon SHALL transition the task to a failed state and report an error indication including the filesystem error reason to the Server
6. IF the workspace directory for a task already exists when creation is attempted, THEN THE Daemon SHALL remove the existing directory and create a fresh empty workspace directory before executing the task
7. IF workspace deletion fails during cleanup, THEN THE Daemon SHALL log the failure and retry deletion on the next cleanup cycle
