import { useState } from "react";
import { Link } from "react-router-dom";
import { truncateResultContent } from "../../lib/taskResultUtils";

export interface FinalResultProps {
  content: string;
  taskId: string;
}

/**
 * Displays the final result content of a completed task.
 *
 * Uses truncateResultContent to determine whether content should be truncated.
 * When truncated, provides an expand/collapse toggle and a "View Full Result" link.
 * Content is rendered in monospace font with pre-wrap whitespace.
 *
 * Validates: Requirements 1.3, 3.3, 3.4
 */
export function FinalResult({ content, taskId }: FinalResultProps) {
  const [expanded, setExpanded] = useState(false);
  const { displayText, isTruncated, fullText } = truncateResultContent(content);

  const visibleText = expanded ? fullText : displayText;

  return (
    <div className="space-y-2">
      <pre className="font-mono whitespace-pre-wrap text-sm text-gray-800 bg-gray-50 rounded-md p-3 overflow-x-auto">
        {visibleText}
      </pre>

      {isTruncated && (
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() => setExpanded((prev) => !prev)}
            className="text-sm text-blue-600 hover:text-blue-800 font-medium"
          >
            {expanded ? "Show Less" : "Show More"}
          </button>

          <Link
            to={`/tasks/${taskId}`}
            className="text-sm text-blue-600 hover:text-blue-800 underline"
          >
            View Full Result
          </Link>
        </div>
      )}
    </div>
  );
}
