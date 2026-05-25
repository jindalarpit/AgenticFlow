import { useState } from "react";
import type { TimelineItem } from "../../lib/tool-chain-parser";

interface ToolResultCardProps {
  item: TimelineItem;
}

const SUMMARY_LENGTH = 200;
const MAX_OUTPUT_LENGTH = 4000;

/**
 * Card for tool_result timeline items.
 * Collapsed: first 200 chars of output as summary.
 * Expanded: full output in scrollable code block (truncated at 4000 chars with "(Content truncated)" indicator).
 * Chevron rotates 90° clockwise when expanded.
 */
export function ToolResultCard({ item }: ToolResultCardProps) {
  const [expanded, setExpanded] = useState(false);

  const output = item.output || "";
  const isLong = output.length > SUMMARY_LENGTH;
  const preview = isLong ? output.slice(0, SUMMARY_LENGTH) + "…" : output;

  const isTruncated = output.length > MAX_OUTPUT_LENGTH;
  const displayOutput = isTruncated
    ? output.slice(0, MAX_OUTPUT_LENGTH) + "\n\n(Content truncated)"
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

        {/* Preview (first 200 chars of output) */}
        <span className="flex-1 min-w-0 text-sm text-gray-600 truncate font-mono">
          {preview || "(empty output)"}
        </span>

        {/* Chevron: rotates 90° clockwise when expanded */}
        <svg
          className={`w-4 h-4 text-gray-400 flex-shrink-0 transition-transform duration-200 ${expanded ? "rotate-90" : ""}`}
          viewBox="0 0 20 20"
          fill="currentColor"
          aria-hidden="true"
        >
          <path
            fillRule="evenodd"
            d="M7.21 14.77a.75.75 0 01.02-1.06L11.168 10 7.23 6.29a.75.75 0 111.04-1.08l4.5 4.25a.75.75 0 010 1.08l-4.5 4.25a.75.75 0 01-1.06-.02z"
            clipRule="evenodd"
          />
        </svg>
      </button>

      {/* Expanded: full output (truncated at 4000 chars) */}
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
