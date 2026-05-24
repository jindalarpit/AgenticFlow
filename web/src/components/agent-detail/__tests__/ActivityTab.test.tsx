import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ActivityTab } from "../ActivityTab";
import type { Agent, AgentStats, PaginatedTasks, AgentTask } from "../../../lib/agent-detail-types";

/* ─── Mocks ─── */

const mockNavigate = vi.fn();

vi.mock("react-router-dom", () => ({
  useNavigate: () => mockNavigate,
}));

const mockUseAgentStats = vi.fn();
const mockUseAgentTasks = vi.fn();

vi.mock("../../../hooks/useAgentDetail", () => ({
  useAgentStats: (...args: unknown[]) => mockUseAgentStats(...args),
  useAgentTasks: (...args: unknown[]) => mockUseAgentTasks(...args),
}));

/* ─── Test Data ─── */

const baseAgent: Agent = {
  id: "agent-1",
  name: "Nexus",
  description: "Test agent",
  instructions: "",
  avatar_url: null,
  runtime_id: "rt-1",
  runtime_name: "Claude CLI",
  custom_env: {},
  custom_args: [],
  model: "claude-sonnet-4-20250514",
  visibility: "private",
  status: "idle",
  max_concurrent_tasks: 1,
  owner_id: "user-1",
  owner_name: "Test User",
  skills: [],
  created_at: "2025-01-01T00:00:00Z",
  updated_at: "2025-01-15T00:00:00Z",
};

function makeTask(overrides: Partial<AgentTask>): AgentTask {
  return {
    id: "task-1",
    status: "completed",
    prompt: "Fix the login bug",
    started_at: "2025-01-10T10:00:00Z",
    completed_at: "2025-01-10T10:05:00Z",
    created_at: "2025-01-10T09:59:00Z",
    duration_ms: 300000,
    ...overrides,
  };
}

/* ─── Setup ─── */

beforeEach(() => {
  vi.clearAllMocks();
  // Default: no stats, no tasks
  mockUseAgentStats.mockReturnValue({ data: undefined });
  mockUseAgentTasks.mockReturnValue({ data: undefined });
});

/* ─── NOW Section Tests ─── */

describe("ActivityTab — NOW section", () => {
  it('shows "Not running" when no active tasks', () => {
    mockUseAgentTasks.mockReturnValue({
      data: { tasks: [], total: 0 } satisfies PaginatedTasks,
    });

    render(<ActivityTab agent={baseAgent} />);

    expect(screen.getByText("Not running")).toBeInTheDocument();
  });

  it("shows active tasks with status icons and truncated prompts", () => {
    const longPrompt = "A".repeat(120);
    const tasks: AgentTask[] = [
      makeTask({ id: "t1", status: "running", prompt: longPrompt, completed_at: null }),
      makeTask({ id: "t2", status: "dispatched", prompt: "Short prompt", completed_at: null }),
    ];

    mockUseAgentTasks.mockReturnValue({
      data: { tasks, total: 2 } satisfies PaginatedTasks,
    });

    render(<ActivityTab agent={baseAgent} />);

    // Should not show "Not running"
    expect(screen.queryByText("Not running")).not.toBeInTheDocument();

    // Should show truncated prompt (100 chars + ellipsis)
    const truncated = "A".repeat(100) + "\u2026";
    expect(screen.getByText(truncated)).toBeInTheDocument();

    // Should show short prompt as-is
    expect(screen.getByText("Short prompt")).toBeInTheDocument();

    // Should show status icons with aria-labels
    expect(screen.getByLabelText("Running")).toBeInTheDocument();
    expect(screen.getByLabelText("Dispatched")).toBeInTheDocument();
  });

  it("tasks sorted by lifecycle priority (running > dispatched > pending)", () => {
    const tasks: AgentTask[] = [
      makeTask({ id: "t-pending", status: "pending", prompt: "Pending task", created_at: "2025-01-10T10:00:00Z", completed_at: null }),
      makeTask({ id: "t-running", status: "running", prompt: "Running task", created_at: "2025-01-10T09:00:00Z", completed_at: null }),
      makeTask({ id: "t-dispatched", status: "dispatched", prompt: "Dispatched task", created_at: "2025-01-10T09:30:00Z", completed_at: null }),
    ];

    mockUseAgentTasks.mockReturnValue({
      data: { tasks, total: 3 } satisfies PaginatedTasks,
    });

    render(<ActivityTab agent={baseAgent} />);

    const listItems = screen.getAllByRole("listitem");
    // Running first, then dispatched, then pending
    expect(listItems[0]).toHaveTextContent("Running task");
    expect(listItems[1]).toHaveTextContent("Dispatched task");
    expect(listItems[2]).toHaveTextContent("Pending task");
  });
});

/* ─── LAST 30 DAYS Section Tests ─── */

describe("ActivityTab — LAST 30 DAYS section", () => {
  it('shows "No activity" when total_terminal is 0', () => {
    const stats: AgentStats = {
      total_runs: 0,
      success_rate: 0,
      avg_duration_ms: 0,
      total_terminal: 0,
    };
    mockUseAgentStats.mockReturnValue({ data: stats });
    mockUseAgentTasks.mockReturnValue({ data: { tasks: [], total: 0 } });

    render(<ActivityTab agent={baseAgent} />);

    expect(screen.getByText("No activity in the last 30 days")).toBeInTheDocument();
  });

  it("shows stats (total runs, success rate, avg duration)", () => {
    const stats: AgentStats = {
      total_runs: 42,
      success_rate: 85,
      avg_duration_ms: 180000, // 3 minutes
      total_terminal: 50,
    };
    mockUseAgentStats.mockReturnValue({ data: stats });
    mockUseAgentTasks.mockReturnValue({ data: { tasks: [], total: 0 } });

    render(<ActivityTab agent={baseAgent} />);

    expect(screen.getByText("42")).toBeInTheDocument();
    expect(screen.getByText("84%")).toBeInTheDocument(); // computeSuccessRate(42, 50) = 84
    expect(screen.getByText("3m 0s")).toBeInTheDocument();
    expect(screen.getByText("Total Runs")).toBeInTheDocument();
    expect(screen.getByText("Success Rate")).toBeInTheDocument();
    expect(screen.getByText("Avg Duration")).toBeInTheDocument();
  });

  it('shows "0%" and "—" when no completed tasks but has failed tasks', () => {
    const stats: AgentStats = {
      total_runs: 0,
      success_rate: 0,
      avg_duration_ms: 0,
      total_terminal: 5, // 5 terminal tasks but 0 completed
    };
    mockUseAgentStats.mockReturnValue({ data: stats });
    mockUseAgentTasks.mockReturnValue({ data: { tasks: [], total: 0 } });

    render(<ActivityTab agent={baseAgent} />);

    expect(screen.getByText("0%")).toBeInTheDocument();
    expect(screen.getByText("\u2014")).toBeInTheDocument(); // em dash
  });
});

/* ─── RECENT WORK Section Tests ─── */

describe("ActivityTab — RECENT WORK section", () => {
  it("shows terminal tasks with status, prompt, time, duration", () => {
    const tasks: AgentTask[] = [
      makeTask({
        id: "t1",
        status: "completed",
        prompt: "Deploy the app",
        completed_at: new Date(Date.now() - 3600000).toISOString(), // 1 hour ago
        duration_ms: 120000,
      }),
      makeTask({
        id: "t2",
        status: "failed",
        prompt: "Run migrations",
        completed_at: new Date(Date.now() - 7200000).toISOString(), // 2 hours ago
        duration_ms: 5000,
      }),
    ];

    mockUseAgentTasks.mockReturnValue({
      data: { tasks, total: 2 } satisfies PaginatedTasks,
    });

    render(<ActivityTab agent={baseAgent} />);

    expect(screen.getByText("Deploy the app")).toBeInTheDocument();
    expect(screen.getByText("Run migrations")).toBeInTheDocument();
    // Duration formatted
    expect(screen.getByText("2m 0s")).toBeInTheDocument();
    expect(screen.getByText("5s")).toBeInTheDocument();
    // Status icons
    expect(screen.getByLabelText("Completed")).toBeInTheDocument();
    expect(screen.getByLabelText("Failed")).toBeInTheDocument();
  });

  it("shows failure reason in red for failed tasks", () => {
    const tasks: AgentTask[] = [
      makeTask({
        id: "t-fail",
        status: "failed",
        prompt: "Run tests",
        failure_reason: "Process exited with code 1",
        completed_at: new Date(Date.now() - 1000).toISOString(),
      }),
    ];

    mockUseAgentTasks.mockReturnValue({
      data: { tasks, total: 1 } satisfies PaginatedTasks,
    });

    render(<ActivityTab agent={baseAgent} />);

    const failureText = screen.getByText("Process exited with code 1");
    expect(failureText).toBeInTheDocument();
    expect(failureText).toHaveClass("text-red-600");
  });

  it('"Show more" button loads additional tasks', () => {
    // Initial load: 5 tasks shown, total > limit means "Show more" appears
    const tasks: AgentTask[] = Array.from({ length: 5 }, (_, i) =>
      makeTask({
        id: `t-${i}`,
        status: "completed",
        prompt: `Task ${i}`,
        completed_at: new Date(Date.now() - i * 60000).toISOString(),
      })
    );

    // First call (for NowSection with limit=50) returns all tasks
    // Second call (for RecentWorkSection with limit=5) returns paginated
    mockUseAgentTasks.mockImplementation((_agentId: string, opts: { limit: number; offset: number }) => {
      if (opts.limit === 50) {
        return { data: { tasks, total: 30 } };
      }
      // RecentWorkSection: total > limit triggers "Show more"
      return { data: { tasks, total: 30 } };
    });

    render(<ActivityTab agent={baseAgent} />);

    const showMoreBtn = screen.getByRole("button", { name: "Show more" });
    expect(showMoreBtn).toBeInTheDocument();

    // Click "Show more" — this increases the limit
    fireEvent.click(showMoreBtn);

    // After click, useAgentTasks should be called with increased limit (5 + 20 = 25)
    // The mock is called with the new limit
    expect(mockUseAgentTasks).toHaveBeenCalledWith("agent-1", { limit: 25, offset: 0 });
  });
});
