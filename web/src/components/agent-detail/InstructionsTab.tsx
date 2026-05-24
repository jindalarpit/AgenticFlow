import { useState, useEffect, useCallback, useRef } from "react";
import type { Agent, EditableTabProps } from "../../lib/agent-detail-types";
import { useToast } from "../Toast";

/* ─── Constants ─── */

const MAX_CHARS = 50_000;

/* ─── Props ─── */

interface InstructionsTabProps extends EditableTabProps {
  agent: Agent;
  isOwner: boolean;
  onDirtyChange: (dirty: boolean) => void;
  onSave: (data: Partial<Agent>) => Promise<void>;
}

/* ─── Component ─── */

/**
 * Instructions tab in the Overview Pane.
 * Full-height monospace text editor for the agent's system prompt.
 *
 * Features:
 * - Tracks dirty state (compares current text to original)
 * - Reports dirty state to parent via onDirtyChange
 * - "Unsaved changes" indicator when dirty
 * - Save button: enabled when dirty AND under 50k chars
 * - 50,000 char limit: disables Save + shows red warning when exceeded
 * - Save: calls onSave({ instructions: value }), shows success toast, clears dirty
 * - Error: shows error toast, preserves content, keeps dirty
 * - Read-only mode for non-owners: textarea is readOnly, Save button hidden
 * - Disabled Save while loading (initial render before agent data is available)
 *
 * Validates: Requirements 12.1, 12.2, 12.3, 12.4, 12.5, 12.6, 12.7, 12.8
 */
export function InstructionsTab({
  agent,
  isOwner,
  onDirtyChange,
  onSave,
}: InstructionsTabProps) {
  const { showToast } = useToast();

  // The "original" value is what was last saved / loaded from the server
  const originalRef = useRef(agent.instructions ?? "");
  const [value, setValue] = useState(agent.instructions ?? "");
  const [isSaving, setIsSaving] = useState(false);
  const [isLoaded, setIsLoaded] = useState(false);

  // Sync when agent data loads or changes externally (e.g., after save)
  useEffect(() => {
    const incoming = agent.instructions ?? "";
    originalRef.current = incoming;
    setValue(incoming);
    setIsLoaded(true);
  }, [agent.instructions]);

  // Derived state
  const isDirty = value !== originalRef.current;
  const isOverLimit = value.length > MAX_CHARS;
  const canSave = isDirty && !isOverLimit && !isSaving && isLoaded;

  // Report dirty state to parent whenever it changes
  useEffect(() => {
    onDirtyChange(isDirty);
  }, [isDirty, onDirtyChange]);

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLTextAreaElement>) => {
      setValue(e.target.value);
    },
    []
  );

  const handleSave = useCallback(async () => {
    if (!canSave) return;

    setIsSaving(true);
    try {
      await onSave({ instructions: value });
      // Update the original ref so dirty state clears
      originalRef.current = value;
      showToast("Instructions saved successfully", "success");
    } catch {
      showToast("Failed to save instructions", "error");
    } finally {
      setIsSaving(false);
    }
  }, [canSave, value, onSave, showToast]);

  return (
    <div className="flex flex-col h-full gap-3">
      {/* Header row: unsaved indicator + save button */}
      {isOwner && (
        <div className="flex items-center justify-between gap-2 flex-shrink-0">
          <div className="flex items-center gap-2">
            {isDirty && (
              <span className="text-xs text-amber-600 font-medium">
                Unsaved changes
              </span>
            )}
            {isOverLimit && (
              <span className="text-xs text-red-600 font-medium">
                Character limit exceeded ({value.length.toLocaleString()}/{MAX_CHARS.toLocaleString()})
              </span>
            )}
          </div>
          <button
            type="button"
            onClick={handleSave}
            disabled={!canSave}
            className="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {isSaving ? "Saving…" : "Save"}
          </button>
        </div>
      )}

      {/* Monospace textarea filling available height */}
      <textarea
        value={value}
        onChange={handleChange}
        readOnly={!isOwner}
        placeholder={isOwner ? "Enter agent instructions…" : "No instructions configured"}
        className={`flex-1 min-h-[300px] w-full resize-none rounded-md border p-3 font-mono text-sm leading-relaxed transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 ${
          !isOwner
            ? "bg-gray-50 text-gray-600 cursor-default"
            : "bg-white text-gray-900"
        } ${isOverLimit ? "border-red-300" : "border-gray-300"}`}
        aria-label="Agent instructions editor"
      />
    </div>
  );
}
