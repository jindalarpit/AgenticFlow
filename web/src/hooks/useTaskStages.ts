import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api";
import { useWSClient } from "../contexts/WebSocketContext";
import type { WSEvent } from "../lib/ws";

export type StageStatus =
  | "pending"
  | "running"
  | "awaiting_approval"
  | "approved"
  | "rejected"
  | "completed"
  | "failed";

export interface TaskStage {
  name: string;
  order: number;
  status: StageStatus;
  output_content: string | null;
  feedback: string | null;
  started_at: string | null;
  completed_at: string | null;
}

/**
 * Fetch all stages for a task and subscribe to WebSocket stage events
 * for real-time updates. Returns null/empty when the task has no stages
 * (single-pass backward compat).
 *
 * GET /api/tasks/{taskId}/stages
 *
 * WebSocket events handled:
 * - stage_awaiting_approval
 * - stage_approved
 * - stage_rejected
 * - stage_started
 */
export function useTaskStages(taskId: string) {
  const queryClient = useQueryClient();
  const wsClient = useWSClient();

  const query = useQuery({
    queryKey: ["tasks", taskId, "stages"],
    queryFn: () => apiFetch<TaskStage[]>(`/api/tasks/${taskId}/stages`),
    enabled: !!taskId,
    refetchInterval: false, // WebSocket handles real-time updates
  });

  // Subscribe to stage-related WebSocket events and refetch on change
  useEffect(() => {
    if (!taskId) return;

    const stageEvents = [
      "stage_awaiting_approval",
      "stage_approved",
      "stage_rejected",
      "stage_started",
    ];

    const unsubscribers = stageEvents.map((eventType) =>
      wsClient.on(eventType, (event: WSEvent) => {
        const payload = event.payload as { task_id?: string };
        if (payload.task_id === taskId) {
          void queryClient.invalidateQueries({
            queryKey: ["tasks", taskId, "stages"],
          });
          // Also refresh the task itself (status may change on final approval)
          void queryClient.invalidateQueries({
            queryKey: ["tasks", taskId],
          });
        }
      })
    );

    return () => {
      unsubscribers.forEach((unsub) => unsub());
    };
  }, [taskId, queryClient, wsClient]);

  return query;
}
