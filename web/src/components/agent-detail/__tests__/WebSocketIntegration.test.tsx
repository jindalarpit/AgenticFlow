/**
 * Integration tests for WebSocket event handling in the Agent Detail UI.
 *
 * Tests that:
 * - task_created/task_completed/task_failed events invalidate correct queries
 * - daemon_connected/daemon_disconnected events invalidate agents query
 * - Events for a different agent_id are ignored
 * - Reconnection refetches all agent data
 * - Optimistic update immediately updates UI
 * - Optimistic rollback reverts on API failure
 *
 * Validates: Requirements 16.1, 16.2, 16.3, 16.4, 17.1, 17.2
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { TaskEvent } from "../../../lib/agent-detail-types";

/* ─── Mock wsClient ─── */

const wsHandlers: Map<string, Set<(event: { type: string; payload: unknown }) => void>> = new Map();
const wsStatusHandlers: Set<(status: string) => void> = new Set();

const mockWsClient = {
  on: vi.fn((eventType: string, handler: (event: { type: string; payload: unknown }) => void) => {
    if (!wsHandlers.has(eventType)) {
      wsHandlers.set(eventType, new Set());
    }
    wsHandlers.get(eventType)!.add(handler);
    return () => {
      wsHandlers.get(eventType)?.delete(handler);
    };
  }),
  onStatusChange: vi.fn((handler: (status: string) => void) => {
    wsStatusHandlers.add(handler);
    return () => {
      wsStatusHandlers.delete(handler);
    };
  }),
};

vi.mock("../../../contexts/WebSocketContext", () => ({
  useWSClient: () => mockWsClient,
}));

/* ─── Mock apiFetch ─── */

vi.mock("../../../lib/api", () => ({
  apiFetch: vi.fn(),
}));

/* ─── Import hooks after mocks ─── */

import { useAgentWebSocket } from "../../../hooks/useAgentWebSocket";
import { useUpdateAgent } from "../../../hooks/useAgentDetail";
import { apiFetch } from "../../../lib/api";
import type { Agent } from "../../../lib/agent-detail-types";

const mockApiFetch = apiFetch as ReturnType<typeof vi.fn>;

/* ─── Test Helpers ─── */

const AGENT_ID = "agent-123";
const OTHER_AGENT_ID = "agent-456";

// Access internal handler maps from the mock
const getMockWsClient = () => ({
  on: mockWsClient.on,
  onStatusChange: mockWsClient.onStatusChange,
  __getHandlers: () => wsHandlers,
  __getStatusHandlers: () => wsStatusHandlers,
});

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: Infinity,
      },
      mutations: {
        retry: false,
      },
    },
  });
}

function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(
      QueryClientProvider,
      { client: queryClient },
      children
    );
  };
}

function emitEvent(eventType: string, payload: unknown) {
  const handlers = getMockWsClient().__getHandlers().get(eventType);
  if (handlers) {
    handlers.forEach((handler) => handler({ type: eventType, payload }));
  }
}

function emitStatusChange(status: string) {
  getMockWsClient().__getStatusHandlers().forEach((handler) => handler(status));
}

function createMockAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: AGENT_ID,
    name: "Test Agent",
    description: "A test agent",
    instructions: "Be helpful",
    avatar_url: null,
    runtime_id: "runtime-1",
    runtime_name: "Local Runtime",
    custom_env: {},
    custom_args: [],
    model: "claude-sonnet",
    visibility: "private",
    status: "idle",
    max_concurrent_tasks: 1,
    owner_id: "user-1",
    owner_name: "Test User",
    skills: [],
    mcp_config: null,
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-01-01T00:00:00Z",
    ...overrides,
  };
}

/* ─── Tests ─── */

describe("WebSocket Integration — Agent Detail", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = createQueryClient();
    getMockWsClient().on.mockClear();
    getMockWsClient().onStatusChange.mockClear();
    mockApiFetch.mockReset();
  });

  afterEach(() => {
    queryClient.clear();
  });

  describe("Task events invalidate correct queries", () => {
    it("task_created event for this agent invalidates agent-tasks, agents, and agent-stats queries", async () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useAgentWebSocket(AGENT_ID), {
        wrapper: createWrapper(queryClient),
      });

      const taskEvent: TaskEvent = {
        task_id: "task-1",
        agent_id: AGENT_ID,
        status: "pending",
        prompt: "Do something",
      };

      act(() => {
        emitEvent("task_created", taskEvent);
      });

      expect(invalidateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ queryKey: ["agent-tasks", AGENT_ID] })
      );
      expect(invalidateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ queryKey: ["agents", AGENT_ID] })
      );
      expect(invalidateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ queryKey: ["agent-stats", AGENT_ID] })
      );
    });

    it("task_completed event for this agent invalidates agent-tasks, agents, and agent-stats queries", async () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useAgentWebSocket(AGENT_ID), {
        wrapper: createWrapper(queryClient),
      });

      const taskEvent: TaskEvent = {
        task_id: "task-2",
        agent_id: AGENT_ID,
        status: "completed",
      };

      act(() => {
        emitEvent("task_completed", taskEvent);
      });

      expect(invalidateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ queryKey: ["agent-tasks", AGENT_ID] })
      );
      expect(invalidateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ queryKey: ["agents", AGENT_ID] })
      );
      expect(invalidateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ queryKey: ["agent-stats", AGENT_ID] })
      );
    });

    it("task_failed event for this agent invalidates agent-tasks, agents, and agent-stats queries", async () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useAgentWebSocket(AGENT_ID), {
        wrapper: createWrapper(queryClient),
      });

      const taskEvent: TaskEvent = {
        task_id: "task-3",
        agent_id: AGENT_ID,
        status: "failed",
        failure_reason: "Timeout",
      };

      act(() => {
        emitEvent("task_failed", taskEvent);
      });

      expect(invalidateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ queryKey: ["agent-tasks", AGENT_ID] })
      );
      expect(invalidateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ queryKey: ["agents", AGENT_ID] })
      );
      expect(invalidateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ queryKey: ["agent-stats", AGENT_ID] })
      );
    });
  });

  describe("Events for a different agent_id are ignored", () => {
    it("task_created event for a different agent does not invalidate queries", () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useAgentWebSocket(AGENT_ID), {
        wrapper: createWrapper(queryClient),
      });

      const taskEvent: TaskEvent = {
        task_id: "task-4",
        agent_id: OTHER_AGENT_ID,
        status: "pending",
        prompt: "Other agent task",
      };

      act(() => {
        emitEvent("task_created", taskEvent);
      });

      expect(invalidateSpy).not.toHaveBeenCalled();
    });

    it("task_completed event for a different agent does not invalidate queries", () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useAgentWebSocket(AGENT_ID), {
        wrapper: createWrapper(queryClient),
      });

      const taskEvent: TaskEvent = {
        task_id: "task-5",
        agent_id: OTHER_AGENT_ID,
        status: "completed",
      };

      act(() => {
        emitEvent("task_completed", taskEvent);
      });

      expect(invalidateSpy).not.toHaveBeenCalled();
    });

    it("task_failed event for a different agent does not invalidate queries", () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useAgentWebSocket(AGENT_ID), {
        wrapper: createWrapper(queryClient),
      });

      const taskEvent: TaskEvent = {
        task_id: "task-6",
        agent_id: OTHER_AGENT_ID,
        status: "failed",
        failure_reason: "Error",
      };

      act(() => {
        emitEvent("task_failed", taskEvent);
      });

      expect(invalidateSpy).not.toHaveBeenCalled();
    });
  });

  describe("Daemon events invalidate agents query", () => {
    it("daemon_connected event invalidates agents query for this agent", () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useAgentWebSocket(AGENT_ID), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        emitEvent("daemon_connected", {
          daemon_id: "daemon-1",
          runtime_ids: ["runtime-1"],
        });
      });

      expect(invalidateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ queryKey: ["agents", AGENT_ID] })
      );
    });

    it("daemon_disconnected event invalidates agents query for this agent", () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useAgentWebSocket(AGENT_ID), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        emitEvent("daemon_disconnected", {
          daemon_id: "daemon-1",
          runtime_ids: ["runtime-1"],
        });
      });

      expect(invalidateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ queryKey: ["agents", AGENT_ID] })
      );
    });
  });

  describe("Reconnection refetches all agent data", () => {
    it("status change to 'connected' invalidates agents, agent-tasks, and agent-stats queries", () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useAgentWebSocket(AGENT_ID), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        emitStatusChange("connected");
      });

      expect(invalidateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ queryKey: ["agents", AGENT_ID] })
      );
      expect(invalidateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ queryKey: ["agent-tasks", AGENT_ID] })
      );
      expect(invalidateSpy).toHaveBeenCalledWith(
        expect.objectContaining({ queryKey: ["agent-stats", AGENT_ID] })
      );
    });

    it("status change to 'disconnected' does not invalidate queries", () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useAgentWebSocket(AGENT_ID), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        emitStatusChange("disconnected");
      });

      expect(invalidateSpy).not.toHaveBeenCalled();
    });
  });

  describe("Optimistic update and rollback", () => {
    it("optimistic update: property change immediately updates cache before server responds", async () => {
      const mockAgent = createMockAgent({ model: "old-model" });
      queryClient.setQueryData(["agents", AGENT_ID], mockAgent);

      // Mock a delayed API response
      mockApiFetch.mockImplementation(
        () =>
          new Promise((resolve) =>
            setTimeout(() => resolve({ ...mockAgent, model: "new-model" }), 100)
          )
      );

      const { result } = renderHook(() => useUpdateAgent(AGENT_ID), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.mutate({ model: "new-model" });
      });

      // Cache should be updated optimistically before server responds
      await waitFor(() => {
        const cached = queryClient.getQueryData<Agent>(["agents", AGENT_ID]);
        expect(cached?.model).toBe("new-model");
      });
    });

    it("optimistic rollback: failed mutation reverts to previous value", async () => {
      const mockAgent = createMockAgent({ model: "original-model" });
      queryClient.setQueryData(["agents", AGENT_ID], mockAgent);

      // Mock API failure
      mockApiFetch.mockRejectedValue(new Error("Server error"));

      const { result } = renderHook(() => useUpdateAgent(AGENT_ID), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.mutate({ model: "attempted-model" });
      });

      // Wait for the mutation to settle (error + rollback)
      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      // Cache should be reverted to original value
      const cached = queryClient.getQueryData<Agent>(["agents", AGENT_ID]);
      expect(cached?.model).toBe("original-model");
    });
  });

  describe("Cleanup on unmount", () => {
    it("unsubscribes from all events when hook unmounts", () => {
      const { unmount } = renderHook(() => useAgentWebSocket(AGENT_ID), {
        wrapper: createWrapper(queryClient),
      });

      const mock = getMockWsClient();
      const handlers = mock.__getHandlers();
      const statusHandlers = mock.__getStatusHandlers();

      // Verify handlers were registered
      expect(mock.on).toHaveBeenCalledTimes(5);
      expect(mock.onStatusChange).toHaveBeenCalledTimes(1);

      // Verify handlers exist
      expect(handlers.get("task_created")?.size).toBe(1);
      expect(handlers.get("task_completed")?.size).toBe(1);
      expect(handlers.get("task_failed")?.size).toBe(1);
      expect(handlers.get("daemon_connected")?.size).toBe(1);
      expect(handlers.get("daemon_disconnected")?.size).toBe(1);
      expect(statusHandlers.size).toBe(1);

      unmount();

      // After unmount, handlers should be removed
      expect(handlers.get("task_created")?.size).toBe(0);
      expect(handlers.get("task_completed")?.size).toBe(0);
      expect(handlers.get("task_failed")?.size).toBe(0);
      expect(handlers.get("daemon_connected")?.size).toBe(0);
      expect(handlers.get("daemon_disconnected")?.size).toBe(0);
      expect(statusHandlers.size).toBe(0);
    });
  });
});
