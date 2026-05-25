/**
 * Unit tests for computeSegments, extractFinalResult, and formatCopyText.
 * Feature: task-tool-chain-ui
 */

import { describe, it, expect } from "vitest";
import {
  computeSegments,
  extractFinalResult,
  formatCopyText,
  type TimelineItem,
} from "../tool-chain-parser";

// ─── computeSegments ─────────────────────────────────────────────────────────

describe("computeSegments", () => {
  it("returns empty array for empty input", () => {
    expect(computeSegments([])).toEqual([]);
  });

  it("returns single segment for single item", () => {
    const items: TimelineItem[] = [
      { seq: 0, type: "tool_use", tool: "Read" },
    ];
    const segments = computeSegments(items);
    expect(segments).toEqual([{ type: "tool_use", startSeq: 0, count: 1 }]);
  });

  it("groups consecutive same-type items into one segment", () => {
    const items: TimelineItem[] = [
      { seq: 0, type: "tool_use", tool: "Read" },
      { seq: 1, type: "tool_use", tool: "Write" },
      { seq: 2, type: "tool_use", tool: "Bash" },
    ];
    const segments = computeSegments(items);
    expect(segments).toEqual([{ type: "tool_use", startSeq: 0, count: 3 }]);
  });

  it("creates separate segments for different types", () => {
    const items: TimelineItem[] = [
      { seq: 0, type: "tool_use", tool: "Read" },
      { seq: 1, type: "tool_result", output: "done" },
      { seq: 2, type: "text", content: "hello" },
    ];
    const segments = computeSegments(items);
    expect(segments).toEqual([
      { type: "tool_use", startSeq: 0, count: 1 },
      { type: "tool_result", startSeq: 1, count: 1 },
      { type: "text", startSeq: 2, count: 1 },
    ]);
  });

  it("handles alternating types correctly", () => {
    const items: TimelineItem[] = [
      { seq: 0, type: "tool_use", tool: "Read" },
      { seq: 1, type: "tool_result", output: "content" },
      { seq: 2, type: "tool_use", tool: "Write" },
      { seq: 3, type: "tool_result", output: "ok" },
    ];
    const segments = computeSegments(items);
    expect(segments).toHaveLength(4);
    expect(segments[0]).toEqual({ type: "tool_use", startSeq: 0, count: 1 });
    expect(segments[1]).toEqual({ type: "tool_result", startSeq: 1, count: 1 });
    expect(segments[2]).toEqual({ type: "tool_use", startSeq: 2, count: 1 });
    expect(segments[3]).toEqual({ type: "tool_result", startSeq: 3, count: 1 });
  });

  it("segment counts sum to total item count", () => {
    const items: TimelineItem[] = [
      { seq: 0, type: "thinking", content: "hmm" },
      { seq: 1, type: "thinking", content: "ok" },
      { seq: 2, type: "tool_use", tool: "Read" },
      { seq: 3, type: "tool_result", output: "data" },
      { seq: 4, type: "text", content: "done" },
      { seq: 5, type: "text", content: "final" },
    ];
    const segments = computeSegments(items);
    const totalCount = segments.reduce((sum, s) => sum + s.count, 0);
    expect(totalCount).toBe(items.length);
  });
});

// ─── extractFinalResult ──────────────────────────────────────────────────────

describe("extractFinalResult", () => {
  const items: TimelineItem[] = [
    { seq: 0, type: "tool_use", tool: "Read" },
    { seq: 1, type: "tool_result", output: "file content" },
    { seq: 2, type: "thinking", content: "analyzing..." },
    { seq: 3, type: "text", content: "Here is the result" },
    { seq: 4, type: "error", content: "Something failed" },
    { seq: 5, type: "text", content: "Final answer" },
  ];

  it("returns last text-type item for completed status", () => {
    const result = extractFinalResult(items, "completed");
    expect(result).not.toBeNull();
    expect(result!.type).toBe("text");
    expect(result!.content).toBe("Final answer");
    expect(result!.seq).toBe(5);
  });

  it("returns last error-type item for failed status", () => {
    const result = extractFinalResult(items, "failed");
    expect(result).not.toBeNull();
    expect(result!.type).toBe("error");
    expect(result!.content).toBe("Something failed");
    expect(result!.seq).toBe(4);
  });

  it("returns null for running status", () => {
    expect(extractFinalResult(items, "running")).toBeNull();
  });

  it("returns null for pending status", () => {
    expect(extractFinalResult(items, "pending")).toBeNull();
  });

  it("returns null for unknown status", () => {
    expect(extractFinalResult(items, "cancelled")).toBeNull();
  });

  it("returns null for completed status when no text items exist", () => {
    const noTextItems: TimelineItem[] = [
      { seq: 0, type: "tool_use", tool: "Read" },
      { seq: 1, type: "error", content: "oops" },
    ];
    expect(extractFinalResult(noTextItems, "completed")).toBeNull();
  });

  it("returns null for failed status when no error items exist", () => {
    const noErrorItems: TimelineItem[] = [
      { seq: 0, type: "tool_use", tool: "Read" },
      { seq: 1, type: "text", content: "hello" },
    ];
    expect(extractFinalResult(noErrorItems, "failed")).toBeNull();
  });

  it("returns null for empty items array", () => {
    expect(extractFinalResult([], "completed")).toBeNull();
    expect(extractFinalResult([], "failed")).toBeNull();
  });
});

// ─── formatCopyText ──────────────────────────────────────────────────────────

describe("formatCopyText", () => {
  it("returns empty string for empty input", () => {
    expect(formatCopyText([])).toBe("");
  });

  it("formats tool_use with tool name as label", () => {
    const items: TimelineItem[] = [
      { seq: 0, type: "tool_use", tool: "Read", input: { path: "/home/user/src/main.ts" } },
    ];
    const text = formatCopyText(items);
    expect(text).toBe("[Read] …/src/main.ts");
  });

  it("uses 'Tool' as label when tool name is missing", () => {
    const items: TimelineItem[] = [
      { seq: 0, type: "tool_use", input: { command: "ls" } },
    ];
    const text = formatCopyText(items);
    expect(text).toBe("[Tool] ls");
  });

  it("formats tool_result with 'Result' label", () => {
    const items: TimelineItem[] = [
      { seq: 0, type: "tool_result", output: "file contents here" },
    ];
    const text = formatCopyText(items);
    expect(text).toBe("[Result] file contents here");
  });

  it("formats thinking with 'Thinking' label", () => {
    const items: TimelineItem[] = [
      { seq: 0, type: "thinking", content: "Let me analyze this" },
    ];
    const text = formatCopyText(items);
    expect(text).toBe("[Thinking] _Let me analyze this_");
  });

  it("formats text with 'Text' label", () => {
    const items: TimelineItem[] = [
      { seq: 0, type: "text", content: "Here is the answer" },
    ];
    const text = formatCopyText(items);
    expect(text).toBe("[Text] Here is the answer");
  });

  it("formats error with 'Error' label", () => {
    const items: TimelineItem[] = [
      { seq: 0, type: "error", content: "Something went wrong" },
    ];
    const text = formatCopyText(items);
    expect(text).toBe("[Error] Something went wrong");
  });

  it("produces one line per item joined by newlines", () => {
    const items: TimelineItem[] = [
      { seq: 0, type: "tool_use", tool: "Read", input: { path: "/a/b/c.ts" } },
      { seq: 1, type: "tool_result", output: "done" },
      { seq: 2, type: "text", content: "All good" },
    ];
    const text = formatCopyText(items);
    const lines = text.split("\n");
    expect(lines).toHaveLength(3);
    expect(lines[0]).toBe("[Read] …/b/c.ts");
    expect(lines[1]).toBe("[Result] done");
    expect(lines[2]).toBe("[Text] All good");
  });

  it("line count equals item count", () => {
    const items: TimelineItem[] = [
      { seq: 0, type: "tool_use", tool: "Bash", input: { command: "echo hi" } },
      { seq: 1, type: "thinking", content: "hmm" },
      { seq: 2, type: "error", content: "oops" },
      { seq: 3, type: "text", content: "done" },
      { seq: 4, type: "tool_result", output: "ok" },
    ];
    const text = formatCopyText(items);
    const lines = text.split("\n");
    expect(lines).toHaveLength(items.length);
  });
});
