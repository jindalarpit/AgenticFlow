/**
 * Tool Chain Parser — Client-side parser that transforms raw TaskMessage[]
 * into structured TimelineItem[] for the execution timeline UI.
 *
 * Feature: task-tool-chain-ui
 */

import type { TaskMessage } from "../hooks/useTasks";

// ─── Types ───────────────────────────────────────────────────────────────────

/** Supported timeline item types */
export type TimelineItemType =
  | "tool_use"
  | "tool_result"
  | "thinking"
  | "text"
  | "error";

/** A single parsed execution step */
export interface TimelineItem {
  seq: number;
  type: TimelineItemType;
  tool?: string; // Tool name (for tool_use / tool_result)
  content?: string; // Text content (for text, thinking, error)
  input?: Record<string, unknown>; // Input params (for tool_use)
  output?: string; // Output text (for tool_result)
}

/** A segment of consecutive same-type items in the timeline */
export interface TimelineSegment {
  type: TimelineItemType;
  startSeq: number;
  count: number;
}

/** Parser configuration for different agent CLI formats */
export interface ParserConfig {
  /** Regex patterns to detect tool_use JSON blocks */
  toolUsePatterns: RegExp[];
  /** Regex patterns to detect tool_result JSON blocks */
  toolResultPatterns: RegExp[];
  /** Regex patterns to detect thinking blocks */
  thinkingPatterns: RegExp[];
  /** Regex patterns to detect error blocks */
  errorPatterns: RegExp[];
}

// ─── Internal Helpers ────────────────────────────────────────────────────────

/**
 * Extract JSON objects from a string using brace counting.
 * Returns an array of { json, startIdx, endIdx } for each top-level JSON object found,
 * plus the non-JSON text segments between them.
 */
interface ExtractedSegment {
  kind: "json" | "text";
  value: string;
}

function extractSegments(content: string): ExtractedSegment[] {
  const segments: ExtractedSegment[] = [];
  let i = 0;
  let textStart = 0;

  while (i < content.length) {
    if (content[i] === "{") {
      // Capture any text before this JSON block
      if (i > textStart) {
        const text = content.slice(textStart, i).trim();
        if (text.length > 0) {
          segments.push({ kind: "text", value: text });
        }
      }

      // Brace counting to find the matching closing brace
      let depth = 0;
      let inString = false;
      let escape = false;
      const jsonStart = i;

      for (; i < content.length; i++) {
        const ch = content[i];

        if (escape) {
          escape = false;
          continue;
        }

        if (ch === "\\" && inString) {
          escape = true;
          continue;
        }

        if (ch === '"' && !escape) {
          inString = !inString;
          continue;
        }

        if (inString) continue;

        if (ch === "{") depth++;
        else if (ch === "}") {
          depth--;
          if (depth === 0) {
            i++; // move past the closing brace
            segments.push({ kind: "json", value: content.slice(jsonStart, i) });
            textStart = i;
            break;
          }
        }
      }

      // If we never closed the braces, treat as text
      if (depth !== 0) {
        segments.push({ kind: "text", value: content.slice(jsonStart) });
        textStart = content.length;
        break;
      }
    } else {
      i++;
    }
  }

  // Capture trailing text
  if (textStart < content.length) {
    const text = content.slice(textStart).trim();
    if (text.length > 0) {
      segments.push({ kind: "text", value: text });
    }
  }

  return segments;
}

/**
 * Classify a parsed JSON object into a TimelineItemType based on its fields.
 */
function classifyJson(
  obj: Record<string, unknown>
): { type: TimelineItemType; item: Partial<TimelineItem> } {
  const typeField = obj["type"] as string | undefined;

  if (typeField === "tool_use") {
    return {
      type: "tool_use",
      item: {
        tool: (obj["name"] as string) || (obj["tool"] as string) || "unknown",
        input: (obj["input"] as Record<string, unknown>) ?? (obj["parameters"] as Record<string, unknown>) ?? undefined,
      },
    };
  }

  if (typeField === "tool_result") {
    return {
      type: "tool_result",
      item: {
        tool: (obj["name"] as string) || (obj["tool"] as string) || undefined,
        output: (obj["output"] as string) || (obj["content"] as string) || JSON.stringify(obj),
      },
    };
  }

  if (typeField === "thinking") {
    return {
      type: "thinking",
      item: {
        content: (obj["content"] as string) || (obj["thinking"] as string) || JSON.stringify(obj),
      },
    };
  }

  if (typeField === "error") {
    return {
      type: "error",
      item: {
        content: (obj["message"] as string) || (obj["content"] as string) || JSON.stringify(obj),
      },
    };
  }

  // Unknown type field — treat as text
  return {
    type: "text",
    item: {
      content: JSON.stringify(obj),
    },
  };
}

// ─── Public API ──────────────────────────────────────────────────────────────

/**
 * Parse a single message content string into zero or more TimelineItems.
 * Handles multi-block messages (e.g., a single stdout chunk containing
 * multiple JSON objects).
 */
export function parseMessageContent(
  content: string,
  stream: "stdout" | "stderr" | "stdin",
  sequence: number
): TimelineItem[] {
  if (!content || content.trim().length === 0) {
    return [];
  }

  const segments = extractSegments(content);
  const items: TimelineItem[] = [];
  let subIndex = 0;

  for (const segment of segments) {
    if (segment.kind === "json") {
      try {
        const parsed = JSON.parse(segment.value) as Record<string, unknown>;
        const { type, item } = classifyJson(parsed);
        items.push({
          seq: sequence * 1000 + subIndex,
          type,
          ...item,
        });
      } catch {
        // Malformed JSON — treat as text
        const type: TimelineItemType = stream === "stderr" ? "error" : "text";
        items.push({
          seq: sequence * 1000 + subIndex,
          type,
          content: segment.value,
        });
      }
    } else {
      // Non-JSON text
      const type: TimelineItemType = stream === "stderr" ? "error" : "text";
      items.push({
        seq: sequence * 1000 + subIndex,
        type,
        content: segment.value,
      });
    }
    subIndex++;
  }

  return items;
}

/**
 * Parse raw TaskMessage[] into structured TimelineItem[].
 * Pure function — no side effects.
 * Deduplicates by source message sequence number.
 */
export function parseMessages(messages: TaskMessage[]): TimelineItem[] {
  const seenSequences = new Set<number>();
  const allItems: TimelineItem[] = [];
  let globalSeq = 0;

  for (const message of messages) {
    // Deduplicate by source message sequence
    if (seenSequences.has(message.sequence)) {
      continue;
    }
    seenSequences.add(message.sequence);

    const items = parseMessageContent(
      message.content,
      message.stream,
      message.sequence
    );

    // Reassign monotonically increasing seq numbers
    for (const item of items) {
      item.seq = globalSeq++;
      allItems.push(item);
    }
  }

  return allItems;
}

/**
 * Detect the agent format from message patterns.
 * Returns a ParserConfig tuned for the detected format.
 */
export function detectAgentFormat(messages: TaskMessage[]): ParserConfig {
  // Sample first N messages to detect format
  const sampleSize = Math.min(messages.length, 20);
  const sample = messages.slice(0, sampleSize).map((m) => m.content).join("\n");

  // Claude Code patterns
  if (
    sample.includes("claude") ||
    sample.includes("anthropic") ||
    sample.includes('"type": "tool_use"') ||
    sample.includes('"type":"tool_use"')
  ) {
    return {
      toolUsePatterns: [/"type"\s*:\s*"tool_use"/],
      toolResultPatterns: [/"type"\s*:\s*"tool_result"/],
      thinkingPatterns: [/"type"\s*:\s*"thinking"/, /\[thinking\]/i],
      errorPatterns: [/"type"\s*:\s*"error"/, /^error:/im],
    };
  }

  // Gemini CLI patterns
  if (
    sample.includes("gemini") ||
    sample.includes("functionCall") ||
    sample.includes("functionResponse")
  ) {
    return {
      toolUsePatterns: [/"functionCall"/, /"type"\s*:\s*"tool_use"/],
      toolResultPatterns: [/"functionResponse"/, /"type"\s*:\s*"tool_result"/],
      thinkingPatterns: [/"type"\s*:\s*"thinking"/, /\[thought\]/i],
      errorPatterns: [/"type"\s*:\s*"error"/, /^ERROR/m],
    };
  }

  // OpenCode patterns
  if (
    sample.includes("opencode") ||
    sample.includes("tool_call") ||
    sample.includes("tool_response")
  ) {
    return {
      toolUsePatterns: [/"tool_call"/, /"type"\s*:\s*"tool_use"/],
      toolResultPatterns: [/"tool_response"/, /"type"\s*:\s*"tool_result"/],
      thinkingPatterns: [/"type"\s*:\s*"thinking"/],
      errorPatterns: [/"type"\s*:\s*"error"/, /\[error\]/i],
    };
  }

  // Kiro patterns
  if (sample.includes("kiro") || sample.includes("antml")) {
    return {
      toolUsePatterns: [/"type"\s*:\s*"tool_use"/, /antml:invoke/],
      toolResultPatterns: [/"type"\s*:\s*"tool_result"/, /function_results/],
      thinkingPatterns: [/"type"\s*:\s*"thinking"/, /antml:thinking/],
      errorPatterns: [/"type"\s*:\s*"error"/],
    };
  }

  // Default / generic config
  return {
    toolUsePatterns: [/"type"\s*:\s*"tool_use"/],
    toolResultPatterns: [/"type"\s*:\s*"tool_result"/],
    thinkingPatterns: [/"type"\s*:\s*"thinking"/],
    errorPatterns: [/"type"\s*:\s*"error"/],
  };
}

/**
 * Derive a one-line summary from a TimelineItem.
 *
 * For tool_use items, priority order:
 * 1. input.query — search queries
 * 2. input.file_path or input.path — shortened to last 2 segments
 * 3. input.command — truncated to 100 chars
 * 4. input.pattern — regex patterns
 * 5. First string value in input ≤120 chars
 * 6. Fallback: "(no details)"
 *
 * The returned string never exceeds 120 characters for tool_use items.
 *
 * For thinking items: first 150 chars italic preview, or "(empty)" if whitespace-only.
 * For error items: content, or "(no error details)" if whitespace-only.
 */
export function deriveSummary(item: TimelineItem): string {
  // Handle thinking items
  if (item.type === "thinking") {
    const content = item.content ?? "";
    if (!content.trim()) {
      return "(empty)";
    }
    if (content.length <= 150) {
      return `_${content}_`;
    }
    return `_${content.slice(0, 150)}…_`;
  }

  // Handle error items
  if (item.type === "error") {
    const content = item.content ?? "";
    if (!content.trim()) {
      return "(no error details)";
    }
    return content;
  }

  // For tool_use items with input, apply priority order
  if (item.input && item.type === "tool_use") {
    const summary = deriveToolUseSummary(item.input);
    return truncate(summary, 120);
  }

  // For non-tool_use items without special handling, use content or output
  if (!item.input) {
    if (item.content) {
      return item.content.slice(0, 120).trim() || "(no details)";
    }
    if (item.output) {
      return item.output.slice(0, 120).trim() || "(no details)";
    }
    return "(no details)";
  }

  // tool_result or other items with input but not tool_use
  if (item.output) {
    return item.output.slice(0, 120).trim() || "(no details)";
  }
  if (item.content) {
    return item.content.slice(0, 120).trim() || "(no details)";
  }
  return "(no details)";
}

/**
 * Derive summary specifically for tool_use input parameters.
 * Returns the raw summary string (may exceed 120 chars — caller truncates).
 */
function deriveToolUseSummary(input: Record<string, unknown>): string {
  // Priority 1: query value
  const query = input["query"] as string;
  if (query && typeof query === "string" && query.length > 0) {
    return query;
  }

  // Priority 2: file_path or path → shortened last 2 segments
  const filePath = (input["file_path"] as string) || (input["path"] as string);
  if (filePath && typeof filePath === "string" && filePath.length > 0) {
    const segments = filePath.split("/").filter((s) => s.length > 0);
    if (segments.length === 0) {
      // Path like "/" has no meaningful segments — fall through
    } else if (segments.length <= 2) {
      return segments.join("/");
    } else {
      return "…/" + segments.slice(-2).join("/");
    }
  }

  // Priority 3: command → truncated to 100 chars
  const command = input["command"] as string;
  if (command && typeof command === "string" && command.length > 0) {
    if (command.length <= 100) {
      return command;
    }
    return command.slice(0, 100) + "…";
  }

  // Priority 4: pattern value
  const pattern = input["pattern"] as string;
  if (pattern && typeof pattern === "string" && pattern.length > 0) {
    return pattern;
  }

  // Priority 5: First string value ≤120 chars
  for (const value of Object.values(input)) {
    if (typeof value === "string" && value.length > 0 && value.length <= 120) {
      return value;
    }
  }

  // Priority 6: fallback
  return "(no details)";
}

/**
 * Truncate a string to maxLen characters, appending ellipsis if truncated.
 */
function truncate(str: string, maxLen: number): string {
  if (str.length <= maxLen) {
    return str;
  }
  return str.slice(0, maxLen - 1) + "…";
}

/**
 * Group consecutive same-type items into segments.
 * Each segment records the type, the seq of the first item, and the count.
 * Empty input → empty output.
 */
export function computeSegments(items: TimelineItem[]): TimelineSegment[] {
  if (items.length === 0) {
    return [];
  }

  const segments: TimelineSegment[] = [];
  let currentType = items[0]!.type;
  let startSeq = items[0]!.seq;
  let count = 1;

  for (let i = 1; i < items.length; i++) {
    const item = items[i]!;
    if (item.type === currentType) {
      count++;
    } else {
      segments.push({ type: currentType, startSeq, count });
      currentType = item.type;
      startSeq = item.seq;
      count = 1;
    }
  }

  // Push the final segment
  segments.push({ type: currentType, startSeq, count });

  return segments;
}

/**
 * Extract the final result item based on task status.
 * - "completed" → last text-type item (or null)
 * - "failed" → last error-type item (or null)
 * - "running" / "pending" / any other → null
 */
export function extractFinalResult(
  items: TimelineItem[],
  status: string
): TimelineItem | null {
  if (status === "completed") {
    for (let i = items.length - 1; i >= 0; i--) {
      if (items[i]!.type === "text") {
        return items[i]!;
      }
    }
    return null;
  }

  if (status === "failed") {
    for (let i = items.length - 1; i >= 0; i--) {
      if (items[i]!.type === "error") {
        return items[i]!;
      }
    }
    return null;
  }

  return null;
}

/**
 * Format visible timeline items as copy-friendly text.
 * Each item becomes one line: `[{TypeLabel}] {summary}`
 * TypeLabel mapping:
 *   tool_use → tool name or "Tool"
 *   tool_result → "Result"
 *   thinking → "Thinking"
 *   text → "Text"
 *   error → "Error"
 * Empty input → empty string.
 */
export function formatCopyText(items: TimelineItem[]): string {
  if (items.length === 0) {
    return "";
  }

  return items
    .map((item) => {
      const label = getTypeLabel(item);
      const summary = deriveSummary(item);
      return `[${label}] ${summary}`;
    })
    .join("\n");
}

/**
 * Get the display label for a timeline item's type.
 */
function getTypeLabel(item: TimelineItem): string {
  switch (item.type) {
    case "tool_use":
      return item.tool || "Tool";
    case "tool_result":
      return "Result";
    case "thinking":
      return "Thinking";
    case "text":
      return "Text";
    case "error":
      return "Error";
  }
}

// ─── Filter & Sort Pure Functions ────────────────────────────────────────────

/**
 * Pure filter function: given items and a filter set, returns matching items.
 * When filters is empty, returns all items.
 * When filters is non-empty, includes items whose type OR `tool:${toolName}` is in the set (OR logic).
 *
 * Validates: Requirements 5.2, 6.2, 6.3
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
 * Validates: Requirements 6.2, 6.3
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
