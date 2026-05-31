package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
	"github.com/agenticflow/agenticflow/server/internal/migrate"
	"github.com/agenticflow/agenticflow/server/internal/realtime"
	"github.com/agenticflow/agenticflow/server/internal/seed"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	// Initialize structured logging.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("starting agenticflow server", "version", version, "commit", commit)

	// Load configuration from environment variables.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://agenticflow:agenticflow@localhost:5432/agenticflow?sslmode=disable"
	}

	// Connect to PostgreSQL.
	ctx := context.Background()
	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		slog.Error("unable to parse database URL", "error", err)
		os.Exit(1)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		slog.Error("unable to create connection pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("unable to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to database")

	// Run database migrations before accepting traffic.
	if err := migrate.Run(dbURL, "migrations/"); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	// Seed built-in skill templates after migrations, before accepting traffic.
	queries := db.New(pool)
	seed.SeedTemplates(ctx, queries)

	// Create WebSocket hub.
	hub := realtime.NewHub()

	// Background context for all background goroutines.
	// Cancelling this signals all background work to stop.
	bgCtx, bgCancel := context.WithCancel(context.Background())

	// WaitGroup tracks all background goroutines for graceful shutdown.
	var wg sync.WaitGroup

	// Start hub run loop, tracked by WaitGroup.
	wg.Add(1)
	go func() {
		defer wg.Done()
		hub.Run(bgCtx)
	}()

	// Create bounded worker pool for async token last_used_at updates (10 workers).
	tokenPool := middleware.NewTokenUpdatePool(10)

	// Build router. Pass bgCtx so AgentStatusService can use it for reconciliation goroutines.
	r := NewRouter(pool, hub, tokenPool, bgCtx, &wg)

	// Start HTTP server.
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		slog.Info("server listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown: wait for SIGINT or SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")

	// Stop accepting new connections and drain in-flight requests (30s timeout).
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	// Cancel background context to signal all background goroutines to stop.
	bgCancel()

	// Drain the token update worker pool within the grace period.
	tokenPool.Shutdown(25 * time.Second)

	// Wait for all background goroutines (hub, reconciliation) with a 30s timeout.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("all background goroutines stopped")
	case <-time.After(30 * time.Second):
		slog.Warn("forced shutdown: background goroutines did not stop within 30s grace period",
			"stuck_goroutines", "hub_run_loop,reconciliation_goroutines")
		os.Exit(1)
	}

	slog.Info("server stopped")
}
