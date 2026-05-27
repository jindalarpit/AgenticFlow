import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { StageOutputViewer } from "../StageOutputViewer";

describe("StageOutputViewer", () => {
  const defaultProps = {
    outputContent: "# Plan\n\nThis is the plan content.",
    stageName: "plan",
    status: "awaiting_approval" as const,
  };

  describe("visibility based on status", () => {
    it("renders when status is awaiting_approval", () => {
      render(<StageOutputViewer {...defaultProps} status="awaiting_approval" />);
      expect(screen.getByRole("region", { name: "Plan Output" })).toBeInTheDocument();
    });

    it("renders when status is completed", () => {
      render(<StageOutputViewer {...defaultProps} status="completed" />);
      expect(screen.getByRole("region", { name: "Plan Output" })).toBeInTheDocument();
    });

    it("renders when status is approved", () => {
      render(<StageOutputViewer {...defaultProps} status="approved" />);
      expect(screen.getByRole("region", { name: "Plan Output" })).toBeInTheDocument();
    });

    it("renders nothing when status is pending", () => {
      const { container } = render(
        <StageOutputViewer {...defaultProps} status="pending" />
      );
      expect(container.innerHTML).toBe("");
    });

    it("renders nothing when status is running", () => {
      const { container } = render(
        <StageOutputViewer {...defaultProps} status="running" />
      );
      expect(container.innerHTML).toBe("");
    });

    it("renders nothing when status is rejected", () => {
      const { container } = render(
        <StageOutputViewer {...defaultProps} status="rejected" />
      );
      expect(container.innerHTML).toBe("");
    });

    it("renders nothing when status is failed", () => {
      const { container } = render(
        <StageOutputViewer {...defaultProps} status="failed" />
      );
      expect(container.innerHTML).toBe("");
    });
  });

  describe("null outputContent handling", () => {
    it("renders nothing when outputContent is null even with visible status", () => {
      const { container } = render(
        <StageOutputViewer
          outputContent={null}
          stageName="plan"
          status="awaiting_approval"
        />
      );
      expect(container.innerHTML).toBe("");
    });

    it("renders nothing when outputContent is null and status is completed", () => {
      const { container } = render(
        <StageOutputViewer
          outputContent={null}
          stageName="design"
          status="completed"
        />
      );
      expect(container.innerHTML).toBe("");
    });
  });

  describe("markdown content rendering", () => {
    it("renders heading content", () => {
      render(
        <StageOutputViewer
          outputContent="# Main Heading"
          stageName="plan"
          status="awaiting_approval"
        />
      );
      expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
        "Main Heading"
      );
    });

    it("renders paragraph content", () => {
      render(
        <StageOutputViewer
          outputContent="This is a paragraph of text."
          stageName="design"
          status="completed"
        />
      );
      expect(screen.getByText("This is a paragraph of text.")).toBeInTheDocument();
    });

    it("renders multiple headings at different levels", () => {
      render(
        <StageOutputViewer
          outputContent={"# H1\n\n## H2\n\n### H3"}
          stageName="tasks"
          status="approved"
        />
      );
      expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("H1");
      expect(screen.getByRole("heading", { level: 2 })).toHaveTextContent("H2");
      // Level 3 has two matches (component header + markdown h3), so use getAllByRole
      const h3Elements = screen.getAllByRole("heading", { level: 3 });
      expect(h3Elements.some((el) => el.textContent === "H3")).toBe(true);
    });

    it("renders unordered list items", () => {
      render(
        <StageOutputViewer
          outputContent={"- Item one\n- Item two\n- Item three"}
          stageName="tasks"
          status="awaiting_approval"
        />
      );
      expect(screen.getByText("Item one")).toBeInTheDocument();
      expect(screen.getByText("Item two")).toBeInTheDocument();
      expect(screen.getByText("Item three")).toBeInTheDocument();
    });

    it("renders code blocks", () => {
      render(
        <StageOutputViewer
          outputContent={"```\nconst x = 1;\n```"}
          stageName="design"
          status="completed"
        />
      );
      expect(screen.getByText("const x = 1;")).toBeInTheDocument();
    });
  });

  describe("stage label display", () => {
    it("shows Plan Output header for plan stage", () => {
      render(<StageOutputViewer {...defaultProps} stageName="plan" />);
      expect(screen.getByText("Plan Output")).toBeInTheDocument();
    });

    it("shows Design Output header for design stage", () => {
      render(<StageOutputViewer {...defaultProps} stageName="design" />);
      expect(screen.getByText("Design Output")).toBeInTheDocument();
    });

    it("shows Tasks Output header for tasks stage", () => {
      render(<StageOutputViewer {...defaultProps} stageName="tasks" />);
      expect(screen.getByText("Tasks Output")).toBeInTheDocument();
    });

    it("shows Execution Output header for execution stage", () => {
      render(<StageOutputViewer {...defaultProps} stageName="execution" />);
      expect(screen.getByText("Execution Output")).toBeInTheDocument();
    });

    it("capitalizes unknown stage names as fallback", () => {
      render(<StageOutputViewer {...defaultProps} stageName="review" />);
      expect(screen.getByText("Review Output")).toBeInTheDocument();
    });
  });

  describe("status badge display", () => {
    it("shows Awaiting Approval badge when status is awaiting_approval", () => {
      render(<StageOutputViewer {...defaultProps} status="awaiting_approval" />);
      expect(screen.getByText("Awaiting Approval")).toBeInTheDocument();
    });

    it("shows Completed badge when status is completed", () => {
      render(<StageOutputViewer {...defaultProps} status="completed" />);
      expect(screen.getByText("Completed")).toBeInTheDocument();
    });

    it("shows Approved badge when status is approved", () => {
      render(<StageOutputViewer {...defaultProps} status="approved" />);
      expect(screen.getByText("Approved")).toBeInTheDocument();
    });
  });
});
