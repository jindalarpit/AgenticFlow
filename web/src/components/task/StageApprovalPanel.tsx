import { useState } from "react";
import { apiFetch } from "../../lib/api";

/* ─── Types ─── */

export interface StageApprovalPanelProps {
  /** The task ID this stage belongs to. */
  taskId: string;
  /** The stage name (e.g., "plan", "design", "tasks", "execution"). */
  stageName: string;
  /** Current status of the stage. Panel only renders when "awaiting_approval". */
  status: string;
  /** Called after a successful approve action. */
  onApprove?: () => void;
  /** Called after a successful reject action. */
  onReject?: () => void;
}

/**
 * StageApprovalPanel — Displays Approve/Reject buttons when a workflow stage
 * is in `awaiting_approval` status. On reject, shows a text input for feedback
 * that must be non-empty before submission.
 *
 * Calls:
 *   POST /api/tasks/{taskId}/stages/{stageName}/approve
 *   POST /api/tasks/{taskId}/stages/{stageName}/reject  { feedback: "..." }
 *
 * Validates: Requirements 4.7, 4.8
 */
export function StageApprovalPanel({
  taskId,
  stageName,
  status,
  onApprove,
  onReject,
}: StageApprovalPanelProps) {
  const [showFeedback, setShowFeedback] = useState(false);
  const [feedback, setFeedback] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Only render when stage is awaiting approval
  if (status !== "awaiting_approval") {
    return null;
  }

  async function handleApprove() {
    setLoading(true);
    setError(null);
    try {
      await apiFetch(`/api/tasks/${taskId}/stages/${stageName}/approve`, {
        method: "POST",
      });
      onApprove?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to approve stage");
    } finally {
      setLoading(false);
    }
  }

  async function handleReject() {
    if (!showFeedback) {
      setShowFeedback(true);
      return;
    }

    // Require non-empty feedback
    if (!feedback.trim()) {
      setError("Feedback is required when rejecting a stage");
      return;
    }

    setLoading(true);
    setError(null);
    try {
      await apiFetch(`/api/tasks/${taskId}/stages/${stageName}/reject`, {
        method: "POST",
        body: JSON.stringify({ feedback: feedback.trim() }),
      });
      setShowFeedback(false);
      setFeedback("");
      onReject?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to reject stage");
    } finally {
      setLoading(false);
    }
  }

  function handleCancel() {
    setShowFeedback(false);
    setFeedback("");
    setError(null);
  }

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4 space-y-3">
      <p className="text-sm font-medium text-gray-700">
        Review stage output and approve or reject to continue.
      </p>

      {error && (
        <p className="text-sm text-red-600" role="alert">
          {error}
        </p>
      )}

      {showFeedback && (
        <div className="space-y-2">
          <label
            htmlFor="rejection-feedback"
            className="block text-sm font-medium text-gray-700"
          >
            Feedback <span className="text-red-500">*</span>
          </label>
          <textarea
            id="rejection-feedback"
            value={feedback}
            onChange={(e) => setFeedback(e.target.value)}
            placeholder="Explain what should be changed..."
            rows={3}
            className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:border-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-100"
            aria-required="true"
            aria-invalid={!!error && !feedback.trim()}
            disabled={loading}
          />
        </div>
      )}

      <div className="flex items-center gap-3">
        {!showFeedback && (
          <button
            type="button"
            onClick={handleApprove}
            disabled={loading}
            className="inline-flex items-center rounded-lg bg-green-600 px-4 py-2 text-sm font-medium text-white hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-green-200 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading ? "Approving…" : "Approve"}
          </button>
        )}

        <button
          type="button"
          onClick={handleReject}
          disabled={loading || (showFeedback && !feedback.trim())}
          className="inline-flex items-center rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-red-200 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {loading && showFeedback ? "Rejecting…" : "Reject"}
        </button>

        {showFeedback && (
          <button
            type="button"
            onClick={handleCancel}
            disabled={loading}
            className="inline-flex items-center rounded-lg border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-gray-100 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Cancel
          </button>
        )}
      </div>
    </div>
  );
}
