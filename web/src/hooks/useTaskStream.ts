import { useEffect, useRef, useState } from "react";
import { wsClient, type WSEvent } from "../lib/ws";
import type { TaskMessage } from "./useTasks";

interface TaskOutputPayload {
  task_id: string;
  message?: TaskMessage;
  // Structured event fields (sent directly on the payload when type is present)
  sequence?: number;
  type?: string;
  tool?: string;
  content?: string;
  input?: Record<string, unknown>;
  output?: string;
  stream?: string;
}

/**
 * Custom hook that subscribes to WebSocket task_output events for a specific task ID.
 * Appends new messages to local state and returns the accumulated messages array.
 *
 * Supports both legacy format (payload.message with stream/content) and
 * structured format (payload with type/tool/content/input/output fields).
 */
export function useTaskStream(taskId: string) {
  const [messages, setMessages] = useState<TaskMessage[]>([]);
  const seenSequences = useRef<Set<number>>(new Set());

  useEffect(() => {
    // Reset state when taskId changes
    setMessages([]);
    seenSequences.current = new Set();

    if (!taskId) return;

    const unsubscribe = wsClient.on("task_output", (event: WSEvent) => {
      const payload = event.payload as TaskOutputPayload;
      if (payload.task_id !== taskId) return;

      let msg: TaskMessage;

      // Detect structured vs legacy format.
      if (payload.type) {
        // Structured format: fields are directly on the payload.
        const seq = payload.sequence ?? 0;
        if (seenSequences.current.has(seq)) return;
        seenSequences.current.add(seq);

        msg = {
          id: `ws-${seq}`,
          task_id: taskId,
          sequence: seq,
          stream: "stdout",
          content: payload.content ?? "",
          created_at: new Date().toISOString(),
          type: payload.type as TaskMessage["type"],
          tool: payload.tool,
          input: payload.input,
          output: payload.output,
        };
      } else if (payload.message) {
        // Legacy format: message object with stream/content.
        msg = payload.message;
        if (seenSequences.current.has(msg.sequence)) return;
        seenSequences.current.add(msg.sequence);
      } else {
        // Fallback: try to construct from flat payload fields (legacy broadcast).
        const seq = payload.sequence ?? 0;
        if (seenSequences.current.has(seq)) return;
        seenSequences.current.add(seq);

        msg = {
          id: `ws-${seq}`,
          task_id: taskId,
          sequence: seq,
          stream: (payload.stream as TaskMessage["stream"]) ?? "stdout",
          content: payload.content ?? "",
          created_at: new Date().toISOString(),
        };
      }

      setMessages((prev) => {
        const next = [...prev, msg];
        next.sort((a, b) => a.sequence - b.sequence);
        return next;
      });
    });

    return unsubscribe;
  }, [taskId]);

  /**
   * Seed the stream with initial messages fetched from the API.
   * Call this after useTaskMessages resolves to merge historical + live data.
   */
  function seedMessages(initial: TaskMessage[]) {
    setMessages((prev) => {
      const merged = [...initial];
      // Add any live messages not in the initial set
      for (const msg of prev) {
        if (!initial.some((m) => m.sequence === msg.sequence)) {
          merged.push(msg);
        }
      }
      merged.sort((a, b) => a.sequence - b.sequence);

      // Update seen sequences
      seenSequences.current = new Set(merged.map((m) => m.sequence));
      return merged;
    });
  }

  return { messages, seedMessages };
}
