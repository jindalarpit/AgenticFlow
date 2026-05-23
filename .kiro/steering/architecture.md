---
inclusion: always
---

# AgenticFlow Architecture Rules

This project mirrors the architecture of [multica](https://github.com/multica-ai/multica). When implementing any feature, reference the multica codebase at `/Users/arpit.jindal/workspace/opensource/multica` for patterns.

## Project Structure

```
AgenticFlow/
├── server/
│   ├── cmd/
│   │   ├── af/          # CLI binary (like multica/server/cmd/multica/)
│   │   └── server/      # Server binary (like multica/server/cmd/server/)
│   ├── internal/
│   │   ├── auth/        # Token management, PAT cache (like multica/server/internal/auth/)
│   │   ├── cli/         # CLI config loading (like multica/server/internal/cli/)
│   │   ├── daemon/      # Daemon runtime (like multica/server/internal/daemon/)
│   │   │   └── execenv/ # Task execution env (like multica/server/internal/daemon/execenv/)
│   │   ├── handler/     # HTTP handlers (like multica/server/internal/handler/)
│   │   ├── middleware/   # Auth middleware (like multica/server/internal/middleware/)
│   │   ├── realtime/    # WebSocket hub (like multica/server/internal/realtime/)
│   │   └── service/     # Business logic (like multica/server/internal/service/)
│   ├── migrations/      # SQL migrations (like multica/server/migrations/)
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
2. **Chi router** — Use `github.com/go-chi/chi/v5` for HTTP routing (same as multica)
3. **pgx/v5 + sqlc** — Use `github.com/jackc/pgx/v5` for PostgreSQL, `sqlc` for type-safe queries
4. **gorilla/websocket** — Use for WebSocket connections (daemon ↔ server, client ↔ server)
5. **golang-migrate** — Use for database migrations
6. **slog** — Use `log/slog` for structured logging (same as multica)

## What NOT to Include

- NO workspace/team management (multica has complex workspace membership — we skip it)
- NO issue tracking, projects, sprints, comments, labels
- NO inbox, notifications, activity log
- NO squads, autopilots
- NO cloud runtime fleet
- NO Redis requirement for single-node deployments
- NO complex RBAC (simple user-owns-their-resources model)

## Default Agent

AgenticFlow creates a default agent called **"Nexus"** on first user setup (similar to how multica creates "Orion"). This agent is bound to the first detected local AI CLI runtime. The default agent:
- Has name "Nexus" with a default avatar
- Is bound to the first available runtime detected by the daemon
- Can be customized (instructions, model, env vars, args)
- Serves as the primary task execution target
