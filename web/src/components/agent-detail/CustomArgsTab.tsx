import { useState, useEffect, useCallback } from "react";
import type { EditableTabProps } from "../../lib/agent-detail-types";
import { splitArgs } from "../../lib/agent-utils";
import { useToast } from "../Toast";

/* ─── Component ─── */

/**
 * Custom Args tab in the Overview Pane.
 * Provides an array editor for the agent's custom CLI arguments.
 *
 * Features:
 * - Text input row per argument
 * - Add button to append empty row
 * - Delete button per row
 * - Dirty state tracking (reports to parent via onDirtyChange)
 * - Save splits space-separated tokens via splitArgs, flattens into single array
 * - Success toast on save
 * - Read-only mode for non-owners: inputs disabled, no Add/Delete/Save
 *
 * Validates: Requirements 15.1, 15.2, 15.3, 15.4, 15.5, 15.6, 15.7, 15.8
 */
export function CustomArgsTab({ agent, isOwner, onDirtyChange, onSave }: EditableTabProps) {
  const { showToast } = useToast();
  const [rows, setRows] = useState<string[]>(() =>
    agent.custom_args.length > 0 ? [...agent.custom_args] : []
  );
  const [original, setOriginal] = useState<string[]>(() => [...agent.custom_args]);
  const [saving, setSaving] = useState(false);

  /* ─── Dirty Detection ─── */

  const isDirty = useCallback(() => {
    if (rows.length !== original.length) return true;
    return rows.some((row, i) => row !== original[i]);
  }, [rows, original]);

  useEffect(() => {
    onDirtyChange(isDirty());
  }, [rows, isDirty, onDirtyChange]);

  /* ─── Sync with agent prop changes (e.g., after external save) ─── */

  useEffect(() => {
    const incoming = agent.custom_args;
    setRows(incoming.length > 0 ? [...incoming] : []);
    setOriginal([...incoming]);
  }, [agent.custom_args]);

  /* ─── Handlers ─── */

  const handleRowChange = (index: number, value: string) => {
    setRows((prev) => {
      const next = [...prev];
      next[index] = value;
      return next;
    });
  };

  const handleAdd = () => {
    setRows((prev) => [...prev, ""]);
  };

  const handleDelete = (index: number) => {
    setRows((prev) => prev.filter((_, i) => i !== index));
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      // For each row, split space-separated tokens, then flatten all into a single array
      const flattenedArgs = rows.flatMap((row) => splitArgs(row));
      await onSave({ custom_args: flattenedArgs });
      showToast("Custom arguments saved", "success");
      // Update original to match saved state
      setRows(flattenedArgs.length > 0 ? [...flattenedArgs] : []);
      setOriginal([...flattenedArgs]);
    } catch {
      showToast("Failed to save custom arguments", "error");
    } finally {
      setSaving(false);
    }
  };

  const dirty = isDirty();

  /* ─── Read-Only Mode ─── */

  if (!isOwner) {
    return (
      <div className="flex flex-col gap-3">
        <h3 className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
          Custom Arguments
        </h3>
        {rows.length === 0 ? (
          <p className="text-sm text-muted-foreground">No custom arguments configured</p>
        ) : (
          <ul className="flex flex-col gap-2" role="list">
            {rows.map((row, index) => (
              <li key={index} className="flex items-center">
                <span className="w-full rounded-md border bg-gray-50 px-3 py-2 text-sm text-gray-700">
                  {row}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    );
  }

  /* ─── Editable Mode ─── */

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h3 className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
          Custom Arguments
        </h3>
        <div className="flex items-center gap-2">
          {dirty && (
            <span className="text-xs text-amber-600" aria-live="polite">
              Unsaved changes
            </span>
          )}
          <button
            type="button"
            onClick={handleSave}
            disabled={!dirty || saving}
            className="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
            aria-label="Save custom arguments"
          >
            {saving ? "Saving…" : "Save"}
          </button>
        </div>
      </div>

      {/* Argument rows */}
      <div className="flex flex-col gap-2">
        {rows.map((row, index) => (
          <div key={index} className="flex items-center gap-2">
            <input
              type="text"
              value={row}
              onChange={(e) => handleRowChange(index, e.target.value)}
              placeholder="e.g., --flag value"
              className="flex-1 rounded-md border px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              aria-label={`Argument ${index + 1}`}
            />
            <button
              type="button"
              onClick={() => handleDelete(index)}
              className="rounded-md border border-red-200 px-2 py-2 text-xs text-red-600 hover:bg-red-50"
              aria-label={`Delete argument ${index + 1}`}
            >
              ✕
            </button>
          </div>
        ))}
      </div>

      {/* Add button */}
      <button
        type="button"
        onClick={handleAdd}
        className="self-start rounded-md border border-dashed border-gray-300 px-3 py-1.5 text-xs text-gray-600 hover:border-gray-400 hover:text-gray-800"
        aria-label="Add argument"
      >
        + Add
      </button>
    </div>
  );
}
