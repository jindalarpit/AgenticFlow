/**
 * StageOutputViewer — Renders stage output content (plan.md, design.md, tasks.md)
 * as formatted markdown within a card/panel using react-markdown.
 *
 * Displays when stage is in "awaiting_approval", "completed", or "approved" status
 * and outputContent is non-null.
 *
 * Validates: Requirements 10.7
 */

import ReactMarkdown from "react-markdown";

export type StageStatus =
  | "pending"
  | "running"
  | "awaiting_approval"
  | "approved"
  | "rejected"
  | "completed"
  | "failed";

export interface StageOutputViewerProps {
  /** The markdown content produced by the stage. */
  outputContent: string | null;
  /** The stage name (e.g., "plan", "design", "tasks"). */
  stageName: string;
  /** Current status of the stage. */
  status: StageStatus;
}

/** Statuses for which the output viewer should render. */
const VISIBLE_STATUSES: ReadonlySet<StageStatus> = new Set([
  "awaiting_approval",
  "completed",
  "approved",
]);

/** Maps stage names to human-readable header labels. */
function getStageLabel(stageName: string): string {
  const labels: Record<string, string> = {
    plan: "Plan Output",
    design: "Design Output",
    tasks: "Tasks Output",
    execution: "Execution Output",
  };
  return labels[stageName] ?? `${stageName.charAt(0).toUpperCase()}${stageName.slice(1)} Output`;
}

export function StageOutputViewer({
  outputContent,
  stageName,
  status,
}: StageOutputViewerProps) {
  // Only render when status is visible and content exists
  if (!VISIBLE_STATUSES.has(status) || !outputContent) {
    return null;
  }

  return (
    <section
      className="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden"
      aria-label={`${getStageLabel(stageName)}`}
    >
      <header className="flex items-center gap-2 border-b border-gray-100 bg-gray-50 px-4 py-3">
        <StageIcon stageName={stageName} />
        <h3 className="text-sm font-semibold text-gray-800">
          {getStageLabel(stageName)}
        </h3>
        <StatusBadge status={status} />
      </header>
      <div className="p-4 overflow-auto max-h-[600px]">
        <div className="prose prose-sm max-w-none text-gray-700">
          <ReactMarkdown>{outputContent}</ReactMarkdown>
        </div>
      </div>
    </section>
  );
}

/* ─── Internal Components ─── */

function StageIcon({ stageName }: { stageName: string }) {
  const icons: Record<string, string> = {
    plan: "📋",
    design: "🏗️",
    tasks: "✅",
    execution: "⚡",
  };
  return (
    <span className="text-base" aria-hidden="true">
      {icons[stageName] ?? "📄"}
    </span>
  );
}

function StatusBadge({ status }: { status: StageStatus }) {
  const styles: Record<string, string> = {
    awaiting_approval: "bg-amber-100 text-amber-700",
    completed: "bg-green-100 text-green-700",
    approved: "bg-blue-100 text-blue-700",
  };

  const labels: Record<string, string> = {
    awaiting_approval: "Awaiting Approval",
    completed: "Completed",
    approved: "Approved",
  };

  return (
    <span
      className={`ml-auto inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${styles[status] ?? "bg-gray-100 text-gray-600"}`}
    >
      {labels[status] ?? status}
    </span>
  );
}


