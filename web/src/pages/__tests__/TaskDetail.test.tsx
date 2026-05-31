/**
 * Unit tests for TaskDetail page integration with structured timeline view.
 * Feature: task-tool-chain-ui
 *
 * Validates: Requirements 6.1, 6.2, 6.3, 9.4
 */

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Task, TaskMessage } from "../../hooks/useTasks";
import type { TimelineItem } from "../../lib/tool-chain-parser";

// ─── Mock Setup ──────────────────────────────────────────────────────────────

// Mock the hooks used by TaskDetail
const mockTask: Task = {
  id: "task-123",
  user_id: "user-1",
  agent_type: "claude",
  prompt: "Fix the login bug",
  status: "completed",
  exit_code: 0,
  error_message: null,
  output_preview: null,
  agent_id: "agent-1",
  agent_name: "Nexus",
  started_at: "2025-01-01T00:00:00Z",
  completed_at: "2025-01-01T00:05:00Z",
  created_at: "2025-01-01T00:00:00Z",
  updated_at: "2025-01-01T00:05:00Z",
  token_usage: null,
};

const mockMessages: TaskMessage[] = [
  {
    id: "msg-1",
    task_id: "task-123",
    sequence: 1,
    stream: "stdout",
    content: JSON.stringify({ type: "tool_use", name: "Read", input: { file_path: "/src/auth.ts" } }),
    created_at: "2025-01-01T00:00:01Z",
  },
  {
    id: "msg-2",
    task_id: "task-123",
    sequence: 2,
    stream: "stdout",
    content: JSON.stringify({ type: "tool_result", name: "Read", output: "file contents" }),
    created_at: "2025-01-01T00:00:02Z",
  },
  {
    id: "msg-3",
    task_id: "task-123",
    sequence: 3,
    stream: "stdout",
    content: JSON.stringify({ type: "tool_use", name: "Write", input: { file_path: "/src/auth.ts" } }),
    created_at: "2025-01-01T00:00:03Z",
  },
  {
    id: "msg-4",
    task_id: "task-123",
    sequence: 4,
    stream: "stderr",
    content: "Warning: deprecated API",
    created_at: "2025-01-01T00:00:04Z",
  },
  {
    id: "msg-5",
    task_id: "task-123",
    sequence: 5,
    stream: "stdout",
    content: "Done! Fixed the login bug.",
    created_at: "2025-01-01T00:00:05Z",
  },
];

// Parsed timeline items matching the mock messages
const mockTimelineItems: TimelineItem[] = [
  { seq: 0, type: "tool_use", tool: "Read", input: { file_path: "/src/auth.ts" } },
  { seq: 1, type: "tool_result", tool: "Read", output: "file contents" },
  { seq: 2, type: "tool_use", tool: "Write", input: { file_path: "/src/auth.ts" } },
  { seq: 3, type: "error", content: "Warning: deprecated API" },
  { seq: 4, type: "text", content: "Done! Fixed the login bug." },
];

// Mock toggleFilter function
const mockToggleFilter = vi.fn();
const mockClearFilters = vi.fn();
const mockSetSortDirection = vi.fn();

// Track filter state for dynamic tests
let mockFilters = new Set<string>();

vi.mock("../../hooks/useTasks", () => ({
  useTask: () => ({
    data: mockTask,
    isLoading: false,
    error: null,
  }),
  useTaskMessages: () => ({
    data: mockMessages,
  }),
  useCancelTask: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
  useCreateTask: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

vi.mock("../../hooks/useTaskStream", () => ({
  useTaskStream: () => ({
    messages: mockMessages,
    seedMessages: vi.fn(),
  }),
}));

vi.mock("../../hooks/useTimeline", () => ({
  useTimeline: () => ({
    items: mockTimelineItems,
    filteredItems: mockFilters.size > 0
      ? mockTimelineItems.filter((item) => mockFilters.has(item.type))
      : mockTimelineItems,
    filters: mockFilters,
    toggleFilter: mockToggleFilter,
    clearFilters: mockClearFilters,
    sortDirection: "chronological" as const,
    setSortDirection: mockSetSortDirection,
    scrollToTopSignal: 0,
    toolCallCount: 2,
    totalCount: 5,
    filterOptions: [
      { value: "tool_use", label: "Tool Use" },
      { value: "tool_result", label: "Tool Result" },
      { value: "error", label: "Error" },
      { value: "text", label: "Text" },
      { value: "tool:Read", label: "Read" },
      { value: "tool:Write", label: "Write" },
    ],
  }),
}));

vi.mock("../../hooks/useSessionState", () => ({
  useSessionState: () => null,
}));

vi.mock("../../hooks/useTaskStages", () => ({
  useTaskStages: () => ({
    data: undefined,
    isLoading: false,
    error: null,
  }),
}));

vi.mock("../../hooks/useTaskInput", () => ({
  useSendTaskInput: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

vi.mock("../../contexts/WebSocketContext", () => ({
  useWSClient: () => ({
    on: () => () => {},
    status: "connected",
    onStatusChange: () => () => {},
    connect: () => {},
    disconnect: () => {},
  }),
}));

vi.mock("../../lib/tool-chain-parser", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/tool-chain-parser")>();
  return {
    ...actual,
    formatCopyText: actual.formatCopyText,
  };
});

// Mock clipboard API
const mockWriteText = vi.fn().mockResolvedValue(undefined);
Object.assign(navigator, {
  clipboard: { writeText: mockWriteText },
});

// ─── Test Utilities ──────────────────────────────────────────────────────────

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

function renderTaskDetail() {
  const queryClient = createQueryClient();

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/tasks/task-123"]}>
        <Routes>
          <Route path="/tasks/:id" element={<TaskDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

// Lazy import to allow mocks to be set up first
let TaskDetailPage: React.ComponentType;

beforeEach(async () => {
  vi.clearAllMocks();
  mockFilters = new Set<string>();
  mockWriteText.mockResolvedValue(undefined);
  // Clear sessionStorage to avoid persisted view mode interference
  sessionStorage.clear();

  const mod = await import("../TaskDetail");
  TaskDetailPage = mod.default;
});

// ─── Tests ───────────────────────────────────────────────────────────────────

describe("TaskDetail - View mode toggle", () => {
  it("shows structured view by default", () => {
    renderTaskDetail();

    // Structured button should be pressed
    const structuredBtn = screen.getByRole("button", { name: /structured/i });
    expect(structuredBtn).toHaveAttribute("aria-pressed", "true");

    // TimelineView should be visible (it renders timeline cards)
    // The TimelineBar renders a progressbar
    expect(screen.getByRole("progressbar", { name: /timeline event distribution/i })).toBeInTheDocument();

    // FilterDropdown trigger button should be visible
    expect(screen.getByRole("button", { name: /filter/i })).toBeInTheDocument();
  });

  it("switches to raw view when Raw button is clicked", () => {
    renderTaskDetail();

    const rawBtn = screen.getByRole("button", { name: /^raw$/i });
    fireEvent.click(rawBtn);

    // Raw button should now be pressed
    expect(rawBtn).toHaveAttribute("aria-pressed", "true");

    // Raw view shows "Output" label in the terminal header
    expect(screen.getByText("Output")).toBeInTheDocument();

    // Timeline bar should not be visible in raw mode
    expect(screen.queryByRole("progressbar", { name: /timeline event distribution/i })).not.toBeInTheDocument();
  });

  it("switches back to structured view when Structured button is clicked", () => {
    renderTaskDetail();

    // Switch to raw
    fireEvent.click(screen.getByRole("button", { name: /^raw$/i }));
    expect(screen.getByText("Output")).toBeInTheDocument();

    // Switch back to structured
    fireEvent.click(screen.getByRole("button", { name: /structured/i }));

    // Timeline bar should be visible again
    expect(screen.getByRole("progressbar", { name: /timeline event distribution/i })).toBeInTheDocument();

    // Raw output header should not be visible
    expect(screen.queryByText("Output")).not.toBeInTheDocument();
  });
});

describe("TaskDetail - Metadata chips display correct counts", () => {
  it("displays tool call count chip with correct count", () => {
    renderTaskDetail();

    // Should show "2 tool calls" (from mockTimelineItems with 2 tool_use items)
    expect(screen.getByText("2 tool calls")).toBeInTheDocument();
  });

  it("displays total event count chip with correct count", () => {
    renderTaskDetail();

    // Should show "5 events" (from mockTimelineItems with 5 total items)
    expect(screen.getByText("5 events")).toBeInTheDocument();
  });
});

describe("TaskDetail - Filter + copy interaction", () => {
  it("shows Copy All button when no filters are active", () => {
    renderTaskDetail();

    expect(screen.getByRole("button", { name: /copy all/i })).toBeInTheDocument();
  });

  it("shows Copy Filtered button when filters are active", () => {
    // Set up active filters before rendering
    mockFilters = new Set(["tool_use"]);

    renderTaskDetail();

    expect(screen.getByRole("button", { name: /copy filtered/i })).toBeInTheDocument();
  });
});

describe("TaskDetail - Structured view shows timeline components", () => {
  it("renders TimelineBar", () => {
    renderTaskDetail();

    expect(screen.getByRole("progressbar", { name: /timeline event distribution/i })).toBeInTheDocument();
  });

  it("renders FilterDropdown", () => {
    renderTaskDetail();

    expect(screen.getByRole("button", { name: /filter/i })).toBeInTheDocument();
  });

  it("renders CopyButton", () => {
    renderTaskDetail();

    expect(screen.getByRole("button", { name: /copy all/i })).toBeInTheDocument();
  });
});

describe("TaskDetail - Raw view shows terminal output", () => {
  it("renders messages with correct stream coloring", () => {
    renderTaskDetail();

    // Switch to raw view
    fireEvent.click(screen.getByRole("button", { name: /^raw$/i }));

    // stderr messages should have red text class
    const stderrMessage = screen.getByText("Warning: deprecated API");
    expect(stderrMessage.closest("div[class]")).toHaveClass("text-red-400");

    // stdout text messages should have gray/white text class
    const stdoutMessage = screen.getByText("Done! Fixed the login bug.");
    expect(stdoutMessage.closest("div[class]")).toHaveClass("text-gray-100");
  });
});
