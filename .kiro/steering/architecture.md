---
inclusion: always
---

# AgenticFlow Architecture Rules

This project follows a modular Go backend + React SPA architecture.

## Project Structure

```
AgenticFlow/
├── go.work              # Go workspace: server/, daemon/, shared/
├── server/
│   ├── cmd/
│   │   └── server/      # Server binary
│   ├── internal/
│   │   ├── handler/     # HTTP handlers
│   │   ├── middleware/   # Auth middleware, body limits, worker pool
│   │   ├── migrate/     # Database migration runner
│   │   ├── realtime/    # WebSocket hub (multi-connection)
│   │   └── service/     # Business logic (TaskService, AgentService)
│   ├── migrations/      # SQL migrations
│   ├── pkg/
│   │   └── db/generated/ # sqlc generated code (Querier interface)
│   ├── queries/         # SQL query files for sqlc
│   ├── go.mod
│   └── go.sum
├── daemon/
│   ├── cmd/
│   │   └── af/          # CLI + daemon binary
│   ├── internal/
│   │   ├── cli/         # CLI config loading
│   │   ├── daemon/      # Daemon runtime, WS push receiver
│   │   ├── detection/   # AI runtime detection
│   │   ├── execution/   # Unified task executor, backpressure buffer
│   │   ├── health/      # Health check
│   │   ├── release/     # Release management
│   │   └── ws/          # WebSocket client for server push
│   ├── pkg/
│   │   ├── agent/       # Agent type definitions
│   │   ├── mcp/         # MCP protocol support
│   │   └── skill/       # Skill definitions
│   ├── go.mod
│   └── go.sum
├── shared/
│   ├── api/             # Request/response types (daemon↔server)
│   ├── constants/       # Status strings, default values
│   ├── httputil/        # HTTP response helpers (WriteJSON, WriteErrorJSON)
│   ├── pgutil/          # PostgreSQL utilities (UUIDToString)
│   ├── go.mod
│   └── go.sum
├── web/                 # Vite + React SPA (NOT Next.js)
│   ├── src/
│   │   ├── components/  # UI components (ErrorBoundary, etc.)
│   │   ├── contexts/    # React contexts (WebSocketProvider)
│   │   ├── pages/
│   │   ├── hooks/
│   │   └── lib/         # WebSocketClient class, utilities
│   └── package.json
├── Makefile
├── Dockerfile
└── docker-compose.yml
```

## Core Principles

1. **Go workspace** — The project uses `go.work` with three modules: `server/`, `daemon/`, and `shared/`
2. **Chi router** — Use `github.com/go-chi/chi/v5` for HTTP routing
3. **pgx/v5 + sqlc** — Use `github.com/jackc/pgx/v5` for PostgreSQL, `sqlc` for type-safe queries
4. **gorilla/websocket** — Use for WebSocket connections (daemon ↔ server, client ↔ server)
5. **golang-migrate** — Use for database migrations
6. **slog** — Use `log/slog` for structured logging

## Shared Module (`shared/`)

The `shared/` module contains constants, types, and utility packages used by both Server and Daemon:

- **`shared/api/`** — Request/response types for daemon↔server communication (task claims, daemon registration, WebSocket events)
- **`shared/constants/`** — Status strings, default configuration values
- **`shared/pgutil/`** — PostgreSQL utility functions (`UUIDToString` for `pgtype.UUID` conversion)
- **`shared/httputil/`** — HTTP response helpers (`WriteJSON`, `WriteErrorJSON`)

Both `server/` and `daemon/` import from `shared/` via the Go workspace. Never duplicate utility functions — add shared code here instead.

## What NOT to Include

- NO workspace/team management
- NO issue tracking, projects, sprints, comments, labels
- NO inbox, notifications, activity log
- NO squads, autopilots
- NO cloud runtime fleet
- NO Redis requirement for single-node deployments
- NO complex RBAC (simple user-owns-their-resources model)

## Default Agent

AgenticFlow creates a default agent called **"Nexus"** on first user setup. This agent is bound to the first detected local AI CLI runtime. The default agent:
- Has name "Nexus" with a default avatar
- Is bound to the first available runtime detected by the daemon
- Can be customized (instructions, model, env vars, args)
- Serves as the primary task execution target
