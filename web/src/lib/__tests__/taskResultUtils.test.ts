import { describe, it, expect } from "vitest";
import {
  extractDashboardResult,
  truncateResultContent,
} from "../taskResultUtils";
import type { TaskMessage } from "../../hooks/useTasks";

/* ─── Helpers ─── */

function makeMessage(
  overrides: Partial<TaskMessage> & { sequence: number; content: string }
): TaskMessage {
  return {
    id: `msg-${overrides.sequence}`,
    task_id: "task-1",
    stream: "stdout",
    created_at: new Date().toISOString(),
    ...overrides,
  };
}

/* ─── extractDashboardResult ─── */

describe("extractDashboardResult", () => {
  it("returns content of the highest-sequence stdout message", () => {
    const messages: TaskMessage[] = [
      makeMessage({ sequence: 1, content: "first" }),
      makeMessage({ sequence: 3, content: "third" }),
      makeMessage({ sequence: 2, content: "second" }),
    ];
    expect(extractDashboardResult(messages, null)).toBe("third");
  });

  it("ignores stderr messages when selecting stdout", () => {
    const messages: TaskMessage[] = [
      makeMessage({ sequence: 1, content: "stdout content", stream: "stdout" }),
      makeMessage({ sequence: 5, content: "stderr content", stream: "stderr" }),
    ];
    expect(extractDashboardResult(messages, null)).toBe("stdout content");
  });

  it("falls back to outputPreview when no stdout messages exist", () => {
    const messages: TaskMessage[] = [
      makeMessage({ sequence: 1, content: "error output", stream: "stderr" }),
    ];
    expect(extractDashboardResult(messages, "preview text")).toBe(
      "preview text"
    );
  });

  it("returns null when no messages and no outputPreview", () => {
    expect(extractDashboardResult([], null)).toBeNull();
  });

  it("returns null when only stderr messages and outputPreview is null", () => {
    const messages: TaskMessage[] = [
      makeMessage({ sequence: 1, content: "err", stream: "stderr" }),
    ];
    expect(extractDashboardResult(messages, null)).toBeNull();
  });

  it("returns outputPreview when messages array is empty", () => {
    expect(extractDashboardResult([], "fallback")).toBe("fallback");
  });

  it("handles single stdout message", () => {
    const messages: TaskMessage[] = [
      makeMessage({ sequence: 1, content: "only one" }),
    ];
    expect(extractDashboardResult(messages, "preview")).toBe("only one");
  });
});

/* ─── truncateResultContent ─── */

describe("truncateResultContent", () => {
  it("returns full content when length <= 2000", () => {
    const content = "a".repeat(2000);
    const result = truncateResultContent(content);
    expect(result.displayText).toBe(content);
    expect(result.isTruncated).toBe(false);
    expect(result.fullText).toBe(content);
  });

  it("truncates to first 500 chars when length > 2000", () => {
    const content = "b".repeat(2001);
    const result = truncateResultContent(content);
    expect(result.displayText).toBe("b".repeat(500));
    expect(result.isTruncated).toBe(true);
    expect(result.fullText).toBe(content);
  });

  it("handles empty string", () => {
    const result = truncateResultContent("");
    expect(result.displayText).toBe("");
    expect(result.isTruncated).toBe(false);
    expect(result.fullText).toBe("");
  });

  it("handles exactly 2000 characters (boundary)", () => {
    const content = "x".repeat(2000);
    const result = truncateResultContent(content);
    expect(result.displayText).toBe(content);
    expect(result.isTruncated).toBe(false);
  });

  it("handles 2001 characters (just over boundary)", () => {
    const content = "y".repeat(2001);
    const result = truncateResultContent(content);
    expect(result.displayText.length).toBe(500);
    expect(result.isTruncated).toBe(true);
  });

  it("always preserves fullText regardless of truncation", () => {
    const content = "z".repeat(5000);
    const result = truncateResultContent(content);
    expect(result.fullText).toBe(content);
    expect(result.fullText.length).toBe(5000);
  });
});
