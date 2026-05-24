/* ─── Props ─── */

interface EmptyStateProps {
  onCreate: () => void;
}

/**
 * Empty state shown when no agents exist in the workspace (zero active and zero archived).
 * Renders bot icon, "No agents yet" title, description, and "New Agent" CTA button.
 *
 * Requirements: 13.1, 13.2
 */
export function EmptyState({ onCreate }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 px-4 text-center">
      {/* Icon */}
      <div className="flex h-16 w-16 items-center justify-center rounded-full bg-gray-100">
        <BotIcon />
      </div>

      {/* Title */}
      <h2 className="mt-4 text-lg font-semibold text-gray-900">
        No agents yet
      </h2>

      {/* Description */}
      <p className="mt-2 max-w-sm text-sm text-gray-500">
        Agents are AI-powered assistants that execute tasks on your behalf.
        Create your first agent to get started.
      </p>

      {/* CTA Button */}
      <button
        type="button"
        onClick={onCreate}
        className="mt-6 inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700 transition-colors"
        aria-label="Create new agent"
      >
        <PlusIcon />
        New Agent
      </button>
    </div>
  );
}

/* ─── Icons ─── */

function BotIcon() {
  return (
    <svg
      className="h-8 w-8 text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M8.25 3v1.5M4.5 8.25H3m18 0h-1.5M4.5 12H3m18 0h-1.5m-15 3.75H3m18 0h-1.5M8.25 19.5V21M12 3v1.5m0 15V21m3.75-18v1.5m0 15V21m-9-1.5h10.5a2.25 2.25 0 0 0 2.25-2.25V6.75a2.25 2.25 0 0 0-2.25-2.25H6.75A2.25 2.25 0 0 0 4.5 6.75v10.5a2.25 2.25 0 0 0 2.25 2.25Zm.75-12h9v9h-9v-9Z"
      />
    </svg>
  );
}

function PlusIcon() {
  return (
    <svg
      className="h-4 w-4"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={2}
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M12 4.5v15m7.5-7.5h-15"
      />
    </svg>
  );
}
