import { useState, useMemo, useCallback, useEffect } from "react";
import { parseMessages, type TimelineItem } from "../lib/tool-chain-parser";
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
  /** Sort direction */
  sortDirection: "chronological" | "newest_first";
  /** Toggle sort direction */
  setSortDirection: (dir: "chronological" | "newest_first") => void;
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
      setSortDirectionState(dir);
    },
    []
  );

  // ─── Filtered items ──────────────────────────────────────────────────────
  const filteredItems = useMemo(() => {
    let result: TimelineItem[];

    if (filters.size === 0) {
      // No filters active — show all items
      result = items;
    } else {
      // Filter: include items whose type OR tool:${toolName} is in the filter set
      result = items.filter((item) => {
        // Check if the item's type matches a filter
        if (filters.has(item.type)) {
          return true;
        }
        // Check if the item's tool-specific key matches a filter
        if (item.tool && filters.has(`tool:${item.tool}`)) {
          return true;
        }
        return false;
      });
    }

    // Apply sort direction
    if (sortDirection === "newest_first") {
      return [...result].reverse();
    }

    return result;
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
    toolCallCount,
    totalCount,
    filterOptions,
  };
}

// ─── Exported Pure Functions (for property-based testing) ────────────────────

/**
 * Pure filter function: given items and a filter set, returns matching items.
 * When filters is empty, returns all items.
 * When filters is non-empty, includes items whose type OR `tool:${toolName}` is in the set.
 *
 * Validates: Requirements 7.1, 7.2
 */
export function filterItems(
  items: TimelineItem[],
  filters: Set<string>
): TimelineItem[] {
  if (filters.size === 0) {
    return items;
  }
  return items.filter((item) => {
    if (filters.has(item.type)) {
      return true;
    }
    if (item.tool && filters.has(`tool:${item.tool}`)) {
      return true;
    }
    return false;
  });
}

/**
 * Pure sort function: given items and a direction, returns sorted items.
 * "chronological" returns items as-is (ascending seq).
 * "newest_first" returns items in reverse order.
 *
 * Validates: Requirements 7.4
 */
export function sortItems(
  items: TimelineItem[],
  direction: "chronological" | "newest_first"
): TimelineItem[] {
  if (direction === "newest_first") {
    return [...items].reverse();
  }
  return items;
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
