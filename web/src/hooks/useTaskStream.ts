import { useEffect, useRef, useState } from "react";
import { wsClient, type WSEvent } from "../lib/ws";
import type { TaskMessage } from "./useTasks";

interface TaskOutputPayload {
  task_id: string;
  message: TaskMessage;
}

/**
 * Custom hook that subscribes to WebSocket task_output events for a specific task ID.
 * Appends new messages to local state and returns the accumulated messages array.
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

      const msg = payload.message;
      // Deduplicate by sequence number
      if (seenSequences.current.has(msg.sequence)) return;
      seenSequences.current.add(msg.sequence);

      setMessages((prev) => {
        // Insert in sequence order
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
