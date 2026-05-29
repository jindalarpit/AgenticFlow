/**
 * Unit tests for the useTaskResultPanel hook.
 *
 * Tests session restoration, setTask/dismiss actions, and polling fallback behavior.
 *
 * Requirements: 1.7, 4.1, 4.2, 4.3, 4.4, 4.5
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement, type ReactNode } from "react";

import { useTaskResultPanel } from "../useTaskResultPanel";
import type { Task, TaskMessage } from "../useTasks";
import type { ConnectionStatus } from "../../lib/ws";

// ─── Mock: taskResultSession ─────────────────────────────────────────────────

const mockLoadState = vi.fn<[], { taskId: string; dismissed: boolean } | null>();
const mockSaveState = vi.fn<[{ taskId: string; dismissed: boolean }], void>();

vi.mock("../../lib/taskResultSession", () => ({
  loadTaskResultPanelState: (...args: unknown[]) => mockLoadState(...(args as [])),
  saveTaskResultPanelState: (...args: unknown[]) => mockSaveState(...(args as [{ taskId: string; dismissed: boolean }])),
}));

// ─── Mock: useTasks ──────────────────────────────────────────────────────────

const mockTaskData = vi.fn<[string], { data: Task | undefined }>();
const mockTaskMessagesData = vi.fn<[string], { data: TaskMessage[] | undefined }>();

vi.mock("../useTasks", () => ({
  useTask: (id: string) => mockTaskData(id),
  useTaskMessages: (id: string) => mockTaskMessagesData(id),
}));

// ─── Mock: useTaskStream ─────────────────────────────────────────────────────

const mockSeedMessages = vi.fn();
const mockStreamMessages: TaskMessage[] = [];

vi.mock("../useTaskStream", () => ({
  useTaskStream: () => ({
    messages: mockStreamMessages,
    seedMessages: mockSeedMessages,
  }),
}));

// ─── Mock: ws ────────────────────────────────────────────────────────────────

let statusChangeCallback: ((status: ConnectionStatus) => void) | null = null;
const mockUnsubscribe = vi.fn();
let mockWsStatus: ConnectionStatus = "connected";

vi.mock("../../contexts/WebSocketContext", () => ({
  useWSClient: () => ({
    get status() {
      return mockWsStatus;
    },
    onStatusChange: (cb: (status: ConnectionStatus) => void) => {
      statusChangeCallback = cb;
      return mockUnsubscribe;
    },
  }),
}));

// ─── Mock: taskResultUtils ───────────────────────────────────────────────────

vi.mock("../../lib/taskResultUtils", () => ({
  extractDashboardResult: (messages: TaskMessage[], outputPreview: string | null) => {
    const stdout = messages.filter((m) => m.stream === "stdout");
    if (stdout.length > 0) {
      let highest = stdout[0]!;
      for (const m of stdout) {
        if (m.sequence > highest.sequence) highest = m;
      }
      return highest.content;
    }
    return outputPreview ?? null;
  },
}));

// ─── Test Helpers ────────────────────────────────────────────────────────────

function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: queryClient }, children);
  };
}

function createTask(overrides: Partial<Task> = {}): Task {
  return {
    id: "task-1",
    user_id: "user-1",
    agent_type: "claude",
    prompt: "Test prompt",
    status: "running",
    exit_code: null,
    error_message: null,
    output_preview: null,
    agent_id: "agent-1",
    agent_name: "Nexus",
    started_at: "2025-01-01T00:00:01Z",
    completed_at: null,
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-01-01T00:00:00Z",
    ...overrides,
  };
}

// ─── Tests ───────────────────────────────────────────────────────────────────

describe("useTaskResultPanel", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.useFakeTimers();
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false, gcTime: Infinity },
      },
    });

    // Default mock returns
    mockLoadState.mockReturnValue(null);
    mockSaveState.mockClear();
    mockTaskData.mockReturnValue({ data: undefined });
    mockTaskMessagesData.mockReturnValue({ data: undefined });
    mockSeedMessages.mockClear();
    mockUnsubscribe.mockClear();
    statusChangeCallback = null;
    mockWsStatus = "connected";
  });

  afterEach(() => {
    vi.useRealTimers();
    queryClient.clear();
    vi.clearAllMocks();
  });

  // ─── Session Restoration ─────────────────────────────────────────────────

  describe("session restoration on mount (Requirement 4.4, 4.5)", () => {
    it("restores panelTaskId and dismissed=false from sessionStorage", () => {
      mockLoadState.mockReturnValue({ taskId: "task-saved", dismissed: false });

      const { result } = renderHook(() => useTaskResultPanel(), {
        wrapper: createWrapper(queryClient),
      });

      expect(result.current.panelTaskId).toBe("task-saved");
      expect(result.current.isVisible).toBe(true);
    });

    it("restores panelTaskId and dismissed=true from sessionStorage", () => {
      mockLoadState.mockReturnValue({ taskId: "task-saved", dismissed: true });

      const { result } = renderHook(() => useTaskResultPanel(), {
        wrapper: createWrapper(queryClient),
      });

      expect(result.current.panelTaskId).toBe("task-saved");
      expect(result.current.isVisible).toBe(false);
    });

    it("starts with null panelTaskId when sessionStorage is empty", () => {
      mockLoadState.mockReturnValue(null);

      const { result } = renderHook(() => useTaskResultPanel(), {
        wrapper: createWrapper(queryClient),
      });

      expect(result.current.panelTaskId).toBeNull();
      expect(result.current.isVisible).toBe(false);
    });
  });

  // ─── setTask ─────────────────────────────────────────────────────────────

  describe("setTask replaces previous task and clears dismissed (Requirement 4.1, 4.2)", () => {
    it("sets panelTaskId and makes panel visible", () => {
      const { result } = renderHook(() => useTaskResultPanel(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.setTask("task-new");
      });

      expect(result.current.panelTaskId).toBe("task-new");
      expect(result.current.isVisible).toBe(true);
    });

    it("replaces previous task with new one", () => {
      mockLoadState.mockReturnValue({ taskId: "task-old", dismissed: false });

      const { result } = renderHook(() => useTaskResultPanel(), {
        wrapper: createWrapper(queryClient),
      });

      expect(result.current.panelTaskId).toBe("task-old");

      act(() => {
        result.current.setTask("task-new");
      });

      expect(result.current.panelTaskId).toBe("task-new");
      expect(result.current.isVisible).toBe(true);
    });

    it("clears dismissed state when setting a new task", () => {
      mockLoadState.mockReturnValue({ taskId: "task-old", dismissed: true });

      const { result } = renderHook(() => useTaskResultPanel(), {
        wrapper: createWrapper(queryClient),
      });

      expect(result.current.isVisible).toBe(false);

      act(() => {
        result.current.setTask("task-new");
      });

      expect(result.current.isVisible).toBe(true);
    });

    it("persists new state to sessionStorage", () => {
      const { result } = renderHook(() => useTaskResultPanel(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.setTask("task-abc");
      });

      expect(mockSaveState).toHaveBeenCalledWith({
        taskId: "task-abc",
        dismissed: false,
      });
    });
  });

  // ─── dismiss ─────────────────────────────────────────────────────────────

  describe("dismiss hides panel and persists (Requirement 4.3, 4.5)", () => {
    it("hides the panel when dismissed", () => {
      const { result } = renderHook(() => useTaskResultPanel(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.setTask("task-1");
      });

      expect(result.current.isVisible).toBe(true);

      act(() => {
        result.current.dismiss();
      });

      expect(result.current.isVisible).toBe(false);
      expect(result.current.panelTaskId).toBe("task-1"); // taskId preserved
    });

    it("persists dismissed state to sessionStorage", () => {
      const { result } = renderHook(() => useTaskResultPanel(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.setTask("task-1");
      });

      mockSaveState.mockClear();

      act(() => {
        result.current.dismiss();
      });

      expect(mockSaveState).toHaveBeenCalledWith({
        taskId: "task-1",
        dismissed: true,
      });
    });
  });

  // ─── Polling Fallback ────────────────────────────────────────────────────

  describe("polling starts when WS disconnected and task is non-terminal (Requirement 1.7)", () => {
    it("starts polling when WS disconnects and task is running", async () => {
      mockTaskData.mockReturnValue({ data: createTask({ status: "running" }) });

      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      const { result } = renderHook(() => useTaskResultPanel(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.setTask("task-1");
      });

      // Simulate WS disconnect
      act(() => {
        statusChangeCallback?.("disconnected");
      });

      expect(result.current.wsConnected).toBe(false);

      // Advance timer by 3 seconds (POLL_INTERVAL_MS)
      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(invalidateSpy).toHaveBeenCalledWith({
        queryKey: ["tasks", "task-1"],
      });
      expect(invalidateSpy).toHaveBeenCalledWith({
        queryKey: ["tasks", "task-1", "messages"],
      });

      invalidateSpy.mockRestore();
    });

    it("does not poll when there is no active task", () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useTaskResultPanel(), {
        wrapper: createWrapper(queryClient),
      });

      // Simulate WS disconnect
      act(() => {
        statusChangeCallback?.("disconnected");
      });

      // Advance timer
      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(invalidateSpy).not.toHaveBeenCalled();
      invalidateSpy.mockRestore();
    });

    it("does not poll when task is in terminal status", () => {
      mockTaskData.mockReturnValue({ data: createTask({ status: "completed" }) });

      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      const { result } = renderHook(() => useTaskResultPanel(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.setTask("task-1");
      });

      // Simulate WS disconnect
      act(() => {
        statusChangeCallback?.("disconnected");
      });

      // Advance timer
      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(invalidateSpy).not.toHaveBeenCalled();
      invalidateSpy.mockRestore();
    });
  });

  // ─── Polling Stops ───────────────────────────────────────────────────────

  describe("polling stops when WS reconnects or task reaches terminal status (Requirement 1.7)", () => {
    it("stops polling when WS reconnects", () => {
      mockTaskData.mockReturnValue({ data: createTask({ status: "running" }) });

      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      const { result } = renderHook(() => useTaskResultPanel(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.setTask("task-1");
      });

      // Simulate WS disconnect
      act(() => {
        statusChangeCallback?.("disconnected");
      });

      // Verify polling is active
      act(() => {
        vi.advanceTimersByTime(3000);
      });
      expect(invalidateSpy).toHaveBeenCalled();
      invalidateSpy.mockClear();

      // Simulate WS reconnect
      act(() => {
        statusChangeCallback?.("connected");
      });

      expect(result.current.wsConnected).toBe(true);

      // Advance timer — should NOT poll anymore
      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(invalidateSpy).not.toHaveBeenCalled();
      invalidateSpy.mockRestore();
    });

    it("stops polling when task reaches terminal status (completed)", () => {
      // Start with running task
      mockTaskData.mockReturnValue({ data: createTask({ status: "running" }) });

      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      const { result, rerender } = renderHook(() => useTaskResultPanel(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.setTask("task-1");
      });

      // Simulate WS disconnect
      act(() => {
        statusChangeCallback?.("disconnected");
      });

      // Verify polling is active
      act(() => {
        vi.advanceTimersByTime(3000);
      });
      expect(invalidateSpy).toHaveBeenCalled();
      invalidateSpy.mockClear();

      // Task transitions to completed
      mockTaskData.mockReturnValue({ data: createTask({ status: "completed" }) });
      rerender();

      // Advance timer — should NOT poll anymore
      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(invalidateSpy).not.toHaveBeenCalled();
      invalidateSpy.mockRestore();
    });

    it("stops polling when task reaches terminal status (failed)", () => {
      mockTaskData.mockReturnValue({ data: createTask({ status: "running" }) });

      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      const { result, rerender } = renderHook(() => useTaskResultPanel(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.setTask("task-1");
      });

      // Simulate WS disconnect
      act(() => {
        statusChangeCallback?.("disconnected");
      });

      // Verify polling is active
      act(() => {
        vi.advanceTimersByTime(3000);
      });
      expect(invalidateSpy).toHaveBeenCalled();
      invalidateSpy.mockClear();

      // Task transitions to failed
      mockTaskData.mockReturnValue({ data: createTask({ status: "failed" }) });
      rerender();

      // Advance timer — should NOT poll anymore
      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(invalidateSpy).not.toHaveBeenCalled();
      invalidateSpy.mockRestore();
    });
  });
});
