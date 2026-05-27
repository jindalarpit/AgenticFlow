import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { StageApprovalPanel } from "../StageApprovalPanel";

// Mock the apiFetch function
vi.mock("../../../lib/api", () => ({
  apiFetch: vi.fn(),
}));

import { apiFetch } from "../../../lib/api";

const mockedApiFetch = vi.mocked(apiFetch);

describe("StageApprovalPanel", () => {
  const defaultProps = {
    taskId: "task-123",
    stageName: "plan",
    status: "awaiting_approval",
    onApprove: vi.fn(),
    onReject: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockedApiFetch.mockResolvedValue(undefined);
  });

  it("renders nothing when status is not awaiting_approval", () => {
    const { container } = render(
      <StageApprovalPanel {...defaultProps} status="pending" />
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders nothing when status is running", () => {
    const { container } = render(
      <StageApprovalPanel {...defaultProps} status="running" />
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders nothing when status is approved", () => {
    const { container } = render(
      <StageApprovalPanel {...defaultProps} status="approved" />
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders Approve and Reject buttons when status is awaiting_approval", () => {
    render(<StageApprovalPanel {...defaultProps} />);

    expect(screen.getByRole("button", { name: "Approve" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reject" })).toBeInTheDocument();
  });

  it("calls approve API endpoint on Approve click", async () => {
    render(<StageApprovalPanel {...defaultProps} />);

    fireEvent.click(screen.getByRole("button", { name: "Approve" }));

    await waitFor(() => {
      expect(mockedApiFetch).toHaveBeenCalledWith(
        "/api/tasks/task-123/stages/plan/approve",
        { method: "POST" }
      );
    });
  });

  it("calls onApprove callback after successful approval", async () => {
    render(<StageApprovalPanel {...defaultProps} />);

    fireEvent.click(screen.getByRole("button", { name: "Approve" }));

    await waitFor(() => {
      expect(defaultProps.onApprove).toHaveBeenCalled();
    });
  });

  it("shows feedback input when Reject is clicked", () => {
    render(<StageApprovalPanel {...defaultProps} />);

    fireEvent.click(screen.getByRole("button", { name: "Reject" }));

    expect(screen.getByLabelText(/Feedback/)).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText("Explain what should be changed...")
    ).toBeInTheDocument();
  });

  it("hides Approve button when feedback input is shown", () => {
    render(<StageApprovalPanel {...defaultProps} />);

    fireEvent.click(screen.getByRole("button", { name: "Reject" }));

    expect(screen.queryByRole("button", { name: "Approve" })).not.toBeInTheDocument();
  });

  it("shows Cancel button when feedback input is shown", () => {
    render(<StageApprovalPanel {...defaultProps} />);

    fireEvent.click(screen.getByRole("button", { name: "Reject" }));

    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
  });

  it("requires non-empty feedback before allowing rejection", () => {
    render(<StageApprovalPanel {...defaultProps} />);

    // Click Reject to show feedback input
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));

    // Reject button should be disabled when feedback is empty
    const rejectButton = screen.getByRole("button", { name: "Reject" });
    expect(rejectButton).toBeDisabled();

    // API should not have been called
    expect(mockedApiFetch).not.toHaveBeenCalled();
  });

  it("calls reject API endpoint with feedback on submit", async () => {
    render(<StageApprovalPanel {...defaultProps} />);

    // Click Reject to show feedback input
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));

    // Enter feedback
    fireEvent.change(screen.getByLabelText(/Feedback/), {
      target: { value: "Please add more detail to the plan" },
    });

    // Click Reject to submit
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));

    await waitFor(() => {
      expect(mockedApiFetch).toHaveBeenCalledWith(
        "/api/tasks/task-123/stages/plan/reject",
        {
          method: "POST",
          body: JSON.stringify({
            feedback: "Please add more detail to the plan",
          }),
        }
      );
    });
  });

  it("calls onReject callback after successful rejection", async () => {
    render(<StageApprovalPanel {...defaultProps} />);

    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    fireEvent.change(screen.getByLabelText(/Feedback/), {
      target: { value: "Needs improvement" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));

    await waitFor(() => {
      expect(defaultProps.onReject).toHaveBeenCalled();
    });
  });

  it("Cancel hides feedback input and resets state", () => {
    render(<StageApprovalPanel {...defaultProps} />);

    // Show feedback
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    fireEvent.change(screen.getByLabelText(/Feedback/), {
      target: { value: "some feedback" },
    });

    // Cancel
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    // Should be back to initial state
    expect(screen.getByRole("button", { name: "Approve" })).toBeInTheDocument();
    expect(screen.queryByLabelText(/Feedback/)).not.toBeInTheDocument();
  });

  it("shows loading state during approve API call", async () => {
    // Make the API call hang
    mockedApiFetch.mockReturnValue(new Promise(() => {}));

    render(<StageApprovalPanel {...defaultProps} />);

    fireEvent.click(screen.getByRole("button", { name: "Approve" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Approving…" })).toBeDisabled();
    });
  });

  it("shows loading state during reject API call", async () => {
    mockedApiFetch.mockReturnValue(new Promise(() => {}));

    render(<StageApprovalPanel {...defaultProps} />);

    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    fireEvent.change(screen.getByLabelText(/Feedback/), {
      target: { value: "Fix this" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Rejecting…" })).toBeDisabled();
    });
  });

  it("displays error message when approve API fails", async () => {
    mockedApiFetch.mockRejectedValue(new Error("Network error"));

    render(<StageApprovalPanel {...defaultProps} />);

    fireEvent.click(screen.getByRole("button", { name: "Approve" }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("Network error");
    });
  });

  it("displays error message when reject API fails", async () => {
    mockedApiFetch.mockRejectedValue(new Error("Server error"));

    render(<StageApprovalPanel {...defaultProps} />);

    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    fireEvent.change(screen.getByLabelText(/Feedback/), {
      target: { value: "Fix this" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("Server error");
    });
  });

  it("trims feedback whitespace before sending", async () => {
    render(<StageApprovalPanel {...defaultProps} />);

    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    fireEvent.change(screen.getByLabelText(/Feedback/), {
      target: { value: "  needs work  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));

    await waitFor(() => {
      expect(mockedApiFetch).toHaveBeenCalledWith(
        "/api/tasks/task-123/stages/plan/reject",
        {
          method: "POST",
          body: JSON.stringify({ feedback: "needs work" }),
        }
      );
    });
  });

  it("whitespace-only feedback is treated as empty (button stays disabled)", () => {
    render(<StageApprovalPanel {...defaultProps} />);

    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    fireEvent.change(screen.getByLabelText(/Feedback/), {
      target: { value: "   " },
    });

    // Reject button should remain disabled for whitespace-only input
    const rejectButton = screen.getByRole("button", { name: "Reject" });
    expect(rejectButton).toBeDisabled();

    // API should not have been called
    expect(mockedApiFetch).not.toHaveBeenCalled();
  });
});
