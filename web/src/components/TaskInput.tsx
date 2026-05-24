import { useState, type FormEvent } from "react";
import { useSendTaskInput } from "../hooks/useTaskInput";

interface TaskInputProps {
  taskId: string;
  isRunning: boolean;
  isWaitingForInput: boolean;
}

/**
 * Input field for sending text to a running task's stdin pipe.
 * Shown only while the task is running; highlights when the agent is waiting for input.
 *
 * Validates: Requirements 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7
 */
export function TaskInput({ taskId, isRunning, isWaitingForInput }: TaskInputProps) {
  const [text, setText] = useState("");
  const sendInput = useSendTaskInput();

  // Hide when task is in terminal status (not running)
  if (!isRunning) return null;

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (!text.trim() || sendInput.isPending) return;

    sendInput.mutate(
      { taskId, text },
      {
        onSuccess: () => setText(""),
        // On error: text is preserved (no setText call), error shown below
      }
    );
  };

  const inputClasses = isWaitingForInput
    ? "flex-1 rounded-md border border-yellow-400 bg-yellow-50 ring-2 ring-yellow-200 px-3 py-2 font-mono text-sm focus:outline-none focus:ring-2 focus:ring-yellow-300"
    : "flex-1 rounded-md border border-gray-300 bg-white px-3 py-2 font-mono text-sm focus:outline-none focus:ring-2 focus:ring-blue-200";

  return (
    <div className="mt-2">
      <form onSubmit={handleSubmit} className="flex gap-2">
        <input
          type="text"
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder={
            isWaitingForInput
              ? "Agent is waiting for input..."
              : "Send input to agent..."
          }
          className={inputClasses}
          disabled={sendInput.isPending}
          aria-label="Task input"
        />
        <button
          type="submit"
          disabled={!text.trim() || sendInput.isPending}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {sendInput.isPending ? "Sending..." : "Send"}
        </button>
      </form>
      {sendInput.isError && (
        <p className="mt-1 text-xs text-red-600" role="alert">
          {sendInput.error?.message || "Failed to send input"}
        </p>
      )}
    </div>
  );
}
