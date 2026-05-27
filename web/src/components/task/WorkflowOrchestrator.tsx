/**
 * WorkflowOrchestrator — Client-side logic controlling the deliverable sequence:
 * plan → design → tasks → execution.
 *
 * This component is purely frontend orchestration. The server accepts any
 * deliverable_type independently — no server-side ordering enforcement.
 *
 * - "Proceed to next" button creates a new task for the next deliverable,
 *   passing all prior completed outputs as prior_context.
 * - "Skip" button skips the current deliverable and moves to the next.
 *
 * Validates: Requirements 7.1, 7.2, 7.3, 7.4, 6.4
 */

import { useState } from "react";
import { type Deliverable, DELIVERABLE_OPTIONS } from "./DeliverableSelector";

/* ─── Types ─── */

export interface DeliverableOutput {
  /** The deliverable type (plan, design, tasks, execution). */
  type: Deliverable;
  /** The text output produced by the agent for this deliverable. */
  content: string;
}

export interface WorkflowOrchestratorProps {
  /** The agent ID to delegate tasks to. */
  agentId: string;
  /** The deliverable type currently being viewed/worked on. */
  currentDeliverableType: Deliverable | null;
  /** Map of completed deliverable type → output content. */
  completedDeliverables: Record<string, string>;
  /**
   * Callback to create a new task for the next deliverable.
   * Receives the deliverable type and prior_context (all completed outputs).
   */
  onCreateTask: (deliverableType: Deliverable, priorContext: string[]) => void;
}

/** Canonical deliverable sequence order. */
const DELIVERABLE_SEQUENCE: Deliverable[] = DELIVERABLE_OPTIONS.map(
  (opt) => opt.value
);

/**
 * WorkflowOrchestrator controls the deliverable sequence and provides
 * "Proceed to next" and "Skip" actions for navigating the workflow.
 */
export function WorkflowOrchestrator({
  agentId: _agentId,
  currentDeliverableType,
  completedDeliverables,
  onCreateTask,
}: WorkflowOrchestratorProps) {
  const [skippedDeliverables, setSkippedDeliverables] = useState<Set<Deliverable>>(
    new Set()
  );

  const nextDeliverable = getNextDeliverable(
    currentDeliverableType,
    completedDeliverables,
    skippedDeliverables
  );

  const isWorkflowComplete = nextDeliverable === null;

  /**
   * Collect all completed deliverable outputs as prior_context array,
   * ordered by the canonical sequence.
   */
  function buildPriorContext(): string[] {
    return DELIVERABLE_SEQUENCE
      .filter((type) => completedDeliverables[type] != null)
      .map((type) => completedDeliverables[type] as string);
  }

  function handleProceed() {
    if (!nextDeliverable) return;
    const priorContext = buildPriorContext();
    onCreateTask(nextDeliverable, priorContext);
  }

  function handleSkip() {
    if (!nextDeliverable) return;
    setSkippedDeliverables((prev) => {
      const next = new Set(prev);
      next.add(nextDeliverable);
      return next;
    });
  }

  // If workflow is complete, show a completion message
  if (isWorkflowComplete) {
    return (
      <div
        className="rounded-lg border border-green-200 bg-green-50 p-4"
        role="status"
        aria-label="Workflow complete"
      >
        <p className="text-sm font-medium text-green-800">
          Workflow complete — all deliverables have been addressed.
        </p>
      </div>
    );
  }

  return (
    <div
      className="flex items-center gap-3 rounded-lg border border-gray-200 bg-white p-4"
      role="group"
      aria-label="Workflow orchestration controls"
    >
      <div className="flex-1">
        <p className="text-sm text-gray-600">
          Next deliverable:{" "}
          <span className="font-medium text-gray-900 capitalize">
            {nextDeliverable}
          </span>
        </p>
      </div>

      <button
        type="button"
        onClick={handleSkip}
        className="inline-flex items-center rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-200"
        aria-label={`Skip ${nextDeliverable} deliverable`}
      >
        Skip
      </button>

      <button
        type="button"
        onClick={handleProceed}
        className="inline-flex items-center rounded-lg bg-blue-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-200"
        aria-label={`Proceed to ${nextDeliverable} deliverable`}
      >
        Proceed to {nextDeliverable}
      </button>
    </div>
  );
}

/* ─── Helpers ─── */

/**
 * Determines the next deliverable in the canonical sequence that hasn't been
 * completed or skipped yet.
 *
 * Returns null if all deliverables are completed or skipped (workflow done).
 */
function getNextDeliverable(
  currentDeliverableType: Deliverable | null,
  completedDeliverables: Record<string, string>,
  skippedDeliverables: Set<Deliverable>
): Deliverable | null {
  // Start searching from after the current deliverable in the sequence
  const startIndex = currentDeliverableType
    ? DELIVERABLE_SEQUENCE.indexOf(currentDeliverableType) + 1
    : 0;

  for (let i = startIndex; i < DELIVERABLE_SEQUENCE.length; i++) {
    const deliverable = DELIVERABLE_SEQUENCE[i];
    const isCompleted = completedDeliverables[deliverable] != null;
    const isSkipped = skippedDeliverables.has(deliverable);

    if (!isCompleted && !isSkipped) {
      return deliverable;
    }
  }

  return null;
}
