package daemon

import (
	"encoding/json"

	"github.com/agenticflow/agenticflow/daemon/internal/ws"
	"github.com/agenticflow/agenticflow/shared/api"
)

// newWSClient creates a WebSocket client configured for push-based task
// assignment. It connects to the server's /ws endpoint using the PAT token
// via Sec-WebSocket-Protocol header and dispatches events to the daemon.
func newWSClient(serverURL, token, daemonID string, d *Daemon) *ws.Client {
	handler := func(event api.WebSocketEvent) {
		// Convert the shared api.WebSocketEvent to raw bytes and dispatch
		// through the daemon's existing HandleWSMessage handler.
		raw, err := json.Marshal(event)
		if err != nil {
			d.logger.Warn("failed to marshal WS event for dispatch", "error", err)
			return
		}
		d.HandleWSMessage(raw)
	}

	onState := func(connected bool) {
		d.SetWSConnected(connected)
	}

	return ws.NewClient(serverURL, token, daemonID, handler, onState)
}
