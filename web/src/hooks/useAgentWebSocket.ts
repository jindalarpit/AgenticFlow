import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useWSClient } from "../contexts/WebSocketContext";
import type { WSEvent } from "../lib/ws";
import type { TaskEvent } from "../lib/agent-detail-types";

/**
 * Hook that subscribes to WebSocket events relevant to a specific agent
 * and invalidates the appropriate React Query caches.
 *
 * Handles:
 * - task_created: invalidates agent-tasks, agents, and agent-stats queries
 * - task_completed / task_failed: same invalidations as task_created
 * - daemon_connected / daemon_disconnected: invalidates agents query
 *
 * On WebSocket reconnection, refetches agent data and active tasks to
 * reconcile any events missed during the disconnect window.
 */
export function useAgentWebSocket(agentId: string): void {
  const queryClient = useQueryClient();
  const wsClient = useWSClient();

  useEffect(() => {
    if (!agentId) return;

    const invalidateTaskQueries = () => {
      queryClient.invalidateQueries({ queryKey: ["agent-tasks", agentId] });
      queryClient.invalidateQueries({ queryKey: ["agents", agentId] });
      queryClient.invalidateQueries({ queryKey: ["agent-stats", agentId] });
    };

    const handleTaskEvent = (event: WSEvent) => {
      const payload = event.payload as TaskEvent;
      if (payload.agent_id === agentId) {
        invalidateTaskQueries();
      }
    };

    const handleDaemonEvent = (_event: WSEvent) => {
      // Daemon connect/disconnect may affect this agent's status
      queryClient.invalidateQueries({ queryKey: ["agents", agentId] });
    };

    const handleReconnect = (status: string) => {
      if (status === "connected") {
        // Refetch agent data and tasks to reconcile missed events
        queryClient.invalidateQueries({ queryKey: ["agents", agentId] });
        queryClient.invalidateQueries({ queryKey: ["agent-tasks", agentId] });
        queryClient.invalidateQueries({ queryKey: ["agent-stats", agentId] });
      }
    };

    const unsubs = [
      wsClient.on("task_created", handleTaskEvent),
      wsClient.on("task_completed", handleTaskEvent),
      wsClient.on("task_failed", handleTaskEvent),
      wsClient.on("daemon_connected", handleDaemonEvent),
      wsClient.on("daemon_disconnected", handleDaemonEvent),
      wsClient.onStatusChange(handleReconnect),
    ];

    return () => {
      unsubs.forEach((fn) => fn());
    };
  }, [agentId, queryClient, wsClient]);
}
