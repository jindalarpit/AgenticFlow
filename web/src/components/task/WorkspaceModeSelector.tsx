import type { ChangeEvent } from "react";

/* ─── Types ─── */

export type WorkspaceMode = "isolated" | "existing";

export interface WorkspaceModeSelectorProps {
  mode: WorkspaceMode;
  path: string;
  onModeChange: (mode: WorkspaceMode) => void;
  onPathChange: (path: string) => void;
}

/**
 * Radio group for selecting workspace mode: "New workspace" (isolated) or
 * "Existing project" (existing). When "Existing project" is selected, a text
 * input for the directory path is shown with validation.
 *
 * Validates: Requirements 5.4, 5.5
 */
export function WorkspaceModeSelector({
  mode,
  path,
  onModeChange,
  onPathChange,
}: WorkspaceModeSelectorProps) {
  const pathError = getPathError(mode, path);

  return (
    <fieldset className="space-y-3">
      <legend className="block text-sm font-medium text-gray-700">
        Workspace
      </legend>

      <div className="grid grid-cols-2 gap-3">
        <WorkspaceModeCard
          id="workspace-mode-isolated"
          label="New workspace"
          description="Creates an isolated directory for this task"
          isSelected={mode === "isolated"}
          onSelect={() => onModeChange("isolated")}
        />
        <WorkspaceModeCard
          id="workspace-mode-existing"
          label="Existing project"
          description="Run in an existing local project directory"
          isSelected={mode === "existing"}
          onSelect={() => onModeChange("existing")}
        />
      </div>

      {mode === "existing" && (
        <div>
          <label
            htmlFor="workspace-path"
            className="block text-sm font-medium text-gray-700"
          >
            Directory path <span className="text-red-500">*</span>
          </label>
          <input
            id="workspace-path"
            type="text"
            value={path}
            onChange={(e: ChangeEvent<HTMLInputElement>) =>
              onPathChange(e.target.value)
            }
            placeholder="/home/user/projects/my-app"
            className={`mt-1 w-full rounded-lg border px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-2 ${
              pathError
                ? "border-red-300 focus:border-red-300 focus:ring-red-100"
                : "border-gray-200 focus:border-blue-300 focus:ring-blue-100"
            }`}
            aria-required="true"
            aria-invalid={!!pathError}
            aria-describedby={pathError ? "workspace-path-error" : undefined}
          />
          {pathError && (
            <p
              id="workspace-path-error"
              className="mt-1 text-xs text-red-600"
              role="alert"
            >
              {pathError}
            </p>
          )}
        </div>
      )}
    </fieldset>
  );
}

/* ─── Validation ─── */

/**
 * Returns an error message if the path is invalid for "existing" mode,
 * or null if valid.
 */
export function getPathError(
  mode: WorkspaceMode,
  path: string
): string | null {
  if (mode !== "existing") return null;
  if (!path.trim()) return "Directory path is required";
  if (!path.startsWith("/")) return "Path must be absolute (start with /)";
  return null;
}

/* ─── Internal Components ─── */

function WorkspaceModeCard({
  id,
  label,
  description,
  isSelected,
  onSelect,
}: {
  id: string;
  label: string;
  description: string;
  isSelected: boolean;
  onSelect: () => void;
}) {
  return (
    <label
      htmlFor={id}
      className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors ${
        isSelected
          ? "border-blue-300 bg-blue-50/50 ring-1 ring-blue-200"
          : "border-gray-200 bg-white hover:bg-gray-50"
      }`}
    >
      <input
        id={id}
        type="radio"
        name="workspace-mode"
        checked={isSelected}
        onChange={onSelect}
        className="mt-0.5 h-4 w-4 border-gray-300 text-blue-600 focus:ring-blue-500"
        aria-label={label}
      />
      <div>
        <span className="block text-sm font-medium text-gray-900">
          {label}
        </span>
        <span className="block text-xs text-gray-500 mt-0.5">
          {description}
        </span>
      </div>
    </label>
  );
}
