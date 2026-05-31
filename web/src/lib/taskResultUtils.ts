/**
 * Pure utility functions for Task Result Panel content extraction and truncation.
 *
 * These functions handle extracting the final displayable result from task messages
 * and truncating content for the dashboard panel display.
 */

import type { TaskMessage } from "../hooks/useTasks";

/* ─── Types ─── */

/** Return type for truncateResultContent. */
export interface TruncatedResult {
  /** The text to display in the panel (may be truncated). */
  displayText: string;
  /** Whether the content was truncated for display. */
  isTruncated: boolean;
  /** The original full content, always preserved. */
  fullText: string;
}

/* ─── Constants ─── */

/** Maximum content length before truncation is applied. */
const MAX_FULL_DISPLAY_LENGTH = 2000;

/** Number of characters to show when content is truncated. */
const TRUNCATED_DISPLAY_LENGTH = 500;

/* ─── Functions ─── */

/**
 * Extract the final displayable result from task messages.
 *
 * Priority:
 * 1. Concatenation of all stdout messages (sorted by sequence number)
 * 2. Task's output_preview field (fallback)
 * 3. null (no content available)
 *
 * The daemon streams output in chunks — each chunk is a separate message.
 * The final result is the concatenation of all stdout chunks in order.
 */
export function extractDashboardResult(
  messages: TaskMessage[],
  outputPreview: string | null
): string | null {
  // Filter to stdout messages only
  const stdoutMessages = messages.filter((msg) => msg.stream === "stdout");

  if (stdoutMessages.length > 0) {
    // Sort by sequence and concatenate all content
    const sorted = [...stdoutMessages].sort((a, b) => a.sequence - b.sequence);
    const fullOutput = sorted.map((msg) => msg.content).join("");
    return fullOutput || null;
  }

  // Fallback to output_preview
  return outputPreview ?? null;
}

/**
 * Truncate content for display in the result panel.
 *
 * Rules:
 * - Content <= 2000 chars: display in full, isTruncated === false
 * - Content > 2000 chars: display first 500 chars, isTruncated === true
 * - fullText always === content
 */
export function truncateResultContent(content: string): TruncatedResult {
  if (content.length <= MAX_FULL_DISPLAY_LENGTH) {
    return {
      displayText: content,
      isTruncated: false,
      fullText: content,
    };
  }

  return {
    displayText: content.slice(0, TRUNCATED_DISPLAY_LENGTH),
    isTruncated: true,
    fullText: content,
  };
}
