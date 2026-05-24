import { useState } from "react";
import {
  computeSegments,
  type TimelineItem,
  type TimelineItemType,
} from "../../lib/tool-chain-parser";

interface TimelineBarProps {
  items: TimelineItem[];
  selectedSeq: number | null;
  onSegmentClick: (seq: number) => void;
}

/** Solid background colors for timeline bar segments */
const SEGMENT_COLORS: Record<TimelineItemType, string> = {
  tool_use: "bg-blue-500",
  tool_result: "bg-gray-400",
  thinking: "bg-violet-500",
  text: "bg-emerald-500",
  error: "bg-red-500",
};

/** Highlighted (selected) segment styles */
const SEGMENT_COLORS_SELECTED: Record<TimelineItemType, string> = {
  tool_use: "bg-blue-400 ring-2 ring-blue-300",
  tool_result: "bg-gray-300 ring-2 ring-gray-200",
  thinking: "bg-violet-400 ring-2 ring-violet-300",
  text: "bg-emerald-400 ring-2 ring-emerald-300",
  error: "bg-red-400 ring-2 ring-red-300",
};

/** Human-readable labels for each type */
const TYPE_LABELS: Record<TimelineItemType, string> = {
  tool_use: "Tool Call",
  tool_result: "Result",
  thinking: "Thinking",
  text: "Text",
  error: "Error",
};

/**
 * Horizontal colored segment bar representing the distribution of event types
 * across the execution timeline. Each segment is proportional to its count.
 * Clicking a segment scrolls to the corresponding item; hovering shows a tooltip.
 */
export function TimelineBar({
  items,
  selectedSeq,
  onSegmentClick,
}: TimelineBarProps) {
  const [hoveredIndex, setHoveredIndex] = useState<number | null>(null);

  if (items.length === 0) {
    return null;
  }

  const segments = computeSegments(items);
  const totalCount = items.length;

  return (
    <div
      className="flex w-full h-3 rounded-full overflow-hidden gap-px"
      role="progressbar"
      aria-label="Timeline event distribution"
    >
      {segments.map((segment, index) => {
        const widthPercent = (segment.count / totalCount) * 100;
        const isSelected =
          selectedSeq !== null &&
          selectedSeq >= segment.startSeq &&
          selectedSeq < segment.startSeq + segment.count;

        const colorClass = isSelected
          ? SEGMENT_COLORS_SELECTED[segment.type]
          : SEGMENT_COLORS[segment.type];

        return (
          <div
            key={`${segment.startSeq}-${segment.type}`}
            className="relative"
            style={{ width: `${widthPercent}%`, minWidth: "4px" }}
          >
            <button
              type="button"
              className={`w-full h-full ${colorClass} hover:opacity-80 transition-opacity cursor-pointer`}
              onClick={() => onSegmentClick(segment.startSeq)}
              onMouseEnter={() => setHoveredIndex(index)}
              onMouseLeave={() => setHoveredIndex(null)}
              aria-label={`${TYPE_LABELS[segment.type]}: ${segment.count} events`}
            />

            {/* Tooltip */}
            {hoveredIndex === index && (
              <div className="absolute bottom-full left-1/2 -translate-x-1/2 mb-1.5 px-2 py-1 text-[11px] font-medium text-white bg-gray-800 rounded shadow-sm whitespace-nowrap z-10 pointer-events-none">
                {TYPE_LABELS[segment.type]}: {segment.count} event
                {segment.count !== 1 ? "s" : ""}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
