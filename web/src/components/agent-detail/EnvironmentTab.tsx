import { useState, useEffect, useCallback } from "react";
import type { EditableTabProps } from "../../lib/agent-detail-types";
import { filterEmptyKeys, hasDuplicateKeys } from "../../lib/agent-utils";
import { useToast } from "../Toast";

/* ─── Types ─── */

interface EnvEntry {
  id: number;
  key: string;
  value: string;
  visible: boolean;
}

/* ─── Constants ─── */

const MAX_ENTRIES = 20;

/* ─── Helpers ─── */

let nextId = 0;

function envMapToEntries(env: Record<string, string>): EnvEntry[] {
  return Object.entries(env).map(([key, value]) => ({
    id: nextId++,
    key,
    value,
    visible: false,
  }));
}

/**
 * EnvironmentTab — Key-value editor for agent custom environment variables.
 *
 * Features:
 * - Key-value input rows (max 20)
 * - Add button (disabled at 20)
 * - Delete button per row
 * - Values masked by default with show/hide toggle per row
 * - "Unsaved changes" indicator + Save button when modified
 * - Save excludes empty keys, validates no duplicate trimmed keys (error toast)
 * - Reports dirty state to parent via onDirtyChange callback
 * - Read-only mode for non-owners
 *
 * Validates: Requirements 14.1, 14.2, 14.3, 14.4, 14.5, 14.6, 14.7, 14.8, 14.9, 14.10
 */
export function EnvironmentTab({ agent, isOwner, onDirtyChange, onSave }: EditableTabProps) {
  const { showToast } = useToast();
  const [entries, setEntries] = useState<EnvEntry[]>(() =>
    envMapToEntries(agent.custom_env ?? {})
  );
  const [saving, setSaving] = useState(false);

  // Compute dirty state by comparing current filtered map to original
  const isDirty = useCallback(() => {
    const currentMap = filterEmptyKeys(entries);
    const originalMap = agent.custom_env ?? {};
    return JSON.stringify(currentMap) !== JSON.stringify(originalMap);
  }, [entries, agent.custom_env]);

  const dirty = isDirty();

  // Report dirty state to parent
  useEffect(() => {
    onDirtyChange(dirty);
  }, [dirty, onDirtyChange]);

  /* ─── Entry Mutations ─── */

  const addEntry = () => {
    if (entries.length >= MAX_ENTRIES) return;
    setEntries((prev) => [
      ...prev,
      { id: nextId++, key: "", value: "", visible: true },
    ]);
  };

  const removeEntry = (index: number) => {
    setEntries((prev) => prev.filter((_, i) => i !== index));
  };

  const updateEntry = (index: number, field: "key" | "value", val: string) => {
    setEntries((prev) =>
      prev.map((entry, i) => (i === index ? { ...entry, [field]: val } : entry))
    );
  };

  const toggleVisibility = (index: number) => {
    setEntries((prev) =>
      prev.map((entry, i) =>
        i === index ? { ...entry, visible: !entry.visible } : entry
      )
    );
  };

  /* ─── Save Handler ─── */

  const handleSave = async () => {
    // Check for duplicate keys (after trimming)
    const nonEmptyEntries = entries.filter((e) => e.key.trim().length > 0);
    if (hasDuplicateKeys(nonEmptyEntries)) {
      showToast("Duplicate environment variable keys detected. Please use unique keys.", "error");
      return;
    }

    setSaving(true);
    try {
      const filteredMap = filterEmptyKeys(entries);
      await onSave({ custom_env: filteredMap });
    } catch (err) {
      const message =
        err instanceof Error && err.message
          ? err.message
          : "Failed to save environment variables";
      showToast(message, "error");
    } finally {
      setSaving(false);
    }
  };

  /* ─── Read-Only Mode ─── */

  if (!isOwner) {
    return (
      <div className="flex flex-col gap-4">
        <p className="text-xs text-gray-500">
          Environment variables are read-only for non-owners.
        </p>
        {entries.length > 0 ? (
          <div className="flex flex-col gap-2">
            {entries.map((entry) => (
              <div key={entry.id} className="flex items-center gap-2">
                <input
                  type="text"
                  value={entry.key}
                  readOnly
                  disabled
                  className="w-[40%] rounded-md border border-gray-200 bg-gray-50 px-3 py-1.5 text-xs font-mono text-gray-700"
                  aria-label="Environment variable key"
                />
                <input
                  type="password"
                  value="••••••••"
                  readOnly
                  disabled
                  className="flex-1 rounded-md border border-gray-200 bg-gray-50 px-3 py-1.5 text-xs font-mono text-gray-500"
                  aria-label="Environment variable value (masked)"
                />
              </div>
            ))}
          </div>
        ) : (
          <p className="text-xs italic text-gray-400">
            No environment variables configured.
          </p>
        )}
      </div>
    );
  }

  /* ─── Editable Mode ─── */

  return (
    <div className="flex flex-col gap-4">
      {/* Header with Add button */}
      <div className="flex items-start justify-between gap-3">
        <p className="text-xs text-gray-500">
          Set custom environment variables passed to the agent runtime during task execution.
          Values are masked by default for security.
        </p>
        <button
          type="button"
          onClick={addEntry}
          disabled={entries.length >= MAX_ENTRIES}
          className="shrink-0 inline-flex items-center gap-1 rounded-md border border-gray-300 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
          aria-label="Add environment variable"
        >
          <span aria-hidden="true">+</span> Add
        </button>
      </div>

      {/* Entry Rows */}
      {entries.length > 0 && (
        <div className="flex flex-col gap-2">
          {entries.map((entry, index) => (
            <div key={entry.id} className="flex items-center gap-2">
              {/* Key input */}
              <input
                type="text"
                value={entry.key}
                onChange={(e) => updateEntry(index, "key", e.target.value)}
                placeholder="KEY"
                className="w-[40%] rounded-md border border-gray-300 px-3 py-1.5 text-xs font-mono focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                aria-label={`Environment variable key ${index + 1}`}
              />

              {/* Value input with visibility toggle */}
              <div className="relative flex-1">
                <input
                  type={entry.visible ? "text" : "password"}
                  value={entry.value}
                  onChange={(e) => updateEntry(index, "value", e.target.value)}
                  placeholder="value"
                  className="w-full rounded-md border border-gray-300 px-3 py-1.5 pr-8 text-xs font-mono focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                  aria-label={`Environment variable value ${index + 1}`}
                />
                <button
                  type="button"
                  onClick={() => toggleVisibility(index)}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 text-xs"
                  aria-label={entry.visible ? "Hide value" : "Show value"}
                >
                  {entry.visible ? "Hide" : "Show"}
                </button>
              </div>

              {/* Delete button */}
              <button
                type="button"
                onClick={() => removeEntry(index)}
                className="shrink-0 rounded-md p-1.5 text-gray-400 hover:text-red-600 hover:bg-red-50"
                aria-label={`Remove environment variable ${index + 1}`}
              >
                <svg
                  className="h-4 w-4"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                  aria-hidden="true"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                  />
                </svg>
              </button>
            </div>
          ))}
        </div>
      )}

      {entries.length === 0 && (
        <p className="text-xs italic text-gray-400 py-4 text-center">
          No environment variables configured. Click "Add" to create one.
        </p>
      )}

      {/* Footer: Unsaved indicator + Save button */}
      <div className="flex items-center justify-end gap-3 pt-2 border-t border-gray-100">
        {dirty && (
          <span className="text-xs text-amber-600">Unsaved changes</span>
        )}
        <button
          type="button"
          onClick={handleSave}
          disabled={!dirty || saving}
          className="inline-flex items-center gap-1.5 rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {saving ? "Saving…" : "Save"}
        </button>
      </div>
    </div>
  );
}
