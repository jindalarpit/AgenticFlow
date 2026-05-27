/**
 * DeliverablePanel — Displays the current deliverable output content rendered
 * as formatted markdown. Shows a placeholder when no output is available yet
 * (stage is pending or running).
 *
 * Validates: Requirements 4.3
 */

import ReactMarkdown from "react-markdown";

export interface DeliverablePanelProps {
  /** The markdown content produced by the agent for this deliverable. */
  outputContent: string | null;
  /** Current status of the deliverable stage. */
  status: string;
}

/** Statuses that indicate the agent hasn't produced output yet. */
const PENDING_STATUSES = new Set(["pending", "running"]);

export function DeliverablePanel({ outputContent, status }: DeliverablePanelProps) {
  // Show placeholder when no output and stage is still pending/running
  if (!outputContent && PENDING_STATUSES.has(status)) {
    return (
      <section
        className="rounded-lg border border-gray-200 bg-white p-6"
        aria-label="Deliverable output"
      >
        <div className="flex items-center gap-3 text-gray-500">
          {status === "running" && (
            <span className="inline-block h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-blue-500" />
          )}
          <p className="text-sm">
            {status === "running"
              ? "Agent is working on this deliverable…"
              : "Waiting for agent response…"}
          </p>
        </div>
      </section>
    );
  }

  // Nothing to show if output is empty and stage isn't pending/running
  if (!outputContent) {
    return null;
  }

  // Render the deliverable output as markdown
  return (
    <section
      className="rounded-lg border border-gray-200 bg-white overflow-hidden"
      aria-label="Deliverable output"
    >
      <div className="p-4 overflow-auto max-h-[600px]">
        <div className="prose prose-sm max-w-none text-gray-700">
          <ReactMarkdown>{outputContent}</ReactMarkdown>
        </div>
      </div>
    </section>
  );
}
