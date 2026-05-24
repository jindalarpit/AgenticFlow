import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api";

export interface Task {
  id: string;
  user_id: string;
  agent_type: string;
  prompt: string;
  status: "pending" | "running" | "completed" | "failed" | "cancelled" | "timeout";
  exit_code: number | null;
  error_message: string | null;
  output_preview: string | null;
  agent_id: string | null;
  agent_name: string | null;
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface TaskMessage {
  id: string;
  task_id: string;
  sequence: number;
  stream: "stdout" | "stderr" | "stdin";
  content: string;
  created_at: string;
}

export interface TasksResponse {
  tasks: Task[];
  total: number;
}

interface CreateTaskInput {
  agent_type: string;
  prompt: string;
  agent_id?: string;
}

/**
 * Fetch paginated task list.
 * GET /api/tasks?limit=<limit>&offset=<offset>
 */
export function useTasks(limit: number = 50, offset: number = 0) {
  return useQuery({
    queryKey: ["tasks", { limit, offset }],
    queryFn: () =>
      apiFetch<TasksResponse>(`/api/tasks?limit=${limit}&offset=${offset}`),
  });
}

/**
 * Fetch a single task by ID.
 * GET /api/tasks/{id}
 */
export function useTask(id: string) {
  return useQuery({
    queryKey: ["tasks", id],
    queryFn: () => apiFetch<Task>(`/api/tasks/${id}`),
    enabled: !!id,
  });
}

/**
 * Fetch messages (streaming output) for a task.
 * GET /api/tasks/{id}/messages
 */
export function useTaskMessages(id: string) {
  return useQuery({
    queryKey: ["tasks", id, "messages"],
    queryFn: () => apiFetch<TaskMessage[]>(`/api/tasks/${id}/messages`),
    enabled: !!id,
    refetchInterval: false, // WebSocket handles real-time updates
  });
}

/**
 * Create a new task.
 * POST /api/tasks
 */
export function useCreateTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateTaskInput) =>
      apiFetch<Task>("/api/tasks", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["tasks"] });
    },
  });
}

/**
 * Cancel a running task.
 * POST /api/tasks/{id}/cancel
 */
export function useCancelTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (taskId: string) =>
      apiFetch<void>(`/api/tasks/${taskId}/cancel`, { method: "POST" }),
    onSuccess: (_data, taskId) => {
      void queryClient.invalidateQueries({ queryKey: ["tasks", taskId] });
      void queryClient.invalidateQueries({ queryKey: ["tasks"] });
    },
  });
}
