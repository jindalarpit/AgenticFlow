/**
 * Unit tests for OverviewPane and tab navigation.
 *
 * Validates: Requirements 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7
 *
 * Tests:
 * - Default tab is Activity (Activity tab content visible on mount)
 * - Clicking a tab switches to that tab's content
 * - Active tab has visual indicator (aria-selected="true")
 * - Inactive tabs don't have active indicator
 * - Dirty-guard dialog appears when switching from dirty tab
 * - Discard in dirty-guard: switches to requested tab, resets dirty
 * - Cancel in dirty-guard: stays on current tab, preserves dirty state
 * - Non-dirty tabs switch immediately without dialog
 */

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { OverviewPane } from "../OverviewPane";
import type { Agent } from "../../../lib/agent-detail-types";

/* ─── Mock child tab components ─── */

vi.mock("../ActivityTab", () => ({
  ActivityTab: ({ agent }: { agent: Agent }) => (
    <div data-testid="activity-tab-content">Activity for {agent.name}</div>
  ),
}));

vi.mock("../TasksTab", () => ({
  TasksTab: ({ agent }: { agent: Agent }) => (
    <div data-testid="tasks-tab-content">Tasks for {agent.name}</div>
  ),
}));

vi.mock("../InstructionsTab", () => ({
  InstructionsTab: ({
    agent,
    onDirtyChange,
  }: {
    agent: Agent;
    isOwner: boolean;
    onDirtyChange: (dirty: boolean) => void;
    onSave: (data: Partial<Agent>) => Promise<void>;
  }) => (
    <div data-testid="instructions-tab-content">
      Instructions for {agent.name}
      <button
        data-testid="set-dirty-btn"
        onClick={() => onDirtyChange(true)}
      >
        Set Dirty
      </button>
      <button
        data-testid="clear-dirty-btn"
        onClick={() => onDirtyChange(false)}
      >
        Clear Dirty
      </button>
    </div>
  ),
}));

vi.mock("../SkillsTab", () => ({
  SkillsTab: ({ agent }: { agent: Agent }) => (
    <div data-testid="skills-tab-content">Skills for {agent.name}</div>
  ),
}));

vi.mock("../ToolsTab", () => ({
  ToolsTab: ({
    agent,
    onDirtyChange,
  }: {
    agent: Agent;
    isOwner: boolean;
    onDirtyChange: (dirty: boolean) => void;
    onSave: (data: Partial<Agent>) => Promise<void>;
  }) => (
    <div data-testid="tools-tab-content">
      Tools for {agent.name}
      <button
        data-testid="tools-set-dirty-btn"
        onClick={() => onDirtyChange(true)}
      >
        Set Dirty
      </button>
    </div>
  ),
}));

vi.mock("../EnvironmentTab", () => ({
  EnvironmentTab: ({
    agent,
    onDirtyChange,
  }: {
    agent: Agent;
    isOwner: boolean;
    onDirtyChange: (dirty: boolean) => void;
    onSave: (data: Partial<Agent>) => Promise<void>;
  }) => (
    <div data-testid="env-tab-content">
      Environment for {agent.name}
      <button
        data-testid="env-set-dirty-btn"
        onClick={() => onDirtyChange(true)}
      >
        Set Dirty
      </button>
    </div>
  ),
}));

vi.mock("../CustomArgsTab", () => ({
  CustomArgsTab: ({
    agent,
    onDirtyChange,
  }: {
    agent: Agent;
    isOwner: boolean;
    onDirtyChange: (dirty: boolean) => void;
    onSave: (data: Partial<Agent>) => Promise<void>;
  }) => (
    <div data-testid="custom-args-tab-content">
      Custom Args for {agent.name}
      <button
        data-testid="args-set-dirty-btn"
        onClick={() => onDirtyChange(true)}
      >
        Set Dirty
      </button>
    </div>
  ),
}));

/* ─── Mock hooks used by child tabs ─── */

vi.mock("../../../hooks/useAgentDetail", () => ({
  useAgentStats: vi.fn(() => ({ data: null, isLoading: false })),
  useAgentTasks: vi.fn(() => ({ data: null, isLoading: false })),
}));

vi.mock("react-router-dom", () => ({
  useNavigate: vi.fn(() => vi.fn()),
}));

vi.mock("../../Toast", () => ({
  useToast: vi.fn(() => ({ success: vi.fn(), error: vi.fn() })),
}));

/* ─── Mock agent fixture ─── */

const mockAgent: Agent = {
  id: "agent-001",
  name: "Nexus",
  description: "Test agent",
  instructions: "Be helpful",
  avatar_url: null,
  runtime_mode: "local",
  runtime_id: "rt-001",
  runtime_name: "Local Runtime",
  custom_env: { API_KEY: "secret" },
  custom_args: ["--verbose"],
  model: "claude-sonnet-4-20250514",
  visibility: "private",
  status: "idle",
  max_concurrent_tasks: 1,
  owner_id: "user-001",
  owner_name: "Test User",
  skills: [{ id: "skill-1", name: "Code Review" }],
  mcp_config: null,
  created_at: "2025-01-01T00:00:00Z",
  updated_at: "2025-01-15T00:00:00Z",
};

const mockOnSave = vi.fn().mockResolvedValue(undefined);

/* ─── Helper ─── */

function renderOverviewPane(isOwner = true) {
  return render(
    <OverviewPane agent={mockAgent} isOwner={isOwner} onSave={mockOnSave} />
  );
}

function getTab(name: string) {
  return screen.getByRole("tab", { name });
}

/* ─── Tests ─── */

describe("OverviewPane — Tab Navigation", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders Activity tab as the default active tab on mount", () => {
    renderOverviewPane();

    // Activity tab content should be visible
    expect(screen.getByTestId("activity-tab-content")).toBeInTheDocument();

    // Other tab contents should NOT be visible
    expect(screen.queryByTestId("tasks-tab-content")).not.toBeInTheDocument();
    expect(screen.queryByTestId("instructions-tab-content")).not.toBeInTheDocument();
    expect(screen.queryByTestId("skills-tab-content")).not.toBeInTheDocument();
    expect(screen.queryByTestId("env-tab-content")).not.toBeInTheDocument();
    expect(screen.queryByTestId("custom-args-tab-content")).not.toBeInTheDocument();
  });

  it("switches to the clicked tab's content", () => {
    renderOverviewPane();

    // Click Tasks tab
    fireEvent.click(getTab("Tasks"));
    expect(screen.getByTestId("tasks-tab-content")).toBeInTheDocument();
    expect(screen.queryByTestId("activity-tab-content")).not.toBeInTheDocument();

    // Click Instructions tab
    fireEvent.click(getTab("Instructions"));
    expect(screen.getByTestId("instructions-tab-content")).toBeInTheDocument();
    expect(screen.queryByTestId("tasks-tab-content")).not.toBeInTheDocument();

    // Click Skills tab
    fireEvent.click(getTab("Skills"));
    expect(screen.getByTestId("skills-tab-content")).toBeInTheDocument();
    expect(screen.queryByTestId("instructions-tab-content")).not.toBeInTheDocument();
  });

  it("active tab has aria-selected='true'", () => {
    renderOverviewPane();

    // Default: Activity is active
    expect(getTab("Activity")).toHaveAttribute("aria-selected", "true");

    // Switch to Tasks
    fireEvent.click(getTab("Tasks"));
    expect(getTab("Tasks")).toHaveAttribute("aria-selected", "true");
  });

  it("inactive tabs have aria-selected='false'", () => {
    renderOverviewPane();

    // All non-active tabs should have aria-selected="false"
    expect(getTab("Tasks")).toHaveAttribute("aria-selected", "false");
    expect(getTab("Instructions")).toHaveAttribute("aria-selected", "false");
    expect(getTab("Skills")).toHaveAttribute("aria-selected", "false");
    expect(getTab("Environment")).toHaveAttribute("aria-selected", "false");
    expect(getTab("Custom Args")).toHaveAttribute("aria-selected", "false");
  });

  it("non-dirty tabs switch immediately without showing a dialog", () => {
    renderOverviewPane();

    // Switch to Tasks — dialog should not be open
    fireEvent.click(getTab("Tasks"));
    expect(screen.getByTestId("tasks-tab-content")).toBeInTheDocument();
    const dialog = screen.getByRole("dialog", { hidden: true });
    expect(dialog).not.toHaveAttribute("open");

    // Switch to Skills — dialog still not open
    fireEvent.click(getTab("Skills"));
    expect(screen.getByTestId("skills-tab-content")).toBeInTheDocument();
    expect(dialog).not.toHaveAttribute("open");
  });
});

describe("OverviewPane — Dirty Guard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows dirty-guard dialog when switching from a dirty tab", () => {
    renderOverviewPane();

    // Navigate to Instructions tab
    fireEvent.click(getTab("Instructions"));
    expect(screen.getByTestId("instructions-tab-content")).toBeInTheDocument();

    // Mark the tab as dirty
    fireEvent.click(screen.getByTestId("set-dirty-btn"));

    // Try to switch to Activity tab — dialog should appear
    fireEvent.click(getTab("Activity"));

    // Dialog should be visible
    expect(screen.getByText("Unsaved Changes")).toBeInTheDocument();
    expect(screen.getByText("You have unsaved changes. Discard them?")).toBeInTheDocument();

    // Should still be on Instructions tab (not switched yet)
    expect(screen.getByTestId("instructions-tab-content")).toBeInTheDocument();
    expect(screen.queryByTestId("activity-tab-content")).not.toBeInTheDocument();
  });

  it("discard in dirty-guard switches to the requested tab and resets dirty", () => {
    renderOverviewPane();

    // Navigate to Instructions tab and mark dirty
    fireEvent.click(getTab("Instructions"));
    fireEvent.click(screen.getByTestId("set-dirty-btn"));

    // Try to switch to Tasks tab
    fireEvent.click(getTab("Tasks"));

    // Dialog appears — click Discard
    fireEvent.click(screen.getByRole("button", { name: "Discard" }));

    // Should now be on Tasks tab
    expect(screen.getByTestId("tasks-tab-content")).toBeInTheDocument();
    expect(screen.queryByTestId("instructions-tab-content")).not.toBeInTheDocument();

    // Dialog should be closed
    const dialog = screen.getByRole("dialog", { hidden: true });
    expect(dialog).not.toHaveAttribute("open");

    // Dirty state should be reset — switching from Tasks should work without dialog
    fireEvent.click(getTab("Activity"));
    expect(screen.getByTestId("activity-tab-content")).toBeInTheDocument();
    expect(dialog).not.toHaveAttribute("open");
  });

  it("cancel in dirty-guard stays on current tab and preserves dirty state", () => {
    renderOverviewPane();

    // Navigate to Instructions tab and mark dirty
    fireEvent.click(getTab("Instructions"));
    fireEvent.click(screen.getByTestId("set-dirty-btn"));

    // Try to switch to Tasks tab
    fireEvent.click(getTab("Tasks"));

    // Dialog appears — click Stay
    fireEvent.click(screen.getByRole("button", { name: "Stay" }));

    // Should still be on Instructions tab
    expect(screen.getByTestId("instructions-tab-content")).toBeInTheDocument();
    expect(screen.queryByTestId("tasks-tab-content")).not.toBeInTheDocument();

    // Dialog should be closed
    const dialog = screen.getByRole("dialog", { hidden: true });
    expect(dialog).not.toHaveAttribute("open");

    // Dirty state preserved — trying to switch again should open dialog again
    fireEvent.click(getTab("Skills"));
    expect(dialog).toHaveAttribute("open");
    expect(screen.getByTestId("instructions-tab-content")).toBeInTheDocument();
  });
});
