import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { DeliverablePanel } from "../DeliverablePanel";

describe("DeliverablePanel", () => {
  describe("placeholder display when no output", () => {
    it("shows waiting placeholder when status is pending and no output", () => {
      render(<DeliverablePanel outputContent={null} status="pending" />);
      expect(screen.getByText("Waiting for agent response…")).toBeInTheDocument();
    });

    it("shows working placeholder when status is running and no output", () => {
      render(<DeliverablePanel outputContent={null} status="running" />);
      expect(
        screen.getByText("Agent is working on this deliverable…")
      ).toBeInTheDocument();
    });

    it("shows a spinner indicator when status is running", () => {
      const { container } = render(
        <DeliverablePanel outputContent={null} status="running" />
      );
      const spinner = container.querySelector(".animate-spin");
      expect(spinner).toBeInTheDocument();
    });

    it("does not show a spinner when status is pending", () => {
      const { container } = render(
        <DeliverablePanel outputContent={null} status="pending" />
      );
      const spinner = container.querySelector(".animate-spin");
      expect(spinner).not.toBeInTheDocument();
    });
  });

  describe("renders nothing when no output and non-pending status", () => {
    it("renders nothing when outputContent is null and status is completed", () => {
      const { container } = render(
        <DeliverablePanel outputContent={null} status="completed" />
      );
      expect(container.innerHTML).toBe("");
    });

    it("renders nothing when outputContent is null and status is failed", () => {
      const { container } = render(
        <DeliverablePanel outputContent={null} status="failed" />
      );
      expect(container.innerHTML).toBe("");
    });
  });

  describe("markdown content rendering", () => {
    it("renders heading content from markdown", () => {
      render(
        <DeliverablePanel outputContent="# Hello World" status="completed" />
      );
      expect(
        screen.getByRole("heading", { level: 1 })
      ).toHaveTextContent("Hello World");
    });

    it("renders paragraph content", () => {
      render(
        <DeliverablePanel
          outputContent="This is a paragraph."
          status="completed"
        />
      );
      expect(screen.getByText("This is a paragraph.")).toBeInTheDocument();
    });

    it("renders list items from markdown", () => {
      render(
        <DeliverablePanel
          outputContent={"- Item A\n- Item B\n- Item C"}
          status="completed"
        />
      );
      expect(screen.getByText("Item A")).toBeInTheDocument();
      expect(screen.getByText("Item B")).toBeInTheDocument();
      expect(screen.getByText("Item C")).toBeInTheDocument();
    });

    it("renders code blocks from markdown", () => {
      render(
        <DeliverablePanel
          outputContent={"```\nconst x = 42;\n```"}
          status="completed"
        />
      );
      expect(screen.getByText("const x = 42;")).toBeInTheDocument();
    });

    it("renders multi-level headings", () => {
      render(
        <DeliverablePanel
          outputContent={"# H1\n\n## H2\n\n### H3"}
          status="completed"
        />
      );
      expect(
        screen.getByRole("heading", { level: 1 })
      ).toHaveTextContent("H1");
      expect(
        screen.getByRole("heading", { level: 2 })
      ).toHaveTextContent("H2");
      expect(
        screen.getByRole("heading", { level: 3 })
      ).toHaveTextContent("H3");
    });
  });

  describe("renders output regardless of status when content exists", () => {
    it("renders markdown when status is completed", () => {
      render(
        <DeliverablePanel outputContent="# Done" status="completed" />
      );
      expect(screen.getByRole("region", { name: "Deliverable output" })).toBeInTheDocument();
    });

    it("renders markdown when status is running and output exists", () => {
      render(
        <DeliverablePanel outputContent="# Partial output" status="running" />
      );
      expect(
        screen.getByRole("heading", { level: 1 })
      ).toHaveTextContent("Partial output");
    });

    it("renders markdown when status is pending and output exists", () => {
      render(
        <DeliverablePanel outputContent="# Early output" status="pending" />
      );
      expect(
        screen.getByRole("heading", { level: 1 })
      ).toHaveTextContent("Early output");
    });
  });

  describe("accessibility", () => {
    it("has an accessible region label when showing placeholder", () => {
      render(<DeliverablePanel outputContent={null} status="pending" />);
      expect(
        screen.getByRole("region", { name: "Deliverable output" })
      ).toBeInTheDocument();
    });

    it("has an accessible region label when showing content", () => {
      render(
        <DeliverablePanel outputContent="Some content" status="completed" />
      );
      expect(
        screen.getByRole("region", { name: "Deliverable output" })
      ).toBeInTheDocument();
    });
  });
});
