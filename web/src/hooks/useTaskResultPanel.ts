import { useState, useEffect, useCallback, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useTask, useTaskMessages, type Task, type TaskMessage } from "./useTasks";
import { useTaskStream } from "./useTaskStream";
import { wsClient, type ConnectionStatus } from "../lib/ws";
import {
  loadTaskResultPanelState,
  saveTaskResultPanelState,
} from "../lib/taskResultSession";
import { extractDashboardResult } from "../lib/taskResultUtils";

/* ─── Types ─── */

export interface UseTaskResultPanelReturn {
  /** The currently tracked task ID (null if no task or dismissed) */
  panelTaskId: string | null;
  /** Whether the panel should be visible */
  isVisible: boolean;
  /** Set a new task to display (replaces previous) */
  setTask: (taskId: string) => void;
  /** Dismiss the panel */
  dismiss: () => void;
  /** Task data from React Query */
  task: Task | undefined;
  /** Streaming messages */
  messages: TaskMessage[];
  /** Whether WebSocket is connected */
  wsConnected: boolean;
  /** Extracted final result content */
  finalContent: string | null;
}

/* ─── Constants ─── */

const TERMINAL_STATUSES: Set<Task["status"]> = new Set([
  "completed",
  "failed",
  "cancelled",
  "timeout",
]);

const POLL_INTERVAL_MS = 3000;

/* ─── Hook ─── */

/**
 * Manages the Task Result Panel lifecycle on the Dashboard.
 *
 * Responsibilities:
 * - Tracks which task is displayed and whether the panel is dismissed
 * - Persists state to sessionStorage for cross-navigation restoration
 * - Subscribes to WebSocket streaming via useTaskStream
 * - Seeds historical messages from the API on mount
 * - Tracks WebSocket connection status
 * - Falls back to polling when WS is disconnected and task is non-terminal
 * - Computes finalContent when task reaches completed status
 */
export function useTaskResultPanel(): UseTaskResultPanelReturn {
  const queryClient = useQueryClient();

  // ─── Core state ───
  const [panelTaskId, setPanelTaskId] = useState<string | null>(null);
  const [dismissed, setDismissed] = useState(false);
  const [wsConnected, setWsConnected] = useState<boolean>(
    wsClient.status === "connected"
  );

  // Track whether we've seeded historical messages for the current task
  const seededRef = useRef(false);

  // ─── Restore state from sessionStorage on mount ───
  useEffect(() => {
    const saved = loadTaskResultPanelState();
    if (saved) {
      setPanelTaskId(saved.taskId);
      setDismissed(saved.dismissed);
    }
  }, []);

  // ─── Derived visibility ───
  const isVisible = panelTaskId !== null && !dismissed;

  // ─── Task data from React Query ───
  const { data: task } = useTask(panelTaskId ?? "");

  // ─── Streaming messages via WebSocket ───
  const { messages, seedMessages } = useTaskStream(panelTaskId ?? "");

  // ─── Historical messages from API ───
  const { data: historicalMessages } = useTaskMessages(panelTaskId ?? "");

  // Seed historical messages once they load (only once per task)
  useEffect(() => {
    if (historicalMessages && historicalMessages.length > 0 && !seededRef.current) {
      seedMessages(historicalMessages);
      seededRef.current = true;
    }
  }, [historicalMessages, seedMessages]);

  // Reset seeded flag when task changes
  useEffect(() => {
    seededRef.current = false;
  }, [panelTaskId]);

  // ─── WebSocket connection status tracking ───
  useEffect(() => {
    const unsubscribe = wsClient.onStatusChange((status: ConnectionStatus) => {
      setWsConnected(status === "connected");
    });
    return unsubscribe;
  }, []);

  // ─── Polling fallback ───
  useEffect(() => {
    const taskStatus = task?.status;
    const isTerminal = taskStatus ? TERMINAL_STATUSES.has(taskStatus) : false;

    if (!wsConnected && panelTaskId && !isTerminal) {
      const interval = setInterval(() => {
        void queryClient.invalidateQueries({ queryKey: ["tasks", panelTaskId] });
        void queryClient.invalidateQueries({
          queryKey: ["tasks", panelTaskId, "messages"],
        });
      }, POLL_INTERVAL_MS);
      return () => clearInterval(interval);
    }
  }, [wsConnected, panelTaskId, task?.status, queryClient]);

  // ─── Compute final content ───
  const finalContent =
    task?.status === "completed"
      ? extractDashboardResult(messages, task.output_preview ?? null)
      : null;

  // ─── Actions ───

  const setTask = useCallback((taskId: string) => {
    setPanelTaskId(taskId);
    setDismissed(false);
    saveTaskResultPanelState({ taskId, dismissed: false });
  }, []);

  const dismiss = useCallback(() => {
    setDismissed(true);
    if (panelTaskId) {
      saveTaskResultPanelState({ taskId: panelTaskId, dismissed: true });
    }
  }, [panelTaskId]);

  return {
    panelTaskId,
    isVisible,
    setTask,
    dismiss,
    task,
    messages,
    wsConnected,
    finalContent,
  };
}
