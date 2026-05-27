import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { DeliverableNav, type DeliverableInfo } from "../DeliverableNav";

const defaultDeliverables: DeliverableInfo[] = [
  { type: "plan", status: "completed" },
  { type: "design", status: "running" },
  { type: "tasks", status: "pending" },
  { type: "execution", status: "skipped" },
];

describe("DeliverableNav", () => {
  describe("rendering", () => {
    it("renders nothing when deliverables array is empty", () => {
      const { container } = render(
        <DeliverableNav deliverables={[]} activeType="plan" onSelect={vi.fn()} />
      );
      expect(container.innerHTML).toBe("");
    });

    it("renders a tab for each deliverable", () => {
      render(
        <DeliverableNav
          deliverables={defaultDeliverables}
          activeType="plan"
          onSelect={vi.fn()}
        />
      );
      expect(screen.getByRole("tab", { name: /Plan/i })).toBeInTheDocument();
      expect(screen.getByRole("tab", { name: /Design/i })).toBeInTheDocument();
      expect(screen.getByRole("tab", { name: /Tasks/i })).toBeInTheDocument();
      expect(screen.getByRole("tab", { name: /Execution/i })).toBeInTheDocument();
    });

    it("renders a navigation landmark", () => {
      render(
        <DeliverableNav
          deliverables={defaultDeliverables}
          activeType="plan"
          onSelect={vi.fn()}
        />
      );
      expect(
        screen.getByRole("navigation", { name: "Deliverable navigation" })
      ).toBeInTheDocument();
    });
  });

  describe("active state", () => {
    it("marks the active tab with aria-selected=true", () => {
      render(
        <DeliverableNav
          deliverables={defaultDeliverables}
          activeType="design"
          onSelect={vi.fn()}
        />
      );
      const designTab = screen.getByRole("tab", { name: /Design/i });
      expect(designTab).toHaveAttribute("aria-selected", "true");
    });

    it("marks non-active tabs with aria-selected=false", () => {
      render(
        <DeliverableNav
          deliverables={defaultDeliverables}
          activeType="design"
          onSelect={vi.fn()}
        />
      );
      const planTab = screen.getByRole("tab", { name: /Plan/i });
      expect(planTab).toHaveAttribute("aria-selected", "false");
    });
  });

  describe("status indicators", () => {
    it("shows a green check icon for completed status", () => {
      const deliverables: DeliverableInfo[] = [
        { type: "plan", status: "completed" },
      ];
      const { container } = render(
        <DeliverableNav
          deliverables={deliverables}
          activeType="plan"
          onSelect={vi.fn()}
        />
      );
      const svg = container.querySelector("svg.text-green-500");
      expect(svg).toBeInTheDocument();
    });

    it("shows a spinner for running status", () => {
      const deliverables: DeliverableInfo[] = [
        { type: "design", status: "running" },
      ];
      const { container } = render(
        <DeliverableNav
          deliverables={deliverables}
          activeType="design"
          onSelect={vi.fn()}
        />
      );
      const spinner = container.querySelector(".animate-spin");
      expect(spinner).toBeInTheDocument();
    });

    it("shows a gray dot for pending status", () => {
      const deliverables: DeliverableInfo[] = [
        { type: "tasks", status: "pending" },
      ];
      const { container } = render(
        <DeliverableNav
          deliverables={deliverables}
          activeType="tasks"
          onSelect={vi.fn()}
        />
      );
      const dot = container.querySelector(".bg-gray-300.rounded-full");
      expect(dot).toBeInTheDocument();
    });

    it("shows strikethrough text for skipped status", () => {
      const deliverables: DeliverableInfo[] = [
        { type: "execution", status: "skipped" },
      ];
      const { container } = render(
        <DeliverableNav
          deliverables={deliverables}
          activeType="execution"
          onSelect={vi.fn()}
        />
      );
      const skippedText = container.querySelector(".line-through");
      expect(skippedText).toBeInTheDocument();
      expect(skippedText).toHaveTextContent("Execution");
    });
  });

  describe("interaction", () => {
    it("calls onSelect with the deliverable type when a tab is clicked", () => {
      const onSelect = vi.fn();
      render(
        <DeliverableNav
          deliverables={defaultDeliverables}
          activeType="plan"
          onSelect={onSelect}
        />
      );

      fireEvent.click(screen.getByRole("tab", { name: /Design/i }));
      expect(onSelect).toHaveBeenCalledWith("design");
    });

    it("calls onSelect when clicking the already active tab", () => {
      const onSelect = vi.fn();
      render(
        <DeliverableNav
          deliverables={defaultDeliverables}
          activeType="plan"
          onSelect={onSelect}
        />
      );

      fireEvent.click(screen.getByRole("tab", { name: /Plan/i }));
      expect(onSelect).toHaveBeenCalledWith("plan");
    });

    it("calls onSelect with correct type for each tab", () => {
      const onSelect = vi.fn();
      render(
        <DeliverableNav
          deliverables={defaultDeliverables}
          activeType="plan"
          onSelect={onSelect}
        />
      );

      fireEvent.click(screen.getByRole("tab", { name: /Tasks/i }));
      expect(onSelect).toHaveBeenCalledWith("tasks");

      fireEvent.click(screen.getByRole("tab", { name: /Execution/i }));
      expect(onSelect).toHaveBeenCalledWith("execution");
    });
  });

  describe("accessibility", () => {
    it("includes status in aria-label for each tab", () => {
      render(
        <DeliverableNav
          deliverables={defaultDeliverables}
          activeType="plan"
          onSelect={vi.fn()}
        />
      );
      expect(
        screen.getByRole("tab", { name: "Plan — completed" })
      ).toBeInTheDocument();
      expect(
        screen.getByRole("tab", { name: "Design — running" })
      ).toBeInTheDocument();
      expect(
        screen.getByRole("tab", { name: "Tasks — pending" })
      ).toBeInTheDocument();
      expect(
        screen.getByRole("tab", { name: "Execution — skipped" })
      ).toBeInTheDocument();
    });

    it("has a tablist role on the container", () => {
      render(
        <DeliverableNav
          deliverables={defaultDeliverables}
          activeType="plan"
          onSelect={vi.fn()}
        />
      );
      expect(screen.getByRole("tablist")).toBeInTheDocument();
    });
  });
});
