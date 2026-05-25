import { useRef, useEffect, useCallback, useState, useLayoutEffect } from "react";
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
  highlightedSeq?: number | null;
  /** Current sort direction — controls auto-scroll behavior */
  sortDirection?: "chronological" | "newest_first";
  /**
   * Monotonically increasing signal — when this value changes,
   * the view scrolls to offset 0 (top). Used when sort direction toggles.
   * Validates: Requirement 6.4
   */
  scrollToTopSignal?: number;
}

const SCROLL_THRESHOLD = 50;
const HIGHLIGHT_DURATION_MS = 2000;

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
 *
 * Auto-scroll behavior is sort-direction-aware:
 * - "chronological" + live: auto-scroll to bottom on new items unless user
 *   has scrolled up more than 50px from the bottom (Req 6.8, 9.4)
 * - "newest_first" + live: new items appear at top; preserve user's current
 *   scroll position by compensating for added content height (Req 6.7)
 *
 * When `scrollToTopSignal` changes, scrolls to offset 0 (Req 6.4).
 *
 * Supports highlight-on-click: when `highlightedSeq` changes to a non-null
 * value, scrolls to that item and applies a temporary highlight for 2 seconds.
 */
export function TimelineView({
  items,
  isLive,
  onScrollStateChange,
  highlightedSeq = null,
  sortDirection = "chronological",
  scrollToTopSignal = 0,
}: TimelineViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const isAtBottomRef = useRef(true);
  const itemRefs = useRef<Map<number, HTMLDivElement>>(new Map());
  const [activeHighlight, setActiveHighlight] = useState<number | null>(null);

  // Track previous scrollHeight for "newest_first" scroll position preservation
  const prevScrollHeightRef = useRef<number>(0);
  const prevItemsLengthRef = useRef<number>(items.length);

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

  /**
   * Before DOM updates: capture current scrollHeight so we can compensate
   * after new items are prepended in "newest_first" mode.
   */
  useLayoutEffect(() => {
    const el = containerRef.current;
    if (el) {
      prevScrollHeightRef.current = el.scrollHeight;
    }
  });

  /**
   * After DOM updates: handle auto-scroll based on sort direction.
   *
   * - Chronological + live: scroll to bottom if user is at bottom
   * - Newest first + live: compensate scrollTop for content added at top
   */
  useEffect(() => {
    if (!isLive) {
      prevItemsLengthRef.current = items.length;
      return;
    }

    const el = containerRef.current;
    if (!el) {
      prevItemsLengthRef.current = items.length;
      return;
    }

    const itemsAdded = items.length > prevItemsLengthRef.current;

    if (sortDirection === "chronological") {
      // Auto-scroll to bottom on new items unless user scrolled up >50px
      if (itemsAdded && isAtBottomRef.current) {
        el.scrollTop = el.scrollHeight;
      }
    } else if (sortDirection === "newest_first") {
      // New items are prepended at top — preserve user's current view position
      // by adjusting scrollTop to compensate for the added height
      if (itemsAdded) {
        const heightDiff = el.scrollHeight - prevScrollHeightRef.current;
        if (heightDiff > 0) {
          el.scrollTop = el.scrollTop + heightDiff;
        }
      }
    }

    prevItemsLengthRef.current = items.length;
  }, [items.length, isLive, sortDirection]);

  /**
   * Scroll to top (offset 0) when scrollToTopSignal changes.
   * This fires when the user toggles sort direction.
   * Validates: Requirement 6.4
   */
  useEffect(() => {
    if (scrollToTopSignal === 0) return; // Skip initial render
    const el = containerRef.current;
    if (el) {
      el.scrollTop = 0;
    }
  }, [scrollToTopSignal]);

  /** Scroll to highlighted item and apply temporary highlight */
  useEffect(() => {
    if (highlightedSeq == null) return;

    const itemEl = itemRefs.current.get(highlightedSeq);
    if (itemEl) {
      itemEl.scrollIntoView({ behavior: "smooth", block: "center" });
    }

    setActiveHighlight(highlightedSeq);

    const timeout = setTimeout(() => {
      setActiveHighlight(null);
    }, HIGHLIGHT_DURATION_MS);

    return () => clearTimeout(timeout);
  }, [highlightedSeq]);

  /** Store a ref for each item element by seq */
  const setItemRef = useCallback(
    (seq: number, el: HTMLDivElement | null) => {
      if (el) {
        itemRefs.current.set(seq, el);
      } else {
        itemRefs.current.delete(seq);
      }
    },
    []
  );

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
        <div
          key={item.seq}
          ref={(el) => setItemRef(item.seq, el)}
          className={`transition-colors duration-500 rounded ${
            activeHighlight === item.seq
              ? "bg-blue-100 dark:bg-blue-900/30"
              : ""
          }`}
        >
          {renderCard(item)}
        </div>
      ))}
    </div>
  );
}
