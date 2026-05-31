import type { TokenUsage } from "../../hooks/useTasks";

interface TokenUsageDisplayProps {
  tokenUsage: TokenUsage;
}

/**
 * Displays token usage metadata (prompt, completion, total tokens)
 * shown after a task completes with online AI provider execution.
 */
export function TokenUsageDisplay({ tokenUsage }: TokenUsageDisplayProps) {
  return (
    <div className="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3">
      <p className="text-xs font-medium text-gray-500 mb-2">Token Usage</p>
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-gray-500">Prompt:</span>
          <span className="text-sm font-medium text-gray-700">
            {tokenUsage.prompt_tokens.toLocaleString()}
          </span>
        </div>
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-gray-500">Completion:</span>
          <span className="text-sm font-medium text-gray-700">
            {tokenUsage.completion_tokens.toLocaleString()}
          </span>
        </div>
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-gray-500">Total:</span>
          <span className="text-sm font-semibold text-gray-900">
            {tokenUsage.total_tokens.toLocaleString()}
          </span>
        </div>
      </div>
    </div>
  );
}
