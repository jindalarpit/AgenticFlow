import { useState, useEffect, useRef } from "react";

interface FilterDropdownProps {
  options: Array<{ value: string; label: string }>;
  activeFilters: Set<string>;
  onToggle: (value: string) => void;
  onClear: () => void;
  filteredCount: number;
  totalCount: number;
}

/**
 * Multi-select dropdown for filtering timeline events by type or tool name.
 * Shows a count badge when filters are active and an "N of M events" summary.
 *
 * Validates: Requirements 7.1, 7.2, 7.3, 7.5
 */
export function FilterDropdown({
  options,
  activeFilters,
  onToggle,
  onClear,
  filteredCount,
  totalCount,
}: FilterDropdownProps) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // Close dropdown on click outside
  useEffect(() => {
    if (!open) return;

    function handleClickOutside(event: MouseEvent) {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setOpen(false);
      }
    }

    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [open]);

  const activeCount = activeFilters.size;

  return (
    <div ref={containerRef} className="relative inline-block">
      {/* Trigger button */}
      <button
        type="button"
        className="inline-flex items-center gap-1.5 rounded border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
        onClick={() => setOpen((prev) => !prev)}
        aria-expanded={open}
        aria-haspopup="listbox"
      >
        Filter
        {activeCount > 0 && (
          <span className="inline-flex items-center justify-center rounded-full bg-blue-500 px-1.5 py-0.5 text-[10px] font-semibold text-white min-w-[18px]">
            {activeCount}
          </span>
        )}
      </button>

      {/* Dropdown panel */}
      {open && (
        <div
          className="absolute left-0 top-full mt-1 z-50 min-w-[200px] rounded border border-gray-200 bg-white shadow-lg"
          role="listbox"
          aria-multiselectable="true"
        >
          {/* Options list */}
          <div className="max-h-[240px] overflow-y-auto p-2">
            {options.length === 0 ? (
              <p className="text-xs text-gray-400 px-2 py-1">No options</p>
            ) : (
              options.map((option) => (
                <label
                  key={option.value}
                  className="flex items-center gap-2 rounded px-2 py-1.5 text-sm text-gray-700 hover:bg-gray-50 cursor-pointer"
                >
                  <input
                    type="checkbox"
                    checked={activeFilters.has(option.value)}
                    onChange={() => onToggle(option.value)}
                    className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                  />
                  <span className="truncate">{option.label}</span>
                </label>
              ))
            )}
          </div>

          {/* Footer: summary + clear */}
          <div className="border-t border-gray-100 px-3 py-2 flex items-center justify-between">
            <span className="text-xs text-gray-500">
              {filteredCount} of {totalCount} events
            </span>
            {activeCount > 0 && (
              <button
                type="button"
                className="text-xs text-blue-600 hover:text-blue-800 font-medium"
                onClick={() => {
                  onClear();
                }}
              >
                Clear all
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
