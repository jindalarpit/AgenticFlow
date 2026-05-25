import { useState } from "react";
import type { TimelineItem } from "../../lib/tool-chain-parser";

interface ErrorCardProps {
  item: TimelineItem;
}

/**
 * Card for error timeline items.
 * Renders with red styling, alert icon in badge, and destructive color summary.
 * Collapsed: alert badge + red summary text.
 * Expanded: red monospace text with preserved whitespace.
 * Displays "(no error details)" when content is empty or whitespace-only.
 *
 * Requirements: 8.1, 8.2, 8.3, 8.5
 */
export function ErrorCard({ item }: ErrorCardProps) {
  const [expanded, setExpanded] = useState(false);

  const content = item.content || "";
  const isEmpty = !content.trim();
  const summary = isEmpty ? "(no error details)" : content;

  return (
    <div
      className="rounded-lg border border-red-200 bg-red-50/50 hover:border-red-300 transition-colors"
      role="article"
      aria-expanded={expanded}
    >
      <button
        type="button"
        className="w-full flex items-center gap-2 px-3 py-2 text-left cursor-pointer"
        onClick={() => setExpanded((prev) => !prev)}
        aria-label={`${expanded ? "Collapse" : "Expand"} error ${item.seq + 1}`}
      >
        {/* Sequence number */}
        <span className="text-[10px] font-mono text-gray-400 min-w-[2ch] text-right">
          {item.seq + 1}
        </span>

        {/* Type badge with alert icon prefix (Req 8.1) */}
        <span className="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-medium bg-red-500/20 text-red-700">
          <span>⚠️</span>
          <span>Error</span>
        </span>

        {/* Summary in destructive (red) color (Req 8.2, 8.5) */}
        <span className="flex-1 min-w-0 text-sm text-red-600 truncate">
          {summary}
        </span>

        {/* Expand/collapse indicator */}
        <span className="text-gray-400 text-xs flex-shrink-0">
          {expanded ? "▾" : "▸"}
        </span>
      </button>

      {/* Expanded: red monospace text with preserved whitespace (Req 8.3) */}
      {expanded && (
        <div className="px-3 pb-3 pt-1 border-t border-red-100">
          <pre className="text-sm text-red-700 font-mono whitespace-pre-wrap break-words">
            {isEmpty ? "(no error details)" : content}
          </pre>
        </div>
      )}
    </div>
  );
}
