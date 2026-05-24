package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agenticflow/agenticflow/internal/handler"
	"github.com/agenticflow/agenticflow/internal/middleware"
	"github.com/agenticflow/agenticflow/internal/realtime"
	"github.com/agenticflow/agenticflow/internal/service"
	db "github.com/agenticflow/agenticflow/pkg/db/generated"
)

// defaultOrigins are the allowed CORS origins for local development.
var defaultOrigins = []string{
	"http://localhost:3000",
	"http://localhost:5173",
	"http://localhost:5174",
}

// allowedOrigins returns the configured CORS origins from the environment,
// falling back to development defaults.
func allowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if raw == "" {
		return defaultOrigins
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return defaultOrigins
	}
	return origins
}

// NewRouter creates the fully-configured Chi router with all middleware and routes.
func NewRouter(pool *pgxpool.Pool, hub *realtime.Hub) chi.Router {
	queries := db.New(pool)
	patCache := middleware.NewPATCache()

	r := chi.NewRouter()

	// Global middleware.
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "X-Daemon-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSONResponse(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Auth routes (public).
	authHandler := handler.NewAuthHandler(queries)
	r.Post("/auth/login", authHandler.Login)
	r.Post("/auth/register", authHandler.Register)
	r.Get("/auth/callback/{provider}", authHandler.OAuthCallback)

	// WebSocket upgrade.
	wsValidator := &wsTokenValidator{queries: queries, cache: patCache}
	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		realtime.HandleWebSocket(hub, wsValidator, w, r)
	})

	// Daemon API routes (require daemon token auth).
	agentStatusSvc := service.NewAgentStatusService(queries, hub)
	sessionStateMgr := service.NewSessionStateManager(hub)
	daemonH := handler.NewDaemonHandler(queries, hub)
	daemonH.AgentStatusService = agentStatusSvc
	daemonH.SessionStateManager = sessionStateMgr
	r.Route("/api/daemon", func(r chi.Router) {
		r.Use(middleware.DaemonAuth(queries, patCache))

		r.Post("/register", daemonH.Register)
		r.Post("/deregister", daemonH.Deregister)
		r.Post("/heartbeat", daemonH.Heartbeat)
		r.Get("/tasks/poll", daemonH.PollTasks)
		r.Post("/tasks/{taskId}/start", daemonH.StartTask)
		r.Post("/tasks/{taskId}/complete", daemonH.CompleteTask)
		r.Post("/tasks/{taskId}/fail", daemonH.FailTask)
		r.Post("/tasks/{taskId}/messages", daemonH.ReportTaskMessages)
		r.Post("/tasks/{taskId}/input-state", daemonH.ReportInputState)
	})

	// Protected API routes (require user PAT auth).
	userH := handler.NewUserHandler(queries, hub)
	userH.AgentStatusService = agentStatusSvc
	patH := handler.NewPATHandler(queries, patCache)
	agentH := handler.NewAgentHandler(queries, hub)
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(queries, patCache))

		// User.
		r.Get("/api/me", userH.GetMe)

		// Daemons.
		r.Get("/api/daemons", userH.ListDaemons)

		// Agent Runtimes (legacy endpoint).
		r.Get("/api/agent-runtimes", userH.ListAgentRuntimes)

		// Agents (full CRUD).
		r.Post("/api/agents", agentH.CreateAgent)
		r.Get("/api/agents", agentH.ListAgents)
		r.Get("/api/agents/{id}", agentH.GetAgent)
		r.Get("/api/agents/{id}/stats", agentH.GetAgentStats)
		r.Put("/api/agents/{id}", agentH.UpdateAgent)
		r.Delete("/api/agents/{id}", agentH.DeleteAgent)

		// Tasks.
		r.Post("/api/tasks", userH.CreateTask)
		r.Get("/api/tasks", userH.ListTasks)
		r.Get("/api/tasks/{taskId}", userH.GetTask)
		r.Get("/api/tasks/{taskId}/messages", userH.ListTaskMessages)
		r.Post("/api/tasks/{taskId}/cancel", userH.CancelTask)
		r.Post("/api/tasks/{id}/input", userH.SendTaskInput)

		// Custom Agents.
		r.Post("/api/custom-agents", userH.CreateCustomAgent)
		r.Get("/api/custom-agents", userH.ListCustomAgents)
		r.Put("/api/custom-agents/{id}", userH.UpdateCustomAgent)
		r.Delete("/api/custom-agents/{id}", userH.DeleteCustomAgent)

		// Tokens (PAT management).
		r.Get("/api/tokens", patH.ListPersonalAccessTokens)
		r.Post("/api/tokens", patH.CreatePersonalAccessToken)
		r.Delete("/api/tokens/{id}", patH.RevokePersonalAccessToken)
	})

	// Static file serving for Web UI.
	// Serve files from ./web/dist if the directory exists.
	webDir := "./web/dist"
	if info, err := os.Stat(webDir); err == nil && info.IsDir() {
		fileServer := http.FileServer(http.Dir(webDir))
		r.Handle("/*", fileServer)
	}

	return r
}

// placeholderHandler returns an HTTP handler that responds with a JSON
// "not implemented" message. These will be replaced with real handlers
// in tasks 8.4, 8.6, and 8.8.
func placeholderHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSONResponse(w, http.StatusNotImplemented, map[string]string{
			"status":  "not implemented",
			"handler": name,
		})
	}
}

// writeJSONResponse writes a JSON response with the given status code.
func writeJSONResponse(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// wsTokenValidator implements realtime.TokenValidator using the PAT cache and DB.
type wsTokenValidator struct {
	queries *db.Queries
	cache   *middleware.PATCache
}

func (v *wsTokenValidator) ValidateToken(token string) (userID string, isDaemon bool, daemonID string, ok bool) {
	if token == "" || !strings.HasPrefix(token, "af_") {
		return "", false, "", false
	}

	hash := hashToken(token)

	// Check cache first.
	if uid, cached := v.cache.Get(hash); cached {
		return uid, false, "", true
	}

	// Look up in DB.
	ctx := context.Background()
	pat, err := v.queries.GetTokenByHash(ctx, hash)
	if err != nil {
		return "", false, "", false
	}

	uid := pgUUIDToString(pat.UserID)
	v.cache.Set(hash, uid, middleware.CacheTTL)
	return uid, false, "", true
}

// hashToken returns the hex-encoded SHA-256 hash of a token.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// pgUUIDToString converts a pgtype.UUID to string.
func pgUUIDToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", u.Bytes[0:4], u.Bytes[4:6], u.Bytes[6:8], u.Bytes[8:10], u.Bytes[10:16])
}
