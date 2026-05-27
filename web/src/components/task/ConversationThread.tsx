/**
 * ConversationThread — Displays prompt history as a chronological chat thread.
 *
 * Fetches prompt history via GET /api/tasks/{taskId}/stages/{stageName}/history
 * and renders user prompts (right-aligned) and agent responses (left-aligned, markdown).
 * Shows a loading state while fetching and auto-scrolls to the latest message.
 *
 * Validates: Requirements 3.4, 4.3
 */

import { useEffect, useRef } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import ReactMarkdown from "react-markdown";
import { apiFetch } from "../../lib/api";
import { wsClient, type WSEvent } from "../../lib/ws";

export interface PromptHistoryEntry {
  id: string;
  task_stage_id: string;
  task_id: string;
  prompt_text: string;
  output_text: string | null;
  created_at: string;
}

export interface ConversationThreadProps {
  taskId: string;
  stageName: string;
}

export function ConversationThread({ taskId, stageName }: ConversationThreadProps) {
  const queryClient = useQueryClient();
  const scrollRef = useRef<HTMLDivElement>(null);

  const { data: history, isLoading } = useQuery({
    queryKey: ["tasks", taskId, "stages", stageName, "history"],
    queryFn: () =>
      apiFetch<PromptHistoryEntry[]>(
        `/api/tasks/${taskId}/stages/${stageName}/history`
      ),
    enabled: !!taskId && !!stageName,
    refetchInterval: false,
  });

  // Subscribe to WebSocket events for real-time updates
  useEffect(() => {
    if (!taskId) return;

    const unsub = wsClient.on("task_completed", (event: WSEvent) => {
      const payload = event.payload as { task_id?: string };
      if (payload.task_id === taskId) {
        void queryClient.invalidateQueries({
          queryKey: ["tasks", taskId, "stages", stageName, "history"],
        });
      }
    });

    return unsub;
  }, [taskId, stageName, queryClient]);

  // Auto-scroll to latest message when history changes
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [history]);

  if (isLoading) {
    return <LoadingSkeleton />;
  }

  if (!history || history.length === 0) {
    return (
      <div className="flex items-center justify-center py-12 text-sm text-gray-400">
        No conversation history yet.
      </div>
    );
  }

  return (
    <div
      ref={scrollRef}
      className="flex flex-col gap-4 overflow-y-auto max-h-[500px] p-4"
      role="log"
      aria-label="Conversation history"
    >
      {history.map((entry) => (
        <ConversationEntry key={entry.id} entry={entry} />
      ))}
    </div>
  );
}

/* ─── Internal Components ─── */

function ConversationEntry({ entry }: { entry: PromptHistoryEntry }) {
  return (
    <div className="flex flex-col gap-2">
      {/* User prompt — right-aligned */}
      <div className="flex justify-end">
        <div className="max-w-[80%] rounded-lg bg-blue-600 px-4 py-2 text-sm text-white shadow-sm">
          <p className="whitespace-pre-wrap">{entry.prompt_text}</p>
          <time className="mt-1 block text-xs text-blue-200">
            {formatTime(entry.created_at)}
          </time>
        </div>
      </div>

      {/* Agent response — left-aligned, markdown */}
      {entry.output_text && (
        <div className="flex justify-start">
          <div className="max-w-[80%] rounded-lg border border-gray-200 bg-gray-50 px-4 py-2 shadow-sm">
            <div className="prose prose-sm max-w-none text-gray-700">
              <ReactMarkdown>{entry.output_text}</ReactMarkdown>
            </div>
            <time className="mt-1 block text-xs text-gray-400">
              {formatTime(entry.created_at)}
            </time>
          </div>
        </div>
      )}
    </div>
  );
}

function LoadingSkeleton() {
  return (
    <div
      className="flex flex-col gap-4 p-4"
      role="status"
      aria-label="Loading conversation"
    >
      {/* Simulated user message skeleton */}
      <div className="flex justify-end">
        <div className="h-10 w-48 animate-pulse rounded-lg bg-gray-200" />
      </div>
      {/* Simulated agent response skeleton */}
      <div className="flex justify-start">
        <div className="h-20 w-64 animate-pulse rounded-lg bg-gray-100" />
      </div>
      {/* Another pair */}
      <div className="flex justify-end">
        <div className="h-10 w-56 animate-pulse rounded-lg bg-gray-200" />
      </div>
      <div className="flex justify-start">
        <div className="h-16 w-72 animate-pulse rounded-lg bg-gray-100" />
      </div>
    </div>
  );
}

function formatTime(isoString: string): string {
  try {
    const date = new Date(isoString);
    return date.toLocaleTimeString(undefined, {
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return "";
  }
}
