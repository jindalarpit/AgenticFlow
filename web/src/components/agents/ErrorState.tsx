/* ─── Props ─── */

interface ErrorStateProps {
  error: Error;
  onRetry: () => void;
}

/**
 * Error state shown when the agent list API request fails.
 * Renders a destructive-colored alert icon, error message, and "Try Again" button.
 * No automatic retry — recovery only occurs through the manual button.
 *
 * Requirements: 16.1, 16.2, 16.3
 */
export function ErrorState({ error, onRetry }: ErrorStateProps) {
  const message =
    error.message || "Failed to load agents. Please try again.";

  return (
    <div
      className="flex flex-col items-center justify-center py-16 px-4 text-center"
      role="alert"
    >
      {/* Destructive Alert Icon */}
      <div className="flex h-14 w-14 items-center justify-center rounded-full bg-red-50">
        <AlertIcon />
      </div>

      {/* Error Message */}
      <h3 className="mt-4 text-sm font-semibold text-gray-900">
        Something went wrong
      </h3>
      <p className="mt-1 max-w-sm text-sm text-red-600">{message}</p>

      {/* Retry Button */}
      <button
        type="button"
        onClick={onRetry}
        className="mt-5 inline-flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 transition-colors"
        aria-label="Retry loading agents"
      >
        <RetryIcon />
        Try Again
      </button>
    </div>
  );
}

/* ─── Icons ─── */

function AlertIcon() {
  return (
    <svg
      className="h-7 w-7 text-red-500"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z"
      />
    </svg>
  );
}

function RetryIcon() {
  return (
    <svg
      className="h-4 w-4 text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99"
      />
    </svg>
  );
}
