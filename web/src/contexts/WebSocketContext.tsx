import { createContext, useContext, useEffect, useRef, type ReactNode } from "react";
import { WebSocketClient } from "../lib/ws";

const WebSocketContext = createContext<WebSocketClient | null>(null);

/**
 * Provides a single WebSocketClient instance to the component tree.
 * The instance is created once via useRef and destroyed on unmount,
 * ensuring proper cleanup during hot-reloads and app teardown.
 */
export function WebSocketProvider({ children }: { children: ReactNode }) {
  const clientRef = useRef<WebSocketClient | null>(null);
  if (!clientRef.current) {
    clientRef.current = new WebSocketClient();
  }

  useEffect(() => {
    return () => {
      clientRef.current?.destroy();
      clientRef.current = null;
    };
  }, []);

  return (
    <WebSocketContext.Provider value={clientRef.current}>
      {children}
    </WebSocketContext.Provider>
  );
}

/**
 * Returns the WebSocketClient instance from context.
 * Must be used within a <WebSocketProvider>.
 */
export function useWSClient(): WebSocketClient {
  const client = useContext(WebSocketContext);
  if (!client) {
    throw new Error("useWSClient must be used within a WebSocketProvider");
  }
  return client;
}
