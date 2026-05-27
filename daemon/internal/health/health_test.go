package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHealth(t *testing.T) {
	s := NewServer(8081, "1.0.0", "test-daemon-id")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusOK)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}

	var status Status
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if status.Status != "ok" {
		t.Errorf("status: got %q, want %q", status.Status, "ok")
	}
	if status.Version != "1.0.0" {
		t.Errorf("version: got %q, want %q", status.Version, "1.0.0")
	}
	if status.DaemonID != "test-daemon-id" {
		t.Errorf("daemon_id: got %q, want %q", status.DaemonID, "test-daemon-id")
	}
	if status.Connected {
		t.Error("connected should be false by default")
	}
}

func TestSetConnected(t *testing.T) {
	s := NewServer(8081, "1.0.0", "test-daemon-id")

	s.SetConnected(true)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	var status Status
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !status.Connected {
		t.Error("connected should be true after SetConnected(true)")
	}
}

func TestNewServer(t *testing.T) {
	s := NewServer(9090, "2.0.0", "my-daemon")

	if s.port != 9090 {
		t.Errorf("port: got %d, want %d", s.port, 9090)
	}
	if s.version != "2.0.0" {
		t.Errorf("version: got %q, want %q", s.version, "2.0.0")
	}
	if s.daemonID != "my-daemon" {
		t.Errorf("daemonID: got %q, want %q", s.daemonID, "my-daemon")
	}
}
