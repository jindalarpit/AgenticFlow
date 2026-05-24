import { SortDropdown } from "./SortDropdown";
import type { SortKey } from "../../lib/agent-sorting";

/* ─── Props ─── */

interface ArchivedToolbarRowProps {
  onBack: () => void;
  archivedCount: number;
  sort: SortKey;
  setSort: (v: SortKey) => void;
}

/**
 * Toolbar for the archived agents view.
 * Renders back link "← Active", title "Archived", count, and SortDropdown.
 * No scope segment or availability chips in this view.
 *
 * Requirements: 8.3
 */
export function ArchivedToolbarRow({
  onBack,
  archivedCount,
  sort,
  setSort,
}: ArchivedToolbarRowProps) {
  return (
    <div className="flex items-center gap-4">
      {/* Back Link */}
      <button
        type="button"
        onClick={onBack}
        className="inline-flex items-center gap-1 text-sm text-blue-600 hover:text-blue-700 transition-colors"
        aria-label="Back to active agents"
      >
        <ArrowLeftIcon />
        Active
      </button>

      {/* Title + Count */}
      <div className="flex items-center gap-2">
        <h2 className="text-sm font-semibold text-gray-900">Archived</h2>
        <span className="inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-gray-100 px-1.5 text-xs font-medium text-gray-600">
          {archivedCount}
        </span>
      </div>

      {/* Spacer */}
      <div className="flex-1" />

      {/* Sort Dropdown */}
      <SortDropdown sort={sort} setSort={setSort} />
    </div>
  );
}

/* ─── Icons ─── */

function ArrowLeftIcon() {
  return (
    <svg
      className="h-4 w-4"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M10.5 19.5 3 12m0 0 7.5-7.5M3 12h18"
      />
    </svg>
  );
}
