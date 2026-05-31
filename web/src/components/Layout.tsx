import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Link, useLocation } from "react-router-dom";
import { useWebSocket } from "../hooks/useWebSocket";
import { ConnectionIndicator } from "./ConnectionIndicator";
import { useWSClient } from "../contexts/WebSocketContext";

interface LayoutProps {
  children: React.ReactNode;
}

/**
 * Application layout for authenticated pages.
 * - Initializes WebSocket connection
 * - Shows connection status indicator in the header
 * - Sets up global React Query cache invalidation on WebSocket events
 * - Refetches stale data on WebSocket reconnection (events may have been missed)
 */
export function Layout({ children }: LayoutProps) {
  const { status } = useWebSocket();
  const queryClient = useQueryClient();
  const wsClient = useWSClient();

  // Refetch agent (and other) data on WebSocket reconnection.
  // Events may have been missed while disconnected, so invalidate caches
  // when the connection is re-established.
  useEffect(() => {
    const unsubStatus = wsClient.onStatusChange((newStatus) => {
      if (newStatus === "connected") {
        queryClient.invalidateQueries({ queryKey: ["agents"] });
        queryClient.invalidateQueries({ queryKey: ["tasks"] });
        queryClient.invalidateQueries({ queryKey: ["daemons"] });
      }
    });

    return () => {
      unsubStatus();
    };
  }, [queryClient, wsClient]);

  // Global WebSocket event → React Query cache invalidation
  useEffect(() => {
    const unsubTaskCreated = wsClient.on("task_created", () => {
      queryClient.invalidateQueries({ queryKey: ["tasks"] });
    });
    const unsubTaskStarted = wsClient.on("task_started", () => {
      queryClient.invalidateQueries({ queryKey: ["tasks"] });
    });
    const unsubTaskCompleted = wsClient.on("task_completed", () => {
      queryClient.invalidateQueries({ queryKey: ["tasks"] });
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    });
    const unsubTaskFailed = wsClient.on("task_failed", () => {
      queryClient.invalidateQueries({ queryKey: ["tasks"] });
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    });
    const unsubDaemonConnected = wsClient.on("daemon_connected", () => {
      queryClient.invalidateQueries({ queryKey: ["daemons"] });
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    });
    const unsubDaemonDisconnected = wsClient.on("daemon_disconnected", () => {
      queryClient.invalidateQueries({ queryKey: ["daemons"] });
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    });
    const unsubAgentStatusChanged = wsClient.on("agent_status_changed", () => {
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    });
    const unsubAgentCreated = wsClient.on("agent_created", () => {
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    });
    const unsubAgentUpdated = wsClient.on("agent_updated", () => {
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    });
    const unsubAgentDeleted = wsClient.on("agent_deleted", () => {
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    });

    return () => {
      unsubTaskCreated();
      unsubTaskStarted();
      unsubTaskCompleted();
      unsubTaskFailed();
      unsubDaemonConnected();
      unsubDaemonDisconnected();
      unsubAgentStatusChanged();
      unsubAgentCreated();
      unsubAgentUpdated();
      unsubAgentDeleted();
    };
  }, [queryClient, wsClient]);

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      <header className="bg-white border-b border-gray-200 px-6 py-3 flex items-center justify-between">
        <div className="flex items-center gap-6">
          <Link to="/" className="text-lg font-semibold text-gray-900">
            AgenticFlow
          </Link>
          <NavLinks />
        </div>
        <ConnectionIndicator status={status} />
      </header>
      <main className="flex-1 overflow-hidden">{children}</main>
    </div>
  );
}

const navItems = [
  { to: "/", label: "Dashboard" },
  { to: "/agents", label: "Agents" },
  { to: "/providers", label: "Providers" },
  { to: "/settings", label: "Settings" },
];

function NavLinks() {
  const location = useLocation();

  return (
    <nav className="flex items-center gap-4">
      {navItems.map((item) => {
        const isActive =
          item.to === "/"
            ? location.pathname === "/"
            : location.pathname.startsWith(item.to);

        return (
          <Link
            key={item.to}
            to={item.to}
            className={`text-sm font-medium transition-colors ${
              isActive
                ? "text-blue-600"
                : "text-gray-500 hover:text-gray-700"
            }`}
          >
            {item.label}
          </Link>
        );
      })}
    </nav>
  );
}
