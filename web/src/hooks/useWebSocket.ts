import { useEffect, useState } from "react";
import { wsClient, type ConnectionStatus } from "../lib/ws";
import { hasToken } from "../lib/api";

/**
 * Hook that manages the WebSocket connection lifecycle.
 * - Connects on mount when authenticated
 * - Disconnects on unmount
 * - Returns the current connection status
 * - Auto-reconnects at 5s intervals (handled by wsClient internally)
 */
export function useWebSocket() {
  const [status, setStatus] = useState<ConnectionStatus>(wsClient.status);

  useEffect(() => {
    // Only connect if the user is authenticated
    if (!hasToken()) {
      return;
    }

    // Subscribe to status changes
    const unsubscribe = wsClient.onStatusChange((newStatus) => {
      setStatus(newStatus);
    });

    // Connect if not already connected
    if (wsClient.status === "disconnected") {
      wsClient.connect();
    }

    // Sync initial status
    setStatus(wsClient.status);

    return () => {
      unsubscribe();
      wsClient.disconnect();
    };
  }, []);

  return { status };
}
