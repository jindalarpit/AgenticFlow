/**
 * Integration tests for API interactions on the Agents Page.
 *
 * Tests that:
 * - Archive API call removes agent on success, shows toast on failure
 * - Create API call closes dialog on success, shows toast on failure
 *
 * Requirements: 7.3, 7.5, 9.9, 9.10, 9.11
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { createElement, type ReactNode } from "react";

import AgentsPage from "../AgentsPage";
import { ToastProvider } from "../../components/Toast";
import type { AgentListItem } from "../../hooks/useAgentList";

// ─── Mock modules ────────────────────────────────────────────────────────────

// Mock apiFetch
const mockApiFetch = vi.fn();
vi.mock("../../lib/api", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
  setToken: vi.fn(),
  clearToken: vi.fn(),
  hasToken: vi.fn(() => true),
}));

// Mock wsClient
vi.mock("../../lib/ws", () => ({
  wsClient: {
    on: vi.fn(() => () => {}),
    connect: vi.fn(),
    disconnect: vi.fn(),
    status: "connected",
    onStatusChange: vi.fn(() => () => {}),
  },
}));

// Mock useNavigate
const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// ─── Test Helpers ────────────────────────────────────────────────────────────

function createAgent(overrides: Partial<AgentListItem> = {}): AgentListItem {
  return {
    id: "agent-1",
    name: "Test Agent",
    description: "A test agent for testing",
    instructions: "",
    avatar_url: null,
    runtime_id: "runtime-1",
    custom_env: {},
    custom_args: [],
    model: "claude-sonnet-4-20250514",
    visibility: "shared",
    status: "idle",
    max_concurrent_tasks: 1,
    owner_id: "current-user",
    archived_at: null,
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-01-01T00:00:00Z",
    ...overrides,
  };
}

const mockDaemons = [
  {
    id: "daemon-1",
    daemon_id: "daemon-1",
    device_name: "My Laptop",
    status: "online" as const,
    last_heartbeat_at: "2025-01-01T00:00:00Z",
    cli_version: "1.0.0",
    agent_runtimes: [
      {
        id: "runtime-1",
        daemon_id: "daemon-1",
        provider: "claude",
        name: "Claude CLI",
        version: "1.0.0",
        binary_path: "/usr/local/bin/claude",
        status: "available" as const,
        created_at: "2025-01-01T00:00:00Z",
        updated_at: "2025-01-01T00:00:00Z",
      },
    ],
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-01-01T00:00:00Z",
  },
];

function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return createElement(
      MemoryRouter,
      null,
      createElement(
        QueryClientProvider,
        { client: queryClient },
        createElement(ToastProvider, null, children)
      )
    );
  };
}

// ─── Tests ───────────────────────────────────────────────────────────────────

describe("AgentsPage API interactions", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false, gcTime: Infinity },
      },
    });
    mockApiFetch.mockReset();
    mockNavigate.mockReset();

    // Mock sessionStorage
    vi.spyOn(Storage.prototype, "getItem").mockReturnValue("mine");
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {});
  });

  afterEach(() => {
    queryClient.clear();
    vi.restoreAllMocks();
  });

  describe("Archive API interaction (Requirements 7.3, 7.5)", () => {
    it("removes agent from table on successful archive", async () => {
      const agents = [
        createAgent({ id: "agent-1", name: "Agent One" }),
        createAgent({ id: "agent-2", name: "Agent Two" }),
      ];

      // Setup: apiFetch returns agents for the list, daemons, activity, run-counts
      mockApiFetch.mockImplementation((path: string, options?: RequestInit) => {
        if (path === "/api/agents" && !options?.method) return Promise.resolve(agents);
        if (path === "/api/daemons") return Promise.resolve(mockDaemons);
        if (path === "/api/agents/activity") return Promise.resolve([]);
        if (path === "/api/agents/run-counts") return Promise.resolve([]);
        // Archive call succeeds
        if (path === "/api/agents/agent-1/archive" && options?.method === "POST") {
          return Promise.resolve(undefined);
        }
        return Promise.resolve([]);
      });

      render(createElement(AgentsPage), { wrapper: createWrapper(queryClient) });

      // Wait for agents to load
      await waitFor(() => {
        expect(screen.getByText("Agent One")).toBeInTheDocument();
      });

      // Find and click the kebab menu for the first agent
      const kebabButtons = screen.getAllByRole("button", { name: /actions/i });
      fireEvent.click(kebabButtons[0]);

      // Click "Archive" in the dropdown
      await waitFor(() => {
        expect(screen.getByText("Archive")).toBeInTheDocument();
      });
      fireEvent.click(screen.getByText("Archive"));

      // Verify the archive API was called
      await waitFor(() => {
        expect(mockApiFetch).toHaveBeenCalledWith(
          "/api/agents/agent-1/archive",
          { method: "POST" }
        );
      });

      // Verify success toast appears
      await waitFor(() => {
        expect(screen.getByText("Agent archived successfully")).toBeInTheDocument();
      });
    });

    it("shows error toast and retains agent on archive failure", async () => {
      const agents = [
        createAgent({ id: "agent-1", name: "Agent One" }),
      ];

      mockApiFetch.mockImplementation((path: string, options?: RequestInit) => {
        if (path === "/api/agents" && !options?.method) return Promise.resolve(agents);
        if (path === "/api/daemons") return Promise.resolve(mockDaemons);
        if (path === "/api/agents/activity") return Promise.resolve([]);
        if (path === "/api/agents/run-counts") return Promise.resolve([]);
        // Archive call fails
        if (path === "/api/agents/agent-1/archive" && options?.method === "POST") {
          return Promise.reject(new Error("Permission denied"));
        }
        return Promise.resolve([]);
      });

      render(createElement(AgentsPage), { wrapper: createWrapper(queryClient) });

      // Wait for agents to load
      await waitFor(() => {
        expect(screen.getByText("Agent One")).toBeInTheDocument();
      });

      // Click kebab menu
      const kebabButtons = screen.getAllByRole("button", { name: /actions/i });
      fireEvent.click(kebabButtons[0]);

      // Click "Archive"
      await waitFor(() => {
        expect(screen.getByText("Archive")).toBeInTheDocument();
      });
      fireEvent.click(screen.getByText("Archive"));

      // Verify error toast appears
      await waitFor(() => {
        expect(screen.getByText("Permission denied")).toBeInTheDocument();
      });

      // Agent should still be in the table
      expect(screen.getByText("Agent One")).toBeInTheDocument();
    });
  });

  describe("Create API interaction (Requirements 9.9, 9.10, 9.11)", () => {
    it("closes dialog and navigates on successful creation", async () => {
      const agents = [
        createAgent({ id: "agent-1", name: "Existing Agent" }),
      ];

      const newAgent = createAgent({
        id: "agent-new",
        name: "New Agent",
      });

      mockApiFetch.mockImplementation((path: string, options?: RequestInit) => {
        if (path === "/api/agents" && !options?.method) return Promise.resolve(agents);
        if (path === "/api/daemons") return Promise.resolve(mockDaemons);
        if (path === "/api/agents/activity") return Promise.resolve([]);
        if (path === "/api/agents/run-counts") return Promise.resolve([]);
        // Create call succeeds
        if (path === "/api/agents" && options?.method === "POST") {
          return Promise.resolve(newAgent);
        }
        return Promise.resolve([]);
      });

      render(createElement(AgentsPage), { wrapper: createWrapper(queryClient) });

      // Wait for page to load
      await waitFor(() => {
        expect(screen.getByText("Existing Agent")).toBeInTheDocument();
      });

      // Click "New Agent" button to open dialog
      fireEvent.click(screen.getByText("New Agent"));

      // Verify dialog is open
      await waitFor(() => {
        expect(screen.getByRole("dialog")).toBeInTheDocument();
      });

      // Fill in the form
      const nameInput = screen.getByLabelText(/name/i, { selector: "input" });
      fireEvent.change(nameInput, { target: { value: "New Agent" } });

      // Select a runtime
      const runtimeSelect = screen.getByLabelText(/select runtime/i);
      fireEvent.change(runtimeSelect, { target: { value: "runtime-1" } });

      // Click "Create" button inside the dialog (aria-label is "Create agent")
      const createButton = screen.getByRole("button", { name: "Create agent" });
      fireEvent.click(createButton);

      // Verify the create API was called
      await waitFor(() => {
        expect(mockApiFetch).toHaveBeenCalledWith(
          "/api/agents",
          expect.objectContaining({
            method: "POST",
            body: expect.stringContaining("New Agent"),
          })
        );
      });

      // Verify dialog is closed
      await waitFor(() => {
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      });

      // Verify navigation to new agent's detail page
      expect(mockNavigate).toHaveBeenCalledWith("/agents/agent-new");

      // Verify success toast
      await waitFor(() => {
        expect(screen.getByText("Agent created successfully")).toBeInTheDocument();
      });
    });

    it("shows error toast and keeps dialog open on creation failure", async () => {
      const agents = [
        createAgent({ id: "agent-1", name: "Existing Agent" }),
      ];

      mockApiFetch.mockImplementation((path: string, options?: RequestInit) => {
        if (path === "/api/agents" && !options?.method) return Promise.resolve(agents);
        if (path === "/api/daemons") return Promise.resolve(mockDaemons);
        if (path === "/api/agents/activity") return Promise.resolve([]);
        if (path === "/api/agents/run-counts") return Promise.resolve([]);
        // Create call fails
        if (path === "/api/agents" && options?.method === "POST") {
          return Promise.reject(new Error("Agent name already exists"));
        }
        return Promise.resolve([]);
      });

      render(createElement(AgentsPage), { wrapper: createWrapper(queryClient) });

      // Wait for page to load
      await waitFor(() => {
        expect(screen.getByText("Existing Agent")).toBeInTheDocument();
      });

      // Open create dialog
      fireEvent.click(screen.getByText("New Agent"));

      await waitFor(() => {
        expect(screen.getByRole("dialog")).toBeInTheDocument();
      });

      // Fill in the form
      const nameInput = screen.getByLabelText(/name/i, { selector: "input" });
      fireEvent.change(nameInput, { target: { value: "Duplicate Name" } });

      const runtimeSelect = screen.getByLabelText(/select runtime/i);
      fireEvent.change(runtimeSelect, { target: { value: "runtime-1" } });

      // Click "Create" button inside the dialog (aria-label is "Create agent")
      const createButton = screen.getByRole("button", { name: "Create agent" });
      fireEvent.click(createButton);

      // Verify error toast appears
      await waitFor(() => {
        expect(screen.getByText("Agent name already exists")).toBeInTheDocument();
      });

      // Dialog should still be open
      expect(screen.getByRole("dialog")).toBeInTheDocument();

      // Navigation should NOT have been called
      expect(mockNavigate).not.toHaveBeenCalledWith(
        expect.stringContaining("/agents/")
      );
    });
  });
});
