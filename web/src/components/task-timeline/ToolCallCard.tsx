import { useState } from "react";
import type { TimelineItem } from "../../lib/tool-chain-parser";
import { deriveSummary } from "../../lib/tool-chain-parser";

interface ToolCallCardProps {
  item: TimelineItem;
}

/**
 * Card for tool_use timeline items.
 * Collapsed: tool name (bold) + derived summary.
 * Expanded: JSON input in formatted code block.
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

        {/* Summary */}
        <span className="flex-1 min-w-0 text-sm text-gray-700 truncate">
          <span className="font-semibold">{toolName}</span>
          {summary && summary !== "(no details)" && (
            <span className="ml-1.5 text-gray-500">{summary}</span>
          )}
        </span>

        {/* Expand/collapse indicator */}
        <span className="text-gray-400 text-xs flex-shrink-0">
          {expanded ? "▾" : "▸"}
        </span>
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
