import { useEffect, useRef } from "react";
import { useParams, Link } from "react-router-dom";
import { useTask, useTaskMessages, useCancelTask } from "../hooks/useTasks";
import { useTaskStream } from "../hooks/useTaskStream";
import { useQueryClient } from "@tanstack/react-query";
import { wsClient, type WSEvent } from "../lib/ws";

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    pending: "bg-yellow-100 text-yellow-800",
    running: "bg-blue-100 text-blue-800",
    completed: "bg-green-100 text-green-800",
    failed: "bg-red-100 text-red-800",
    cancelled: "bg-gray-100 text-gray-800",
    timeout: "bg-orange-100 text-orange-800",
  };

  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${styles[status] ?? "bg-gray-100 text-gray-800"}`}
    >
      {status}
    </span>
  );
}

function formatDuration(startedAt: string | null, completedAt: string | null): string {
  if (!startedAt) return "—";
  const start = new Date(startedAt).getTime();
  const end = completedAt ? new Date(completedAt).getTime() : Date.now();
  const seconds = Math.floor((end - start) / 1000);

  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  if (minutes < 60) return `${minutes}m ${remainingSeconds}s`;
  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return `${hours}h ${remainingMinutes}m`;
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString();
}

export default function TaskDetail() {
  const { id } = useParams<{ id: string }>();
  const taskId = id ?? "";

  const { data: task, isLoading: taskLoading, error: taskError } = useTask(taskId);
  const { data: initialMessages } = useTaskMessages(taskId);
  const { messages, seedMessages } = useTaskStream(taskId);
  const cancelTask = useCancelTask();
  const queryClient = useQueryClient();

  const outputRef = useRef<HTMLDivElement>(null);
  const autoScrollRef = useRef(true);

  // Seed stream with initial messages from API
  useEffect(() => {
    if (initialMessages) {
      seedMessages(initialMessages);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialMessages]);

  // Listen for task_completed / task_failed to refresh task metadata
  useEffect(() => {
    if (!taskId) return;

    const unsub1 = wsClient.on("task_completed", (event: WSEvent) => {
      const payload = event.payload as { task_id: string };
      if (payload.task_id === taskId) {
        void queryClient.invalidateQueries({ queryKey: ["tasks", taskId] });
      }
    });

    const unsub2 = wsClient.on("task_failed", (event: WSEvent) => {
      const payload = event.payload as { task_id: string };
      if (payload.task_id === taskId) {
        void queryClient.invalidateQueries({ queryKey: ["tasks", taskId] });
      }
    });

    const unsub3 = wsClient.on("task_started", (event: WSEvent) => {
      const payload = event.payload as { task_id: string };
      if (payload.task_id === taskId) {
        void queryClient.invalidateQueries({ queryKey: ["tasks", taskId] });
      }
    });

    return () => {
      unsub1();
      unsub2();
      unsub3();
    };
  }, [taskId, queryClient]);

  // Auto-scroll output to bottom
  useEffect(() => {
    if (autoScrollRef.current && outputRef.current) {
      outputRef.current.scrollTop = outputRef.current.scrollHeight;
    }
  }, [messages]);

  function handleScroll() {
    if (!outputRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = outputRef.current;
    // If user scrolled up more than 50px from bottom, disable auto-scroll
    autoScrollRef.current = scrollHeight - scrollTop - clientHeight < 50;
  }

  function handleCancel() {
    if (taskId) {
      cancelTask.mutate(taskId);
    }
  }

  if (taskLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-gray-500">Loading task…</p>
      </div>
    );
  }

  if (taskError || !task) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <p className="text-red-600">
            {taskError instanceof Error ? taskError.message : "Task not found"}
          </p>
          <Link to="/" className="mt-2 inline-block text-sm text-blue-600 hover:underline">
            ← Back to Dashboard
          </Link>
        </div>
      </div>
    );
  }

  const isRunning = task.status === "running";
  const isTerminal = ["completed", "failed", "cancelled", "timeout"].includes(task.status);

  return (
    <div>
      {/* Header */}
      <div className="border-b border-gray-200 bg-white px-6 py-4">
        <div className="mx-auto max-w-5xl">
          <Link to="/" className="text-sm text-blue-600 hover:underline">
            ← Dashboard
          </Link>
          <div className="mt-2 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <h1 className="text-lg font-semibold text-gray-900">Task</h1>
              <StatusBadge status={task.status} />
            </div>
            {isRunning && (
              <button
                onClick={handleCancel}
                disabled={cancelTask.isPending}
                className="rounded-md bg-red-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50"
              >
                {cancelTask.isPending ? "Cancelling…" : "Cancel"}
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Metadata */}
      <div className="mx-auto max-w-5xl px-6 py-4">
        <div className="grid grid-cols-2 gap-4 rounded-lg border border-gray-200 bg-white p-4 sm:grid-cols-4">
          <div>
            <p className="text-xs font-medium text-gray-500">Agent</p>
            <p className="mt-1 text-sm text-gray-900">
              {task.agent_id && task.agent_name ? (
                <Link
                  to={`/agents/${task.agent_id}`}
                  className="text-blue-600 hover:text-blue-700 hover:underline"
                >
                  {task.agent_name}
                </Link>
              ) : (
                task.agent_name || task.agent_type
              )}
            </p>
          </div>
          <div>
            <p className="text-xs font-medium text-gray-500">Created</p>
            <p className="mt-1 text-sm text-gray-900">{formatTime(task.created_at)}</p>
          </div>
          <div>
            <p className="text-xs font-medium text-gray-500">Duration</p>
            <p className="mt-1 text-sm text-gray-900">
              {formatDuration(task.started_at, task.completed_at)}
              {isRunning && <span className="ml-1 text-blue-600">●</span>}
            </p>
          </div>
          <div>
            <p className="text-xs font-medium text-gray-500">Status</p>
            <p className="mt-1 text-sm text-gray-900 capitalize">{task.status}</p>
          </div>
        </div>

        {/* Prompt */}
        <details className="mt-4 rounded-lg border border-gray-200 bg-white" open>
          <summary className="cursor-pointer px-4 py-3 text-sm font-medium text-gray-700">
            Prompt
          </summary>
          <div className="border-t border-gray-100 px-4 py-3">
            <p className="whitespace-pre-wrap text-sm text-gray-800">{task.prompt}</p>
          </div>
        </details>

        {/* Error message */}
        {task.error_message && isTerminal && (
          <div className="mt-4 rounded-lg border border-red-200 bg-red-50 p-4">
            <p className="text-xs font-medium text-red-600">Error</p>
            <p className="mt-1 whitespace-pre-wrap font-mono text-sm text-red-800">
              {task.error_message}
            </p>
          </div>
        )}

        {/* Terminal output */}
        <div className="mt-4">
          <div className="flex items-center justify-between rounded-t-lg border border-b-0 border-gray-700 bg-gray-800 px-4 py-2">
            <span className="text-xs font-medium text-gray-300">Output</span>
            {isRunning && (
              <span className="flex items-center gap-1 text-xs text-green-400">
                <span className="inline-block h-2 w-2 animate-pulse rounded-full bg-green-400" />
                Streaming
              </span>
            )}
          </div>
          <div
            ref={outputRef}
            onScroll={handleScroll}
            className="h-96 overflow-y-auto rounded-b-lg border border-gray-700 bg-gray-900 p-4 font-mono text-sm leading-relaxed"
          >
            {messages.length === 0 && !isRunning && isTerminal && (
              <p className="text-gray-500">No output recorded.</p>
            )}
            {messages.length === 0 && (task.status === "pending" || isRunning) && (
              <p className="text-gray-500">Waiting for output…</p>
            )}
            {messages.map((msg) => (
              <div
                key={msg.sequence}
                className={
                  msg.stream === "stderr" ? "text-red-400" : "text-gray-100"
                }
              >
                <span className="whitespace-pre-wrap">{msg.content}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
