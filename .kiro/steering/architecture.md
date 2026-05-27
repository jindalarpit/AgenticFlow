---
inclusion: always
---

# AgenticFlow Architecture Rules

This project follows a modular Go backend + React SPA architecture.

## Project Structure

```
AgenticFlow/
├── server/
│   ├── cmd/
│   │   ├── af/          # CLI binary
│   │   └── server/      # Server binary
│   ├── internal/
│   │   ├── auth/        # Token management, PAT cache
│   │   ├── cli/         # CLI config loading
│   │   ├── daemon/      # Daemon runtime
│   │   │   └── execenv/ # Task execution env
│   │   ├── handler/     # HTTP handlers
│   │   ├── middleware/   # Auth middleware
│   │   ├── realtime/    # WebSocket hub
│   │   └── service/     # Business logic
│   ├── migrations/      # SQL migrations
│   ├── pkg/
│   │   └── db/generated/ # sqlc generated code
│   ├── go.mod
│   └── go.sum
├── web/                 # Vite + React SPA (NOT Next.js)
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── hooks/
│   │   └── lib/
│   └── package.json
├── Makefile
├── Dockerfile
└── docker-compose.yml
```

## Core Principles

1. **Single Go module** — All Go code lives under `server/` with a single `go.mod`
2. **Chi router** — Use `github.com/go-chi/chi/v5` for HTTP routing
3. **pgx/v5 + sqlc** — Use `github.com/jackc/pgx/v5` for PostgreSQL, `sqlc` for type-safe queries
4. **gorilla/websocket** — Use for WebSocket connections (daemon ↔ server, client ↔ server)
5. **golang-migrate** — Use for database migrations
6. **slog** — Use `log/slog` for structured logging

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
