import { useState, useCallback, useRef } from "react";

interface CopyButtonProps {
  text: string;
  hasActiveFilters: boolean;
}

/**
 * Button that copies formatted timeline text to clipboard.
 * Shows "Copied ✓" confirmation for 2 seconds after successful copy.
 * Label changes to "Copy Filtered" when filters are active.
 *
 * Validates: Requirements 9.1, 9.2, 9.3, 9.4
 */
export function CopyButton({ text, hasActiveFilters }: CopyButtonProps) {
  const [status, setStatus] = useState<"idle" | "copied" | "error">("idle");
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const handleCopy = useCallback(async () => {
    // Clear any existing timeout
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }

    try {
      await navigator.clipboard.writeText(text);
      setStatus("copied");
    } catch {
      setStatus("error");
    }

    // Reset after 2 seconds
    timeoutRef.current = setTimeout(() => {
      setStatus("idle");
      timeoutRef.current = null;
    }, 2000);
  }, [text]);

  const label = (() => {
    if (status === "copied") return "Copied ✓";
    if (status === "error") return "Copy failed";
    return hasActiveFilters ? "Copy Filtered" : "Copy All";
  })();

  return (
    <button
      type="button"
      className={`inline-flex items-center rounded border px-3 py-1.5 text-sm font-medium transition-colors ${
        status === "copied"
          ? "border-green-300 bg-green-50 text-green-700"
          : status === "error"
            ? "border-red-300 bg-red-50 text-red-700"
            : "border-gray-300 bg-white text-gray-700 hover:bg-gray-50"
      }`}
      onClick={handleCopy}
      disabled={status === "copied"}
    >
      {label}
    </button>
  );
}
