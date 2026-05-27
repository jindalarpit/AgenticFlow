import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import {
  WorkspaceModeSelector,
  getPathError,
} from "../WorkspaceModeSelector";

describe("WorkspaceModeSelector", () => {
  const defaultProps = {
    mode: "isolated" as const,
    path: "",
    onModeChange: vi.fn(),
    onPathChange: vi.fn(),
  };

  it("renders both workspace mode options", () => {
    render(<WorkspaceModeSelector {...defaultProps} />);

    expect(screen.getByLabelText("New workspace")).toBeInTheDocument();
    expect(screen.getByLabelText("Existing project")).toBeInTheDocument();
  });

  it("selects isolated mode by default", () => {
    render(<WorkspaceModeSelector {...defaultProps} mode="isolated" />);

    const radio = screen.getByLabelText("New workspace") as HTMLInputElement;
    expect(radio.checked).toBe(true);
  });

  it("calls onModeChange when selecting existing mode", () => {
    const onModeChange = vi.fn();
    render(
      <WorkspaceModeSelector {...defaultProps} onModeChange={onModeChange} />
    );

    fireEvent.click(screen.getByLabelText("Existing project"));
    expect(onModeChange).toHaveBeenCalledWith("existing");
  });

  it("calls onModeChange when selecting isolated mode", () => {
    const onModeChange = vi.fn();
    render(
      <WorkspaceModeSelector
        {...defaultProps}
        mode="existing"
        onModeChange={onModeChange}
      />
    );

    fireEvent.click(screen.getByLabelText("New workspace"));
    expect(onModeChange).toHaveBeenCalledWith("isolated");
  });

  it("hides path input when mode is isolated", () => {
    render(<WorkspaceModeSelector {...defaultProps} mode="isolated" />);

    expect(screen.queryByLabelText(/directory path/i)).not.toBeInTheDocument();
  });

  it("shows path input when mode is existing", () => {
    render(<WorkspaceModeSelector {...defaultProps} mode="existing" />);

    expect(screen.getByLabelText(/directory path/i)).toBeInTheDocument();
  });

  it("calls onPathChange when typing in path input", () => {
    const onPathChange = vi.fn();
    render(
      <WorkspaceModeSelector
        {...defaultProps}
        mode="existing"
        onPathChange={onPathChange}
      />
    );

    fireEvent.change(screen.getByLabelText(/directory path/i), {
      target: { value: "/home/user/project" },
    });
    expect(onPathChange).toHaveBeenCalledWith("/home/user/project");
  });

  it("shows error when path is empty in existing mode", () => {
    render(
      <WorkspaceModeSelector {...defaultProps} mode="existing" path="" />
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Directory path is required"
    );
  });

  it("shows error when path is not absolute in existing mode", () => {
    render(
      <WorkspaceModeSelector
        {...defaultProps}
        mode="existing"
        path="relative/path"
      />
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Path must be absolute (start with /)"
    );
  });

  it("shows no error when path is valid absolute path", () => {
    render(
      <WorkspaceModeSelector
        {...defaultProps}
        mode="existing"
        path="/home/user/project"
      />
    );

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("marks path input as aria-invalid when validation fails", () => {
    render(
      <WorkspaceModeSelector {...defaultProps} mode="existing" path="" />
    );

    const input = screen.getByLabelText(/directory path/i);
    expect(input).toHaveAttribute("aria-invalid", "true");
  });

  it("marks path input as aria-invalid=false when valid", () => {
    render(
      <WorkspaceModeSelector
        {...defaultProps}
        mode="existing"
        path="/valid/path"
      />
    );

    const input = screen.getByLabelText(/directory path/i);
    expect(input).toHaveAttribute("aria-invalid", "false");
  });
});

describe("getPathError", () => {
  it("returns null for isolated mode regardless of path", () => {
    expect(getPathError("isolated", "")).toBeNull();
    expect(getPathError("isolated", "relative")).toBeNull();
  });

  it("returns error for empty path in existing mode", () => {
    expect(getPathError("existing", "")).toBe("Directory path is required");
    expect(getPathError("existing", "   ")).toBe("Directory path is required");
  });

  it("returns error for relative path in existing mode", () => {
    expect(getPathError("existing", "relative/path")).toBe(
      "Path must be absolute (start with /)"
    );
    expect(getPathError("existing", "./relative")).toBe(
      "Path must be absolute (start with /)"
    );
  });

  it("returns null for valid absolute path in existing mode", () => {
    expect(getPathError("existing", "/home/user/project")).toBeNull();
    expect(getPathError("existing", "/")).toBeNull();
  });
});
