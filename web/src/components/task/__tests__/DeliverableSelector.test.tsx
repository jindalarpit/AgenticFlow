import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import {
  DeliverableSelector,
  DELIVERABLE_OPTIONS,
  DEFAULT_DELIVERABLES,
} from "../DeliverableSelector";

describe("DeliverableSelector", () => {
  const defaultProps = {
    value: [] as string[],
    onChange: vi.fn(),
  };

  it("renders all 4 checkboxes in canonical order (plan, design, tasks, execution)", () => {
    render(<DeliverableSelector {...defaultProps} />);

    const checkboxes = screen.getAllByRole("checkbox");
    expect(checkboxes).toHaveLength(4);

    // Verify canonical order via aria-labels
    expect(checkboxes[0]).toHaveAttribute(
      "aria-label",
      "Select Plan deliverable"
    );
    expect(checkboxes[1]).toHaveAttribute(
      "aria-label",
      "Select Design deliverable"
    );
    expect(checkboxes[2]).toHaveAttribute(
      "aria-label",
      "Select Tasks deliverable"
    );
    expect(checkboxes[3]).toHaveAttribute(
      "aria-label",
      "Select Execution deliverable"
    );
  });

  it("defaults to execution checked when value is empty", () => {
    render(<DeliverableSelector {...defaultProps} value={[]} />);

    const executionCheckbox = screen.getByLabelText(
      "Select Execution deliverable"
    ) as HTMLInputElement;
    expect(executionCheckbox.checked).toBe(true);

    // Other checkboxes should be unchecked
    const planCheckbox = screen.getByLabelText(
      "Select Plan deliverable"
    ) as HTMLInputElement;
    const designCheckbox = screen.getByLabelText(
      "Select Design deliverable"
    ) as HTMLInputElement;
    const tasksCheckbox = screen.getByLabelText(
      "Select Tasks deliverable"
    ) as HTMLInputElement;
    expect(planCheckbox.checked).toBe(false);
    expect(designCheckbox.checked).toBe(false);
    expect(tasksCheckbox.checked).toBe(false);
  });

  it("selecting a checkbox calls onChange with updated array", () => {
    const onChange = vi.fn();
    render(
      <DeliverableSelector
        value={["execution"]}
        onChange={onChange}
      />
    );

    fireEvent.click(screen.getByLabelText("Select Plan deliverable"));
    expect(onChange).toHaveBeenCalledWith(["plan", "execution"]);
  });

  it("deselecting all falls back to ['execution']", () => {
    const onChange = vi.fn();
    render(
      <DeliverableSelector
        value={["execution"]}
        onChange={onChange}
      />
    );

    // Deselect execution (the only selected item)
    fireEvent.click(screen.getByLabelText("Select Execution deliverable"));
    expect(onChange).toHaveBeenCalledWith(["execution"]);
  });

  it("selecting multiple maintains canonical order", () => {
    const onChange = vi.fn();
    // Start with design and execution selected
    render(
      <DeliverableSelector
        value={["design", "execution"]}
        onChange={onChange}
      />
    );

    // Add plan — result should be plan, design, execution (canonical order)
    fireEvent.click(screen.getByLabelText("Select Plan deliverable"));
    expect(onChange).toHaveBeenCalledWith(["plan", "design", "execution"]);
  });

  it("deselecting one of multiple keeps remaining in canonical order", () => {
    const onChange = vi.fn();
    render(
      <DeliverableSelector
        value={["plan", "design", "tasks", "execution"]}
        onChange={onChange}
      />
    );

    // Remove design — result should be plan, tasks, execution
    fireEvent.click(screen.getByLabelText("Select Design deliverable"));
    expect(onChange).toHaveBeenCalledWith(["plan", "tasks", "execution"]);
  });

  it("reflects provided value correctly", () => {
    render(
      <DeliverableSelector
        {...defaultProps}
        value={["plan", "tasks"]}
      />
    );

    const planCheckbox = screen.getByLabelText(
      "Select Plan deliverable"
    ) as HTMLInputElement;
    const designCheckbox = screen.getByLabelText(
      "Select Design deliverable"
    ) as HTMLInputElement;
    const tasksCheckbox = screen.getByLabelText(
      "Select Tasks deliverable"
    ) as HTMLInputElement;
    const executionCheckbox = screen.getByLabelText(
      "Select Execution deliverable"
    ) as HTMLInputElement;

    expect(planCheckbox.checked).toBe(true);
    expect(designCheckbox.checked).toBe(false);
    expect(tasksCheckbox.checked).toBe(true);
    expect(executionCheckbox.checked).toBe(false);
  });
});

describe("DeliverableSelector constants", () => {
  it("DELIVERABLE_OPTIONS has 4 items in canonical order", () => {
    expect(DELIVERABLE_OPTIONS).toHaveLength(4);
    expect(DELIVERABLE_OPTIONS[0].value).toBe("plan");
    expect(DELIVERABLE_OPTIONS[1].value).toBe("design");
    expect(DELIVERABLE_OPTIONS[2].value).toBe("tasks");
    expect(DELIVERABLE_OPTIONS[3].value).toBe("execution");
  });

  it("DEFAULT_DELIVERABLES is ['execution']", () => {
    expect(DEFAULT_DELIVERABLES).toEqual(["execution"]);
  });
});
