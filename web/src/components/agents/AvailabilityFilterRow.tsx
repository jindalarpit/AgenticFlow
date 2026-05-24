import type { AvailabilityFilter } from "../../lib/agent-filters";

/* ─── Props ─── */

interface AvailabilityFilterRowProps {
  value: AvailabilityFilter;
  onChange: (v: AvailabilityFilter) => void;
  counts: Record<"online" | "unstable" | "offline", number>;
  totalCount: number;
}

/* ─── Config ─── */

interface ChipConfig {
  key: AvailabilityFilter;
  label: string;
  dotColor?: string;
}

const CHIPS: ChipConfig[] = [
  { key: "all", label: "All" },
  { key: "online", label: "Online", dotColor: "bg-green-500" },
  { key: "unstable", label: "Unstable", dotColor: "bg-amber-500" },
  { key: "offline", label: "Offline", dotColor: "bg-gray-400" },
];

/**
 * Row of chip buttons for filtering agents by availability status.
 * Each chip shows a colored dot indicator and a count badge.
 *
 * Requirements: 5.1, 5.2, 5.3, 5.5, 5.6
 */
export function AvailabilityFilterRow({
  value,
  onChange,
  counts,
  totalCount,
}: AvailabilityFilterRowProps) {
  return (
    <div
      className="flex items-center gap-2 flex-wrap"
      role="group"
      aria-label="Filter by availability"
    >
      {CHIPS.map((chip) => {
        const count =
          chip.key === "all" ? totalCount : counts[chip.key];
        const isActive = value === chip.key;

        return (
          <button
            key={chip.key}
            type="button"
            onClick={() => onChange(chip.key)}
            aria-pressed={isActive}
            className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium transition-colors border ${
              isActive
                ? "border-blue-200 bg-blue-50 text-blue-700"
                : "border-gray-200 bg-white text-gray-600 hover:bg-gray-50"
            }`}
          >
            {chip.dotColor && (
              <span
                className={`h-2 w-2 rounded-full ${chip.dotColor}`}
                aria-hidden="true"
              />
            )}
            {chip.label}
            <span
              className={`inline-flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[10px] font-semibold ${
                isActive
                  ? "bg-blue-100 text-blue-700"
                  : "bg-gray-100 text-gray-500"
              }`}
            >
              {count}
            </span>
          </button>
        );
      })}
    </div>
  );
}
