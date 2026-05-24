# AgenticFlow

A self-hosted platform for delegating coding tasks to local AI agents. Run AI coding assistants (Claude Code, Codex, Gemini CLI) on your own machine, manage them through a web dashboard, and stream their output in real time.

## Why AgenticFlow?

- **Self-hosted** — Your code never leaves your machine. No cloud runtimes, no third-party access.
- **Agent-first** — Define agents with custom instructions, environment variables, and model preferences. Delegate tasks and let them work.
- **Real-time streaming** — Watch agent output as it happens via WebSocket-powered terminal view.
- **Multi-runtime** — Supports any CLI-based AI tool (Claude Code, OpenAI Codex, Gemini CLI, custom scripts).
- **Simple deployment** — Single Go binary + PostgreSQL. No Redis, no Kubernetes, no complex infrastructure.

## Architecture

```
┌─────────────┐       WebSocket        ┌──────────────┐
│   Web UI    │◄──────────────────────►│    Server    │
│  (React)    │       REST API          │   (Go/Chi)   │
└─────────────┘                         └──────┬───────┘
                                               │
                                               │ Poll + Report
                                               │
                                        ┌──────▼───────┐
                                        │    Daemon    │
                                        │  (per host)  │
                                        └──────┬───────┘
                                               │
                                               │ Spawns
                                               │
                                   ┌───────────▼───────────┐
                                   │   AI CLI Runtimes     │
                                   │ (Claude, Codex, etc.) │
                                   └───────────────────────┘
```

**Server** — Go HTTP server handling auth, task management, agent configuration, and WebSocket broadcasting.

**Daemon** — Lightweight process running on your dev machine. Detects installed AI CLIs, polls the server for pending tasks, executes them, and streams output back.

**Web UI** — Vite + React SPA for managing agents, delegating tasks, and viewing real-time output.

## Quick Start

### Prerequisites

- Go 1.23+
- Node.js 20+
- PostgreSQL 16+
- An AI CLI installed (e.g., `claude`, `codex`, `gemini`)

### 1. Clone and configure

```bash
git clone https://github.com/agenticflow/agenticflow.git
cd agenticflow
cp .env.example .env
# Edit .env with your database credentials
```

### 2. Start PostgreSQL

```bash
# Using Docker
docker compose up postgres -d

# Or use an existing PostgreSQL instance — just set DATABASE_URL in .env
```

### 3. Run migrations

```bash
make migrate-up
```

### 4. Start the server

```bash
make dev
```

The server starts on `http://localhost:8080`.

### 5. Start the daemon

In a separate terminal:

```bash
make daemon
```

The daemon connects to the server, detects installed AI CLIs, and begins polling for tasks.

### 6. Start the web UI (development)

```bash
cd web
npm install
npm run dev
```

The UI is available at `http://localhost:5173` (proxies API calls to the server).

## Docker Deployment

Run the full stack with a single command:

```bash
docker compose up -d
```

This starts:
- **server** — Go backend serving the API and the built web UI on port 8080
- **postgres** — PostgreSQL 16 database

The daemon still runs on your host machine (it needs access to local AI CLIs):

```bash
make daemon
```

### Build from source

```bash
docker compose build
docker compose up -d
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgres://agenticflow:agenticflow@localhost:5432/agenticflow?sslmode=disable` | PostgreSQL connection string |
| `PORT` | `8080` | Server listen port |
| `GITHUB_CLIENT_ID` | — | GitHub OAuth app client ID |
| `GITHUB_CLIENT_SECRET` | — | GitHub OAuth app client secret |
| `GOOGLE_CLIENT_ID` | — | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | — | Google OAuth client secret |

### Daemon Configuration

The daemon stores its config at `~/.agenticflow/daemon.json`. On first run it auto-detects installed AI CLIs and registers them as available runtimes.

## Usage

### 1. Create an agent

After logging in, navigate to **Agents → New Agent**. Configure:

- **Name** — e.g., "Nexus" (your default coding agent)
- **Runtime** — Select a detected CLI (Claude Code, Codex, etc.)
- **Model** — Override the default model (optional)
- **Instructions** — System prompt for the agent
- **Environment Variables** — API keys or config the agent needs
- **Custom Arguments** — Extra CLI flags
- **Max Concurrent Tasks** — How many tasks this agent can run simultaneously (1–20)

A default agent called **"Nexus"** is created automatically on first setup, bound to the first detected runtime.

### 2. Delegate a task

From the dashboard, select an agent and enter your prompt:

```
Fix the authentication bug in server/internal/auth/token.go — 
the token expiry check is off by one hour.
```

Click **Run**. The task enters the queue and the daemon picks it up.

### 3. Watch real-time output

The task detail page streams agent output as it works — stdout in white, stderr in orange, with timestamps. Auto-scrolls to follow progress.

### 4. Review results

When the agent finishes, the task shows a completion status with exit code. Review the full output log and the changes the agent made.

## Task Lifecycle

```
pending → running → completed
                  → failed
                  → cancelled
                  → timeout
```

1. **pending** — Task created, waiting for daemon to claim it
2. **running** — Daemon executing the agent CLI
3. **completed** — Agent finished successfully (exit code 0)
4. **failed** — Agent exited with non-zero code
5. **cancelled** — User cancelled the task
6. **timeout** — Execution exceeded time limit

## Project Structure

```
AgenticFlow/
├── server/                    # Go backend
│   ├── cmd/
│   │   ├── af/               # CLI binary (daemon, token management)
│   │   └── server/           # HTTP server binary
│   ├── internal/
│   │   ├── auth/             # Token management, PAT cache
│   │   ├── cli/              # CLI config loading
│   │   ├── daemon/           # Daemon runtime, task execution
│   │   ├── handler/          # HTTP route handlers
│   │   ├── middleware/       # Auth middleware
│   │   ├── realtime/         # WebSocket hub
│   │   └── service/          # Business logic layer
│   ├── migrations/           # SQL migrations (golang-migrate)
│   ├── pkg/db/generated/     # sqlc generated code
│   ├── queries/              # SQL query definitions for sqlc
│   └── go.mod
├── web/                      # Vite + React SPA
│   ├── src/
│   │   ├── components/       # Reusable UI components
│   │   ├── pages/            # Route pages
│   │   ├── hooks/            # React Query hooks
│   │   └── lib/              # API client, WebSocket, utilities
│   └── package.json
├── Makefile                  # Build, test, dev commands
├── Dockerfile                # Multi-stage production build
├── docker-compose.yml        # Full stack deployment
└── .env.example              # Environment template
```

## Development

### Make targets

```bash
make help             # Show all available targets
make dev              # Start server in dev mode
make daemon           # Run daemon in foreground
make build            # Build server + CLI binaries
make test             # Run all Go tests
make test-race        # Run tests with race detector
make check            # Full verification (vet + build + test)
make migrate-up       # Apply database migrations
make migrate-down     # Roll back migrations
make sqlc-generate    # Regenerate type-safe query code
make clean            # Remove build artifacts
```

### Web development

```bash
cd web
npm run dev           # Start Vite dev server
npm run build         # Production build
npm run test          # Run Vitest tests
```

### Tech Stack

| Layer | Technology |
|-------|-----------|
| HTTP Router | [chi/v5](https://github.com/go-chi/chi) |
| Database | PostgreSQL 16 + [pgx/v5](https://github.com/jackc/pgx) |
| Query Generation | [sqlc](https://sqlc.dev) |
| Migrations | [golang-migrate](https://github.com/golang-migrate/migrate) |
| WebSocket | [gorilla/websocket](https://github.com/gorilla/websocket) |
| CLI Framework | [cobra](https://github.com/spf13/cobra) |
| Logging | `log/slog` (structured) |
| Frontend | React 19 + TypeScript + Vite |
| Styling | Tailwind CSS 4 |
| Server State | [@tanstack/react-query](https://tanstack.com/query) |
| Routing | react-router-dom v7 |
| Testing | Vitest + fast-check (property-based) |

## API Overview

### Authentication

- `POST /api/auth/register` — Create account
- `POST /api/auth/login` — Email/password login
- `POST /api/auth/github` — GitHub OAuth callback
- `POST /api/auth/google` — Google OAuth callback

### Agents

- `GET /api/agents` — List user's agents
- `POST /api/agents` — Create agent
- `GET /api/agents/:id` — Get agent details
- `PUT /api/agents/:id` — Update agent
- `DELETE /api/agents/:id` — Delete agent

### Tasks

- `POST /api/tasks` — Create and queue a task
- `GET /api/tasks` — List tasks (paginated, filterable)
- `GET /api/tasks/:id` — Get task details
- `GET /api/tasks/:id/messages` — Get task output messages
- `POST /api/tasks/:id/cancel` — Cancel a running task

### Daemon (internal)

- `POST /api/daemon/register` — Register daemon + runtimes
- `GET /api/daemon/tasks/poll` — Poll for pending tasks
- `POST /api/daemon/tasks/:id/start` — Report task started
- `POST /api/daemon/tasks/:id/messages` — Stream output chunks
- `POST /api/daemon/tasks/:id/complete` — Report task finished

### WebSocket

- `GET /ws?token=<jwt>` — Real-time event stream

Events: `task_created`, `task_output`, `task_completed`, `task_failed`, `daemon_connected`, `daemon_disconnected`

## Database Schema

Core tables:

- **user** — Accounts (email/password or OAuth)
- **personal_access_token** — PATs for daemon authentication
- **daemon** — Registered daemon instances per user
- **agent_runtime** — Detected AI CLIs per daemon
- **agent** — Configured agents with instructions, env, model
- **task** — Delegated tasks with status tracking
- **task_message** — Streaming output chunks (stdout/stderr)

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Run the verification pipeline: `make check`
5. Run web tests: `cd web && npm run test`
6. Submit a pull request

## License

MIT
