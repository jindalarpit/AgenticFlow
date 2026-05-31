import { describe, it, expect, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { TaskResultPanel } from "../index";
import type { Task } from "../../../hooks/useTasks";

// Mock useCancelTask from the hooks module
const mockMutate = vi.fn();
let mockIsPending = false;
vi.mock("../../../hooks/useTasks", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../hooks/useTasks")>();
  return {
    ...actual,
    useCancelTask: () => ({
      mutate: mockMutate,
      isPending: mockIsPending,
    }),
  };
});

function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    id: "task-123",
    user_id: "user-1",
    agent_type: "claude",
    prompt: "Fix the login bug",
    status: "completed",
    exit_code: 0,
    error_message: null,
    output_preview: "Task completed successfully",
    agent_id: "agent-1",
    agent_name: "Nexus",
    started_at: "2025-01-01T00:00:00Z",
    completed_at: "2025-01-01T00:01:00Z",
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-01-01T00:01:00Z",
    token_usage: null,
    ...overrides,
  };
}

function renderPanel(props: Partial<React.ComponentProps<typeof TaskResultPanel>> = {}) {
  const defaultProps = {
    taskId: "task-123",
    onDismiss: vi.fn(),
    ...props,
  };
  return render(
    <MemoryRouter>
      <TaskResultPanel {...defaultProps} />
    </MemoryRouter>
  );
}

describe("TaskResultPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsPending = false;
  });

  describe("loading state", () => {
    it("renders loading state when task is undefined", () => {
      renderPanel({ task: undefined });
      expect(screen.getByText("Loading task...")).toBeInTheDocument();
    });
  });

  describe("pending status", () => {
    it("renders pending status badge and spinner when task.status is 'pending'", () => {
      renderPanel({ task: makeTask({ status: "pending" }) });
      expect(screen.getByText("Pending")).toBeInTheDocument();
      // Spinner is an SVG with animate-spin class
      const spinner = document.querySelector("svg.animate-spin");
      expect(spinner).toBeInTheDocument();
    });
  });

  describe("running status", () => {
    it("renders running status badge and pulsing dot when task.status is 'running'", () => {
      renderPanel({ task: makeTask({ status: "running" }) });
      expect(screen.getByText("Running")).toBeInTheDocument();
      // Pulsing dot uses animate-ping class
      const pulsingDot = document.querySelector(".animate-ping");
      expect(pulsingDot).toBeInTheDocument();
    });
  });

  describe("completed state", () => {
    it("renders completed state with timeline view and status badge", () => {
      renderPanel({
        task: makeTask({ status: "completed", output_preview: "Final answer here" }),
        messages: [],
      });
      expect(screen.getByText("Completed")).toBeInTheDocument();
      // Unified timeline view renders "No events yet" when messages are empty
      expect(screen.getByText("No events yet")).toBeInTheDocument();
    });
  });

  describe("failed state", () => {
    it("renders failed state with timeline view and status badge", () => {
      renderPanel({
        task: makeTask({ status: "failed", error_message: "Connection refused" }),
      });
      // Status badge in header
      const failedBadges = screen.getAllByText("Failed");
      expect(failedBadges.length).toBeGreaterThanOrEqual(1);
      // Unified timeline view renders controls (Filter, Sort, Copy)
      expect(screen.getByRole("group", { name: "Sort direction" })).toBeInTheDocument();
    });
  });

  describe("cancelled state", () => {
    it("renders cancelled state with timeline view and status badge", () => {
      renderPanel({
        task: makeTask({ status: "cancelled" }),
      });
      // Status badge in header
      const cancelledBadges = screen.getAllByText("Cancelled");
      expect(cancelledBadges.length).toBeGreaterThanOrEqual(1);
      // Unified timeline view renders controls
      expect(screen.getByRole("group", { name: "Sort direction" })).toBeInTheDocument();
    });
  });

  describe("timeout state", () => {
    it("renders timeout state with timeline view and status badge", () => {
      renderPanel({
        task: makeTask({ status: "timeout" }),
      });
      // Status badge in header
      const timeoutBadges = screen.getAllByText("Timeout");
      expect(timeoutBadges.length).toBeGreaterThanOrEqual(1);
      // Unified timeline view renders controls
      expect(screen.getByRole("group", { name: "Sort direction" })).toBeInTheDocument();
    });
  });

  describe("dismiss button", () => {
    it("calls onDismiss when dismiss button is clicked", () => {
      const onDismiss = vi.fn();
      renderPanel({ task: makeTask(), onDismiss });
      const dismissButton = screen.getByLabelText("Dismiss result panel");
      fireEvent.click(dismissButton);
      expect(onDismiss).toHaveBeenCalledTimes(1);
    });
  });

  describe("copy button", () => {
    it("shows 'Copied' feedback after click", async () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.assign(navigator, {
        clipboard: { writeText },
      });

      renderPanel({
        task: makeTask({ status: "completed", output_preview: "Some result" }),
        messages: [],
      });

      const copyButton = screen.getByLabelText("Copy result to clipboard");
      fireEvent.click(copyButton);

      await waitFor(() => {
        expect(screen.getByText("Copied")).toBeInTheDocument();
      });
      expect(writeText).toHaveBeenCalledWith("Some result");
    });
  });

  describe("cancel button", () => {
    it("shows 'Cancelling…' when mutation is pending", () => {
      mockIsPending = true;
      renderPanel({
        task: makeTask({ status: "running" }),
      });
      expect(screen.getByText("Cancelling…")).toBeInTheDocument();
      const cancelButton = screen.getByLabelText("Cancel task");
      expect(cancelButton).toBeDisabled();
      mockIsPending = false;
    });
  });

  describe("View Detail link", () => {
    it("has correct href to /tasks/:id", () => {
      renderPanel({
        taskId: "task-abc-123",
        task: makeTask({ id: "task-abc-123", status: "completed", output_preview: "result" }),
        messages: [],
      });
      const link = screen.getByText("View Detail");
      expect(link).toHaveAttribute("href", "/tasks/task-abc-123");
    });
  });
});
