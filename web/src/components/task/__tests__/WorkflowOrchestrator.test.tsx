import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { WorkflowOrchestrator } from "../WorkflowOrchestrator";

describe("WorkflowOrchestrator", () => {
  const defaultProps = {
    agentId: "agent-123",
    currentDeliverableType: null as "plan" | "design" | "tasks" | "execution" | null,
    completedDeliverables: {} as Record<string, string>,
    onCreateTask: vi.fn(),
  };

  it("shows 'plan' as the next deliverable when nothing is completed", () => {
    render(<WorkflowOrchestrator {...defaultProps} />);

    expect(screen.getByText("plan")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /proceed to plan/i })
    ).toBeInTheDocument();
  });

  it("shows 'design' as next when current is plan", () => {
    render(
      <WorkflowOrchestrator
        {...defaultProps}
        currentDeliverableType="plan"
        completedDeliverables={{ plan: "Plan output" }}
      />
    );

    expect(screen.getByText("design")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /proceed to design/i })
    ).toBeInTheDocument();
  });

  it("shows 'tasks' as next when plan and design are completed", () => {
    render(
      <WorkflowOrchestrator
        {...defaultProps}
        currentDeliverableType="design"
        completedDeliverables={{
          plan: "Plan output",
          design: "Design output",
        }}
      />
    );

    expect(screen.getByText("tasks")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /proceed to tasks/i })
    ).toBeInTheDocument();
  });

  it("shows 'execution' as next when plan, design, and tasks are completed", () => {
    render(
      <WorkflowOrchestrator
        {...defaultProps}
        currentDeliverableType="tasks"
        completedDeliverables={{
          plan: "Plan output",
          design: "Design output",
          tasks: "Tasks output",
        }}
      />
    );

    expect(screen.getByText("execution")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /proceed to execution/i })
    ).toBeInTheDocument();
  });

  it("shows workflow complete when all deliverables are completed", () => {
    render(
      <WorkflowOrchestrator
        {...defaultProps}
        currentDeliverableType="execution"
        completedDeliverables={{
          plan: "Plan output",
          design: "Design output",
          tasks: "Tasks output",
          execution: "Execution output",
        }}
      />
    );

    expect(
      screen.getByText(/workflow complete/i)
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /proceed/i })
    ).not.toBeInTheDocument();
  });

  it("calls onCreateTask with next deliverable type and prior context on proceed", () => {
    const onCreateTask = vi.fn();

    render(
      <WorkflowOrchestrator
        {...defaultProps}
        currentDeliverableType="plan"
        completedDeliverables={{ plan: "Plan output" }}
        onCreateTask={onCreateTask}
      />
    );

    fireEvent.click(
      screen.getByRole("button", { name: /proceed to design/i })
    );

    expect(onCreateTask).toHaveBeenCalledWith("design", ["Plan output"]);
  });

  it("passes all completed outputs as prior_context in canonical order", () => {
    const onCreateTask = vi.fn();

    render(
      <WorkflowOrchestrator
        {...defaultProps}
        currentDeliverableType="design"
        completedDeliverables={{
          plan: "Plan output",
          design: "Design output",
        }}
        onCreateTask={onCreateTask}
      />
    );

    fireEvent.click(
      screen.getByRole("button", { name: /proceed to tasks/i })
    );

    expect(onCreateTask).toHaveBeenCalledWith("tasks", [
      "Plan output",
      "Design output",
    ]);
  });

  it("skips a deliverable when skip button is clicked", () => {
    render(
      <WorkflowOrchestrator
        {...defaultProps}
        currentDeliverableType={null}
        completedDeliverables={{}}
      />
    );

    // Initially shows plan as next
    expect(screen.getByText("plan")).toBeInTheDocument();

    // Click skip
    fireEvent.click(screen.getByRole("button", { name: /skip plan/i }));

    // Now shows design as next
    expect(screen.getByText("design")).toBeInTheDocument();
  });

  it("can skip multiple deliverables in sequence", () => {
    render(
      <WorkflowOrchestrator
        {...defaultProps}
        currentDeliverableType={null}
        completedDeliverables={{}}
      />
    );

    // Skip plan
    fireEvent.click(screen.getByRole("button", { name: /skip plan/i }));
    // Skip design
    fireEvent.click(screen.getByRole("button", { name: /skip design/i }));

    // Now shows tasks as next
    expect(screen.getByText("tasks")).toBeInTheDocument();
  });

  it("shows workflow complete when all remaining deliverables are skipped", () => {
    render(
      <WorkflowOrchestrator
        {...defaultProps}
        currentDeliverableType="design"
        completedDeliverables={{
          plan: "Plan output",
          design: "Design output",
        }}
      />
    );

    // Skip tasks
    fireEvent.click(screen.getByRole("button", { name: /skip tasks/i }));
    // Skip execution
    fireEvent.click(
      screen.getByRole("button", { name: /skip execution/i })
    );

    expect(
      screen.getByText(/workflow complete/i)
    ).toBeInTheDocument();
  });

  it("does not include skipped deliverables in prior_context", () => {
    const onCreateTask = vi.fn();

    render(
      <WorkflowOrchestrator
        {...defaultProps}
        currentDeliverableType={null}
        completedDeliverables={{}}
        onCreateTask={onCreateTask}
      />
    );

    // Skip plan, then proceed to design
    fireEvent.click(screen.getByRole("button", { name: /skip plan/i }));
    fireEvent.click(
      screen.getByRole("button", { name: /proceed to design/i })
    );

    // prior_context should be empty since plan was skipped (not completed)
    expect(onCreateTask).toHaveBeenCalledWith("design", []);
  });

  it("renders skip button with correct aria-label", () => {
    render(
      <WorkflowOrchestrator
        {...defaultProps}
        currentDeliverableType={null}
        completedDeliverables={{}}
      />
    );

    expect(
      screen.getByRole("button", { name: /skip plan deliverable/i })
    ).toBeInTheDocument();
  });

  it("renders proceed button with correct aria-label", () => {
    render(
      <WorkflowOrchestrator
        {...defaultProps}
        currentDeliverableType={null}
        completedDeliverables={{}}
      />
    );

    expect(
      screen.getByRole("button", { name: /proceed to plan deliverable/i })
    ).toBeInTheDocument();
  });

  it("has accessible group role for orchestration controls", () => {
    render(
      <WorkflowOrchestrator
        {...defaultProps}
        currentDeliverableType={null}
        completedDeliverables={{}}
      />
    );

    expect(
      screen.getByRole("group", { name: /workflow orchestration controls/i })
    ).toBeInTheDocument();
  });

  it("has accessible status role for workflow complete state", () => {
    render(
      <WorkflowOrchestrator
        {...defaultProps}
        currentDeliverableType="execution"
        completedDeliverables={{
          plan: "P",
          design: "D",
          tasks: "T",
          execution: "E",
        }}
      />
    );

    expect(
      screen.getByRole("status", { name: /workflow complete/i })
    ).toBeInTheDocument();
  });
});
