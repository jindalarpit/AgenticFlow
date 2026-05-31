package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agenticflow/agenticflow/server/internal/crypto"
	"github.com/agenticflow/agenticflow/server/internal/handler"
	"github.com/agenticflow/agenticflow/server/internal/middleware"
	"github.com/agenticflow/agenticflow/server/internal/provider"
	"github.com/agenticflow/agenticflow/server/internal/realtime"
	"github.com/agenticflow/agenticflow/server/internal/service"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
	"github.com/agenticflow/agenticflow/shared/httputil"
	"github.com/agenticflow/agenticflow/shared/pgutil"
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
func NewRouter(pool *pgxpool.Pool, hub *realtime.Hub, tokenPool *middleware.TokenUpdatePool, bgCtx context.Context, wg *sync.WaitGroup) chi.Router {
	queries := db.New(pool)
	patCache := middleware.NewPATCache()

	r := chi.NewRouter()

	// Global middleware.
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.BodyLimit(1 << 20))          // 1 MB default body limit
	r.Use(middleware.BodyLimitErrorHandler)        // Return JSON 413 on oversized bodies
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "X-Daemon-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
	agentStatusSvc := service.NewAgentStatusService(queries, hub, bgCtx)
	// Register the AgentStatusService's WaitGroup with the parent WaitGroup
	// so shutdown waits for all reconciliation goroutines to complete.
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Block until bgCtx is cancelled, then wait for reconciliation goroutines.
		<-bgCtx.Done()
		agentStatusSvc.Wait()
	}()
	sessionStateMgr := service.NewSessionStateManager(hub)
	daemonH := handler.NewDaemonHandler(queries, hub)
	daemonH.AgentStatusService = agentStatusSvc
	daemonH.SessionStateManager = sessionStateMgr
	r.Route("/api/daemon", func(r chi.Router) {
		r.Use(middleware.DaemonAuth(queries, patCache, tokenPool))

		r.Post("/register", daemonH.Register)
		r.Post("/deregister", daemonH.Deregister)
		r.Post("/heartbeat", daemonH.Heartbeat)
		r.Get("/tasks/poll", daemonH.PollTasks)
		r.Post("/tasks/{taskId}/start", daemonH.StartTask)
		r.Post("/tasks/{taskId}/complete", daemonH.CompleteTask)
		r.Post("/tasks/{taskId}/fail", daemonH.FailTask)
		r.With(middleware.BodyLimit(64 << 10)).Post("/tasks/{taskId}/messages", daemonH.ReportTaskMessages) // 64 KB limit
		r.Post("/tasks/{taskId}/input-state", daemonH.ReportInputState)
		r.Post("/tasks/{taskId}/stages/{stageName}/complete", daemonH.CompleteStage)
	})

	// Initialize online AI provider components.
	// Credential encryptor requires AGENTICFLOW_ENCRYPTION_KEY (64 hex chars).
	// If not set, online providers won't work but the server still starts.
	var encryptor *crypto.CredentialEncryptor
	encryptionKey := os.Getenv("AGENTICFLOW_ENCRYPTION_KEY")
	if encryptionKey == "" {
		slog.Warn("AGENTICFLOW_ENCRYPTION_KEY not set: online AI providers will be unavailable")
	} else {
		var err error
		encryptor, err = crypto.NewCredentialEncryptor(encryptionKey)
		if err != nil {
			slog.Warn("failed to initialize credential encryptor: online AI providers will be unavailable", "error", err)
		}
	}

	registry := provider.NewRegistry()
	providerService := service.NewProviderService(queries, hub, encryptor, registry)
	onlineEngine := service.NewOnlineExecutionEngine(queries, hub, encryptor, registry)

	// Create TaskService and wire the online execution engine.
	taskService := service.NewTaskService(queries, hub)
	taskService.SetOnlineExecutionEngine(onlineEngine)

	providerHandler := handler.NewProviderHandler(providerService)
	deliverableTypeHandler := handler.NewDeliverableTypeHandler(queries)

	// Protected API routes (require user PAT auth).
	userH := handler.NewUserHandler(queries, hub)
	userH.AgentStatusService = agentStatusSvc
	patH := handler.NewPATHandler(queries, patCache)
	agentH := handler.NewAgentHandler(queries, hub)
	runtimeH := handler.NewRuntimeHandler(queries)
	skillH := handler.NewSkillHandler(queries)
	templateH := handler.NewSkillTemplateHandler(queries)
	agentSkillH := handler.NewAgentSkillHandler(queries, pool)
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(queries, patCache, tokenPool))

		// User.
		r.Get("/api/me", userH.GetMe)

		// Daemons.
		r.Get("/api/daemons", userH.ListDaemons)

		// Agent Runtimes (legacy endpoint).
		r.Get("/api/agent-runtimes", userH.ListAgentRuntimes)

		// Runtimes.
		r.Get("/api/runtimes/{id}/models", runtimeH.ListRuntimeModels)

		// Agents (full CRUD).
		r.Post("/api/agents", agentH.CreateAgent)
		r.Get("/api/agents", agentH.ListAgents)
		r.Get("/api/agents/activity", agentH.GetAgentsActivity)
		r.Get("/api/agents/run-counts", agentH.GetAgentsRunCounts)
		r.Get("/api/agents/{id}", agentH.GetAgent)
		r.Get("/api/agents/{id}/stats", agentH.GetAgentStats)
		r.Put("/api/agents/{id}", agentH.UpdateAgent)
		r.Delete("/api/agents/{id}", agentH.DeleteAgent)
		r.Post("/api/agents/{id}/archive", agentH.ArchiveAgent)
		r.Post("/api/agents/{id}/restore", agentH.RestoreAgent)

		// Tasks.
		r.With(middleware.BodyLimit(32 << 10)).Post("/api/tasks", userH.CreateTask) // 32 KB limit
		r.Get("/api/tasks", userH.ListTasks)
		r.Get("/api/tasks/{taskId}", userH.GetTask)
		r.Get("/api/tasks/{taskId}/messages", userH.ListTaskMessages)
		r.Get("/api/tasks/{taskId}/stages", userH.ListStages)
		r.Post("/api/tasks/{taskId}/cancel", userH.CancelTask)
		r.Post("/api/tasks/{id}/input", userH.SendTaskInput)
		r.Post("/api/tasks/{taskId}/stages/{stageName}/approve", userH.ApproveStage)
		r.Post("/api/tasks/{taskId}/stages/{stageName}/reject", userH.RejectStage)
		r.Post("/api/tasks/{taskId}/stages/{stageName}/follow-up", userH.FollowUpStage)
		r.Get("/api/tasks/{taskId}/stages/{stageName}/history", userH.GetStageHistory)

		// Custom Agents.
		r.Post("/api/custom-agents", userH.CreateCustomAgent)
		r.Get("/api/custom-agents", userH.ListCustomAgents)
		r.Put("/api/custom-agents/{id}", userH.UpdateCustomAgent)
		r.Delete("/api/custom-agents/{id}", userH.DeleteCustomAgent)

		// Skill Templates (registered before /api/skills to avoid route conflicts).
		r.Get("/api/skill-templates", templateH.List)
		r.Get("/api/skill-templates/{slug}", templateH.GetBySlug)
		r.Post("/api/skill-templates/{slug}/instantiate", templateH.Instantiate)

		// Skills.
		r.Post("/api/skills", skillH.Create)
		r.Get("/api/skills", skillH.List)
		r.Post("/api/skills/import", skillH.Import)
		r.Get("/api/skills/{id}", skillH.Get)
		r.Put("/api/skills/{id}", skillH.Update)
		r.Delete("/api/skills/{id}", skillH.Delete)

		// Agent-Skill Associations.
		r.Put("/api/agents/{id}/skills", agentSkillH.SetSkills)
		r.Get("/api/agents/{id}/skills", agentSkillH.GetSkills)

		// Online AI Providers.
		providerHandler.RegisterRoutes(r)

		// Deliverable Types.
		deliverableTypeHandler.RegisterRoutes(r)

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
		httputil.WriteJSON(w, http.StatusNotImplemented, map[string]string{
			"status":  "not implemented",
			"handler": name,
		})
	}
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

	// Check token expiry: reject if expires_at is set and in the past.
	if pat.ExpiresAt.Valid && time.Now().After(pat.ExpiresAt.Time) {
		return "", false, "", false
	}

	uid := pgutil.UUIDToString(pat.UserID)

	// Cache with TTL that respects token expiry.
	var expiresAt time.Time
	if pat.ExpiresAt.Valid {
		expiresAt = pat.ExpiresAt.Time
	}
	v.cache.Set(hash, uid, middleware.TTLForExpiry(time.Now(), expiresAt))

	return uid, false, "", true
}

// hashToken returns the hex-encoded SHA-256 hash of a token.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
