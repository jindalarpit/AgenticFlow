/* ─── Props ─── */

interface PageHeaderBarProps {
  totalCount: number;
  onCreate: () => void;
}

/**
 * Page header for the Agents list page.
 * Renders bot icon, "Agents" title, total active count, tagline, and "New Agent" button.
 *
 * Requirements: 1.1, 1.2, 1.3
 */
export function PageHeaderBar({ totalCount, onCreate }: PageHeaderBarProps) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div className="flex items-start gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-blue-50">
          <BotIcon />
        </div>
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-lg font-semibold text-gray-900">Agents</h1>
            <span className="text-sm text-gray-500">
              {totalCount} {totalCount === 1 ? "Agent" : "Agents"}
            </span>
          </div>
          <p className="mt-0.5 text-sm text-gray-500">
            Create and manage AI agents that execute tasks on your behalf.
          </p>
        </div>
      </div>

      <button
        type="button"
        onClick={onCreate}
        className="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700 transition-colors"
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
      className="h-5 w-5 text-blue-600"
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
