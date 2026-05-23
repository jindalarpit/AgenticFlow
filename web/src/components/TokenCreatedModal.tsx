import { useState, useCallback, useEffect } from "react";

interface TokenCreatedModalProps {
  /** The full unmasked token value to display */
  token: string;
  /** Called when the user closes the modal */
  onClose: () => void;
}

/**
 * Modal displayed after a PAT is successfully created.
 * Shows the full unmasked token with a copy-to-clipboard button.
 * Warns the user that the token won't be shown again.
 * Stays open until the user explicitly clicks "Done".
 *
 * Requirements: 4.1, 4.2, 4.3, 4.4
 */
export function TokenCreatedModal({ token, onClose }: TokenCreatedModalProps) {
  const [copyState, setCopyState] = useState<"idle" | "copied" | "error">(
    "idle"
  );

  // Reset copy state after 2 seconds on success
  useEffect(() => {
    if (copyState === "copied") {
      const timer = setTimeout(() => setCopyState("idle"), 2000);
      return () => clearTimeout(timer);
    }
  }, [copyState]);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(token);
      setCopyState("copied");
    } catch {
      setCopyState("error");
    }
  }, [token]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center"
      role="dialog"
      aria-modal="true"
      aria-labelledby="token-modal-title"
    >
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/40" aria-hidden="true" />

      {/* Modal content */}
      <div className="relative bg-white rounded-lg shadow-xl w-full max-w-lg mx-4 p-6">
        <h2
          id="token-modal-title"
          className="text-lg font-semibold text-gray-900 mb-4"
        >
          Token Created
        </h2>

        {/* Warning */}
        <div className="flex items-start gap-2 mb-4 p-3 bg-amber-50 border border-amber-200 rounded-md">
          <span className="text-amber-600 text-lg leading-none" aria-hidden="true">
            ⚠
          </span>
          <p className="text-sm text-amber-800">
            This token will not be shown again. Copy it now.
          </p>
        </div>

        {/* Token display */}
        <div className="mb-4">
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Your new token
          </label>
          <div className="flex items-center gap-2">
            <code className="flex-1 block p-3 bg-gray-100 border border-gray-200 rounded-md text-sm font-mono text-gray-900 break-all select-all">
              {token}
            </code>
            <button
              onClick={handleCopy}
              className={`shrink-0 inline-flex items-center gap-1.5 px-3 py-2 text-sm font-medium rounded-md focus:outline-none focus:ring-2 focus:ring-offset-2 transition-colors ${
                copyState === "copied"
                  ? "bg-green-100 text-green-700 focus:ring-green-500"
                  : "bg-blue-600 text-white hover:bg-blue-700 focus:ring-blue-500"
              }`}
              aria-label="Copy token to clipboard"
            >
              {copyState === "copied" ? (
                <>
                  <CheckIcon />
                  Copied!
                </>
              ) : (
                <>
                  <CopyIcon />
                  Copy
                </>
              )}
            </button>
          </div>
        </div>

        {/* Clipboard error message */}
        {copyState === "error" && (
          <p className="mb-4 text-sm text-red-600" role="alert">
            Failed to copy to clipboard. Please select the token above and copy
            it manually.
          </p>
        )}

        {/* Close button */}
        <div className="flex justify-end">
          <button
            onClick={onClose}
            className="inline-flex items-center px-4 py-2 bg-gray-100 text-gray-700 text-sm font-medium rounded-md hover:bg-gray-200 focus:outline-none focus:ring-2 focus:ring-gray-500 focus:ring-offset-2"
          >
            Done
          </button>
        </div>
      </div>
    </div>
  );
}

/* ─── Icons ─── */

function CopyIcon() {
  return (
    <svg
      className="h-4 w-4"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 01-1.125-1.125V7.875c0-.621.504-1.125 1.125-1.125H6.75a9.06 9.06 0 011.5.124m7.5 10.376h3.375c.621 0 1.125-.504 1.125-1.125V11.25c0-4.46-3.243-8.161-7.5-8.876a9.06 9.06 0 00-1.5-.124H9.375c-.621 0-1.125.504-1.125 1.125v3.5m7.5 10.375H9.375a1.125 1.125 0 01-1.125-1.125v-9.25m12 6.625v-1.875a3.375 3.375 0 00-3.375-3.375h-1.5a1.125 1.125 0 01-1.125-1.125v-1.5a3.375 3.375 0 00-3.375-3.375H9.75"
      />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg
      className="h-4 w-4"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={2}
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M4.5 12.75l6 6 9-13.5"
      />
    </svg>
  );
}
