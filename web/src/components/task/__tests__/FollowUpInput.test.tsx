import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { FollowUpInput } from "../FollowUpInput";

// Mock the apiFetch function
vi.mock("../../../lib/api", () => ({
  apiFetch: vi.fn(),
}));

import { apiFetch } from "../../../lib/api";

const mockedApiFetch = vi.mocked(apiFetch);

describe("FollowUpInput", () => {
  const defaultProps = {
    taskId: "task-123",
    stageName: "plan",
    stageStatus: "completed" as const,
    onFollowUpSent: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockedApiFetch.mockResolvedValue(undefined);
  });

  it("renders textarea and send button", () => {
    render(<FollowUpInput {...defaultProps} />);

    expect(screen.getByLabelText("Follow-up message")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Send" })).toBeInTheDocument();
  });

  it("enables textarea when stageStatus is completed", () => {
    render(<FollowUpInput {...defaultProps} stageStatus="completed" />);

    const textarea = screen.getByLabelText("Follow-up message");
    expect(textarea).not.toBeDisabled();
  });

  it("disables textarea when stageStatus is pending", () => {
    render(<FollowUpInput {...defaultProps} stageStatus="pending" />);

    const textarea = screen.getByLabelText("Follow-up message");
    expect(textarea).toBeDisabled();
  });

  it("disables textarea when stageStatus is running", () => {
    render(<FollowUpInput {...defaultProps} stageStatus="running" />);

    const textarea = screen.getByLabelText("Follow-up message");
    expect(textarea).toBeDisabled();
  });

  it("shows waiting message when stageStatus is pending", () => {
    render(<FollowUpInput {...defaultProps} stageStatus="pending" />);

    expect(
      screen.getByText("Waiting for agent to complete…")
    ).toBeInTheDocument();
  });

  it("shows waiting message when stageStatus is running", () => {
    render(<FollowUpInput {...defaultProps} stageStatus="running" />);

    expect(
      screen.getByText("Waiting for agent to complete…")
    ).toBeInTheDocument();
  });

  it("does not show waiting message when stageStatus is completed", () => {
    render(<FollowUpInput {...defaultProps} stageStatus="completed" />);

    expect(
      screen.queryByText("Waiting for agent to complete…")
    ).not.toBeInTheDocument();
  });

  it("send button is disabled when textarea is empty", () => {
    render(<FollowUpInput {...defaultProps} />);

    const button = screen.getByRole("button", { name: "Send" });
    expect(button).toBeDisabled();
  });

  it("send button is enabled when textarea has content and stage is completed", () => {
    render(<FollowUpInput {...defaultProps} />);

    fireEvent.change(screen.getByLabelText("Follow-up message"), {
      target: { value: "Please add more detail" },
    });

    const button = screen.getByRole("button", { name: "Send" });
    expect(button).not.toBeDisabled();
  });

  it("send button is disabled when textarea has only whitespace", () => {
    render(<FollowUpInput {...defaultProps} />);

    fireEvent.change(screen.getByLabelText("Follow-up message"), {
      target: { value: "   " },
    });

    const button = screen.getByRole("button", { name: "Send" });
    expect(button).toBeDisabled();
  });

  it("calls follow-up API endpoint on submit", async () => {
    render(<FollowUpInput {...defaultProps} />);

    fireEvent.change(screen.getByLabelText("Follow-up message"), {
      target: { value: "Add error handling" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(mockedApiFetch).toHaveBeenCalledWith(
        "/api/tasks/task-123/stages/plan/follow-up",
        {
          method: "POST",
          body: JSON.stringify({ prompt: "Add error handling" }),
        }
      );
    });
  });

  it("trims prompt whitespace before sending", async () => {
    render(<FollowUpInput {...defaultProps} />);

    fireEvent.change(screen.getByLabelText("Follow-up message"), {
      target: { value: "  refine the plan  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(mockedApiFetch).toHaveBeenCalledWith(
        "/api/tasks/task-123/stages/plan/follow-up",
        {
          method: "POST",
          body: JSON.stringify({ prompt: "refine the plan" }),
        }
      );
    });
  });

  it("clears input after successful send", async () => {
    render(<FollowUpInput {...defaultProps} />);

    fireEvent.change(screen.getByLabelText("Follow-up message"), {
      target: { value: "Add more detail" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(screen.getByLabelText("Follow-up message")).toHaveValue("");
    });
  });

  it("calls onFollowUpSent callback after successful send", async () => {
    render(<FollowUpInput {...defaultProps} />);

    fireEvent.change(screen.getByLabelText("Follow-up message"), {
      target: { value: "Improve the design" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(defaultProps.onFollowUpSent).toHaveBeenCalled();
    });
  });

  it("shows loading state during API call", async () => {
    mockedApiFetch.mockReturnValue(new Promise(() => {}));

    render(<FollowUpInput {...defaultProps} />);

    fireEvent.change(screen.getByLabelText("Follow-up message"), {
      target: { value: "Some feedback" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Sending…" })
      ).toBeDisabled();
    });
  });

  it("displays error message when API call fails", async () => {
    mockedApiFetch.mockRejectedValue(new Error("Network error"));

    render(<FollowUpInput {...defaultProps} />);

    fireEvent.change(screen.getByLabelText("Follow-up message"), {
      target: { value: "Some feedback" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("Network error");
    });
  });

  it("preserves input text when API call fails", async () => {
    mockedApiFetch.mockRejectedValue(new Error("Server error"));

    render(<FollowUpInput {...defaultProps} />);

    fireEvent.change(screen.getByLabelText("Follow-up message"), {
      target: { value: "My feedback" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });

    expect(screen.getByLabelText("Follow-up message")).toHaveValue(
      "My feedback"
    );
  });

  it("does not call API when stage is not completed", () => {
    render(<FollowUpInput {...defaultProps} stageStatus="running" />);

    // Textarea is disabled, so we can't type, but verify button is disabled
    const button = screen.getByRole("button", { name: "Send" });
    expect(button).toBeDisabled();
    expect(mockedApiFetch).not.toHaveBeenCalled();
  });

  it("disables textarea when stageStatus is failed", () => {
    render(<FollowUpInput {...defaultProps} stageStatus="failed" />);

    const textarea = screen.getByLabelText("Follow-up message");
    expect(textarea).toBeDisabled();
  });

  it("uses correct API path with different taskId and stageName", async () => {
    render(
      <FollowUpInput
        {...defaultProps}
        taskId="task-456"
        stageName="design"
      />
    );

    fireEvent.change(screen.getByLabelText("Follow-up message"), {
      target: { value: "Update the design" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(mockedApiFetch).toHaveBeenCalledWith(
        "/api/tasks/task-456/stages/design/follow-up",
        {
          method: "POST",
          body: JSON.stringify({ prompt: "Update the design" }),
        }
      );
    });
  });
});
