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
	Port             int
	ShutdownCallback func()
}

// DaemonStateProvider is an interface for retrieving daemon state
// needed by the health endpoint.
type DaemonStateProvider interface {
	GetDaemonID() string
	GetDeviceName() string
	GetServerURL() string
	GetCLIVersion() string
	GetActiveTaskCount() int64
	GetAgents() []AgentInfo
	GetStartTime() time.Time
}

// HealthServer serves the daemon's local health HTTP endpoint.
type HealthServer struct {
	cfg      HealthServerConfig
	state    DaemonStateProvider
	srv      *http.Server
	listener net.Listener
	done     chan struct{}
}

// NewHealthServer creates a new HealthServer with the given configuration.
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
	<-h.done
	return err
}

// Addr returns the listener's address.
func (h *HealthServer) Addr() string {
	if h.listener == nil {
		return ""
	}
	return h.listener.Addr().String()
}

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

	if resp.Agents == nil {
		resp.Agents = []AgentInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *HealthServer) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "shutting down"})

	if h.cfg.ShutdownCallback != nil {
		go h.cfg.ShutdownCallback()
	}
}
