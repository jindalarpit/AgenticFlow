import { useState } from "react";
import type { TimelineItem } from "../../lib/tool-chain-parser";

interface ErrorCardProps {
  item: TimelineItem;
}

const EXPAND_THRESHOLD = 200;

/**
 * Card for error timeline items.
 * Shows error content with red styling and error icon.
 * Short errors (≤200 chars): displayed inline.
 * Long errors (>200 chars): expandable.
 */
export function ErrorCard({ item }: ErrorCardProps) {
  const content = item.content || "";
  const isLong = content.length > EXPAND_THRESHOLD;
  const [expanded, setExpanded] = useState(!isLong);

  const preview = isLong ? content.slice(0, EXPAND_THRESHOLD) + "…" : content;

  // Short error — simple non-collapsible card
  if (!isLong) {
    return (
      <div
        className="rounded-lg border border-red-200 bg-red-50/50 px-3 py-2 flex items-center gap-2"
        role="article"
      >
        {/* Sequence number */}
        <span className="text-[10px] font-mono text-gray-400 min-w-[2ch] text-right">
          {item.seq + 1}
        </span>

        {/* Type badge */}
        <span className="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium bg-red-500/20 text-red-700">
          Error
        </span>

        {/* Error icon + content */}
        <span className="flex-1 min-w-0 text-sm text-red-700">
          <span className="mr-1">⚠️</span>
          {content}
        </span>
      </div>
    );
  }

  // Long error — collapsible card
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

        {/* Type badge */}
        <span className="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium bg-red-500/20 text-red-700">
          Error
        </span>

        {/* Error icon + preview */}
        {!expanded && (
          <span className="flex-1 min-w-0 text-sm text-red-700 truncate">
            <span className="mr-1">⚠️</span>
            {preview}
          </span>
        )}
        {expanded && <span className="flex-1" />}

        {/* Expand/collapse indicator */}
        <span className="text-gray-400 text-xs flex-shrink-0">
          {expanded ? "▾" : "▸"}
        </span>
      </button>

      {/* Expanded: full error content */}
      {expanded && (
        <div className="px-3 pb-3 pt-1 border-t border-red-100">
          <p className="text-sm text-red-700 whitespace-pre-wrap">
            <span className="mr-1">⚠️</span>
            {content}
          </p>
        </div>
      )}
    </div>
  );
}
