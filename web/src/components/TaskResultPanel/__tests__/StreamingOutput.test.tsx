import { describe, it, expect } from "vitest";
import "@testing-library/jest-dom/vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { StreamingOutput } from "../StreamingOutput";
import type { TaskMessage } from "../../../hooks/useTasks";

function makeMessage(
  overrides: Partial<TaskMessage> & { id: string; sequence: number }
): TaskMessage {
  return {
    task_id: "task-1",
    stream: "stdout",
    content: `Output line ${overrides.sequence}`,
    created_at: "2025-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("StreamingOutput", () => {
  describe("rendering messages", () => {
    it("renders messages in monospace font with pre-wrap whitespace", () => {
      const messages: TaskMessage[] = [
        makeMessage({ id: "m1", sequence: 1, content: "Hello world" }),
      ];
      const { container } = render(
        <StreamingOutput messages={messages} isLive={true} />
      );
      const scrollContainer = container.querySelector("[role='log']");
      expect(scrollContainer).toHaveClass("font-mono", "whitespace-pre-wrap");
    });

    it("renders stdout messages with normal text color", () => {
      const messages: TaskMessage[] = [
        makeMessage({ id: "m1", sequence: 1, stream: "stdout", content: "normal output" }),
      ];
      render(<StreamingOutput messages={messages} isLive={true} />);
      const el = screen.getByText("normal output");
      expect(el).toHaveClass("text-gray-800");
    });

    it("renders stderr messages with red text color", () => {
      const messages: TaskMessage[] = [
        makeMessage({ id: "m1", sequence: 1, stream: "stderr", content: "error output" }),
      ];
      render(<StreamingOutput messages={messages} isLive={true} />);
      const el = screen.getByText("error output");
      expect(el).toHaveClass("text-red-500");
    });

    it("shows 'Waiting for output...' when no messages and isLive", () => {
      render(<StreamingOutput messages={[]} isLive={true} />);
      expect(screen.getByText("Waiting for output...")).toBeInTheDocument();
    });

    it("shows 'No output received yet.' when no messages and not live", () => {
      render(<StreamingOutput messages={[]} isLive={false} />);
      expect(screen.getByText("No output received yet.")).toBeInTheDocument();
    });
  });

  describe("reconnecting indicator", () => {
    it("shows reconnecting indicator when isLive is false and messages exist", () => {
      const messages: TaskMessage[] = [
        makeMessage({ id: "m1", sequence: 1, content: "some output" }),
      ];
      render(<StreamingOutput messages={messages} isLive={false} />);
      expect(screen.getByText("Reconnecting...")).toBeInTheDocument();
    });

    it("does not show reconnecting indicator when isLive is true", () => {
      const messages: TaskMessage[] = [
        makeMessage({ id: "m1", sequence: 1, content: "some output" }),
      ];
      render(<StreamingOutput messages={messages} isLive={true} />);
      expect(screen.queryByText("Reconnecting...")).not.toBeInTheDocument();
    });

    it("reconnecting indicator has role=status for accessibility", () => {
      const messages: TaskMessage[] = [
        makeMessage({ id: "m1", sequence: 1, content: "output" }),
      ];
      render(<StreamingOutput messages={messages} isLive={false} />);
      const indicator = screen.getByRole("status");
      expect(indicator).toBeInTheDocument();
    });
  });

  describe("auto-scroll behavior", () => {
    it("has a scrollable container with max-h-64 and overflow-y-auto", () => {
      const messages: TaskMessage[] = [
        makeMessage({ id: "m1", sequence: 1, content: "line" }),
      ];
      const { container } = render(
        <StreamingOutput messages={messages} isLive={true} />
      );
      const scrollContainer = container.querySelector("[role='log']");
      expect(scrollContainer).toHaveClass("max-h-64", "overflow-y-auto");
    });

    it("auto-scrolls to bottom when new messages arrive and user has not scrolled up", () => {
      const messages: TaskMessage[] = [
        makeMessage({ id: "m1", sequence: 1, content: "line 1" }),
      ];
      const { container, rerender } = render(
        <StreamingOutput messages={messages} isLive={true} />
      );
      const scrollContainer = container.querySelector("[role='log']") as HTMLElement;

      // Mock scrollHeight to simulate content taller than container
      Object.defineProperty(scrollContainer, "scrollHeight", { value: 500, configurable: true });

      const newMessages = [
        ...messages,
        makeMessage({ id: "m2", sequence: 2, content: "line 2" }),
      ];
      rerender(<StreamingOutput messages={newMessages} isLive={true} />);

      // scrollTop should be set to scrollHeight (auto-scroll)
      expect(scrollContainer.scrollTop).toBe(500);
    });

    it("does not auto-scroll when user has scrolled up more than 50px", () => {
      const messages: TaskMessage[] = [
        makeMessage({ id: "m1", sequence: 1, content: "line 1" }),
      ];
      const { container, rerender } = render(
        <StreamingOutput messages={messages} isLive={true} />
      );
      const scrollContainer = container.querySelector("[role='log']") as HTMLElement;

      // Simulate user scrolling up: scrollHeight - scrollTop - clientHeight > 50
      Object.defineProperty(scrollContainer, "scrollHeight", { value: 500, configurable: true });
      Object.defineProperty(scrollContainer, "clientHeight", { value: 200, configurable: true });
      Object.defineProperty(scrollContainer, "scrollTop", { value: 100, writable: true, configurable: true });
      // distanceFromBottom = 500 - 100 - 200 = 200 > 50

      fireEvent.scroll(scrollContainer);

      const newMessages = [
        ...messages,
        makeMessage({ id: "m2", sequence: 2, content: "line 2" }),
      ];
      rerender(<StreamingOutput messages={newMessages} isLive={true} />);

      // scrollTop should NOT be updated to scrollHeight
      expect(scrollContainer.scrollTop).toBe(100);
    });
  });
});
