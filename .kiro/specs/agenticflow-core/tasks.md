# Implementation Plan: AgenticFlow Core

## Overview

Build AgenticFlow incrementally: foundation first (Go module, DB schema, migrations), then daemon (detection, lifecycle, execution), then server (API, WebSocket, auth), then web UI (pages, real-time streaming). The implementation mirrors multica's architecture patterns throughout.

## Tasks

- [x] 1. Set up project foundation and Go module
  - [x] 1.1 Initialize Go module and project structure
    - Create `server/go.mod` with module `github.com/agenticflow/agenticflow`
    - Create directory structure: `server/cmd/af/`, `server/cmd/server/`, `server/internal/auth/`, `server/internal/cli/`, `server/internal/daemon/`, `server/internal/daemon/execenv/`, `server/internal/handler/`, `server/internal/middleware/`, `server/internal/realtime/`, `server/internal/service/`, `server/pkg/db/generated/`, `server/migrations/`
    - Add core dependencies: `chi/v5`, `pgx/v5`, `gorilla/websocket`, `golang-migrate`, `cobra`, `sqlc`
    - Create `Makefile` with targets: `build`, `dev`, `test`, `migrate-up`, `migrate-down`, `sqlc-generate`, `check`
    - _Requirements: 8.1, 8.2_

  - [x] 1.2 Create database schema and migrations
    - Create migration `001_initial_schema.up.sql` with tables: `user`, `personal_access_token`, `daemon`, `agent_runtime`, `custom_agent`, `task`, `task_message`
    - Create corresponding `001_initial_schema.down.sql`
    - Include all indexes defined in the design (idx_daemon_user, idx_task_status, idx_task_message_task, etc.)
    - _Requirements: 7.2, 7.3, 8.8_

  - [x] 1.3 Configure sqlc and generate type-safe query code
    - Create `server/sqlc.yaml` configuration pointing to migrations and generated output
    - Write SQL queries for all CRUD operations: users, tokens, daemons, agent_runtimes, custom_agents, tasks, task_messages
    - Run `sqlc generate` to produce type-safe Go code in `server/pkg/db/generated/`
    - _Requirements: 7.1, 7.2_

  - [x] 1.4 Create Docker and deployment configuration
    - Create `Dockerfile` (multi-stage: Go build + web build + minimal runtime)
    - Create `docker-compose.yml` with server + PostgreSQL services
    - Configure environment variables for database URL, port, OAuth providers
    - _Requirements: 8.1, 8.2, 8.7_

- [x] 2. Implement CLI configuration and authentication
  - [x] 2.1 Implement CLI config loading and persistence
    - Create `server/internal/cli/config.go` with Config struct (server_url, token, token_expires_at, user_email, poll_interval, heartbeat_interval, agent_timeout, max_concurrent_tasks)
    - Implement `LoadConfig()` reading from `~/.agenticflow/config.json`
    - Implement `SaveConfig()` writing formatted JSON
    - Implement config validation: URL scheme check, duration ranges, integer bounds
    - Handle missing/corrupt config file by creating defaults
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6, 9.7_

  - [x] 2.2 Write property tests for configuration
    - **Property 3: Configuration Resolution Precedence**
    - **Property 4: Configuration Serialization Round-Trip**
    - **Property 5: Configuration Value Validation**
    - **Validates: Requirements 2.8, 9.4, 9.6, 9.7**

  - [x] 2.3 Implement CLI command tree with Cobra
    - Create `server/cmd/af/main.go` with root command
    - Implement `af setup` — prompt server URL, open browser auth, start daemon
    - Implement `af login` — open browser OAuth, wait 120s for callback
    - Implement `af login --token <token>` — validate and store token
    - Implement `af auth status` — display server URL, user, token validity
    - Implement `af auth logout` — remove stored PAT
    - Implement `af config show` — display formatted JSON config
    - Implement `af config set <key> <value>` — validate and update config
    - _Requirements: 3.1, 3.2, 3.3, 3.5, 3.6, 3.7, 9.2, 9.3_

  - [x] 2.4 Write property test for token storage
    - **Property 16: Token Storage Round-Trip**
    - **Validates: Requirements 3.3**

- [x] 3. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. Implement daemon detection and lifecycle
  - [x] 4.1 Implement agent detection scanner
    - Create `server/internal/daemon/detection.go` with `DetectAgents()` function
    - Implement `probe()` pattern using `exec.LookPath()` for each known agent binary
    - Support custom paths via `AF_<AGENT_NAME>_PATH` environment variables with precedence over PATH
    - Support model override via `AF_<AGENT_NAME>_MODEL` environment variables
    - Handle version detection (fallback to "unknown")
    - Log warnings for invalid custom paths
    - _Requirements: 1.1, 1.2, 1.3, 1.5, 1.6, 1.7, 1.8_

  - [x] 4.2 Write property tests for agent detection
    - **Property 1: Agent Detection with Precedence**
    - **Property 2: Agent Deregistration Equals Scan Difference**
    - **Validates: Requirements 1.1, 1.4, 1.5, 1.6, 1.7, 1.8**

  - [x] 4.3 Implement daemon config resolution
    - Create `server/internal/daemon/config.go` with Config struct and LoadConfig()
    - Implement resolution order: CLI flags > env vars > config file > defaults
    - Support env vars: `AF_SERVER_URL`, `AF_DAEMON_POLL_INTERVAL`, `AF_DAEMON_HEARTBEAT_INTERVAL`, `AF_AGENT_TIMEOUT`, `AF_DAEMON_MAX_CONCURRENT_TASKS`
    - _Requirements: 2.8_

  - [x] 4.4 Implement daemon run loop and lifecycle
    - Create `server/internal/daemon/daemon.go` with Daemon struct
    - Implement `Run(ctx)`: register → start heartbeat loop → start poll loop → start GC loop → deregister on shutdown
    - Implement `Stop()`: graceful shutdown with 30s timeout
    - Implement PID file management (write on start, check stale on startup, remove on stop)
    - Implement heartbeat loop with retry (3× with 5s delay)
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.9, 2.10, 2.11_

  - [x] 4.5 Implement daemon CLI commands
    - Implement `af daemon start` — start background process, write PID
    - Implement `af daemon start --foreground` — run in foreground, log to stdout
    - Implement `af daemon stop` — graceful terminate, deregister, remove PID
    - Implement `af daemon status` — report state, PID, uptime, agents, heartbeat
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.9, 2.10_

  - [x] 4.6 Write property test for daemon status output
    - **Property 17: Daemon Status Output Completeness**
    - **Validates: Requirements 2.4**

- [x] 5. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. Implement task execution environment
  - [x] 6.1 Implement workspace isolation and exec environment
    - Create `server/internal/daemon/execenv/execenv.go` with ExecEnv struct
    - Implement `Setup()`: create workspace dir at `~/.agenticflow/workspaces/<task-id>/`
    - Implement `Run(ctx, stdout, stderr)`: spawn agent CLI with working dir, env vars, args template
    - Implement `Cleanup()`: remove workspace after retention period
    - Handle workspace already exists (remove and recreate)
    - Handle filesystem errors (transition task to failed)
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7_

  - [x] 6.2 Write property tests for workspace isolation and templates
    - **Property 10: Workspace Isolation**
    - **Property 11: Template Variable Substitution**
    - **Validates: Requirements 10.1, 10.4, 4.2, 5.4**

  - [x] 6.3 Implement task execution in daemon poll loop
    - Implement task polling with concurrency limit check
    - Implement task claim → start → execute → complete/fail flow
    - Implement output streaming (stdout/stderr) to server via HTTP
    - Implement timeout handling: SIGTERM → 10s wait → SIGKILL
    - Implement output truncation (1 MB stdout, 4096 chars stderr)
    - Implement local output buffering on WebSocket disconnect (up to 5 MB)
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.8, 4.9, 4.10_

  - [x] 6.4 Write property tests for task execution
    - **Property 8: Concurrent Task Polling Suppression**
    - **Property 9: Output Truncation**
    - **Validates: Requirements 4.8, 4.4, 4.9**

- [x] 7. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 8. Implement server API and authentication
  - [x] 8.1 Implement PAT authentication and middleware
    - Create `server/internal/auth/pat.go` with GeneratePAT(), HashToken(), ValidatePAT()
    - Use `af_` prefix for tokens, 90-day expiry
    - Create `server/internal/middleware/auth.go` with Auth middleware (user PAT validation)
    - Create `server/internal/middleware/daemon_auth.go` with DaemonAuth middleware
    - Implement in-memory PAT cache for performance
    - Return HTTP 401 for missing, malformed, or expired tokens
    - _Requirements: 7.4, 7.9_

  - [x] 8.2 Write property tests for authentication
    - **Property 13: PAT Authentication Enforcement**
    - **Property 15: Password Validation**
    - **Validates: Requirements 7.4, 8.5**

  - [x] 8.3 Implement server main and router
    - Create `server/cmd/server/main.go`: load config, run migrations, init DB pool, create hub, build router, start HTTP server with graceful shutdown
    - Create `server/cmd/server/router.go` with Chi router: global middleware, health endpoint, auth routes, WebSocket route, daemon API group, protected user API group, static file serving
    - Implement CORS configuration
    - Implement graceful shutdown (stop accepting, drain 30s, exit)
    - _Requirements: 7.1, 7.8, 8.7, 8.8_

  - [x] 8.4 Implement daemon API handlers
    - Create `server/internal/handler/daemon.go`
    - Implement `POST /api/daemon/register` — upsert daemon + agent_runtimes
    - Implement `POST /api/daemon/deregister` — mark offline, deregister runtimes
    - Implement `POST /api/daemon/heartbeat` — update last_heartbeat_at
    - Implement `GET /api/daemon/tasks/poll` — claim queued task matching daemon's runtimes
    - Implement `POST /api/daemon/tasks/{id}/start` — mark task running
    - Implement `POST /api/daemon/tasks/{id}/complete` — mark task completed
    - Implement `POST /api/daemon/tasks/{id}/fail` — mark task failed
    - Implement `POST /api/daemon/tasks/{id}/messages` — store streaming output
    - _Requirements: 4.7, 7.1, 7.6, 7.7_

  - [x] 8.5 Write property tests for task assignment and daemon offline
    - **Property 7: Task Assignment Matching**
    - **Property 14: Daemon Offline Detection**
    - **Validates: Requirements 4.7, 7.7**

  - [x] 8.6 Implement user API handlers
    - Create `server/internal/handler/user.go`
    - Implement `GET /api/me` — return current user info
    - Implement `GET /api/daemons` — list user's daemons
    - Implement `GET /api/agents` — list agent runtimes
    - Implement task CRUD: `POST /api/tasks`, `GET /api/tasks` (paginated), `GET /api/tasks/{id}`, `GET /api/tasks/{id}/messages`, `POST /api/tasks/{id}/cancel`
    - Implement custom agent CRUD: `POST /api/custom-agents`, `GET /api/custom-agents`, `PUT /api/custom-agents/{id}`, `DELETE /api/custom-agents/{id}`
    - Implement token management: `GET /api/tokens`, `POST /api/tokens`, `DELETE /api/tokens/{id}`
    - _Requirements: 5.1, 5.2, 5.5, 5.6, 6.1, 7.1_

  - [x] 8.7 Write property tests for task and custom agent validation
    - **Property 6: Task Prompt Validation**
    - **Property 12: Custom Agent Name Validation**
    - **Validates: Requirements 4.6, 5.1, 6.3**

  - [x] 8.8 Implement auth handlers (login, register, OAuth)
    - Create `server/internal/handler/auth.go`
    - Implement `POST /auth/login` — email/password login, return PAT
    - Implement `POST /auth/register` — create user with password (min 8 chars)
    - Implement `GET /auth/callback/{provider}` — OAuth callback (GitHub, Google)
    - Create `server/internal/auth/oauth.go` with OAuthConfig and handler
    - _Requirements: 3.2, 8.4, 8.5_

- [x] 9. Implement WebSocket real-time hub
  - [x] 9.1 Implement WebSocket hub and client management
    - Create `server/internal/realtime/hub.go` with Hub struct (clients map, daemons map, broadcast channel)
    - Implement `Run(ctx)` — main event loop processing register/unregister/broadcast
    - Implement `Broadcast(event)`, `SendToUser(userID, event)`, `SendToDaemon(daemonID, event)`
    - Create `server/internal/realtime/client.go` with Client struct and read/write pumps
    - Implement WebSocket upgrade handler with PAT authentication via query param or header
    - _Requirements: 7.5, 7.9_

  - [x] 9.2 Wire WebSocket events into handlers
    - Broadcast `task_created` on task creation
    - Broadcast `task_started` when daemon starts execution
    - Broadcast `task_output` for streaming stdout/stderr chunks
    - Broadcast `task_completed` / `task_failed` on task finish
    - Broadcast `daemon_connected` / `daemon_disconnected` on daemon register/deregister
    - _Requirements: 4.3, 6.4, 6.7, 7.5_

- [x] 10. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 11. Implement Web UI foundation
  - [x] 11.1 Initialize Vite + React + TypeScript project
    - Create `web/` directory with Vite + React + TypeScript template
    - Install dependencies: `@tanstack/react-query`, `react-router-dom`, `tailwindcss`
    - Configure Tailwind CSS
    - Set up project structure: `src/components/`, `src/pages/`, `src/hooks/`, `src/lib/`
    - Create API client in `src/lib/api.ts` with PAT auth header injection
    - Create WebSocket client in `src/lib/ws.ts` with auto-reconnect (5s intervals)
    - _Requirements: 6.1, 6.8_

  - [x] 11.2 Implement authentication pages
    - Create Login page (`/login`) with email/password form and OAuth buttons
    - Implement auth state management (store token, redirect on expiry)
    - Implement protected route wrapper (redirect unauthenticated users to login)
    - _Requirements: 6.9, 6.10_

  - [x] 11.3 Implement Dashboard page
    - Create Dashboard page (`/`) showing: connected daemons with status, detected agent runtimes per daemon, task queue with up to 50 pending tasks and pagination
    - Wire WebSocket events for real-time daemon connect/disconnect updates
    - _Requirements: 6.1, 6.7_

  - [x] 11.4 Implement Task submission and detail pages
    - Create task submission form: agent type dropdown (from available runtimes), prompt textarea (1–10,000 chars), validation (empty prompt, no agent selected)
    - Create Task Detail page (`/tasks/:id`) with streaming output display via WebSocket
    - Show task status badge (pending → running → completed/failed)
    - Render terminal-like output with stdout/stderr differentiation
    - _Requirements: 6.2, 6.3, 6.4_

  - [x] 11.5 Implement Task History and Custom Agents pages
    - Create Task History page (`/history`) with paginated list (25 per page), status, duration, agent, output preview (200 chars)
    - Create Custom Agents page (`/agents`) with CRUD form: name, command, args template, model override, env vars (key-value editor)
    - _Requirements: 6.5, 6.6_

  - [x] 11.6 Implement WebSocket connection indicator and error handling
    - Add connection status indicator (connected/disconnected) in app header
    - Implement reconnect logic at 5s intervals on disconnect
    - Implement toast notifications for API errors
    - Implement React Query cache invalidation on WebSocket events
    - _Requirements: 6.8_

- [x] 12. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- The Go backend uses `rapid` library for property-based testing
- The Web UI uses Vitest + React Testing Library for component tests
- Reference multica codebase at `/Users/arpit.jindal/workspace/opensource/multica` for implementation patterns throughout

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "1.4"] },
    { "id": 2, "tasks": ["1.3"] },
    { "id": 3, "tasks": ["2.1", "4.1"] },
    { "id": 4, "tasks": ["2.2", "2.3", "4.2", "4.3"] },
    { "id": 5, "tasks": ["2.4", "4.4"] },
    { "id": 6, "tasks": ["4.5", "4.6"] },
    { "id": 7, "tasks": ["6.1"] },
    { "id": 8, "tasks": ["6.2", "6.3"] },
    { "id": 9, "tasks": ["6.4"] },
    { "id": 10, "tasks": ["8.1", "9.1"] },
    { "id": 11, "tasks": ["8.2", "8.3"] },
    { "id": 12, "tasks": ["8.4", "8.8"] },
    { "id": 13, "tasks": ["8.5", "8.6", "9.2"] },
    { "id": 14, "tasks": ["8.7"] },
    { "id": 15, "tasks": ["11.1"] },
    { "id": 16, "tasks": ["11.2"] },
    { "id": 17, "tasks": ["11.3", "11.4"] },
    { "id": 18, "tasks": ["11.5", "11.6"] }
  ]
}
```
