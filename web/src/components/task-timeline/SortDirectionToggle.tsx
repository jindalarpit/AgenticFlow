export type SortDirection = "chronological" | "newest_first";

interface SortDirectionToggleProps {
  direction: SortDirection;
  onChange: (direction: SortDirection) => void;
}

/**
 * Segmented toggle control for switching between chronological (oldest-first)
 * and newest-first event ordering in the timeline view.
 */
export function SortDirectionToggle({ direction, onChange }: SortDirectionToggleProps) {
  return (
    <div className="inline-flex rounded-lg border border-gray-200 bg-gray-50 p-0.5" role="group" aria-label="Sort direction">
      <button
        type="button"
        className={`rounded-md px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
          direction === "chronological"
            ? "bg-white text-gray-900 shadow-sm"
            : "text-gray-500 hover:text-gray-700"
        }`}
        onClick={() => onChange("chronological")}
        aria-pressed={direction === "chronological"}
      >
        Chronological
      </button>
      <button
        type="button"
        className={`rounded-md px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
          direction === "newest_first"
            ? "bg-white text-gray-900 shadow-sm"
            : "text-gray-500 hover:text-gray-700"
        }`}
        onClick={() => onChange("newest_first")}
        aria-pressed={direction === "newest_first"}
      >
        Newest first
      </button>
    </div>
  );
}
