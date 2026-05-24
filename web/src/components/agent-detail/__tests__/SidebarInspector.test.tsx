import { describe, it, expect, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import type { Agent } from "../../../lib/agent-detail-types";
import { SidebarInspector } from "../SidebarInspector";

/* ─── Mock Agent Factory ─── */

function createMockAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: "agent-1",
    name: "TestAgent",
    description: "A test agent for unit testing",
    instructions: "Do things",
    avatar_url: null,
    runtime_id: "rt-1",
    runtime_name: "Claude CLI",
    custom_env: {},
    custom_args: [],
    model: "claude-sonnet-4-20250514",
    visibility: "private",
    status: "idle",
    max_concurrent_tasks: 3,
    owner_id: "user-1",
    owner_name: "Alice",
    skills: [
      { id: "skill-1", name: "code-review" },
      { id: "skill-2", name: "testing" },
    ],
    created_at: new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString(), // 3 days ago
    updated_at: new Date(Date.now() - 1 * 60 * 60 * 1000).toISOString(), // 1 hour ago
    ...overrides,
  };
}

const mockOnUpdate = vi.fn().mockResolvedValue(undefined);

/* ─── Identity Section Tests ─── */

describe("SidebarInspector — Identity Section", () => {
  it("renders avatar image when avatar_url is provided", () => {
    const agent = createMockAgent({ avatar_url: "https://example.com/avatar.png" });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    const img = screen.getByRole("img", { name: /TestAgent avatar/i });
    expect(img).toBeInTheDocument();
    expect(img).toHaveAttribute("src", "https://example.com/avatar.png");
  });

  it("renders initials fallback when avatar_url is null", () => {
    const agent = createMockAgent({ avatar_url: null, name: "TestAgent" });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    // Should show initials "T" (first letter of "TestAgent")
    const fallback = screen.getByLabelText(/TestAgent avatar/i);
    expect(fallback).toBeInTheDocument();
    expect(fallback.textContent).toBe("T");
  });

  it("renders multi-word name initials correctly", () => {
    const agent = createMockAgent({ avatar_url: null, name: "My-Agent" });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    const fallback = screen.getByLabelText(/My-Agent avatar/i);
    expect(fallback).toBeInTheDocument();
    // "My-Agent" splits on "-" → ["My", "Agent"] → "MA"
    expect(fallback.textContent).toBe("MA");
  });

  it("renders agent name in semibold text", () => {
    const agent = createMockAgent({ name: "Nexus" });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    const nameEl = screen.getByText("Nexus");
    expect(nameEl).toBeInTheDocument();
    expect(nameEl).toHaveClass("font-semibold");
  });

  it("renders description when provided", () => {
    const agent = createMockAgent({ description: "My helpful agent" });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    expect(screen.getByText("My helpful agent")).toBeInTheDocument();
  });

  it('renders "No description" placeholder when description is empty', () => {
    const agent = createMockAgent({ description: "" });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    expect(screen.getByText("No description")).toBeInTheDocument();
  });

  it("renders status badge with green dot for idle status", () => {
    const agent = createMockAgent({ status: "idle" });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    const badge = screen.getByLabelText("Status: Idle");
    expect(badge).toBeInTheDocument();
    expect(badge.textContent).toContain("Idle");
  });

  it("renders status badge with amber dot for working status", () => {
    const agent = createMockAgent({ status: "working" });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    const badge = screen.getByLabelText("Status: Working");
    expect(badge).toBeInTheDocument();
    expect(badge.textContent).toContain("Working");
  });

  it("renders status badge with gray dot for offline status", () => {
    const agent = createMockAgent({ status: "offline" });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    const badge = screen.getByLabelText("Status: Offline");
    expect(badge).toBeInTheDocument();
    expect(badge.textContent).toContain("Offline");
  });
});

/* ─── Identity Section — Owner Mode ─── */

describe("SidebarInspector — Owner Edit Affordances", () => {
  it("shows edit affordance for name when user is owner", () => {
    const agent = createMockAgent({ name: "Nexus" });
    render(<SidebarInspector agent={agent} isOwner={true} onUpdate={mockOnUpdate} />);

    const editNameBtn = screen.getByLabelText("Edit agent name");
    expect(editNameBtn).toBeInTheDocument();
  });

  it("shows edit affordance for description when user is owner", () => {
    const agent = createMockAgent({ description: "A description" });
    render(<SidebarInspector agent={agent} isOwner={true} onUpdate={mockOnUpdate} />);

    const editDescBtn = screen.getByLabelText("Edit agent description");
    expect(editDescBtn).toBeInTheDocument();
  });

  it("does not show edit affordances when user is not owner", () => {
    const agent = createMockAgent();
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    expect(screen.queryByLabelText("Edit agent name")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Edit agent description")).not.toBeInTheDocument();
  });
});

/* ─── Properties Section ─── */

describe("SidebarInspector — Properties Section", () => {
  it("renders Runtime, Model, Visibility, and Concurrency values", () => {
    const agent = createMockAgent({
      runtime_name: "Claude CLI",
      model: "claude-sonnet-4-20250514",
      visibility: "private",
      max_concurrent_tasks: 5,
    });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    expect(screen.getByText("Runtime")).toBeInTheDocument();
    expect(screen.getByText("Claude CLI")).toBeInTheDocument();
    expect(screen.getByText("Model")).toBeInTheDocument();
    expect(screen.getByText("claude-sonnet-4-20250514")).toBeInTheDocument();
    expect(screen.getByText("Visibility")).toBeInTheDocument();
    expect(screen.getByText("Private")).toBeInTheDocument();
    expect(screen.getByText("Concurrency")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
  });

  it('shows "Default" when model is empty', () => {
    const agent = createMockAgent({ model: "" });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    expect(screen.getByText("Default")).toBeInTheDocument();
  });

  it('shows "Shared" for shared visibility', () => {
    const agent = createMockAgent({ visibility: "shared" });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    expect(screen.getByText("Shared")).toBeInTheDocument();
  });

  it("shows interactive controls for properties when user is owner", () => {
    const agent = createMockAgent();
    render(<SidebarInspector agent={agent} isOwner={true} onUpdate={mockOnUpdate} />);

    // Runtime picker (select element)
    expect(screen.getByLabelText("Select runtime")).toBeInTheDocument();
    // Model edit button
    expect(screen.getByLabelText("Edit model")).toBeInTheDocument();
    // Visibility toggle (switch role)
    expect(screen.getByRole("switch")).toBeInTheDocument();
    // Concurrency number input
    expect(screen.getByLabelText("Max concurrent tasks (1-20)")).toBeInTheDocument();
  });

  it("shows read-only text for properties when user is not owner", () => {
    const agent = createMockAgent({
      model: "gpt-4",
      visibility: "shared",
      max_concurrent_tasks: 7,
      skills: [], // avoid collision with skills count
    });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    // No interactive controls
    expect(screen.queryByLabelText("Select runtime")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Edit model")).not.toBeInTheDocument();
    expect(screen.queryByRole("switch")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Max concurrent tasks (1-20)")).not.toBeInTheDocument();

    // Values displayed as plain text
    expect(screen.getByText("gpt-4")).toBeInTheDocument();
    expect(screen.getByText("Shared")).toBeInTheDocument();
    expect(screen.getByText("7")).toBeInTheDocument();
  });
});

/* ─── Details Section ─── */

describe("SidebarInspector — Details Section", () => {
  it("shows Owner, Created, and Updated labels", () => {
    const agent = createMockAgent();
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    expect(screen.getByText("Owner")).toBeInTheDocument();
    expect(screen.getByText("Created")).toBeInTheDocument();
    expect(screen.getByText("Updated")).toBeInTheDocument();
  });

  it("displays owner name", () => {
    const agent = createMockAgent({ owner_name: "Bob" });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    expect(screen.getByText("Bob")).toBeInTheDocument();
  });

  it("displays relative time for created_at", () => {
    // 3 days ago
    const agent = createMockAgent({
      created_at: new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString(),
    });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    expect(screen.getByText("3 days ago")).toBeInTheDocument();
  });

  it("displays relative time for updated_at", () => {
    // 1 hour ago
    const agent = createMockAgent({
      updated_at: new Date(Date.now() - 1 * 60 * 60 * 1000).toISOString(),
    });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    expect(screen.getByText("1 hour ago")).toBeInTheDocument();
  });

  it('shows "Unknown" when owner_name is not provided', () => {
    const agent = createMockAgent({ owner_name: undefined });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    expect(screen.getByText("Unknown")).toBeInTheDocument();
  });
});

/* ─── Skills Section ─── */

describe("SidebarInspector — Skills Section", () => {
  it("shows skill count", () => {
    const agent = createMockAgent({
      max_concurrent_tasks: 1, // avoid collision with skills count
      skills: [
        { id: "s1", name: "code-review" },
        { id: "s2", name: "testing" },
        { id: "s3", name: "debugging" },
      ],
    });
    const { container } = render(
      <SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />
    );

    // The skills count is rendered in a specific font-mono tabular-nums span
    const countEl = container.querySelector(
      ".font-mono.tabular-nums"
    );
    expect(countEl).not.toBeNull();
    expect(countEl!.textContent).toBe("3");
  });

  it("shows skill name badges", () => {
    const agent = createMockAgent({
      skills: [
        { id: "s1", name: "code-review" },
        { id: "s2", name: "testing" },
      ],
    });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    expect(screen.getByText("code-review")).toBeInTheDocument();
    expect(screen.getByText("testing")).toBeInTheDocument();
  });

  it('shows "0" when no skills are attached', () => {
    const agent = createMockAgent({ skills: [] });
    render(<SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />);

    expect(screen.getByText("0")).toBeInTheDocument();
  });

  it("does not render skill badges when skills array is empty", () => {
    const agent = createMockAgent({ skills: [] });
    const { container } = render(
      <SidebarInspector agent={agent} isOwner={false} onUpdate={mockOnUpdate} />
    );

    // The skills section should exist but have no badge elements
    const skillsHeading = screen.getByText("Skills");
    expect(skillsHeading).toBeInTheDocument();

    // No badge elements rendered (badges use bg-muted class)
    const badges = container.querySelectorAll(".bg-muted");
    expect(badges.length).toBe(0);
  });
});
