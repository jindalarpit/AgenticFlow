import { useState } from "react";

interface KeyValueEditorProps {
  label?: string;
  value: Record<string, string>;
  onChange: (value: Record<string, string>) => void;
  maxPairs?: number;
  error?: string;
}

/**
 * KeyValueEditor — editable list of key-value pairs.
 *
 * Accepts a Record<string, string>, renders each pair with key/value inputs
 * and a remove button. Provides an "Add" row for new pairs.
 * Prevents adding beyond maxPairs (default 20).
 */
export function KeyValueEditor({
  label = "Environment Variables",
  value,
  onChange,
  maxPairs = 20,
  error,
}: KeyValueEditorProps) {
  const [newKey, setNewKey] = useState("");
  const [newValue, setNewValue] = useState("");

  const entries = Object.entries(value);
  const atLimit = entries.length >= maxPairs;

  const handleAdd = () => {
    const trimmedKey = newKey.trim();
    if (!trimmedKey || atLimit) return;
    onChange({ ...value, [trimmedKey]: newValue });
    setNewKey("");
    setNewValue("");
  };

  const handleRemove = (key: string) => {
    const { [key]: _, ...rest } = value;
    onChange(rest);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      handleAdd();
    }
  };

  return (
    <div className="space-y-2">
      <label className="block text-sm font-medium text-gray-700">{label}</label>

      {/* Existing pairs */}
      {entries.length > 0 && (
        <div className="space-y-2">
          {entries.map(([k, v]) => (
            <div key={k} className="flex items-center gap-2">
              <input
                type="text"
                value={k}
                readOnly
                className="flex-1 rounded-md border border-gray-200 bg-gray-50 px-3 py-1.5 text-sm text-gray-700"
                aria-label="Key"
              />
              <input
                type="text"
                value={v}
                onChange={(e) => onChange({ ...value, [k]: e.target.value })}
                className="flex-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                aria-label={`Value for ${k}`}
              />
              <button
                type="button"
                onClick={() => handleRemove(k)}
                className="flex-shrink-0 rounded p-1 text-gray-400 hover:text-red-600 hover:bg-red-50 focus:outline-none focus:ring-2 focus:ring-red-500"
                aria-label={`Remove ${k}`}
              >
                <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Add new pair */}
      {!atLimit && (
        <div className="flex items-center gap-2">
          <input
            type="text"
            value={newKey}
            onChange={(e) => setNewKey(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="KEY"
            className="flex-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            aria-label="New key"
          />
          <input
            type="text"
            value={newValue}
            onChange={(e) => setNewValue(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="value"
            className="flex-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            aria-label="New value"
          />
          <button
            type="button"
            onClick={handleAdd}
            disabled={!newKey.trim()}
            className="flex-shrink-0 rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-blue-500"
            aria-label="Add pair"
          >
            Add
          </button>
        </div>
      )}

      {atLimit && (
        <p className="text-xs text-amber-600">
          Maximum {maxPairs} pairs reached.
        </p>
      )}

      {error && (
        <p className="text-sm text-red-600">{error}</p>
      )}
    </div>
  );
}
