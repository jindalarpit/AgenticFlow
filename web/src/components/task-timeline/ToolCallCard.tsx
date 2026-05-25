import { useState } from "react";
import type { TimelineItem } from "../../lib/tool-chain-parser";
import { deriveSummary } from "../../lib/tool-chain-parser";

interface ToolCallCardProps {
  item: TimelineItem;
}

/**
 * Card for tool_use timeline items.
 * Collapsed: tool name badge + derived summary (max 120 chars via deriveSummary).
 * Expanded: JSON input in formatted code block.
 * Chevron rotates 90° clockwise when expanded.
 */
export function ToolCallCard({ item }: ToolCallCardProps) {
  const [expanded, setExpanded] = useState(false);

  const toolName = item.tool || "unknown";
  const summary = deriveSummary(item);

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
        aria-label={`${expanded ? "Collapse" : "Expand"} tool call ${item.seq + 1}`}
      >
        {/* Sequence number */}
        <span className="text-[10px] font-mono text-gray-400 min-w-[2ch] text-right">
          {item.seq + 1}
        </span>

        {/* Type badge */}
        <span className="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium bg-blue-500/20 text-blue-700">
          {toolName}
        </span>

        {/* Summary (max 120 chars enforced by deriveSummary) */}
        <span className="flex-1 min-w-0 text-sm text-gray-700 truncate">
          <span className="font-semibold">{toolName}</span>
          {summary && summary !== "(no details)" && (
            <span className="ml-1.5 text-gray-500">{summary}</span>
          )}
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

      {/* Expanded: JSON input */}
      {expanded && (
        <div className="px-3 pb-3 pt-1 border-t border-gray-100">
          <pre className="overflow-x-auto rounded bg-gray-50 p-3 text-xs font-mono text-gray-800">
            <code>
              {item.input
                ? JSON.stringify(item.input, null, 2)
                : "(no input)"}
            </code>
          </pre>
        </div>
      )}
    </div>
  );
}
