import { useState, useCallback } from "react";

interface McpConfigEditorProps {
  value: Record<string, unknown> | null;
  onChange: (value: Record<string, unknown> | null) => void;
  error?: string;
}

/**
 * McpConfigEditor — JSON textarea with syntax validation on blur.
 *
 * Stores the raw text internally and validates on blur.
 * Shows error for invalid JSON. Empty textarea sets value to null.
 */
export function McpConfigEditor({ value, onChange, error }: McpConfigEditorProps) {
  const [text, setText] = useState<string>(
    value ? JSON.stringify(value, null, 2) : ""
  );
  const [localError, setLocalError] = useState<string | undefined>(undefined);

  const handleBlur = useCallback(() => {
    const trimmed = text.trim();

    // Empty → null
    if (!trimmed) {
      setLocalError(undefined);
      onChange(null);
      return;
    }

    // Validate JSON
    try {
      const parsed = JSON.parse(trimmed);
      if (typeof parsed !== "object" || Array.isArray(parsed) || parsed === null) {
        setLocalError("MCP config must be a JSON object (not array or primitive)");
        return;
      }
      setLocalError(undefined);
      onChange(parsed);
    } catch {
      setLocalError("Invalid JSON syntax");
    }
  }, [text, onChange]);

  const displayError = error || localError;

  return (
    <div className="space-y-1">
      <label htmlFor="mcp-config" className="block text-sm font-medium text-gray-700">
        MCP Config
      </label>
      <textarea
        id="mcp-config"
        value={text}
        onChange={(e) => setText(e.target.value)}
        onBlur={handleBlur}
        rows={6}
        placeholder='{"mcpServers": { ... }}'
        className={`block w-full rounded-md border px-3 py-2 text-sm font-mono shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 ${
          displayError
            ? "border-red-300 focus:ring-red-500"
            : "border-gray-300"
        }`}
        aria-invalid={!!displayError}
        aria-describedby={displayError ? "mcp-config-error" : undefined}
      />
      {displayError && (
        <p id="mcp-config-error" className="text-sm text-red-600">
          {displayError}
        </p>
      )}
      <p className="text-xs text-gray-500">
        Optional JSON configuration for MCP servers. Leave empty for none.
      </p>
    </div>
  );
}
