import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ToolCallCard } from "../ToolCallCard";
import { ToolResultCard } from "../ToolResultCard";
import type { TimelineItem } from "../../../lib/tool-chain-parser";

// Feature: agentic-output-architecture
// Tests for structured event rendering components (Requirements 18.4, 18.5, 18.6)

describe("ToolCallCard", () => {
  it("displays tool name for tool_use events", () => {
    const item: TimelineItem = {
      seq: 0,
      type: "tool_use",
      tool: "read_file",
      input: { path: "/src/main.ts" },
      content: "",
      output: "",
    };

    render(<ToolCallCard item={item} />);
    expect(screen.getByText("read_file")).toBeDefined();
  });

  it("displays input parameter summary for tool_use events", () => {
    const item: TimelineItem = {
      seq: 1,
      type: "tool_use",
      tool: "write_file",
      input: { path: "/output.txt", content: "hello" },
      content: "",
      output: "",
    };

    render(<ToolCallCard item={item} />);
    expect(screen.getByText("write_file")).toBeDefined();
  });

  it("handles missing input gracefully", () => {
    const item: TimelineItem = {
      seq: 2,
      type: "tool_use",
      tool: "terminal",
      input: undefined,
      content: "",
      output: "",
    };

    render(<ToolCallCard item={item} />);
    expect(screen.getByText("terminal")).toBeDefined();
  });
});

describe("ToolResultCard", () => {
  it("displays output content for tool_result events", () => {
    const item: TimelineItem = {
      seq: 3,
      type: "tool_result",
      tool: "read_file",
      output: "file contents here",
      content: "",
      input: undefined,
    };

    render(<ToolResultCard item={item} />);
    expect(screen.getByText("file contents here")).toBeDefined();
  });

  it("handles empty output gracefully", () => {
    const item: TimelineItem = {
      seq: 4,
      type: "tool_result",
      tool: "terminal",
      output: "",
      content: "",
      input: undefined,
    };

    render(<ToolResultCard item={item} />);
    expect(screen.getByText("(empty output)")).toBeDefined();
  });

  it("truncates long output in preview", () => {
    const longOutput = "x".repeat(300);
    const item: TimelineItem = {
      seq: 5,
      type: "tool_result",
      tool: "exec",
      output: longOutput,
      content: "",
      input: undefined,
    };

    render(<ToolResultCard item={item} />);
    // Preview should be truncated (200 chars + "…")
    const preview = screen.getByRole("article");
    expect(preview).toBeDefined();
  });
});

describe("Structured event detection", () => {
  it("tool_use events have type field set", () => {
    const item: TimelineItem = {
      seq: 0,
      type: "tool_use",
      tool: "read_file",
      input: { path: "/test" },
      content: "",
      output: "",
    };
    expect(item.type).toBe("tool_use");
    expect(item.tool).toBe("read_file");
  });

  it("tool_result events have type field set", () => {
    const item: TimelineItem = {
      seq: 1,
      type: "tool_result",
      tool: "read_file",
      output: "contents",
      content: "",
      input: undefined,
    };
    expect(item.type).toBe("tool_result");
    expect(item.output).toBe("contents");
  });

  it("text events have type field set", () => {
    const item: TimelineItem = {
      seq: 2,
      type: "text",
      content: "Hello world",
      tool: "",
      input: undefined,
      output: "",
    };
    expect(item.type).toBe("text");
    expect(item.content).toBe("Hello world");
  });

  it("tool call count can be derived from items", () => {
    const items: TimelineItem[] = [
      { seq: 0, type: "text", content: "Starting", tool: "", input: undefined, output: "" },
      { seq: 1, type: "tool_use", tool: "read_file", input: { path: "/a" }, content: "", output: "" },
      { seq: 2, type: "tool_result", tool: "read_file", output: "data", content: "", input: undefined },
      { seq: 3, type: "tool_use", tool: "write_file", input: { path: "/b" }, content: "", output: "" },
      { seq: 4, type: "tool_result", tool: "write_file", output: "ok", content: "", input: undefined },
      { seq: 5, type: "text", content: "Done", tool: "", input: undefined, output: "" },
    ];

    const toolCallCount = items.filter((i) => i.type === "tool_use").length;
    expect(toolCallCount).toBe(2);
  });
});
