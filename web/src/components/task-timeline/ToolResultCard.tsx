import { useState } from "react";
import type { TimelineItem } from "../../lib/tool-chain-parser";

interface ToolResultCardProps {
  item: TimelineItem;
}

const PREVIEW_LENGTH = 120;
const MAX_OUTPUT_LENGTH = 4000;

/**
 * Card for tool_result timeline items.
 * Collapsed: truncated output preview (first 120 chars).
 * Expanded: full output in scrollable code block (max 4000 chars with truncation indicator).
 */
export function ToolResultCard({ item }: ToolResultCardProps) {
  const [expanded, setExpanded] = useState(false);

  const output = item.output || "";
  const isLong = output.length > PREVIEW_LENGTH;
  const preview = isLong ? output.slice(0, PREVIEW_LENGTH) + "…" : output;

  const displayOutput =
    output.length > MAX_OUTPUT_LENGTH
      ? output.slice(0, MAX_OUTPUT_LENGTH) + "\n\n... truncated"
      : output;

  return (
    <div
      className="rounded-lg border border-gray-200 bg-white hover:border-gray-300 transition-colors"
      role="article"
      aria-expanded={expanded}
    >
      <button
        type="button"
        className="w-full flex items-center gap-2 px-3 py-2 text-left cursor-pointer"
        onClick={() => setExpanded((prev) => !prev)}
        aria-label={`${expanded ? "Collapse" : "Expand"} tool result ${item.seq + 1}`}
      >
        {/* Sequence number */}
        <span className="text-[10px] font-mono text-gray-400 min-w-[2ch] text-right">
          {item.seq + 1}
        </span>

        {/* Type badge */}
        <span className="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium bg-gray-200 text-gray-700">
          Result
        </span>

        {/* Preview */}
        <span className="flex-1 min-w-0 text-sm text-gray-600 truncate font-mono">
          {preview || "(empty output)"}
        </span>

        {/* Expand/collapse indicator */}
        <span className="text-gray-400 text-xs flex-shrink-0">
          {expanded ? "▾" : "▸"}
        </span>
      </button>

      {/* Expanded: full output */}
      {expanded && (
        <div className="px-3 pb-3 pt-1 border-t border-gray-100">
          <pre className="overflow-auto max-h-96 rounded bg-gray-50 p-3 text-xs font-mono text-gray-800 whitespace-pre-wrap">
            <code>{displayOutput || "(empty output)"}</code>
          </pre>
        </div>
      )}
    </div>
  );
}
