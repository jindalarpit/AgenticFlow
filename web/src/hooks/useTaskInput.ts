import { useMutation } from "@tanstack/react-query";
import { apiFetch } from "../lib/api";

interface SendTaskInputParams {
  taskId: string;
  text: string;
}

interface SendTaskInputResponse {
  status: string;
  task_id: string;
  timestamp: string;
}

/**
 * Send input text to a running task's stdin pipe.
 * POST /api/tasks/{id}/input
 *
 * Validates: Requirements 4.2
 */
export function useSendTaskInput() {
  return useMutation({
    mutationFn: ({ taskId, text }: SendTaskInputParams) =>
      apiFetch<SendTaskInputResponse>(`/api/tasks/${taskId}/input`, {
        method: "POST",
        body: JSON.stringify({ text }),
      }),
  });
}
