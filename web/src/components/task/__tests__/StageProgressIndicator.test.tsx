import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  StageProgressIndicator,
  type StageInfo,
} from "../StageProgressIndicator";

describe("StageProgressIndicator", () => {
  it("renders nothing when stages array is empty", () => {
    const { container } = render(<StageProgressIndicator stages={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders all stages in order with their labels", () => {
    const stages: StageInfo[] = [
      { name: "plan", status: "completed" },
      { name: "design", status: "running" },
      { name: "tasks", status: "pending" },
      { name: "execution", status: "pending" },
    ];

    render(<StageProgressIndicator stages={stages} />);

    expect(screen.getByText("Plan")).toBeInTheDocument();
    expect(screen.getByText("Design")).toBeInTheDocument();
    expect(screen.getByText("Tasks")).toBeInTheDocument();
    expect(screen.getByText("Execution")).toBeInTheDocument();
  });

  it("displays pending stage with neutral visual state", () => {
    const stages: StageInfo[] = [{ name: "plan", status: "pending" }];
    render(<StageProgressIndicator stages={stages} />);

    expect(screen.getByLabelText("Pending")).toBeInTheDocument();
  });

  it("displays running stage with animated visual state", () => {
    const stages: StageInfo[] = [{ name: "plan", status: "running" }];
    render(<StageProgressIndicator stages={stages} />);

    const indicator = screen.getByLabelText("Running");
    expect(indicator).toBeInTheDocument();
    expect(indicator.className).toContain("animate-pulse");
  });

  it("displays awaiting_approval stage with attention visual state", () => {
    const stages: StageInfo[] = [{ name: "design", status: "awaiting_approval" }];
    render(<StageProgressIndicator stages={stages} />);

    const indicator = screen.getByLabelText("Awaiting approval");
    expect(indicator).toBeInTheDocument();
    expect(indicator.className).toContain("border-amber-400");
  });

  it("displays approved stage with success visual state", () => {
    const stages: StageInfo[] = [{ name: "plan", status: "approved" }];
    render(<StageProgressIndicator stages={stages} />);

    const indicator = screen.getByLabelText("Completed");
    expect(indicator).toBeInTheDocument();
    expect(indicator.className).toContain("border-green-400");
  });

  it("displays completed stage with success visual state", () => {
    const stages: StageInfo[] = [{ name: "plan", status: "completed" }];
    render(<StageProgressIndicator stages={stages} />);

    const indicator = screen.getByLabelText("Completed");
    expect(indicator).toBeInTheDocument();
    expect(indicator.className).toContain("border-green-400");
  });

  it("displays rejected stage with warning visual state", () => {
    const stages: StageInfo[] = [{ name: "tasks", status: "rejected" }];
    render(<StageProgressIndicator stages={stages} />);

    const indicator = screen.getByLabelText("Rejected");
    expect(indicator).toBeInTheDocument();
    expect(indicator.className).toContain("border-red-400");
  });

  it("renders connector lines between stages", () => {
    const stages: StageInfo[] = [
      { name: "plan", status: "completed" },
      { name: "design", status: "running" },
      { name: "tasks", status: "pending" },
    ];

    render(<StageProgressIndicator stages={stages} />);

    // There should be N-1 connector lines (2 for 3 stages)
    expect(screen.getByTestId("connector-plan")).toBeInTheDocument();
    expect(screen.getByTestId("connector-design")).toBeInTheDocument();
    expect(screen.queryByTestId("connector-tasks")).not.toBeInTheDocument();
  });

  it("connector after completed stage is green", () => {
    const stages: StageInfo[] = [
      { name: "plan", status: "completed" },
      { name: "design", status: "pending" },
    ];

    render(<StageProgressIndicator stages={stages} />);

    const connector = screen.getByTestId("connector-plan");
    expect(connector.className).toContain("bg-green-400");
  });

  it("connector after pending stage is gray", () => {
    const stages: StageInfo[] = [
      { name: "plan", status: "pending" },
      { name: "design", status: "pending" },
    ];

    render(<StageProgressIndicator stages={stages} />);

    const connector = screen.getByTestId("connector-plan");
    expect(connector.className).toContain("bg-gray-200");
  });

  it("has accessible navigation landmark", () => {
    const stages: StageInfo[] = [{ name: "plan", status: "pending" }];
    render(<StageProgressIndicator stages={stages} />);

    expect(screen.getByRole("navigation", { name: "Workflow stage progress" })).toBeInTheDocument();
  });

  it("uses stage name as fallback label for unknown stage names", () => {
    const stages: StageInfo[] = [{ name: "custom-stage", status: "pending" }];
    render(<StageProgressIndicator stages={stages} />);

    expect(screen.getByText("custom-stage")).toBeInTheDocument();
  });

  it("renders a full workflow with mixed statuses correctly", () => {
    const stages: StageInfo[] = [
      { name: "plan", status: "approved" },
      { name: "design", status: "approved" },
      { name: "tasks", status: "awaiting_approval" },
      { name: "execution", status: "pending" },
    ];

    render(<StageProgressIndicator stages={stages} />);

    const completedIndicators = screen.getAllByLabelText("Completed");
    expect(completedIndicators).toHaveLength(2);

    expect(screen.getByLabelText("Awaiting approval")).toBeInTheDocument();
    expect(screen.getByLabelText("Pending")).toBeInTheDocument();
  });
});
