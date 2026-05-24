import { useEffect } from "react";

type ViewMode = "structured" | "raw";

interface ViewModeToggleProps {
  taskId: string;
  mode: ViewMode;
  onChange: (mode: ViewMode) => void;
}

function getStorageKey(taskId: string): string {
  return `af_view_mode_${taskId}`;
}

/**
 * Toggle between "Structured" (default) and "Raw" view modes.
 * Persists the selected mode to sessionStorage keyed by task ID.
 */
export function ViewModeToggle({ taskId, mode, onChange }: ViewModeToggleProps) {
  // On mount, read persisted mode from sessionStorage
  useEffect(() => {
    const stored = sessionStorage.getItem(getStorageKey(taskId));
    if (stored === "structured" || stored === "raw") {
      if (stored !== mode) {
        onChange(stored);
      }
    }
  }, [taskId]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleChange = (newMode: ViewMode) => {
    if (newMode !== mode) {
      sessionStorage.setItem(getStorageKey(taskId), newMode);
      onChange(newMode);
    }
  };

  return (
    <div className="inline-flex rounded-lg border border-gray-200 bg-gray-50 p-0.5" role="group" aria-label="View mode">
      <button
        type="button"
        className={`rounded-md px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
          mode === "structured"
            ? "bg-white text-gray-900 shadow-sm"
            : "text-gray-500 hover:text-gray-700"
        }`}
        onClick={() => handleChange("structured")}
        aria-pressed={mode === "structured"}
      >
        Structured
      </button>
      <button
        type="button"
        className={`rounded-md px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
          mode === "raw"
            ? "bg-white text-gray-900 shadow-sm"
            : "text-gray-500 hover:text-gray-700"
        }`}
        onClick={() => handleChange("raw")}
        aria-pressed={mode === "raw"}
      >
        Raw
      </button>
    </div>
  );
}
