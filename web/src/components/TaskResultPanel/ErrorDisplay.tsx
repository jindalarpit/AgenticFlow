export interface ErrorDisplayProps {
  status: "failed" | "cancelled" | "timeout";
  errorMessage: string | null;
}

/**
 * Renders error/terminal status display for failed, cancelled, or timed-out tasks.
 *
 * - failed: red-toned background with red left border, displays error_message
 * - cancelled: grey styling with "Task was cancelled by user" message
 * - timeout: red-toned styling with "Task exceeded the allowed execution time" message
 */
export function ErrorDisplay({ status, errorMessage }: ErrorDisplayProps) {
  if (status === "failed") {
    return (
      <div className="bg-red-50 border-l-4 border-red-500 p-4 rounded-r">
        <div className="flex items-center gap-2 mb-2">
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-700">
            Failed
          </span>
        </div>
        <pre className="font-mono text-sm text-red-800 whitespace-pre-wrap break-words">
          {errorMessage || "Task failed"}
        </pre>
      </div>
    );
  }

  if (status === "cancelled") {
    return (
      <div className="bg-gray-50 border-l-4 border-gray-300 p-4 rounded-r">
        <div className="flex items-center gap-2 mb-2">
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-600">
            Cancelled
          </span>
        </div>
        <p className="font-mono text-sm text-gray-700">
          Task was cancelled by user
        </p>
      </div>
    );
  }

  // status === "timeout"
  return (
    <div className="bg-red-50 border-l-4 border-red-500 p-4 rounded-r">
      <div className="flex items-center gap-2 mb-2">
        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-700">
          Timeout
        </span>
      </div>
      <p className="font-mono text-sm text-red-800">
        Task exceeded the allowed execution time
      </p>
    </div>
  );
}
