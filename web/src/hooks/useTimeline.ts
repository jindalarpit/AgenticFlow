import { useState, useMemo, useCallback, useEffect, useRef } from "react";
import { parseMessages, filterItems, sortItems, type TimelineItem } from "../lib/tool-chain-parser";
import type { TaskMessage } from "./useTasks";

// ─── Types ───────────────────────────────────────────────────────────────────

export interface UseTimelineOptions {
  taskId: string;
  messages: TaskMessage[];
}

export interface UseTimelineResult {
  /** All parsed timeline items */
  items: TimelineItem[];
  /** Items after applying active filters */
  filteredItems: TimelineItem[];
  /** Currently active type/tool filters */
  filters: Set<string>;
  /** Toggle a filter value */
  toggleFilter: (value: string) => void;
  /** Clear all filters */
  clearFilters: () => void;
  /** Sort direction — defaults to "chronological", persists in React state (session only) */
  sortDirection: "chronological" | "newest_first";
  /** Set sort direction — triggers scrollToTopSignal increment */
  setSortDirection: (dir: "chronological" | "newest_first") => void;
  /**
   * Monotonically increasing counter that increments each time sortDirection changes.
   * TimelineView should observe this value and scroll to offset 0 when it changes.
   * Validates: Requirement 6.4
   */
  scrollToTopSignal: number;
  /** Count of tool_use items */
  toolCallCount: number;
  /** Total item count */
  totalCount: number;
  /** Available filter options derived from items */
  filterOptions: Array<{ value: string; label: string }>;
}

// ─── Hook Implementation ─────────────────────────────────────────────────────

/**
 * Hook that composes parsed timeline items with filtering and sorting state.
 * Builds on top of existing useTaskMessages / useTaskStream hooks.
 *
 * - Memoizes parseMessages() call — only re-parses when messages array reference changes
 * - Provides filter state as Set<string> with toggleFilter, clearFilters
 * - Provides sort direction toggle (chronological / newest_first)
 * - Derives toolCallCount, totalCount, filterOptions from items
 * - Resets filters when taskId changes
 *
 * Validates: Requirements 5.1, 5.5, 7.1, 7.2, 7.3, 7.4, 7.5
 */
export function useTimeline(options: UseTimelineOptions): UseTimelineResult {
  const { taskId, messages } = options;

  // ─── Parse messages with memoization ─────────────────────────────────────
  const items = useMemo(() => parseMessages(messages), [messages]);

  // ─── Filter state ────────────────────────────────────────────────────────
  const [filters, setFilters] = useState<Set<string>>(new Set());

  // ─── Sort direction state ────────────────────────────────────────────────
  const [sortDirection, setSortDirectionState] = useState<
    "chronological" | "newest_first"
  >("chronological");

  // ─── Scroll-to-top signal (increments when sort direction changes) ───────
  const scrollToTopSignalRef = useRef(0);
  const [scrollToTopSignal, setScrollToTopSignal] = useState(0);

  // ─── Reset filters when taskId changes ───────────────────────────────────
  useEffect(() => {
    setFilters(new Set());
    setSortDirectionState("chronological");
  }, [taskId]);

  // ─── Toggle a filter value ───────────────────────────────────────────────
  const toggleFilter = useCallback((value: string) => {
    setFilters((prev) => {
      const next = new Set(prev);
      if (next.has(value)) {
        next.delete(value);
      } else {
        next.add(value);
      }
      return next;
    });
  }, []);

  // ─── Clear all filters ───────────────────────────────────────────────────
  const clearFilters = useCallback(() => {
    setFilters(new Set());
  }, []);

  // ─── Set sort direction ──────────────────────────────────────────────────
  const setSortDirection = useCallback(
    (dir: "chronological" | "newest_first") => {
      setSortDirectionState((prev) => {
        if (prev !== dir) {
          // Increment scroll-to-top signal when direction actually changes
          scrollToTopSignalRef.current += 1;
          setScrollToTopSignal(scrollToTopSignalRef.current);
        }
        return dir;
      });
    },
    []
  );

  // ─── Filtered items ──────────────────────────────────────────────────────
  const filteredItems = useMemo(() => {
    const result = filterItems(items, filters);

    // Apply sort direction
    return sortItems(result, sortDirection);
  }, [items, filters, sortDirection]);

  // ─── Derived values ──────────────────────────────────────────────────────
  const toolCallCount = useMemo(
    () => items.filter((item) => item.type === "tool_use").length,
    [items]
  );

  const totalCount = items.length;

  // ─── Filter options derived from items ───────────────────────────────────
  const filterOptions = useMemo(() => {
    const options: Array<{ value: string; label: string }> = [];
    const seenTypes = new Set<string>();
    const seenTools = new Set<string>();

    for (const item of items) {
      // Add type-based filter options
      if (!seenTypes.has(item.type)) {
        seenTypes.add(item.type);
        options.push({
          value: item.type,
          label: formatTypeLabel(item.type),
        });
      }

      // Add tool-specific filter options
      if (item.tool && !seenTools.has(item.tool)) {
        seenTools.add(item.tool);
        options.push({
          value: `tool:${item.tool}`,
          label: item.tool,
        });
      }
    }

    return options;
  }, [items]);

  return {
    items,
    filteredItems,
    filters,
    toggleFilter,
    clearFilters,
    sortDirection,
    setSortDirection,
    scrollToTopSignal,
    toolCallCount,
    totalCount,
    filterOptions,
  };
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

/**
 * Format a TimelineItemType into a human-readable label for filter options.
 */
function formatTypeLabel(type: string): string {
  switch (type) {
    case "tool_use":
      return "Tool Use";
    case "tool_result":
      return "Tool Result";
    case "thinking":
      return "Thinking";
    case "text":
      return "Text";
    case "error":
      return "Error";
    default:
      return type;
  }
}
