import { ScopeSegment } from "./ScopeSegment";
import { SortDropdown } from "./SortDropdown";
import type { SortKey } from "../../lib/agent-sorting";

/* ─── Props ─── */

interface ActiveToolbarRowProps {
  scope: "mine" | "all";
  setScope: (v: "mine" | "all") => void;
  scopeCounts: { all: number; mine: number };
  sort: SortKey;
  setSort: (v: SortKey) => void;
  search: string;
  setSearch: (v: string) => void;
  visibleCount: number;
  totalCount: number;
  archivedCount: number;
  onShowArchived: () => void;
}

/**
 * Toolbar for the active agents view.
 * Composes SearchInput, ScopeSegment, SortDropdown, archived link, and visible/total count.
 *
 * Requirements: 2.1, 3.1, 4.1, 8.1, 18.1
 */
export function ActiveToolbarRow({
  scope,
  setScope,
  scopeCounts,
  sort,
  setSort,
  search,
  setSearch,
  visibleCount,
  totalCount,
  archivedCount,
  onShowArchived,
}: ActiveToolbarRowProps) {
  return (
    <div className="flex flex-wrap items-center gap-3">
      {/* Search Input */}
      <div className="relative flex-1 min-w-[200px] max-w-xs">
        <SearchIcon />
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search agents..."
          className="w-full rounded-lg border border-gray-200 bg-white py-1.5 pl-9 pr-3 text-sm text-gray-900 placeholder:text-gray-400 focus:border-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-100 transition-colors"
          aria-label="Search agents"
        />
      </div>

      {/* Scope Segment */}
      <ScopeSegment scope={scope} setScope={setScope} counts={scopeCounts} />

      {/* Sort Dropdown */}
      <SortDropdown sort={sort} setSort={setSort} />

      {/* Spacer */}
      <div className="flex-1" />

      {/* Archived Link */}
      {archivedCount > 0 && (
        <button
          type="button"
          onClick={onShowArchived}
          className="text-sm text-gray-500 hover:text-gray-700 transition-colors"
          aria-label={`View ${archivedCount} archived agents`}
        >
          Archived ({archivedCount})
        </button>
      )}

      {/* Visible / Total Count */}
      <span className="text-xs text-gray-400 tabular-nums">
        {visibleCount} of {totalCount}
      </span>
    </div>
  );
}

/* ─── Icons ─── */

function SearchIcon() {
  return (
    <svg
      className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z"
      />
    </svg>
  );
}
