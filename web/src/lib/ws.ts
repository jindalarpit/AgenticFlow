export type WSEventHandler = (event: WSEvent) => void;

export interface WSEvent {
  type: string;
  payload: unknown;
}

export type ConnectionStatus = "connected" | "disconnected" | "connecting";

type StatusListener = (status: ConnectionStatus) => void;

const RECONNECT_INTERVAL = 5000; // 5 seconds

/**
 * WebSocket client with auto-reconnect at 5-second intervals.
 * Manages a single WebSocket connection to the server.
 */
class WebSocketClient {
  private ws: WebSocket | null = null;
  private url: string = "";
  private handlers: Map<string, Set<WSEventHandler>> = new Map();
  private statusListeners: Set<StatusListener> = new Set();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private intentionalClose = false;
  private _status: ConnectionStatus = "disconnected";

  get status(): ConnectionStatus {
    return this._status;
  }

  private setStatus(status: ConnectionStatus): void {
    this._status = status;
    this.statusListeners.forEach((listener) => listener(status));
  }

  /**
   * Connect to the WebSocket server.
   * Uses the stored PAT token for authentication via query parameter.
   */
  connect(): void {
    const token = localStorage.getItem("af_token");
    if (!token) {
      return;
    }

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    this.url = `${protocol}//${window.location.host}/ws?token=${encodeURIComponent(token)}`;

    this.intentionalClose = false;
    this.doConnect();
  }

  private doConnect(): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      return;
    }

    this.setStatus("connecting");

    try {
      this.ws = new WebSocket(this.url);

      this.ws.onopen = () => {
        this.setStatus("connected");
        this.clearReconnectTimer();
      };

      this.ws.onmessage = (event) => {
        try {
          const wsEvent = JSON.parse(event.data) as WSEvent;
          this.dispatch(wsEvent);
        } catch {
          // Ignore malformed messages
        }
      };

      this.ws.onclose = () => {
        this.setStatus("disconnected");
        if (!this.intentionalClose) {
          this.scheduleReconnect();
        }
      };

      this.ws.onerror = () => {
        // onclose will fire after onerror, triggering reconnect
      };
    } catch {
      this.setStatus("disconnected");
      this.scheduleReconnect();
    }
  }

  /**
   * Disconnect from the WebSocket server.
   */
  disconnect(): void {
    this.intentionalClose = true;
    this.clearReconnectTimer();
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.setStatus("disconnected");
  }

  /**
   * Subscribe to a specific event type.
   * Returns an unsubscribe function.
   */
  on(eventType: string, handler: WSEventHandler): () => void {
    if (!this.handlers.has(eventType)) {
      this.handlers.set(eventType, new Set());
    }
    this.handlers.get(eventType)!.add(handler);

    return () => {
      this.handlers.get(eventType)?.delete(handler);
    };
  }

  /**
   * Subscribe to connection status changes.
   * Returns an unsubscribe function.
   */
  onStatusChange(listener: StatusListener): () => void {
    this.statusListeners.add(listener);
    return () => {
      this.statusListeners.delete(listener);
    };
  }

  private dispatch(event: WSEvent): void {
    // Dispatch to specific type handlers
    const handlers = this.handlers.get(event.type);
    if (handlers) {
      handlers.forEach((handler) => handler(event));
    }

    // Dispatch to wildcard handlers
    const wildcardHandlers = this.handlers.get("*");
    if (wildcardHandlers) {
      wildcardHandlers.forEach((handler) => handler(event));
    }
  }

  private scheduleReconnect(): void {
    this.clearReconnectTimer();
    this.reconnectTimer = setTimeout(() => {
      this.doConnect();
    }, RECONNECT_INTERVAL);
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }
}

// Singleton instance
export const wsClient = new WebSocketClient();
