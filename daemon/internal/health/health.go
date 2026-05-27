// Package health provides a lightweight HTTP health endpoint for the daemon.
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// Status represents the daemon's current health status.
type Status struct {
	Status    string `json:"status"`
	Uptime    string `json:"uptime"`
	Version   string `json:"version"`
	DaemonID  string `json:"daemon_id,omitempty"`
	Connected bool   `json:"connected"`
}

// Server is a lightweight HTTP server that exposes a health endpoint.
type Server struct {
	port      int
	version   string
	daemonID  string
	startTime time.Time
	server    *http.Server

	mu        sync.RWMutex
	connected bool
}

// NewServer creates a new health server on the specified port.
func NewServer(port int, version, daemonID string) *Server {
	return &Server{
		port:      port,
		version:   version,
		daemonID:  daemonID,
		startTime: time.Now(),
	}
}

// SetConnected updates the server connection status.
func (s *Server) SetConnected(connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connected = connected
}

// Start begins serving the health endpoint. It blocks until the context
// is cancelled or an error occurs.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)

	s.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext:       func(_ net.Listener) context.Context { return ctx },
	}

	slog.Info("health server starting", "port", s.port)

	errCh := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// handleHealth responds with the daemon's current health status.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	connected := s.connected
	s.mu.RUnlock()

	status := Status{
		Status:    "ok",
		Uptime:    time.Since(s.startTime).Truncate(time.Second).String(),
		Version:   s.version,
		DaemonID:  s.daemonID,
		Connected: connected,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}
