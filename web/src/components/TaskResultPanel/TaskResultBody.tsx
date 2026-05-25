import { useState } from "react";
import type { Task, TaskMessage } from "../../hooks/useTasks";
import { useTimeline } from "../../hooks/useTimeline";
import { formatCopyText } from "../../lib/tool-chain-parser";
import {
  TimelineBar,
  TimelineView,
  MetadataChips,
  FilterDropdown,
  SortDirectionToggle,
  CopyButton,
} from "../task-timeline";

export interface TaskResultBodyProps {
  task: Task;
  messages: TaskMessage[];
  isStreaming: boolean;
}

/**
 * Renders the structured timeline view for ALL task statuses (pending, running,
 * completed, failed, cancelled, timeout). Replaces the previous routing logic
 * that used StreamingOutput / FinalResult / ErrorDisplay.
 *
 * Validates: Requirements 9.1, 9.2, 9.3, 9.4, 9.5, 9.6, 9.7
 */
export function TaskResultBody({ task, messages, isStreaming }: TaskResultBodyProps) {
  const timeline = useTimeline({ taskId: task.id, messages });

  // State for timeline bar segment click → highlight in TimelineView
  const [highlightedSeq, setHighlightedSeq] = useState<number | null>(null);

  const isRunning = task.status === "running";

  // Prepare copy text from filtered items
  const copyText = formatCopyText(timeline.filteredItems);

  return (
    <div className="flex flex-col gap-2 px-4 py-3">
      {/* Header: status indicator, metadata chips, filter, sort, copy */}
      <div className="flex items-center justify-between gap-2 flex-wrap">
        <div className="flex items-center gap-2">
          {/* Animated spinner when task is running */}
          {isRunning && <RunningSpinner />}

          {/* Metadata chips: tool call count, total events, duration */}
          <MetadataChips
            toolCallCount={timeline.toolCallCount}
            totalCount={timeline.totalCount}
            taskStatus={task.status}
            startedAt={task.started_at ?? undefined}
            completedAt={task.completed_at ?? undefined}
          />
        </div>

        <div className="flex items-center gap-2">
          <FilterDropdown
            options={timeline.filterOptions}
            activeFilters={timeline.filters}
            onToggle={timeline.toggleFilter}
            onClear={timeline.clearFilters}
            filteredCount={timeline.filteredItems.length}
            totalCount={timeline.totalCount}
          />
          <SortDirectionToggle
            direction={timeline.sortDirection}
            onChange={timeline.setSortDirection}
          />
          <CopyButton
            text={copyText}
            hasActiveFilters={timeline.filters.size > 0}
          />
        </div>
      </div>

      {/* Timeline progress bar */}
      <TimelineBar
        items={timeline.items}
        selectedSeq={highlightedSeq}
        onSegmentClick={setHighlightedSeq}
      />

      {/* Main scrollable timeline content */}
      <div className="h-[300px] flex flex-col">
        <TimelineView
          items={timeline.filteredItems}
          isLive={isStreaming}
          highlightedSeq={highlightedSeq}
          sortDirection={timeline.sortDirection}
          scrollToTopSignal={timeline.scrollToTopSignal}
        />
      </div>
    </div>
  );
}

/* ─── Running Spinner ─── */

function RunningSpinner() {
  return (
    <svg
      className="animate-spin h-4 w-4 text-blue-500"
      xmlns="http://www.w3.org/2000/svg"
      fill="none"
      viewBox="0 0 24 24"
      aria-label="Task is running"
    >
      <circle
        className="opacity-25"
        cx="12"
        cy="12"
        r="10"
        stroke="currentColor"
        strokeWidth="4"
      />
      <path
        className="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
      />
    </svg>
  );
}
