import { useState, type ReactNode } from "react";
import type { TimelineItemType } from "../../lib/tool-chain-parser";

interface TimelineCardProps {
  children: ReactNode;
  type: TimelineItemType;
  seq: number;
  defaultExpanded?: boolean;
  badge?: string;
}

/** Color classes for each timeline item type */
const TYPE_COLORS: Record<TimelineItemType, string> = {
  tool_use: "bg-blue-500/20 text-blue-700",
  tool_result: "bg-gray-200 text-gray-700",
  thinking: "bg-violet-500/20 text-violet-700",
  text: "bg-emerald-500/20 text-emerald-700",
  error: "bg-red-500/20 text-red-700",
};

/** Default badge labels for each type */
const TYPE_LABELS: Record<TimelineItemType, string> = {
  tool_use: "Tool",
  tool_result: "Result",
  thinking: "Thinking",
  text: "Text",
  error: "Error",
};

/**
 * Base collapsible card for timeline items.
 * Renders a sequence number, color-coded type badge, and expandable content area.
 */
export function TimelineCard({
  children,
  type,
  seq,
  defaultExpanded = false,
  badge,
}: TimelineCardProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  const badgeLabel = badge || TYPE_LABELS[type];
  const colorClasses = TYPE_COLORS[type];

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
        aria-label={`${expanded ? "Collapse" : "Expand"} item ${seq + 1}`}
      >
        {/* Sequence number */}
        <span className="text-[10px] font-mono text-gray-400 min-w-[2ch] text-right">
          {seq + 1}
        </span>

        {/* Type badge */}
        <span
          className={`inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium ${colorClasses}`}
        >
          {badgeLabel}
        </span>

        {/* Content area (collapsed preview) */}
        <div className="flex-1 min-w-0 text-sm text-gray-700 truncate">
          {!expanded && children}
        </div>

        {/* Expand/collapse indicator */}
        <span className="text-gray-400 text-xs flex-shrink-0">
          {expanded ? "▾" : "▸"}
        </span>
      </button>

      {/* Expanded content */}
      {expanded && (
        <div className="px-3 pb-3 pt-1 border-t border-gray-100">
          {children}
        </div>
      )}
    </div>
  );
}
