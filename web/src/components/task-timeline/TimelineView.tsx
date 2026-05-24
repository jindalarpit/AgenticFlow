import { useRef, useEffect, useCallback } from "react";
import type { TimelineItem } from "../../lib/tool-chain-parser";
import { ToolCallCard } from "./ToolCallCard";
import { ToolResultCard } from "./ToolResultCard";
import { ThinkingCard } from "./ThinkingCard";
import { TextCard } from "./TextCard";
import { ErrorCard } from "./ErrorCard";

interface TimelineViewProps {
  items: TimelineItem[];
  isLive: boolean;
  onScrollStateChange?: (isAtBottom: boolean) => void;
}

const SCROLL_THRESHOLD = 50;

/**
 * Renders the appropriate card component for a given timeline item type.
 */
function renderCard(item: TimelineItem) {
  switch (item.type) {
    case "tool_use":
      return <ToolCallCard item={item} />;
    case "tool_result":
      return <ToolResultCard item={item} />;
    case "thinking":
      return <ThinkingCard item={item} />;
    case "text":
      return <TextCard item={item} />;
    case "error":
      return <ErrorCard item={item} />;
    default:
      return <TextCard item={item} />;
  }
}

/**
 * TimelineView — scrollable container that renders timeline cards.
 * Manages auto-scroll behavior: scrolls to bottom on new items
 * unless the user has scrolled up more than 50px from the bottom.
 */
export function TimelineView({
  items,
  isLive,
  onScrollStateChange,
}: TimelineViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const isAtBottomRef = useRef(true);

  /** Check if the scroll container is at (or near) the bottom */
  const checkIsAtBottom = useCallback(() => {
    const el = containerRef.current;
    if (!el) return true;
    return el.scrollTop + el.clientHeight >= el.scrollHeight - SCROLL_THRESHOLD;
  }, []);

  /** Handle scroll events — track whether user is at bottom */
  const handleScroll = useCallback(() => {
    const atBottom = checkIsAtBottom();
    if (atBottom !== isAtBottomRef.current) {
      isAtBottomRef.current = atBottom;
      onScrollStateChange?.(atBottom);
    }
  }, [checkIsAtBottom, onScrollStateChange]);

  /** Auto-scroll to bottom when new items arrive (if live and at bottom) */
  useEffect(() => {
    if (isLive && isAtBottomRef.current) {
      const el = containerRef.current;
      if (el) {
        el.scrollTop = el.scrollHeight;
      }
    }
  }, [items.length, isLive]);

  if (items.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center text-gray-400 text-sm py-12">
        No events yet
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className="flex-1 overflow-y-auto space-y-1.5 py-2"
      onScroll={handleScroll}
    >
      {items.map((item) => (
        <div key={item.seq}>{renderCard(item)}</div>
      ))}
    </div>
  );
}
