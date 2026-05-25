import { describe, it, expect, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { TaskResultBody } from "../TaskResultBody";
import type { Task, TaskMessage } from "../../../hooks/useTasks";

// ─── Helpers ─────────────────────────────────────────────────────────────────

function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    id: "task-1",
    user_id: "user-1",
    agent_type: "claude",
    prompt: "Fix the bug",
    status: "completed",
    exit_code: 0,
    error_message: null,
    output_preview: null,
    agent_id: "agent-1",
    agent_name: "Nexus",
    started_at: "2025-01-01T00:00:00Z",
    completed_at: "2025-01-01T00:01:00Z",
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-01-01T00:01:00Z",
    ...overrides,
  };
}

function makeMessage(overrides: Partial<TaskMessage> = {}): TaskMessage {
  return {
    id: `msg-${overrides.sequence ?? 1}`,
    task_id: "task-1",
    sequence: 1,
    stream: "stdout",
    content: "Hello world",
    created_at: "2025-01-01T00:00:01Z",
    ...overrides,
  };
}

function makeStructuredMessage(
  sequence: number,
  type: string,
  extra: Record<string, unknown> = {}
): TaskMessage {
  const payload: Record<string, unknown> = { type, ...extra };
  return makeMessage({
    sequence,
    content: JSON.stringify(payload),
  });
}

// ─── Tests ───────────────────────────────────────────────────────────────────

describe("TaskResultBody", () => {
  describe("running status with live items", () => {
    it("renders TimelineView with live items and shows running spinner", () => {
      const messages: TaskMessage[] = [
        makeStructuredMessage(1, "thinking", { content: "Let me analyze..." }),
        makeStructuredMessage(2, "tool_use", {
          name: "read_file",
          input: { path: "/src/auth.ts" },
        }),
        makeStructuredMessage(3, "tool_result", {
          name: "read_file",
          output: "file contents here",
        }),
      ];

      render(
        <TaskResultBody
          task={makeTask({ status: "running" })}
          messages={messages}
          isStreaming={true}
        />
      );

      // Running spinner should be present
      const spinner = document.querySelector('svg[aria-label="Task is running"]');
      expect(spinner).toBeInTheDocument();

      // Timeline items should be rendered (not "No events yet")
      expect(screen.queryByText("No events yet")).not.toBeInTheDocument();
    });

    it("shows 'No events yet' placeholder when running with no messages", () => {
      render(
        <TaskResultBody
          task={makeTask({ status: "running" })}
          messages={[]}
          isStreaming={true}
        />
      );

      expect(screen.getByText("No events yet")).toBeInTheDocument();
    });
  });

  describe("completed status with full event set", () => {
    it("renders TimelineView with all event types for completed task", () => {
      const messages: TaskMessage[] = [
        makeStructuredMessage(1, "thinking", { content: "Planning approach..." }),
        makeStructuredMessage(2, "tool_use", {
          name: "read_file",
          input: { path: "/src/main.ts" },
        }),
        makeStructuredMessage(3, "tool_result", {
          name: "read_file",
          output: "export function main() {}",
        }),
        makeStructuredMessage(4, "text", { content: "I've fixed the issue." }),
      ];

      render(
        <TaskResultBody
          task={makeTask({ status: "completed" })}
          messages={messages}
          isStreaming={false}
        />
      );

      // No spinner for completed tasks
      const spinner = document.querySelector('svg[aria-label="Task is running"]');
      expect(spinner).not.toBeInTheDocument();

      // Timeline items should be rendered
      expect(screen.queryByText("No events yet")).not.toBeInTheDocument();

      // The timeline bar should be rendered (it has role="progressbar")
      expect(
        screen.getByRole("progressbar", { name: "Timeline event distribution" })
      ).toBeInTheDocument();
    });
  });

  describe("failed status including error events", () => {
    it("renders TimelineView with error events for failed task", () => {
      const messages: TaskMessage[] = [
        makeStructuredMessage(1, "thinking", { content: "Trying to fix..." }),
        makeStructuredMessage(2, "tool_use", {
          name: "write_file",
          input: { path: "/src/broken.ts" },
        }),
        makeStructuredMessage(3, "error", {
          content: "Permission denied: /src/broken.ts",
        }),
      ];

      render(
        <TaskResultBody
          task={makeTask({ status: "failed", error_message: "Task failed" })}
          messages={messages}
          isStreaming={false}
        />
      );

      // No spinner for failed tasks
      const spinner = document.querySelector('svg[aria-label="Task is running"]');
      expect(spinner).not.toBeInTheDocument();

      // Timeline items should be rendered (not empty)
      expect(screen.queryByText("No events yet")).not.toBeInTheDocument();

      // The timeline bar should be rendered
      expect(
        screen.getByRole("progressbar", { name: "Timeline event distribution" })
      ).toBeInTheDocument();
    });
  });

  describe("legacy messages (no type field)", () => {
    it("renders legacy stdout messages as text timeline items", () => {
      const messages: TaskMessage[] = [
        makeMessage({ sequence: 1, stream: "stdout", content: "Starting task..." }),
        makeMessage({ sequence: 2, stream: "stdout", content: "Processing files..." }),
        makeMessage({ sequence: 3, stream: "stderr", content: "Warning: deprecated API" }),
      ];

      render(
        <TaskResultBody
          task={makeTask({ status: "completed" })}
          messages={messages}
          isStreaming={false}
        />
      );

      // Should render timeline items without errors
      expect(screen.queryByText("No events yet")).not.toBeInTheDocument();

      // The timeline bar should be rendered (items exist)
      expect(
        screen.getByRole("progressbar", { name: "Timeline event distribution" })
      ).toBeInTheDocument();
    });
  });

  describe("filter, sort, and copy controls", () => {
    it("renders FilterDropdown, SortDirectionToggle, and CopyButton", () => {
      const messages: TaskMessage[] = [
        makeStructuredMessage(1, "tool_use", {
          name: "read_file",
          input: { path: "/src/app.ts" },
        }),
        makeStructuredMessage(2, "text", { content: "Done." }),
      ];

      render(
        <TaskResultBody
          task={makeTask({ status: "completed" })}
          messages={messages}
          isStreaming={false}
        />
      );

      // Filter button should be present
      expect(
        screen.getByRole("button", { name: /filter/i })
      ).toBeInTheDocument();

      // Sort direction toggle group should be present
      expect(
        screen.getByRole("group", { name: "Sort direction" })
      ).toBeInTheDocument();

      // Sort toggle buttons should be present
      expect(
        screen.getByRole("button", { name: /chronological/i })
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /newest first/i })
      ).toBeInTheDocument();

      // Copy button should be present (either "Copy All" or "Copy Filtered")
      expect(
        screen.getByRole("button", { name: /copy all/i })
      ).toBeInTheDocument();
    });
  });
});
