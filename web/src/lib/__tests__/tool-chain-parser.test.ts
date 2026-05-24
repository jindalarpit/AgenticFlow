/**
 * Unit tests for tool-chain-parser module.
 * Feature: task-tool-chain-ui
 */

import { describe, it, expect } from "vitest";
import {
  parseMessageContent,
  parseMessages,
  detectAgentFormat,
  deriveSummary,
  type TimelineItem,
} from "../tool-chain-parser";
import type { TaskMessage } from "../../hooks/useTasks";

// ─── Helper ──────────────────────────────────────────────────────────────────

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

// ─── parseMessageContent ─────────────────────────────────────────────────────

describe("parseMessageContent", () => {
  it("returns empty array for empty content", () => {
    expect(parseMessageContent("", "stdout", 1)).toEqual([]);
    expect(parseMessageContent("   ", "stdout", 1)).toEqual([]);
  });

  it("classifies tool_use JSON correctly", () => {
    const content = JSON.stringify({
      type: "tool_use",
      name: "Read",
      input: { file_path: "/src/main.ts" },
    });
    const items = parseMessageContent(content, "stdout", 1);
    expect(items).toHaveLength(1);
    expect(items[0]!.type).toBe("tool_use");
    expect(items[0]!.tool).toBe("Read");
    expect(items[0]!.input).toEqual({ file_path: "/src/main.ts" });
  });

  it("classifies tool_result JSON correctly", () => {
    const content = JSON.stringify({
      type: "tool_result",
      name: "Read",
      output: "file contents here",
    });
    const items = parseMessageContent(content, "stdout", 1);
    expect(items).toHaveLength(1);
    expect(items[0]!.type).toBe("tool_result");
    expect(items[0]!.tool).toBe("Read");
    expect(items[0]!.output).toBe("file contents here");
  });

  it("classifies thinking JSON correctly", () => {
    const content = JSON.stringify({
      type: "thinking",
      content: "Let me analyze this...",
    });
    const items = parseMessageContent(content, "stdout", 1);
    expect(items).toHaveLength(1);
    expect(items[0]!.type).toBe("thinking");
    expect(items[0]!.content).toBe("Let me analyze this...");
  });

  it("classifies error JSON correctly", () => {
    const content = JSON.stringify({
      type: "error",
      message: "Something went wrong",
    });
    const items = parseMessageContent(content, "stdout", 1);
    expect(items).toHaveLength(1);
    expect(items[0]!.type).toBe("error");
    expect(items[0]!.content).toBe("Something went wrong");
  });

  it("treats stderr as error type for plain text", () => {
    const items = parseMessageContent("Warning: deprecated API", "stderr", 1);
    expect(items).toHaveLength(1);
    expect(items[0]!.type).toBe("error");
    expect(items[0]!.content).toBe("Warning: deprecated API");
  });

  it("treats stderr JSON with explicit type as that type (not error)", () => {
    const content = JSON.stringify({ type: "tool_use", name: "Bash", input: { command: "ls" } });
    const items = parseMessageContent(content, "stderr", 1);
    expect(items).toHaveLength(1);
    expect(items[0]!.type).toBe("tool_use");
  });

  it("treats plain text on stdout as text type", () => {
    const items = parseMessageContent("Hello world", "stdout", 1);
    expect(items).toHaveLength(1);
    expect(items[0]!.type).toBe("text");
    expect(items[0]!.content).toBe("Hello world");
  });

  it("handles multiple JSON blocks in one message", () => {
    const block1 = JSON.stringify({ type: "tool_use", name: "Read", input: {} });
    const block2 = JSON.stringify({ type: "tool_result", output: "done" });
    const content = block1 + "\n" + block2;
    const items = parseMessageContent(content, "stdout", 1);
    expect(items).toHaveLength(2);
    expect(items[0]!.type).toBe("tool_use");
    expect(items[1]!.type).toBe("tool_result");
  });

  it("handles mixed JSON and text", () => {
    const json = JSON.stringify({ type: "tool_use", name: "Write", input: {} });
    const content = "Starting task\n" + json + "\nDone";
    const items = parseMessageContent(content, "stdout", 1);
    expect(items).toHaveLength(3);
    expect(items[0]!.type).toBe("text");
    expect(items[1]!.type).toBe("tool_use");
    expect(items[2]!.type).toBe("text");
  });

  it("treats malformed JSON as text", () => {
    const content = '{"type": "tool_use", "name": "Read"'; // missing closing brace
    const items = parseMessageContent(content, "stdout", 1);
    expect(items).toHaveLength(1);
    expect(items[0]!.type).toBe("text");
  });

  it("treats unknown type field as text", () => {
    const content = JSON.stringify({ type: "unknown_type", data: "foo" });
    const items = parseMessageContent(content, "stdout", 1);
    expect(items).toHaveLength(1);
    expect(items[0]!.type).toBe("text");
  });

  it("sets tool to 'unknown' when tool_use has no name", () => {
    const content = JSON.stringify({ type: "tool_use", input: { x: 1 } });
    const items = parseMessageContent(content, "stdout", 1);
    expect(items).toHaveLength(1);
    expect(items[0]!.type).toBe("tool_use");
    expect(items[0]!.tool).toBe("unknown");
  });
});

// ─── parseMessages ───────────────────────────────────────────────────────────

describe("parseMessages", () => {
  it("assigns monotonically increasing seq numbers", () => {
    const messages: TaskMessage[] = [
      makeMessage({ sequence: 1, content: "hello" }),
      makeMessage({ sequence: 2, content: "world" }),
      makeMessage({ sequence: 3, content: JSON.stringify({ type: "tool_use", name: "Read", input: {} }) }),
    ];
    const items = parseMessages(messages);
    for (let i = 1; i < items.length; i++) {
      expect(items[i]!.seq).toBeGreaterThan(items[i - 1]!.seq);
    }
  });

  it("deduplicates by source message sequence", () => {
    const messages: TaskMessage[] = [
      makeMessage({ sequence: 1, content: "hello" }),
      makeMessage({ sequence: 1, content: "hello" }), // duplicate
      makeMessage({ sequence: 2, content: "world" }),
    ];
    const items = parseMessages(messages);
    expect(items).toHaveLength(2);
  });

  it("skips empty content messages", () => {
    const messages: TaskMessage[] = [
      makeMessage({ sequence: 1, content: "" }),
      makeMessage({ sequence: 2, content: "hello" }),
    ];
    const items = parseMessages(messages);
    expect(items).toHaveLength(1);
    expect(items[0]!.content).toBe("hello");
  });
});

// ─── detectAgentFormat ───────────────────────────────────────────────────────

describe("detectAgentFormat", () => {
  it("detects Claude Code format", () => {
    const messages: TaskMessage[] = [
      makeMessage({ sequence: 1, content: '{"type": "tool_use", "name": "Read"}' }),
    ];
    const config = detectAgentFormat(messages);
    expect(config.toolUsePatterns.length).toBeGreaterThan(0);
    expect(config.toolResultPatterns.length).toBeGreaterThan(0);
  });

  it("detects Gemini CLI format", () => {
    const messages: TaskMessage[] = [
      makeMessage({ sequence: 1, content: '{"functionCall": {"name": "read_file"}}' }),
    ];
    const config = detectAgentFormat(messages);
    expect(config.toolUsePatterns.some((p) => p.source.includes("functionCall"))).toBe(true);
  });

  it("detects OpenCode format", () => {
    const messages: TaskMessage[] = [
      makeMessage({ sequence: 1, content: "opencode tool_call started" }),
    ];
    const config = detectAgentFormat(messages);
    expect(config.toolUsePatterns.some((p) => p.source.includes("tool_call"))).toBe(true);
  });

  it("detects Kiro format", () => {
    const messages: TaskMessage[] = [
      makeMessage({ sequence: 1, content: "antml:invoke name=\"read_file\"" }),
    ];
    const config = detectAgentFormat(messages);
    expect(config.toolUsePatterns.some((p) => p.source.includes("antml"))).toBe(true);
  });

  it("returns default config for unknown format", () => {
    const messages: TaskMessage[] = [
      makeMessage({ sequence: 1, content: "just some random text" }),
    ];
    const config = detectAgentFormat(messages);
    expect(config.toolUsePatterns.length).toBeGreaterThan(0);
  });
});

// ─── deriveSummary ───────────────────────────────────────────────────────────

describe("deriveSummary", () => {
  it("returns shortened file_path (last 2 segments)", () => {
    const item: TimelineItem = {
      seq: 0,
      type: "tool_use",
      tool: "Read",
      input: { file_path: "/home/user/project/src/main.ts" },
    };
    expect(deriveSummary(item)).toBe("…/src/main.ts");
  });

  it("returns shortened path (last 2 segments)", () => {
    const item: TimelineItem = {
      seq: 0,
      type: "tool_use",
      tool: "Write",
      input: { path: "/a/b/c/d.txt" },
    };
    expect(deriveSummary(item)).toBe("…/c/d.txt");
  });

  it("returns full path when 2 or fewer segments", () => {
    const item: TimelineItem = {
      seq: 0,
      type: "tool_use",
      tool: "Read",
      input: { file_path: "main.ts" },
    };
    expect(deriveSummary(item)).toBe("main.ts");
  });

  it("returns command truncated to 100 chars", () => {
    const longCommand = "a".repeat(150);
    const item: TimelineItem = {
      seq: 0,
      type: "tool_use",
      tool: "Bash",
      input: { command: longCommand },
    };
    const summary = deriveSummary(item);
    expect(summary.length).toBe(101); // 100 chars + "…"
    expect(summary.endsWith("…")).toBe(true);
  });

  it("returns command as-is when under 100 chars", () => {
    const item: TimelineItem = {
      seq: 0,
      type: "tool_use",
      tool: "Bash",
      input: { command: "npm run test" },
    };
    expect(deriveSummary(item)).toBe("npm run test");
  });

  it("returns query value", () => {
    const item: TimelineItem = {
      seq: 0,
      type: "tool_use",
      tool: "Search",
      input: { query: "find all imports" },
    };
    expect(deriveSummary(item)).toBe("find all imports");
  });

  it("returns pattern value", () => {
    const item: TimelineItem = {
      seq: 0,
      type: "tool_use",
      tool: "Grep",
      input: { pattern: "TODO|FIXME" },
    };
    expect(deriveSummary(item)).toBe("TODO|FIXME");
  });

  it("returns first short string value as fallback", () => {
    const item: TimelineItem = {
      seq: 0,
      type: "tool_use",
      tool: "Custom",
      input: { some_field: "short value" },
    };
    expect(deriveSummary(item)).toBe("short value");
  });

  it('returns "(no details)" when no suitable summary found', () => {
    const item: TimelineItem = {
      seq: 0,
      type: "tool_use",
      tool: "Custom",
      input: { num: 42 } as unknown as Record<string, unknown>,
    };
    expect(deriveSummary(item)).toBe("(no details)");
  });

  it('returns "(no details)" when input is undefined', () => {
    const item: TimelineItem = {
      seq: 0,
      type: "tool_use",
      tool: "Custom",
    };
    expect(deriveSummary(item)).toBe("(no details)");
  });

  it("prefers file_path over command", () => {
    const item: TimelineItem = {
      seq: 0,
      type: "tool_use",
      tool: "Write",
      input: { file_path: "/home/user/src/index.ts", command: "echo hello" },
    };
    expect(deriveSummary(item)).toBe("…/src/index.ts");
  });

  it("prefers command over query", () => {
    const item: TimelineItem = {
      seq: 0,
      type: "tool_use",
      tool: "Multi",
      input: { command: "ls -la", query: "find files" },
    };
    expect(deriveSummary(item)).toBe("ls -la");
  });

  it("uses content for non-tool_use items without input", () => {
    const item: TimelineItem = {
      seq: 0,
      type: "text",
      content: "Agent is analyzing the code...",
    };
    expect(deriveSummary(item)).toBe("Agent is analyzing the code...");
  });
});
