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
 * 1. Last stdout message content (highest sequence number among items with stream "stdout")
 * 2. Task's output_preview field (fallback)
 * 3. null (no content available)
 */
export function extractDashboardResult(
  messages: TaskMessage[],
  outputPreview: string | null
): string | null {
  // Filter to stdout messages only
  const stdoutMessages = messages.filter((msg) => msg.stream === "stdout");

  if (stdoutMessages.length > 0) {
    // Find the message with the highest sequence number
    let highest = stdoutMessages[0]!;
    for (let i = 1; i < stdoutMessages.length; i++) {
      if (stdoutMessages[i]!.sequence > highest.sequence) {
        highest = stdoutMessages[i]!;
      }
    }
    return highest.content;
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
