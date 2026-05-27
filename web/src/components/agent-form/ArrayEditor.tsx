import { useState } from "react";

interface ArrayEditorProps {
  label?: string;
  value: string[];
  onChange: (value: string[]) => void;
  placeholder?: string;
  error?: string;
}

/**
 * ArrayEditor — editable list of string items.
 *
 * Shows each item with a remove button and an input to add new items.
 */
export function ArrayEditor({
  label = "Custom Arguments",
  value,
  onChange,
  placeholder = "--flag or value",
  error,
}: ArrayEditorProps) {
  const [newItem, setNewItem] = useState("");

  const handleAdd = () => {
    const trimmed = newItem.trim();
    if (!trimmed) return;
    onChange([...value, trimmed]);
    setNewItem("");
  };

  const handleRemove = (index: number) => {
    onChange(value.filter((_, i) => i !== index));
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

      {/* Existing items */}
      {value.length > 0 && (
        <div className="space-y-1.5">
          {value.map((item, index) => (
            <div key={index} className="flex items-center gap-2">
              <input
                type="text"
                value={item}
                onChange={(e) => {
                  const updated = [...value];
                  updated[index] = e.target.value;
                  onChange(updated);
                }}
                className="flex-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500"
                aria-label={`Argument ${index + 1}`}
              />
              <button
                type="button"
                onClick={() => handleRemove(index)}
                className="flex-shrink-0 rounded p-1 text-gray-400 hover:text-red-600 hover:bg-red-50 focus:outline-none focus:ring-2 focus:ring-red-500"
                aria-label={`Remove argument ${index + 1}`}
              >
                <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Add new item */}
      <div className="flex items-center gap-2">
        <input
          type="text"
          value={newItem}
          onChange={(e) => setNewItem(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          className="flex-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500"
          aria-label="New argument"
        />
        <button
          type="button"
          onClick={handleAdd}
          disabled={!newItem.trim()}
          className="flex-shrink-0 rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-blue-500"
          aria-label="Add argument"
        >
          Add
        </button>
      </div>

      {error && (
        <p className="text-sm text-red-600">{error}</p>
      )}
    </div>
  );
}
