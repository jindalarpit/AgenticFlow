import { describe, it, expect } from "vitest";
import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { ErrorDisplay } from "../ErrorDisplay";

describe("ErrorDisplay", () => {
  describe("failed status", () => {
    it("renders error message with red-toned background and red left border", () => {
      const { container } = render(
        <ErrorDisplay status="failed" errorMessage="Something went wrong" />
      );
      const wrapper = container.firstElementChild as HTMLElement;
      expect(wrapper).toHaveClass("bg-red-50", "border-l-4", "border-red-500");
    });

    it("displays the error_message content", () => {
      render(
        <ErrorDisplay status="failed" errorMessage="Connection refused" />
      );
      expect(screen.getByText("Connection refused")).toBeInTheDocument();
    });

    it("displays fallback message when errorMessage is null", () => {
      render(<ErrorDisplay status="failed" errorMessage={null} />);
      expect(screen.getByText("Task failed")).toBeInTheDocument();
    });

    it("renders a red status badge", () => {
      render(
        <ErrorDisplay status="failed" errorMessage="Error" />
      );
      const badge = screen.getByText("Failed");
      expect(badge).toHaveClass("bg-red-100", "text-red-700");
    });
  });

  describe("cancelled status", () => {
    it("displays 'Task was cancelled by user' message", () => {
      render(<ErrorDisplay status="cancelled" errorMessage={null} />);
      expect(
        screen.getByText("Task was cancelled by user")
      ).toBeInTheDocument();
    });

    it("renders a grey status badge", () => {
      render(<ErrorDisplay status="cancelled" errorMessage={null} />);
      const badge = screen.getByText("Cancelled");
      expect(badge).toHaveClass("bg-gray-100", "text-gray-600");
    });

    it("uses grey-toned styling", () => {
      const { container } = render(
        <ErrorDisplay status="cancelled" errorMessage={null} />
      );
      const wrapper = container.firstElementChild as HTMLElement;
      expect(wrapper).toHaveClass("bg-gray-50", "border-l-4", "border-gray-300");
    });
  });

  describe("timeout status", () => {
    it("displays 'Task exceeded the allowed execution time' message", () => {
      render(<ErrorDisplay status="timeout" errorMessage={null} />);
      expect(
        screen.getByText("Task exceeded the allowed execution time")
      ).toBeInTheDocument();
    });

    it("renders a red status badge", () => {
      render(<ErrorDisplay status="timeout" errorMessage={null} />);
      const badge = screen.getByText("Timeout");
      expect(badge).toHaveClass("bg-red-100", "text-red-700");
    });

    it("uses red-toned styling", () => {
      const { container } = render(
        <ErrorDisplay status="timeout" errorMessage={null} />
      );
      const wrapper = container.firstElementChild as HTMLElement;
      expect(wrapper).toHaveClass("bg-red-50", "border-l-4", "border-red-500");
    });
  });
});
