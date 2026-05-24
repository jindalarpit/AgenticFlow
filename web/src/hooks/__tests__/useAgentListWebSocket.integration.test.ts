/**
 * Integration tests for WebSocket event handling in the Agent List.
 *
 * Tests that WebSocket events correctly update the React Query cache,
 * which drives the agent list UI.
 *
 * Requirements: 15.1–15.5
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider, useQueryClient } from "@tanstack/react-query";
import { createElement, type ReactNode } from "react";

import { useAgentListWebSocket } from "../useAgentListWebSocket";
import type { AgentListItem } from "../useAgentList";
import { wsClient, type WSEvent } from "../../lib/ws";

// ─── Mock wsClient ───────────────────────────────────────────────────────────

vi.mock("../../lib/ws", () => {
  const handlers = new Map<string, Set<(event: WSEvent) => void>>();

  return {
    wsClient: {
      on: vi.fn((eventType: string, handler: (event: WSEvent) => void) => {
        if (!handlers.has(eventType)) {
          handlers.set(eventType, new Set());
        }
        handlers.get(eventType)!.add(handler);
        return () => {
          handlers.get(eventType)?.delete(handler);
        };
      }),
      // Helper to simulate events in tests
      __simulateEvent: (event: WSEvent) => {
        const typeHandlers = handlers.get(event.type);
        if (typeHandlers) {
          typeHandlers.forEach((handler) => handler(event));
        }
      },
      __clearHandlers: () => {
        handlers.clear();
      },
    },
  };
});

// Access the test helper
const simulateEvent = (wsClient as unknown as { __simulateEvent: (e: WSEvent) => void }).__simulateEvent;
const clearHandlers = (wsClient as unknown as { __clearHandlers: () => void }).__clearHandlers;

// ─── Test Helpers ────────────────────────────────────────────────────────────

function createAgent(overrides: Partial<AgentListItem> = {}): AgentListItem {
  return {
    id: "agent-1",
    name: "Test Agent",
    description: "A test agent",
    instructions: "",
    avatar_url: null,
    runtime_id: "runtime-1",
    custom_env: {},
    custom_args: [],
    model: "claude-sonnet-4-20250514",
    visibility: "shared",
    status: "idle",
    max_concurrent_tasks: 1,
    owner_id: "user-1",
    archived_at: null,
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-01-01T00:00:00Z",
    ...overrides,
  };
}

function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: queryClient }, children);
  };
}

// ─── Tests ───────────────────────────────────────────────────────────────────

describe("useAgentListWebSocket integration", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false, gcTime: Infinity },
      },
    });
    clearHandlers();
  });

  afterEach(() => {
    queryClient.clear();
  });

  describe("agent_status_changed event (Requirement 15.1)", () => {
    it("updates the correct agent's status in the cache", async () => {
      const agents: AgentListItem[] = [
        createAgent({ id: "agent-1", status: "idle" }),
        createAgent({ id: "agent-2", status: "idle", name: "Agent 2" }),
      ];

      // Seed the cache
      queryClient.setQueryData<AgentListItem[]>(["agents"], agents);

      // Render the hook
      renderHook(() => useAgentListWebSocket(), {
        wrapper: createWrapper(queryClient),
      });

      // Simulate the WebSocket event
      act(() => {
        simulateEvent({
          type: "agent_status_changed",
          payload: { agent_id: "agent-1", status: "working" },
        });
      });

      // Verify the cache was updated
      const cached = queryClient.getQueryData<AgentListItem[]>(["agents"]);
      expect(cached).toHaveLength(2);
      expect(cached![0].id).toBe("agent-1");
      expect(cached![0].status).toBe("working");
      // Other agent should be unchanged
      expect(cached![1].id).toBe("agent-2");
      expect(cached![1].status).toBe("idle");
    });

    it("does not modify other fields of the agent", () => {
      const agents: AgentListItem[] = [
        createAgent({ id: "agent-1", status: "idle", name: "Original Name" }),
      ];

      queryClient.setQueryData<AgentListItem[]>(["agents"], agents);

      renderHook(() => useAgentListWebSocket(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        simulateEvent({
          type: "agent_status_changed",
          payload: { agent_id: "agent-1", status: "working" },
        });
      });

      const cached = queryClient.getQueryData<AgentListItem[]>(["agents"]);
      expect(cached![0].name).toBe("Original Name");
      expect(cached![0].status).toBe("working");
    });

    it("falls back to invalidateQueries when payload is missing agent_id", () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      queryClient.setQueryData<AgentListItem[]>(["agents"], [createAgent()]);

      renderHook(() => useAgentListWebSocket(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        simulateEvent({
          type: "agent_status_changed",
          payload: {},
        });
      });

      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["agents"] });
      invalidateSpy.mockRestore();
    });
  });

  describe("agent_created event (Requirement 15.2)", () => {
    it("invalidates the agents query to trigger a refetch", () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      queryClient.setQueryData<AgentListItem[]>(["agents"], [createAgent()]);

      renderHook(() => useAgentListWebSocket(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        simulateEvent({
          type: "agent_created",
          payload: { id: "agent-new", name: "New Agent" },
        });
      });

      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["agents"] });
      invalidateSpy.mockRestore();
    });

    it("triggers refetch regardless of payload content", () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      queryClient.setQueryData<AgentListItem[]>(["agents"], []);

      renderHook(() => useAgentListWebSocket(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        simulateEvent({
          type: "agent_created",
          payload: null,
        });
      });

      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["agents"] });
      invalidateSpy.mockRestore();
    });
  });

  describe("agent_deleted event (Requirement 15.4)", () => {
    it("removes the agent from the cache", () => {
      const agents: AgentListItem[] = [
        createAgent({ id: "agent-1", name: "Agent 1" }),
        createAgent({ id: "agent-2", name: "Agent 2" }),
        createAgent({ id: "agent-3", name: "Agent 3" }),
      ];

      queryClient.setQueryData<AgentListItem[]>(["agents"], agents);

      renderHook(() => useAgentListWebSocket(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        simulateEvent({
          type: "agent_deleted",
          payload: { agent_id: "agent-2" },
        });
      });

      const cached = queryClient.getQueryData<AgentListItem[]>(["agents"]);
      expect(cached).toHaveLength(2);
      expect(cached!.map((a) => a.id)).toEqual(["agent-1", "agent-3"]);
    });

    it("does nothing when agent_id does not exist in cache", () => {
      const agents: AgentListItem[] = [
        createAgent({ id: "agent-1" }),
      ];

      queryClient.setQueryData<AgentListItem[]>(["agents"], agents);

      renderHook(() => useAgentListWebSocket(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        simulateEvent({
          type: "agent_deleted",
          payload: { agent_id: "nonexistent" },
        });
      });

      const cached = queryClient.getQueryData<AgentListItem[]>(["agents"]);
      expect(cached).toHaveLength(1);
      expect(cached![0].id).toBe("agent-1");
    });

    it("falls back to invalidateQueries when payload is missing agent_id", () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      queryClient.setQueryData<AgentListItem[]>(["agents"], [createAgent()]);

      renderHook(() => useAgentListWebSocket(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        simulateEvent({
          type: "agent_deleted",
          payload: {},
        });
      });

      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["agents"] });
      invalidateSpy.mockRestore();
    });
  });

  describe("daemon_disconnected event (Requirement 15.5)", () => {
    it("invalidates both daemons and agents queries", () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      queryClient.setQueryData<AgentListItem[]>(["agents"], [
        createAgent({ id: "agent-1", runtime_id: "runtime-on-daemon-1" }),
        createAgent({ id: "agent-2", runtime_id: "runtime-on-daemon-1" }),
      ]);

      renderHook(() => useAgentListWebSocket(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        simulateEvent({
          type: "daemon_disconnected",
          payload: { daemon_id: "daemon-1" },
        });
      });

      // Should invalidate both daemons and agents queries
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["daemons"] });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["agents"] });
      invalidateSpy.mockRestore();
    });

    it("triggers refetch even when payload is empty", () => {
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      queryClient.setQueryData<AgentListItem[]>(["agents"], []);

      renderHook(() => useAgentListWebSocket(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        simulateEvent({
          type: "daemon_disconnected",
          payload: null,
        });
      });

      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["daemons"] });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["agents"] });
      invalidateSpy.mockRestore();
    });
  });

  describe("cleanup on unmount", () => {
    it("unsubscribes from all events when the hook unmounts", () => {
      queryClient.setQueryData<AgentListItem[]>(["agents"], [createAgent()]);

      const { unmount } = renderHook(() => useAgentListWebSocket(), {
        wrapper: createWrapper(queryClient),
      });

      // Unmount the hook
      unmount();

      // Simulate an event after unmount — should not modify cache
      const agentsBefore = queryClient.getQueryData<AgentListItem[]>(["agents"]);

      act(() => {
        simulateEvent({
          type: "agent_status_changed",
          payload: { agent_id: "agent-1", status: "working" },
        });
      });

      const agentsAfter = queryClient.getQueryData<AgentListItem[]>(["agents"]);
      // Status should remain unchanged since handler was unsubscribed
      expect(agentsAfter![0].status).toBe(agentsBefore![0].status);
    });
  });
});
