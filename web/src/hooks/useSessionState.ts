import { useEffect, useState } from "react";
import { useWSClient } from "../contexts/WebSocketContext";
import type { WSEvent } from "../lib/ws";

/**
 * Possible session states for a running task's interaction lifecycle.
 */
export type SessionState = "idle" | "producing_output" | "waiting_for_input";

interface InputRequestedPayload {
  task_id: string;
}

interface InputClearedPayload {
  task_id: string;
}

interface SessionStateChangedPayload {
  task_id: string;
  state: SessionState;
}

/**
 * Hook that tracks the session interaction state for a specific task
 * by listening to WebSocket events.
 *
 * Listens for:
 * - `input_requested` → sets state to "waiting_for_input"
 * - `input_cleared` → sets state to "producing_output"
 * - `session_state_changed` → sets state to the value from the event payload
 *
 * All events are filtered by the provided taskId.
 *
 * Validates: Requirements 7.5, 5.3
 */
export function useSessionState(taskId: string): SessionState {
  const wsClient = useWSClient();
  const [state, setState] = useState<SessionState>("idle");

  useEffect(() => {
    if (!taskId) return;

    // Reset state when taskId changes
    setState("idle");

    const unsub1 = wsClient.on("input_requested", (event: WSEvent) => {
      const payload = event.payload as InputRequestedPayload;
      if (payload.task_id === taskId) {
        setState("waiting_for_input");
      }
    });

    const unsub2 = wsClient.on("input_cleared", (event: WSEvent) => {
      const payload = event.payload as InputClearedPayload;
      if (payload.task_id === taskId) {
        setState("producing_output");
      }
    });

    const unsub3 = wsClient.on("session_state_changed", (event: WSEvent) => {
      const payload = event.payload as SessionStateChangedPayload;
      if (payload.task_id === taskId) {
        setState(payload.state);
      }
    });

    return () => {
      unsub1();
      unsub2();
      unsub3();
    };
  }, [taskId, wsClient]);

  return state;
}
