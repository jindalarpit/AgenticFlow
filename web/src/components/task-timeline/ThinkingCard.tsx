import { useState } from "react";
import type { TimelineItem } from "../../lib/tool-chain-parser";

interface ThinkingCardProps {
  item: TimelineItem;
}

const PREVIEW_LENGTH = 150;

/**
 * Card for thinking timeline items.
 * Collapsed: brain emoji + italic preview (first 150 chars).
 * Expanded: full thinking content.
 */
export function ThinkingCard({ item }: ThinkingCardProps) {
  const [expanded, setExpanded] = useState(false);

  const content = item.content || "";
  const isLong = content.length > PREVIEW_LENGTH;
  const preview = isLong ? content.slice(0, PREVIEW_LENGTH) + "…" : content;

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
        aria-label={`${expanded ? "Collapse" : "Expand"} thinking ${item.seq + 1}`}
      >
        {/* Sequence number */}
        <span className="text-[10px] font-mono text-gray-400 min-w-[2ch] text-right">
          {item.seq + 1}
        </span>

        {/* Type badge */}
        <span className="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium bg-violet-500/20 text-violet-700">
          Thinking
        </span>

        {/* Brain icon + italic preview */}
        <span className="flex-1 min-w-0 text-sm text-gray-600 truncate">
          <span className="mr-1">🧠</span>
          <span className="italic">{preview || "(empty)"}</span>
        </span>

        {/* Expand/collapse indicator */}
        <span className="text-gray-400 text-xs flex-shrink-0">
          {expanded ? "▾" : "▸"}
        </span>
      </button>

      {/* Expanded: full thinking content */}
      {expanded && (
        <div className="px-3 pb-3 pt-1 border-t border-gray-100">
          <p className="text-sm text-gray-700 whitespace-pre-wrap">{content}</p>
        </div>
      )}
    </div>
  );
}
