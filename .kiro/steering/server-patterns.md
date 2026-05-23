---
inclusion: fileMatch
fileMatchPattern: "**/cmd/server/**,**/handler/**,**/middleware/**,**/realtime/**"
---

# Server Implementation Patterns

Reference: `/Users/arpit.jindal/workspace/opensource/multica/server/cmd/server/router.go`

## Router Structure (from multica)

Follow the exact same route grouping pattern:

```go
func NewRouter(pool *pgxpool.Pool, hub *realtime.Hub) chi.Router {
    r := chi.NewRouter()
    
    // Global middleware (same order as multica)
    r.Use(chimw.RequestID)
    r.Use(middleware.RequestLogger)
    r.Use(chimw.Recoverer)
    r.Use(cors.Handler(corsOptions))
    
    // Health (public, no auth)
    r.Get("/health", healthHandler)
    
    // Auth (public, rate-limited)
    r.Post("/auth/login", h.Login)
    r.Post("/auth/register", h.Register)
    r.Get("/auth/callback/{provider}", h.OAuthCallback)
    
    // WebSocket (public, auth via query param or header)
    r.Get("/ws", h.WebSocket)
    
    // Daemon API (daemon token auth — separate from user auth)
    r.Route("/api/daemon", func(r chi.Router) {
        r.Use(middleware.DaemonAuth(queries))
        // ... daemon routes
    })
    
    // Protected API (user PAT auth)
    r.Group(func(r chi.Router) {
        r.Use(middleware.Auth(queries, patCache))
        // ... user routes
    })
    
    // Static files (Web UI)
    r.Handle("/*", http.FileServer(http.Dir("./web/dist")))
}
```

## Daemon API Routes

These routes are called by the daemon process, NOT by the web UI:

```
POST /api/daemon/register      — Register daemon + runtimes
POST /api/daemon/deregister    — Deregister on shutdown
POST /api/daemon/heartbeat     — Periodic heartbeat
GET  /api/daemon/tasks/poll    — Poll for assigned tasks
POST /api/daemon/tasks/{id}/start    — Mark task as running
POST /api/daemon/tasks/{id}/complete — Report success
POST /api/daemon/tasks/{id}/fail     — Report failure
POST /api/daemon/tasks/{id}/messages — Stream output messages
```

## User API Routes

These routes are called by the web UI:

```
GET    /api/me                    — Current user info
GET    /api/daemons               — List connected daemons
GET    /api/agents                — List agent runtimes
POST   /api/tasks                 — Create/delegate a task
GET    /api/tasks                 — List tasks (with pagination)
GET    /api/tasks/{id}            — Get task detail
GET    /api/tasks/{id}/messages   — Get task output messages
POST   /api/tasks/{id}/cancel     — Cancel a running task
POST   /api/custom-agents         — Create custom agent
GET    /api/custom-agents         — List custom agents
PUT    /api/custom-agents/{id}    — Update custom agent
DELETE /api/custom-agents/{id}    — Delete custom agent
```

## Handler Pattern (from multica)

Every handler follows this pattern:

```go
func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request body
    var req CreateTaskRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    
    // 2. Validate
    if req.Prompt == "" {
        writeError(w, http.StatusBadRequest, "prompt is required")
        return
    }
    
    // 3. Get user context
    userID := requestUserID(r)
    
    // 4. Execute business logic (DB query)
    task, err := h.Queries.CreateTask(r.Context(), db.CreateTaskParams{...})
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to create task")
        return
    }
    
    // 5. Broadcast real-time event
    h.Hub.Broadcast(realtime.Event{Type: "task_created", Payload: task})
    
    // 6. Return response
    writeJSON(w, http.StatusCreated, taskToResponse(task))
}
```

## WebSocket Events (from multica's realtime pattern)

Events broadcast to connected clients:

```go
// Server → Web UI
"task_created"        // New task enqueued
"task_started"        // Daemon started executing
"task_output"         // Streaming stdout/stderr chunk
"task_completed"      // Task finished successfully
"task_failed"         // Task failed
"task_cancelled"      // Task was cancelled
"daemon_connected"    // Daemon came online
"daemon_disconnected" // Daemon went offline
"agent_updated"       // Agent runtime status changed
```

## Error Response Format

Always use this format (same as multica):

```go
func writeError(w http.ResponseWriter, status int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{"error": message})
}
```

## Authentication Middleware

Two separate auth middlewares (same as multica):

1. **DaemonAuth** — Validates daemon PAT tokens (prefix `af_`)
2. **Auth** — Validates user PAT tokens (also prefix `af_`)

Both use the same token format but different validation paths. The daemon token is scoped to a specific user's resources.
