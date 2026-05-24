import { useState } from "react";
import type { TimelineItem } from "../../lib/tool-chain-parser";

interface TextCardProps {
  item: TimelineItem;
}

const EXPAND_THRESHOLD = 200;

/**
 * Card for text timeline items.
 * Short text (≤200 chars): displayed inline, no expand/collapse.
 * Long text (>200 chars): expandable with preview.
 */
export function TextCard({ item }: TextCardProps) {
  const content = item.content || "";
  const isLong = content.length > EXPAND_THRESHOLD;
  const [expanded, setExpanded] = useState(!isLong);

  const preview = isLong ? content.slice(0, EXPAND_THRESHOLD) + "…" : content;

  // Short text — simple non-collapsible card
  if (!isLong) {
    return (
      <div
        className="rounded-lg border border-gray-200 bg-white px-3 py-2 flex items-center gap-2"
        role="article"
      >
        {/* Sequence number */}
        <span className="text-[10px] font-mono text-gray-400 min-w-[2ch] text-right">
          {item.seq + 1}
        </span>

        {/* Type badge */}
        <span className="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium bg-emerald-500/20 text-emerald-700">
          Text
        </span>

        {/* Content */}
        <span className="flex-1 min-w-0 text-sm text-gray-700">
          {content}
        </span>
      </div>
    );
  }

  // Long text — collapsible card
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
        aria-label={`${expanded ? "Collapse" : "Expand"} text ${item.seq + 1}`}
      >
        {/* Sequence number */}
        <span className="text-[10px] font-mono text-gray-400 min-w-[2ch] text-right">
          {item.seq + 1}
        </span>

        {/* Type badge */}
        <span className="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium bg-emerald-500/20 text-emerald-700">
          Text
        </span>

        {/* Preview or full content */}
        {!expanded && (
          <span className="flex-1 min-w-0 text-sm text-gray-700 truncate">
            {preview}
          </span>
        )}
        {expanded && <span className="flex-1" />}

        {/* Expand/collapse indicator */}
        <span className="text-gray-400 text-xs flex-shrink-0">
          {expanded ? "▾" : "▸"}
        </span>
      </button>

      {/* Expanded: full text content */}
      {expanded && (
        <div className="px-3 pb-3 pt-1 border-t border-gray-100">
          <p className="text-sm text-gray-700 whitespace-pre-wrap">{content}</p>
        </div>
      )}
    </div>
  );
}
