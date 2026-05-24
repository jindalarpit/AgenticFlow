/* ─── Props ─── */

interface NoMatchesStateProps {
  view: "active" | "archived";
  search: string;
  scope: "mine" | "all";
}

/**
 * No-matches state shown when search/filter criteria produce zero results.
 * Renders search icon, "No matches" title, and contextual message referencing active filters.
 * Appears in place of the table body, preserving the toolbar and filter row above.
 *
 * Requirements: 14.1, 14.2
 */
export function NoMatchesState({ view, search, scope }: NoMatchesStateProps) {
  const message = buildContextualMessage(view, search, scope);

  return (
    <div className="flex flex-col items-center justify-center py-12 px-4 text-center">
      {/* Icon */}
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-gray-100">
        <SearchIcon />
      </div>

      {/* Title */}
      <h3 className="mt-3 text-sm font-semibold text-gray-900">No matches</h3>

      {/* Contextual Message */}
      <p className="mt-1 max-w-sm text-sm text-gray-500">{message}</p>
    </div>
  );
}

/* ─── Helpers ─── */

function buildContextualMessage(
  view: "active" | "archived",
  search: string,
  scope: "mine" | "all"
): string {
  const parts: string[] = [];

  if (search) {
    parts.push(`matching "${search}"`);
  }

  if (scope === "mine") {
    parts.push("owned by you");
  }

  const viewLabel = view === "archived" ? "archived" : "active";

  if (parts.length > 0) {
    return `No ${viewLabel} agents found ${parts.join(" and ")}. Try adjusting your search or filters.`;
  }

  return `No ${viewLabel} agents match the current filters. Try adjusting your criteria.`;
}

/* ─── Icons ─── */

function SearchIcon() {
  return (
    <svg
      className="h-6 w-6 text-gray-400"
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
