import { useState, type FormEvent } from "react";
import { apiFetch } from "../../lib/api";

/* ─── Types ─── */

export type StageStatus = "pending" | "running" | "completed" | "failed";

export interface FollowUpInputProps {
  /** The task ID this stage belongs to. */
  taskId: string;
  /** The stage name (e.g., "plan", "design", "tasks", "execution"). */
  stageName: string;
  /** Current status of the stage. Input is enabled only when "completed". */
  stageStatus: StageStatus;
  /** Called after a successful follow-up message is sent. */
  onFollowUpSent?: () => void;
}

/**
 * FollowUpInput — Text input with send button for sending follow-up messages
 * to refine a deliverable's output. Calls POST /api/tasks/{taskId}/stages/{stageName}/follow-up.
 *
 * - Enabled when stageStatus is "completed"
 * - Disabled when stageStatus is "pending" or "running"
 * - Clears input and calls onFollowUpSent callback on success
 * - Shows error message if the request fails
 *
 * Validates: Requirements 2.1, 6.5
 */
export function FollowUpInput({
  taskId,
  stageName,
  stageStatus,
  onFollowUpSent,
}: FollowUpInputProps) {
  const [prompt, setPrompt] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isDisabled = stageStatus !== "completed";
  const canSend = !isDisabled && prompt.trim().length > 0 && !loading;

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!canSend) return;

    setLoading(true);
    setError(null);

    try {
      await apiFetch(`/api/tasks/${taskId}/stages/${stageName}/follow-up`, {
        method: "POST",
        body: JSON.stringify({ prompt: prompt.trim() }),
      });
      setPrompt("");
      onFollowUpSent?.();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to send follow-up message"
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4 space-y-2">
      {isDisabled && (
        <p className="text-sm text-gray-500 italic">
          Waiting for agent to complete…
        </p>
      )}

      <form onSubmit={handleSubmit} className="flex gap-2">
        <textarea
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          placeholder={
            isDisabled
              ? "Follow-up available after stage completes"
              : "Send a follow-up message to refine the output…"
          }
          rows={2}
          disabled={isDisabled || loading}
          className="flex-1 rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:border-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-100 disabled:bg-gray-50 disabled:text-gray-400 disabled:cursor-not-allowed resize-none"
          aria-label="Follow-up message"
        />
        <button
          type="submit"
          disabled={!canSend}
          className="self-end inline-flex items-center rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-200 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {loading ? "Sending…" : "Send"}
        </button>
      </form>

      {error && (
        <p className="text-sm text-red-600" role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
