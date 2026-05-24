package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// DefaultHealthPort is the default port for the daemon health endpoint.
const DefaultHealthPort = 19514

// HealthResponse is returned by the daemon's local health endpoint.
type HealthResponse struct {
	Status          string      `json:"status"`
	PID             int         `json:"pid"`
	Uptime          string      `json:"uptime"`
	DaemonID        string      `json:"daemon_id"`
	DeviceName      string      `json:"device_name"`
	ServerURL       string      `json:"server_url"`
	CLIVersion      string      `json:"cli_version"`
	ActiveTaskCount int64       `json:"active_task_count"`
	Agents          []AgentInfo `json:"agents"`
}

// AgentInfo describes a detected agent runtime in the health response.
type AgentInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

// HealthServerConfig holds configuration for the health HTTP server.
type HealthServerConfig struct {
	// Port is the TCP port to bind to (default: 19514).
	Port int
	// ShutdownCallback is called when POST /shutdown is received.
	// It should initiate graceful daemon shutdown.
	ShutdownCallback func()
}

// DaemonStateProvider is an interface for retrieving daemon state
// needed by the health endpoint. This decouples the health server
// from the Daemon struct, making it testable independently.
type DaemonStateProvider interface {
	// GetDaemonID returns the daemon's unique identifier.
	GetDaemonID() string
	// GetDeviceName returns the machine's device name.
	GetDeviceName() string
	// GetServerURL returns the configured server URL.
	GetServerURL() string
	// GetCLIVersion returns the CLI version string.
	GetCLIVersion() string
	// GetActiveTaskCount returns the number of currently executing tasks.
	GetActiveTaskCount() int64
	// GetAgents returns the list of detected agent runtimes.
	GetAgents() []AgentInfo
	// GetStartTime returns when the daemon started.
	GetStartTime() time.Time
}

// HealthServer serves the daemon's local health HTTP endpoint.
// It provides GET /health for status checks and POST /shutdown
// for graceful shutdown initiation. The server binds to localhost
// only (127.0.0.1) to prevent external access.
type HealthServer struct {
	cfg      HealthServerConfig
	state    DaemonStateProvider
	srv      *http.Server
	listener net.Listener
	done     chan struct{}
}

// NewHealthServer creates a new HealthServer with the given configuration
// and state provider. Call Start() to begin serving.
// If Port is negative, it defaults to DefaultHealthPort.
// If Port is 0, the OS assigns an ephemeral port (useful for tests).
func NewHealthServer(cfg HealthServerConfig, state DaemonStateProvider) *HealthServer {
	if cfg.Port < 0 {
		cfg.Port = DefaultHealthPort
	}
	return &HealthServer{
		cfg:   cfg,
		state: state,
		done:  make(chan struct{}),
	}
}

// Start binds the health server to 127.0.0.1:<port> and begins serving.
// Returns an error if the port is already in use (indicating another daemon
// is running). Start is non-blocking; the server runs in a background goroutine.
func (h *HealthServer) Start() error {
	addr := fmt.Sprintf("127.0.0.1:%d", h.cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("another daemon is already running on %s: %w", addr, err)
	}
	h.listener = ln

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/shutdown", h.handleShutdown)

	h.srv = &http.Server{Handler: mux}

	go func() {
		defer close(h.done)
		if err := h.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			// Log but don't crash — health server is non-critical.
			_ = err
		}
	}()

	return nil
}

// Stop gracefully shuts down the health server with a 5-second timeout.
func (h *HealthServer) Stop() error {
	if h.srv == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := h.srv.Shutdown(ctx)

	// Wait for the serve goroutine to exit.
	<-h.done

	return err
}

// Addr returns the listener's address, useful for tests that use port 0.
// Returns empty string if the server hasn't started.
func (h *HealthServer) Addr() string {
	if h.listener == nil {
		return ""
	}
	return h.listener.Addr().String()
}

// handleHealth responds to GET /health with the current daemon state as JSON.
func (h *HealthServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(h.state.GetStartTime()).Truncate(time.Second).String()

	resp := HealthResponse{
		Status:          "running",
		PID:             os.Getpid(),
		Uptime:          uptime,
		DaemonID:        h.state.GetDaemonID(),
		DeviceName:      h.state.GetDeviceName(),
		ServerURL:       h.state.GetServerURL(),
		CLIVersion:      h.state.GetCLIVersion(),
		ActiveTaskCount: h.state.GetActiveTaskCount(),
		Agents:          h.state.GetAgents(),
	}

	// Ensure agents is never null in JSON output.
	if resp.Agents == nil {
		resp.Agents = []AgentInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleShutdown responds to POST /shutdown by triggering the graceful
// shutdown callback and returning a confirmation response.
func (h *HealthServer) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "shutting down"})

	// Trigger shutdown asynchronously so the response flushes first.
	if h.cfg.ShutdownCallback != nil {
		go h.cfg.ShutdownCallback()
	}
}
