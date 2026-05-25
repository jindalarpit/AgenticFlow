import { useEffect, useRef, useCallback } from "react";
import type { TaskMessage } from "../../hooks/useTasks";

export interface StreamingOutputProps {
  messages: TaskMessage[];
  isLive: boolean; // true when WS is connected and task is running
}

/**
 * Renders streaming task output messages in real-time.
 *
 * - Monospace font with preserved whitespace (pre-wrap)
 * - Auto-scrolls to bottom on new messages unless user has scrolled up >50px
 * - Shows a reconnecting indicator when WebSocket is disconnected (isLive=false)
 * - Distinguishes stdout (normal) from stderr (red/orange text)
 */
export function StreamingOutput({ messages, isLive }: StreamingOutputProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const userScrolledUpRef = useRef(false);

  /**
   * Determine if the user has scrolled up more than 50px from the bottom.
   * If so, we won't auto-scroll on new messages.
   */
  const handleScroll = useCallback(() => {
    const el = containerRef.current;
    if (!el) return;
    const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
    userScrolledUpRef.current = distanceFromBottom > 50;
  }, []);

  /**
   * Auto-scroll to bottom when new messages arrive,
   * unless the user has scrolled up more than 50px.
   */
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    if (!userScrolledUpRef.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [messages]);

  return (
    <div className="relative">
      {/* Reconnecting indicator */}
      {!isLive && messages.length > 0 && (
        <div
          className="sticky top-0 z-10 flex items-center gap-2 bg-yellow-50 border-b border-yellow-200 px-3 py-1.5"
          role="status"
          aria-live="polite"
        >
          <span
            className="inline-block h-2 w-2 rounded-full bg-yellow-500 animate-pulse"
            aria-hidden="true"
          />
          <span className="text-xs font-medium text-yellow-700">
            Reconnecting...
          </span>
        </div>
      )}

      {/* Scrollable output container */}
      <div
        ref={containerRef}
        onScroll={handleScroll}
        className="max-h-64 overflow-y-auto p-3 font-mono text-sm whitespace-pre-wrap"
        aria-label="Task output stream"
        role="log"
      >
        {messages.length === 0 && isLive && (
          <span className="text-gray-400 italic">Waiting for output...</span>
        )}

        {messages.length === 0 && !isLive && (
          <span className="text-gray-400 italic">No output received yet.</span>
        )}

        {messages.map((msg) => (
          <div
            key={msg.id}
            className={
              msg.stream === "stderr"
                ? "text-red-500"
                : "text-gray-800"
            }
          >
            {msg.content}
          </div>
        ))}
      </div>
    </div>
  );
}
