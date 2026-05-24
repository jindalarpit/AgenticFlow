/* ─── Props ─── */

interface RuntimeCellProps {
  runtimeName: string | null;
  provider: string | null;
  mode: "local" | "cloud";
}

/**
 * Table cell for the Runtime column.
 * Renders the runtime name with a provider badge and device icon (local/cloud).
 *
 * Requirements: 6.5
 */
export function RuntimeCell({ runtimeName, provider, mode }: RuntimeCellProps) {
  if (!runtimeName) {
    return <span className="text-sm text-gray-400">—</span>;
  }

  return (
    <div className="flex items-center gap-2 min-w-0">
      {/* Device icon */}
      {mode === "local" ? <LocalIcon /> : <CloudIcon />}

      {/* Runtime name */}
      <span className="truncate text-sm text-gray-700">{runtimeName}</span>

      {/* Provider badge */}
      {provider && (
        <span className="shrink-0 rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium uppercase text-gray-500">
          {provider}
        </span>
      )}
    </div>
  );
}

/* ─── Icons ─── */

function LocalIcon() {
  return (
    <svg
      className="h-4 w-4 shrink-0 text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
      aria-label="Local runtime"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M9 17.25v1.007a3 3 0 0 1-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0 1 15 18.257V17.25m6-12V15a2.25 2.25 0 0 1-2.25 2.25H5.25A2.25 2.25 0 0 1 3 15V5.25A2.25 2.25 0 0 1 5.25 3h13.5A2.25 2.25 0 0 1 21 5.25Z"
      />
    </svg>
  );
}

function CloudIcon() {
  return (
    <svg
      className="h-4 w-4 shrink-0 text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
      aria-label="Cloud runtime"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M2.25 15a4.5 4.5 0 0 0 4.5 4.5H18a3.75 3.75 0 0 0 1.332-7.257 3 3 0 0 0-3.758-3.848 5.25 5.25 0 0 0-10.233 2.33A4.502 4.502 0 0 0 2.25 15Z"
      />
    </svg>
  );
}
