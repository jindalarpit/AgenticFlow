/* ─── Props ─── */

interface ScopeSegmentProps {
  scope: "mine" | "all";
  setScope: (v: "mine" | "all") => void;
  counts: { all: number; mine: number };
}

/**
 * Pill-shaped toggle with "Mine" and "All" buttons, each showing a count badge.
 * Used in the toolbar to filter agents by ownership scope.
 *
 * Requirements: 3.1, 3.4, 3.5
 */
export function ScopeSegment({ scope, setScope, counts }: ScopeSegmentProps) {
  return (
    <div
      className="inline-flex rounded-full border border-gray-200 bg-gray-50 p-0.5"
      role="group"
      aria-label="Agent scope filter"
    >
      <ScopeButton
        label="Mine"
        count={counts.mine}
        isActive={scope === "mine"}
        onClick={() => setScope("mine")}
      />
      <ScopeButton
        label="All"
        count={counts.all}
        isActive={scope === "all"}
        onClick={() => setScope("all")}
      />
    </div>
  );
}

/* ─── Internal ─── */

function ScopeButton({
  label,
  count,
  isActive,
  onClick,
}: {
  label: string;
  count: number;
  isActive: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium transition-colors ${
        isActive
          ? "bg-white text-gray-900 shadow-sm"
          : "text-gray-500 hover:text-gray-700"
      }`}
      aria-pressed={isActive}
    >
      {label}
      <span
        className={`inline-flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[10px] font-semibold ${
          isActive
            ? "bg-blue-100 text-blue-700"
            : "bg-gray-200 text-gray-600"
        }`}
      >
        {count}
      </span>
    </button>
  );
}
