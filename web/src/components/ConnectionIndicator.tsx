import type { ConnectionStatus } from "../lib/ws";

interface ConnectionIndicatorProps {
  status: ConnectionStatus;
}

const statusConfig: Record<
  ConnectionStatus,
  { dot: string; label: string; text: string }
> = {
  connected: {
    dot: "bg-green-500",
    label: "Connected",
    text: "text-green-700",
  },
  connecting: {
    dot: "bg-yellow-500",
    label: "Reconnecting...",
    text: "text-yellow-700",
  },
  disconnected: {
    dot: "bg-red-500",
    label: "Disconnected",
    text: "text-red-700",
  },
};

/**
 * Small indicator showing WebSocket connection status.
 * Displays a colored dot and label text.
 */
export function ConnectionIndicator({ status }: ConnectionIndicatorProps) {
  const config = statusConfig[status];

  return (
    <div className="flex items-center gap-1.5" aria-live="polite" role="status">
      <span
        className={`inline-block h-2 w-2 rounded-full ${config.dot}`}
        aria-hidden="true"
      />
      <span className={`text-xs font-medium ${config.text}`}>
        {config.label}
      </span>
    </div>
  );
}
