import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useWSClient } from "../contexts/WebSocketContext";
import type { WSEvent } from "../lib/ws";
import type { AgentListItem } from "./useAgentList";

/**
 * Hook that subscribes to WebSocket events relevant to the Agent List Page
 * and performs surgical React Query cache patches for real-time updates.
 *
 * Handles:
 * - agent_status_changed: patches the affected agent's status in cache
 * - agent_created: invalidates agents query (full refetch for complete object)
 * - agent_updated: merges updated fields into cached agent
 * - agent_deleted: removes agent from cache
 * - daemon_disconnected: invalidates both daemons and agents queries
 *
 * Falls back to invalidateQueries when payload is insufficient for a local patch.
 * Cleans up all subscriptions on unmount.
 */
export function useAgentListWebSocket(): void {
  const queryClient = useQueryClient();
  const wsClient = useWSClient();

  useEffect(() => {
    const handleStatusChanged = (event: WSEvent) => {
      const payload = event.payload as { agent_id: string; status: string } | null;
      if (!payload?.agent_id) {
        queryClient.invalidateQueries({ queryKey: ["agents"] });
        return;
      }

      queryClient.setQueryData<AgentListItem[]>(["agents"], (old) => {
        if (!old) return old;
        return old.map((agent) =>
          agent.id === payload.agent_id
            ? { ...agent, status: payload.status as AgentListItem["status"] }
            : agent
        );
      });
    };

    const handleCreated = (_event: WSEvent) => {
      // Full refetch since we need the complete agent object
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    };

    const handleUpdated = (event: WSEvent) => {
      const payload = event.payload as (Partial<AgentListItem> & { id: string }) | null;
      if (!payload?.id) {
        queryClient.invalidateQueries({ queryKey: ["agents"] });
        return;
      }

      queryClient.setQueryData<AgentListItem[]>(["agents"], (old) => {
        if (!old) return old;
        return old.map((agent) =>
          agent.id === payload.id ? { ...agent, ...payload } : agent
        );
      });
    };

    const handleDeleted = (event: WSEvent) => {
      const payload = event.payload as { agent_id: string } | null;
      if (!payload?.agent_id) {
        queryClient.invalidateQueries({ queryKey: ["agents"] });
        return;
      }

      queryClient.setQueryData<AgentListItem[]>(["agents"], (old) => {
        if (!old) return old;
        return old.filter((agent) => agent.id !== payload.agent_id);
      });
    };

    const handleDaemonDisconnected = (_event: WSEvent) => {
      queryClient.invalidateQueries({ queryKey: ["daemons"] });
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    };

    const unsubs = [
      wsClient.on("agent_status_changed", handleStatusChanged),
      wsClient.on("agent_created", handleCreated),
      wsClient.on("agent_updated", handleUpdated),
      wsClient.on("agent_deleted", handleDeleted),
      wsClient.on("daemon_disconnected", handleDaemonDisconnected),
    ];

    return () => {
      unsubs.forEach((fn) => fn());
    };
  }, [queryClient, wsClient]);
}
