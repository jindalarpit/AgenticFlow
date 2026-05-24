/**
 * Pure utility functions for the Agent Detail UI.
 *
 * These functions handle formatting, sorting, and validation logic
 * used across the agent detail page components.
 */

import type { AgentTask } from "./agent-detail-types";

/* ─── Types ─── */

/** Subset of AgentTask used for active task sorting. */
export type ActiveTask = Pick<AgentTask, "id" | "status" | "created_at">;

/* ─── Functions ─── */

/**
 * Truncates a string to a maximum length, appending an ellipsis ("…") if truncated.
 * Default max is 100 characters.
 *
 * Property 2: if s.length <= max, output === s;
 *             if s.length > max, output === s.slice(0, max) + "…" and output.length === max + 1
 */
export function truncatePrompt(s: string, max: number = 100): string {
  if (s.length <= max) return s;
  return s.slice(0, max) + "\u2026";
}

/**
 * Sorts active tasks by lifecycle priority: running > dispatched > pending.
 * Within the same status group, tasks are ordered by created_at descending (most recent first).
 *
 * Property 3: running precedes dispatched precedes pending;
 *             within same status, ordered by created_at descending.
 */
export function sortActiveTasks(tasks: ActiveTask[]): ActiveTask[] {
  const priorityMap: Record<string, number> = {
    running: 0,
    dispatched: 1,
    pending: 2,
  };

  return [...tasks].sort((a, b) => {
    const pa = priorityMap[a.status] ?? 3;
    const pb = priorityMap[b.status] ?? 3;
    if (pa !== pb) return pa - pb;
    // Within same status, most recent first (descending created_at)
    return b.created_at.localeCompare(a.created_at);
  });
}

/**
 * Computes success rate as a 0-100 integer percentage.
 * Returns 0 when totalTerminal is 0.
 *
 * Property 4: result === Math.round((completed / totalTerminal) * 100) when totalTerminal > 0;
 *             result === 0 when totalTerminal === 0; result in [0, 100].
 */
export function computeSuccessRate(completed: number, totalTerminal: number): number {
  if (totalTerminal === 0) return 0;
  return Math.round((completed / totalTerminal) * 100);
}

/**
 * Formats a duration in milliseconds to a human-readable string.
 * Output matches one of: "<1s", "Xs", "Xm Ys", "Xh Ym".
 *
 * Property 5: numeric values in the output are consistent with input ms.
 */
export function formatDuration(ms: number): string {
  if (ms < 1000) return "<1s";

  const totalSeconds = Math.floor(ms / 1000);

  if (totalSeconds < 60) {
    return `${totalSeconds}s`;
  }

  const totalMinutes = Math.floor(totalSeconds / 60);
  const remainingSeconds = totalSeconds % 60;

  if (totalMinutes < 60) {
    return `${totalMinutes}m ${remainingSeconds}s`;
  }

  const hours = Math.floor(totalMinutes / 60);
  const remainingMinutes = totalMinutes % 60;
  return `${hours}h ${remainingMinutes}m`;
}

/**
 * Filters out entries with empty (after trim) keys and returns a Record<string, string>.
 *
 * Property 6: output contains only entries where key.trim().length > 0;
 *             output size <= input length.
 */
export function filterEmptyKeys(
  entries: { key: string; value: string }[]
): Record<string, string> {
  const result: Record<string, string> = {};
  for (const entry of entries) {
    const trimmedKey = entry.key.trim();
    if (trimmedKey.length > 0) {
      result[trimmedKey] = entry.value;
    }
  }
  return result;
}

/**
 * Detects whether any two entries have identical trimmed keys (case-sensitive).
 *
 * Property 7: returns true iff two or more entries have identical trimmed keys.
 */
export function hasDuplicateKeys(
  entries: { key: string; value: string }[]
): boolean {
  const seen = new Set<string>();
  for (const entry of entries) {
    const trimmedKey = entry.key.trim();
    if (seen.has(trimmedKey)) return true;
    seen.add(trimmedKey);
  }
  return false;
}

/**
 * Splits a space-separated string into an array of non-empty tokens.
 * Empty strings produce an empty array.
 *
 * Property 8: joining result with single space equals s.trim().replace(/\s+/g, ' ');
 *             empty strings produce empty array.
 */
export function splitArgs(input: string): string[] {
  const trimmed = input.trim();
  if (trimmed.length === 0) return [];
  return trimmed.split(/\s+/);
}

/**
 * Formats an ISO date string as a relative time string (e.g., "3 days ago", "just now").
 * Uses simple bucket-based formatting without external dependencies.
 */
export function formatRelativeTime(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();

  if (diffMs < 0) return "just now";

  const seconds = Math.floor(diffMs / 1000);
  if (seconds < 60) return "just now";

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return minutes === 1 ? "1 minute ago" : `${minutes} minutes ago`;
  }

  const hours = Math.floor(minutes / 60);
  if (hours < 24) {
    return hours === 1 ? "1 hour ago" : `${hours} hours ago`;
  }

  const days = Math.floor(hours / 24);
  if (days < 30) {
    return days === 1 ? "1 day ago" : `${days} days ago`;
  }

  const months = Math.floor(days / 30);
  if (months < 12) {
    return months === 1 ? "1 month ago" : `${months} months ago`;
  }

  const years = Math.floor(months / 12);
  return years === 1 ? "1 year ago" : `${years} years ago`;
}
