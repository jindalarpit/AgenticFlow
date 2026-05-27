import { useState, useEffect, useCallback, useRef } from "react";
import type { Agent, EditableTabProps } from "../../lib/agent-detail-types";
import { useToast } from "../Toast";

/* ─── Constants ─── */

const MCP_EXAMPLE = `{
  "filesystem": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path"],
    "env": {}
  }
}`;

/* ─── Props ─── */

interface ToolsTabProps extends EditableTabProps {
  agent: Agent;
  isOwner: boolean;
  onDirtyChange: (dirty: boolean) => void;
  onSave: (data: Partial<Agent>) => Promise<void>;
}

/* ─── Component ─── */

/**
 * Tools tab in the Overview Pane.
 * JSON editor for the agent's MCP (Model Context Protocol) configuration.
 *
 * Features:
 * - Displays current mcp_config pretty-printed, or empty {} placeholder
 * - Validates JSON syntax on save, shows inline error for invalid JSON
 * - Save button: parses JSON, calls onSave with { mcp_config: parsed }
 * - Clear button: sets mcp_config to null
 * - Brief explanation of MCP config format with example
 * - Reports dirty state to parent via onDirtyChange
 *
 * Validates: Requirements 11.1, 11.2, 11.3, 11.4, 11.5, 11.6
 */
export function ToolsTab({
  agent,
  isOwner,
  onDirtyChange,
  onSave,
}: ToolsTabProps) {
  const { showToast } = useToast();

  const formatConfig = (config: Record<string, unknown> | null): string => {
    if (!config || Object.keys(config).length === 0) return "{}";
    return JSON.stringify(config, null, 2);
  };

  const originalRef = useRef(formatConfig(agent.mcp_config));
  const [value, setValue] = useState(formatConfig(agent.mcp_config));
  const [jsonError, setJsonError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);

  // Sync when agent data loads or changes externally
  useEffect(() => {
    const incoming = formatConfig(agent.mcp_config);
    originalRef.current = incoming;
    setValue(incoming);
    setJsonError(null);
  }, [agent.mcp_config]);

  // Derived state
  const isDirty = value !== originalRef.current;

  // Report dirty state to parent
  useEffect(() => {
    onDirtyChange(isDirty);
  }, [isDirty, onDirtyChange]);

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLTextAreaElement>) => {
      setValue(e.target.value);
      // Clear error as user types
      setJsonError(null);
    },
    []
  );

  const handleSave = useCallback(async () => {
    // Validate JSON syntax
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(value);
    } catch (err) {
      const message =
        err instanceof SyntaxError ? err.message : "Invalid JSON";
      setJsonError(message);
      return;
    }

    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
      setJsonError("MCP config must be a JSON object");
      return;
    }

    setIsSaving(true);
    try {
      await onSave({ mcp_config: parsed });
      originalRef.current = value;
      setJsonError(null);
      showToast("MCP config saved successfully", "success");
    } catch {
      showToast("Failed to save MCP config", "error");
    } finally {
      setIsSaving(false);
    }
  }, [value, onSave, showToast]);

  const handleClear = useCallback(async () => {
    setIsSaving(true);
    try {
      await onSave({ mcp_config: null });
      const empty = "{}";
      setValue(empty);
      originalRef.current = empty;
      setJsonError(null);
      showToast("MCP config cleared", "success");
    } catch {
      showToast("Failed to clear MCP config", "error");
    } finally {
      setIsSaving(false);
    }
  }, [onSave, showToast]);

  return (
    <div className="flex flex-col h-full gap-4">
      {/* Explanation section */}
      <div className="rounded-md border border-gray-200 bg-gray-50 p-3">
        <p className="text-sm text-gray-700 mb-2">
          Configure MCP (Model Context Protocol) servers to give this agent
          access to external tools.
        </p>
        <details className="text-xs text-gray-600">
          <summary className="cursor-pointer font-medium text-gray-700 hover:text-gray-900">
            Example configuration
          </summary>
          <pre className="mt-2 rounded bg-gray-100 p-2 font-mono text-xs overflow-x-auto">
            {MCP_EXAMPLE}
          </pre>
        </details>
      </div>

      {/* Action buttons */}
      {isOwner && (
        <div className="flex items-center justify-between gap-2 flex-shrink-0">
          <div className="flex items-center gap-2">
            {isDirty && (
              <span className="text-xs text-amber-600 font-medium">
                Unsaved changes
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={handleClear}
              disabled={isSaving}
              className="rounded-md border border-gray-300 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
            >
              Clear
            </button>
            <button
              type="button"
              onClick={handleSave}
              disabled={isSaving}
              className="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {isSaving ? "Saving…" : "Save"}
            </button>
          </div>
        </div>
      )}

      {/* JSON editor textarea */}
      <textarea
        value={value}
        onChange={handleChange}
        readOnly={!isOwner}
        placeholder="{}"
        spellCheck={false}
        className={`flex-1 min-h-[250px] w-full resize-none rounded-md border p-3 font-mono text-sm leading-relaxed transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 ${
          !isOwner
            ? "bg-gray-50 text-gray-600 cursor-default"
            : "bg-white text-gray-900"
        } ${jsonError ? "border-red-300" : "border-gray-300"}`}
        aria-label="MCP configuration JSON editor"
        aria-invalid={!!jsonError}
        aria-describedby={jsonError ? "mcp-json-error" : undefined}
      />

      {/* Inline JSON error */}
      {jsonError && (
        <p
          id="mcp-json-error"
          className="text-xs text-red-600 -mt-2"
          role="alert"
        >
          Invalid JSON: {jsonError}
        </p>
      )}
    </div>
  );
}
