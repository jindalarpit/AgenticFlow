import { useState, useCallback } from "react";
import { Link } from "react-router-dom";
import type { Task } from "../../hooks/useTasks";
import { useCancelTask } from "../../hooks/useTasks";

interface TaskResultActionsProps {
  taskId: string;
  status: Task["status"];
  fullContent: string;
  onCancelSuccess?: () => void;
}

type CopyState = "idle" | "copied" | "error";
type CancelErrorState = "idle" | "error";

/**
 * Action buttons for the Task Result Panel.
 * - Copy to clipboard (always visible when content exists)
 * - View Detail link (always visible)
 * - Cancel button (visible only when task is running)
 */
export function TaskResultActions({
  taskId,
  status,
  fullContent,
  onCancelSuccess,
}: TaskResultActionsProps) {
  const [copyState, setCopyState] = useState<CopyState>("idle");
  const [cancelError, setCancelError] = useState<CancelErrorState>("idle");
  const cancelMutation = useCancelTask();

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(fullContent);
      setCopyState("copied");
      setTimeout(() => setCopyState("idle"), 2000);
    } catch {
      setCopyState("error");
      setTimeout(() => setCopyState("idle"), 2000);
    }
  }, [fullContent]);

  const handleCancel = useCallback(() => {
    setCancelError("idle");
    cancelMutation.mutate(taskId, {
      onSuccess: () => {
        onCancelSuccess?.();
      },
      onError: () => {
        setCancelError("error");
        setTimeout(() => setCancelError("idle"), 2000);
      },
    });
  }, [taskId, cancelMutation, onCancelSuccess]);

  return (
    <div className="flex items-center gap-2 px-4 py-3 border-t border-gray-200">
      {/* Copy to clipboard button */}
      <button
        onClick={handleCopy}
        disabled={!fullContent}
        className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded transition-colors ${
          copyState === "copied"
            ? "bg-green-100 text-green-700"
            : copyState === "error"
              ? "bg-red-100 text-red-700"
              : "bg-gray-100 text-gray-700 hover:bg-gray-200"
        } disabled:opacity-50 disabled:cursor-not-allowed`}
        aria-label="Copy result to clipboard"
      >
        {copyState === "copied" ? (
          <>
            <CheckIcon />
            Copied
          </>
        ) : copyState === "error" ? (
          <>
            <ErrorIcon />
            Failed
          </>
        ) : (
          <>
            <CopyIcon />
            Copy
          </>
        )}
      </button>

      {/* View Detail link */}
      <Link
        to={`/tasks/${taskId}`}
        className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded bg-gray-100 text-gray-700 hover:bg-gray-200 transition-colors"
      >
        <ExternalLinkIcon />
        View Detail
      </Link>

      {/* Cancel button (only when running) */}
      {status === "running" && (
        <button
          onClick={handleCancel}
          disabled={cancelMutation.isPending}
          className={`ml-auto inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded transition-colors ${
            cancelError === "error"
              ? "bg-red-100 text-red-700"
              : cancelMutation.isPending
                ? "bg-gray-100 text-gray-500 cursor-not-allowed"
                : "bg-red-50 text-red-600 hover:bg-red-100"
          }`}
          aria-label="Cancel task"
        >
          {cancelMutation.isPending ? (
            "Cancelling…"
          ) : cancelError === "error" ? (
            <>
              <ErrorIcon />
              Cancel failed
            </>
          ) : (
            <>
              <StopIcon />
              Cancel
            </>
          )}
        </button>
      )}
    </div>
  );
}

/* ─── Icons ─── */

function CopyIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      className="h-3.5 w-3.5"
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path d="M8 3a1 1 0 011-1h2a1 1 0 110 2H9a1 1 0 01-1-1z" />
      <path d="M6 3a2 2 0 00-2 2v11a2 2 0 002 2h8a2 2 0 002-2V5a2 2 0 00-2-2 3 3 0 01-3 3H9a3 3 0 01-3-3z" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      className="h-3.5 w-3.5"
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
        clipRule="evenodd"
      />
    </svg>
  );
}

function ErrorIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      className="h-3.5 w-3.5"
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z"
        clipRule="evenodd"
      />
    </svg>
  );
}

function ExternalLinkIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      className="h-3.5 w-3.5"
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path d="M11 3a1 1 0 100 2h2.586l-6.293 6.293a1 1 0 101.414 1.414L15 6.414V9a1 1 0 102 0V4a1 1 0 00-1-1h-5z" />
      <path d="M5 5a2 2 0 00-2 2v8a2 2 0 002 2h8a2 2 0 002-2v-3a1 1 0 10-2 0v3H5V7h3a1 1 0 000-2H5z" />
    </svg>
  );
}

function StopIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      className="h-3.5 w-3.5"
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z"
        clipRule="evenodd"
      />
    </svg>
  );
}
