import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { SortDirectionToggle } from "../SortDirectionToggle";

describe("SortDirectionToggle", () => {
  it("calls onChange with 'newest_first' when clicking 'Newest first' button", () => {
    const onChange = vi.fn();
    render(
      <SortDirectionToggle direction="chronological" onChange={onChange} />
    );

    fireEvent.click(screen.getByText("Newest first"));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith("newest_first");
  });

  it("calls onChange with 'chronological' when clicking 'Chronological' button", () => {
    const onChange = vi.fn();
    render(
      <SortDirectionToggle direction="newest_first" onChange={onChange} />
    );

    fireEvent.click(screen.getByText("Chronological"));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith("chronological");
  });

  it("applies selected visual style to the active 'Chronological' button", () => {
    render(
      <SortDirectionToggle direction="chronological" onChange={() => {}} />
    );

    const chronologicalBtn = screen.getByText("Chronological");
    const newestFirstBtn = screen.getByText("Newest first");

    // Active button has white background and dark text
    expect(chronologicalBtn).toHaveClass("bg-white", "text-gray-900", "shadow-sm");
    // Inactive button has muted text without white background
    expect(newestFirstBtn).toHaveClass("text-gray-500");
    expect(newestFirstBtn).not.toHaveClass("bg-white");
  });

  it("applies selected visual style to the active 'Newest first' button", () => {
    render(
      <SortDirectionToggle direction="newest_first" onChange={() => {}} />
    );

    const chronologicalBtn = screen.getByText("Chronological");
    const newestFirstBtn = screen.getByText("Newest first");

    // Active button has white background and dark text
    expect(newestFirstBtn).toHaveClass("bg-white", "text-gray-900", "shadow-sm");
    // Inactive button has muted text without white background
    expect(chronologicalBtn).toHaveClass("text-gray-500");
    expect(chronologicalBtn).not.toHaveClass("bg-white");
  });

  it("renders with correct aria-pressed attributes", () => {
    render(
      <SortDirectionToggle direction="chronological" onChange={() => {}} />
    );

    const chronologicalBtn = screen.getByText("Chronological");
    const newestFirstBtn = screen.getByText("Newest first");

    expect(chronologicalBtn).toHaveAttribute("aria-pressed", "true");
    expect(newestFirstBtn).toHaveAttribute("aria-pressed", "false");
  });

  it("renders as a group with accessible label", () => {
    render(
      <SortDirectionToggle direction="chronological" onChange={() => {}} />
    );

    const group = screen.getByRole("group", { name: "Sort direction" });
    expect(group).toBeInTheDocument();
  });
});
