import { useEffect, useRef, useState, useCallback } from "react";
import { useParams, Link, useNavigate } from "react-router-dom";
import { useTask, useTaskMessages, useCancelTask, useCreateTask } from "../hooks/useTasks";
import { useTaskStream } from "../hooks/useTaskStream";
import { useTimeline } from "../hooks/useTimeline";
import { useSessionState } from "../hooks/useSessionState";
import { useTaskStages } from "../hooks/useTaskStages";
import { useQueryClient } from "@tanstack/react-query";
import { wsClient, type WSEvent } from "../lib/ws";
import { TaskInput } from "../components/TaskInput";
import { formatCopyText } from "../lib/tool-chain-parser";
import {
  ViewModeToggle,
  TimelineBar,
  TimelineView,
  FinalResultPanel,
  FilterDropdown,
  CopyButton,
  MetadataChips,
} from "../components/task-timeline";
import { StageProgressIndicator } from "../components/task/StageProgressIndicator";
import { StageApprovalPanel } from "../components/task/StageApprovalPanel";
import { StageOutputViewer } from "../components/task/StageOutputViewer";
import { ConversationThread } from "../components/task/ConversationThread";
import { DeliverablePanel } from "../components/task/DeliverablePanel";
import { FollowUpInput, type StageStatus } from "../components/task/FollowUpInput";
import { WorkflowOrchestrator } from "../components/task/WorkflowOrchestrator";
import { DeliverableNav, type DeliverableInfo } from "../components/task/DeliverableNav";
import type { Deliverable } from "../components/task/DeliverableSelector";

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

/** Valid deliverable types for conversational tasks. */
const VALID_DELIVERABLE_TYPES = new Set(["plan", "design", "tasks", "execution"]);

export default function TaskDetail() {
  const { id } = useParams<{ id: string }>();
  const taskId = id ?? "";
  const navigate = useNavigate();

  const { data: task, isLoading: taskLoading, error: taskError } = useTask(taskId);
  const { data: initialMessages } = useTaskMessages(taskId);
  const { messages, seedMessages } = useTaskStream(taskId);
  const { data: stages } = useTaskStages(taskId);
  const cancelTask = useCancelTask();
  const createTask = useCreateTask();
  const queryClient = useQueryClient();
  const sessionState = useSessionState(taskId);

  // View mode state: "structured" (default) or "raw"
  const [viewMode, setViewMode] = useState<"structured" | "raw">("structured");

  // Conversational task state: active deliverable tab
  const [activeDeliverable, setActiveDeliverable] = useState<string | null>(null);

  // Reset active deliverable when navigating to a different task
  useEffect(() => {
    setActiveDeliverable(null);
  }, [taskId]);

  // Wire useTimeline hook with messages from useTaskStream
  const {
    items,
    filteredItems,
    filters,
    toggleFilter,
    clearFilters,
    toolCallCount,
    totalCount,
    filterOptions,
  } = useTimeline({ taskId, messages });

  // State for timeline bar segment click → scroll to item
  const [selectedSeq, setSelectedSeq] = useState<number | null>(null);

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

  // Auto-scroll output to bottom (raw view)
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

  // Stage UI: only show when the task has stages (backward compat for single-pass tasks)
  const hasStages = Array.isArray(stages) && stages.length > 0;

  // Detect conversational task: has stages where stage name is a valid deliverable type
  const isConversationalTask = hasStages
    ? stages.some((s) => VALID_DELIVERABLE_TYPES.has(s.name))
    : false;

  // For legacy approval-gate tasks (non-conversational staged tasks)
  const awaitingStage = hasStages && !isConversationalTask
    ? stages.find((s) => s.status === "awaiting_approval")
    : undefined;
  // Stages whose output should be visible (awaiting_approval, completed, approved)
  const visibleOutputStages = hasStages && !isConversationalTask
    ? stages.filter(
        (s) =>
          s.status === "awaiting_approval" ||
          s.status === "completed" ||
          s.status === "approved"
      )
    : [];

  // Conversational task: derive deliverable info for DeliverableNav
  const deliverableInfos: DeliverableInfo[] = isConversationalTask && stages
    ? stages
        .filter((s) => VALID_DELIVERABLE_TYPES.has(s.name))
        .map((s) => ({
          type: s.name,
          status: s.status === "completed" ? "completed"
            : s.status === "running" ? "running"
            : "pending" as DeliverableInfo["status"],
        }))
    : [];

  // Set initial active deliverable when stages load
  useEffect(() => {
    if (!isConversationalTask || !stages || stages.length === 0) return;
    if (activeDeliverable) return; // Already set

    // Default to the first non-completed stage, or the last stage if all completed
    const validStages = stages.filter((s) => VALID_DELIVERABLE_TYPES.has(s.name));
    const activeStage = validStages.find((s) => s.status === "running")
      ?? validStages.find((s) => s.status === "pending")
      ?? validStages[validStages.length - 1];
    if (activeStage) {
      setActiveDeliverable(activeStage.name);
    }
  }, [isConversationalTask, stages, activeDeliverable]);

  // Get the active stage data for the conversational UI
  const activeStageData = isConversationalTask && activeDeliverable && stages
    ? stages.find((s) => s.name === activeDeliverable)
    : null;

  // Build completed deliverables map for WorkflowOrchestrator
  const completedDeliverables: Record<string, string> = {};
  if (isConversationalTask && stages) {
    for (const s of stages) {
      if (s.status === "completed" && s.output_content) {
        completedDeliverables[s.name] = s.output_content;
      }
    }
  }

  // Handle creating a new task for the next deliverable (WorkflowOrchestrator callback)
  const handleCreateNextDeliverable = useCallback(
    (deliverableType: Deliverable, priorContext: string[]) => {
      if (!task) return;
      createTask.mutate(
        {
          agent_type: task.agent_type,
          prompt: task.prompt,
          agent_id: task.agent_id ?? undefined,
          deliverable_type: deliverableType,
          prior_context: priorContext,
        },
        {
          onSuccess: (newTask) => {
            // Navigate to the newly created task
            navigate(`/tasks/${newTask.id}`);
          },
        }
      );
    },
    [task, createTask, navigate]
  );

  // Handle follow-up sent: refresh stages and history
  const handleFollowUpSent = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ["tasks", taskId, "stages"] });
    void queryClient.invalidateQueries({ queryKey: ["tasks", taskId] });
  }, [queryClient, taskId]);

  // Subscribe to WebSocket events for conversational task real-time updates
  useEffect(() => {
    if (!taskId || !isConversationalTask) return;

    const unsub1 = wsClient.on("task_output", (event: WSEvent) => {
      const payload = event.payload as { task_id?: string };
      if (payload.task_id === taskId) {
        // task_output events are handled by useTaskStream for raw output
        // For conversational tasks, we also refresh stages to update status
        void queryClient.invalidateQueries({ queryKey: ["tasks", taskId, "stages"] });
      }
    });

    const unsub2 = wsClient.on("task_completed", (event: WSEvent) => {
      const payload = event.payload as { task_id?: string; deliverable_type?: string };
      if (payload.task_id === taskId) {
        void queryClient.invalidateQueries({ queryKey: ["tasks", taskId, "stages"] });
        // Refresh history for the active deliverable
        if (activeDeliverable) {
          void queryClient.invalidateQueries({
            queryKey: ["tasks", taskId, "stages", activeDeliverable, "history"],
          });
        }
      }
    });

    const unsub3 = wsClient.on("task_failed", (event: WSEvent) => {
      const payload = event.payload as { task_id?: string };
      if (payload.task_id === taskId) {
        void queryClient.invalidateQueries({ queryKey: ["tasks", taskId, "stages"] });
      }
    });

    const unsub4 = wsClient.on("task_started", (event: WSEvent) => {
      const payload = event.payload as { task_id?: string };
      if (payload.task_id === taskId) {
        void queryClient.invalidateQueries({ queryKey: ["tasks", taskId, "stages"] });
      }
    });

    return () => {
      unsub1();
      unsub2();
      unsub3();
      unsub4();
    };
  }, [taskId, isConversationalTask, activeDeliverable, queryClient]);

  // Prepare copy text from filtered items
  const copyText = formatCopyText(filteredItems);

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
              {sessionState === "waiting_for_input" && (
                <span className="inline-flex items-center gap-1.5 rounded-full bg-amber-100 px-2.5 py-0.5 text-xs font-medium text-amber-800">
                  <span className="inline-block h-2 w-2 animate-pulse rounded-full bg-amber-400" />
                  Waiting for input…
                </span>
              )}
            </div>
            <div className="flex items-center gap-3">
              <ViewModeToggle taskId={taskId} mode={viewMode} onChange={setViewMode} />
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

          {/* Metadata Chips */}
          <div className="mt-3">
            <MetadataChips
              toolCallCount={toolCallCount}
              totalCount={totalCount}
              taskStatus={task.status}
              startedAt={task.started_at ?? undefined}
              completedAt={task.completed_at ?? undefined}
            />
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

        {/* Conversational Task UI — shown for tasks with deliverable_type */}
        {isConversationalTask && activeDeliverable && (
          <div className="mt-4 space-y-4">
            {/* Deliverable Navigation Tabs */}
            <div className="rounded-lg border border-gray-200 bg-white overflow-hidden">
              <DeliverableNav
                deliverables={deliverableInfos}
                activeType={activeDeliverable}
                onSelect={setActiveDeliverable}
              />
            </div>

            {/* Conversation Thread — chat history for the active deliverable */}
            <div className="rounded-lg border border-gray-200 bg-white">
              <div className="border-b border-gray-100 px-4 py-2">
                <h3 className="text-sm font-medium text-gray-700">Conversation</h3>
              </div>
              <ConversationThread taskId={taskId} stageName={activeDeliverable} />
            </div>

            {/* Deliverable Panel — current output as markdown */}
            {activeStageData && (
              <DeliverablePanel
                outputContent={activeStageData.output_content}
                status={activeStageData.status}
              />
            )}

            {/* Follow-Up Input — send refinement messages */}
            {activeStageData && (
              <FollowUpInput
                taskId={taskId}
                stageName={activeDeliverable}
                stageStatus={activeStageData.status as StageStatus}
                onFollowUpSent={handleFollowUpSent}
              />
            )}

            {/* Workflow Orchestrator — proceed/skip controls */}
            {activeStageData?.status === "completed" && task.agent_id && (
              <WorkflowOrchestrator
                agentId={task.agent_id}
                currentDeliverableType={activeDeliverable as Deliverable}
                completedDeliverables={completedDeliverables}
                onCreateTask={handleCreateNextDeliverable}
              />
            )}
          </div>
        )}

        {/* Legacy Stage Progress & Approval UI — only shown for non-conversational staged tasks */}
        {hasStages && !isConversationalTask && (
          <div className="mt-4 space-y-4">
            {/* Stage Progress Indicator */}
            <div className="rounded-lg border border-gray-200 bg-white p-4">
              <StageProgressIndicator
                stages={stages.map((s) => ({ name: s.name, status: s.status }))}
              />
            </div>

            {/* Stage Output Viewers for completed/awaiting stages */}
            {visibleOutputStages.map((stage) => (
              <StageOutputViewer
                key={stage.name}
                stageName={stage.name}
                status={stage.status}
                outputContent={stage.output_content}
              />
            ))}

            {/* Approval Panel for the stage awaiting approval */}
            {awaitingStage && (
              <StageApprovalPanel
                taskId={taskId}
                stageName={awaitingStage.name}
                status={awaitingStage.status}
                onApprove={() => {
                  void queryClient.invalidateQueries({
                    queryKey: ["tasks", taskId, "stages"],
                  });
                  void queryClient.invalidateQueries({
                    queryKey: ["tasks", taskId],
                  });
                }}
                onReject={() => {
                  void queryClient.invalidateQueries({
                    queryKey: ["tasks", taskId, "stages"],
                  });
                  void queryClient.invalidateQueries({
                    queryKey: ["tasks", taskId],
                  });
                }}
              />
            )}
          </div>
        )}

        {/* Error message */}
        {task.error_message && isTerminal && (
          <div className="mt-4 rounded-lg border border-red-200 bg-red-50 p-4">
            <p className="text-xs font-medium text-red-600">Error</p>
            <p className="mt-1 whitespace-pre-wrap font-mono text-sm text-red-800">
              {task.error_message}
            </p>
          </div>
        )}

        {/* Structured View */}
        {viewMode === "structured" && (
          <div className="mt-4 flex flex-col gap-3">
            {/* Toolbar: Filter + Copy */}
            <div className="flex items-center justify-between">
              <FilterDropdown
                options={filterOptions}
                activeFilters={filters}
                onToggle={toggleFilter}
                onClear={clearFilters}
                filteredCount={filteredItems.length}
                totalCount={totalCount}
              />
              <CopyButton text={copyText} hasActiveFilters={filters.size > 0} />
            </div>

            {/* Timeline Bar */}
            <TimelineBar
              items={items}
              selectedSeq={selectedSeq}
              onSegmentClick={setSelectedSeq}
            />

            {/* Timeline View */}
            <div className="h-[500px] flex flex-col rounded-lg border border-gray-200 bg-white p-3">
              <TimelineView
                items={filteredItems}
                isLive={isRunning}
                highlightedSeq={selectedSeq}
              />
            </div>

            {/* Final Result Panel */}
            <FinalResultPanel items={items} taskStatus={task.status} />
          </div>
        )}

        {/* Raw View — existing terminal output */}
        {viewMode === "raw" && (
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
                    msg.stream === "stderr"
                      ? "text-red-400"
                      : msg.stream === "stdin"
                        ? "text-green-400"
                        : "text-gray-100"
                  }
                >
                  <span className="whitespace-pre-wrap">
                    {msg.stream === "stdin" ? `> ${msg.content}` : msg.content}
                  </span>
                </div>
              ))}
            </div>
            <TaskInput
              taskId={taskId}
              isRunning={isRunning}
              isWaitingForInput={sessionState === "waiting_for_input"}
            />
          </div>
        )}

        {/* TaskInput for structured view (still need input capability) */}
        {viewMode === "structured" && (
          <div className="mt-2">
            <TaskInput
              taskId={taskId}
              isRunning={isRunning}
              isWaitingForInput={sessionState === "waiting_for_input"}
            />
          </div>
        )}
      </div>
    </div>
  );
}
