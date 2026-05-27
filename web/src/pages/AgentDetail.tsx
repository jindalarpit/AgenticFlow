import { useEffect, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { useAgent, useUpdateAgent, useDeleteAgent } from "../hooks/useAgentDetail";
import { useAgentWebSocket } from "../hooks/useAgentWebSocket";
import { PageHeader } from "../components/agent-detail/PageHeader";
import { SidebarInspector } from "../components/agent-detail/SidebarInspector";
import { OverviewPane } from "../components/agent-detail/OverviewPane";
import type { Agent } from "../lib/agent-detail-types";

/* ─── Helper: get current user ID from JWT token ─── */

function getCurrentUserId(): string | null {
  try {
    const token = localStorage.getItem("af_token");
    if (!token) return null;
    const parts = token.split(".");
    const part = parts[1];
    if (!part) return null;
    const payload = JSON.parse(atob(part));
    return payload.sub || payload.user_id || null;
  } catch {
    return null;
  }
}

/* ─── Skeleton Placeholder ─── */

function AgentDetailSkeleton() {
  return (
    <div className="flex flex-col h-full" aria-busy="true" aria-label="Loading agent details">
      {/* Header skeleton */}
      <div className="flex items-center gap-3 px-4 py-3 border-b border-gray-200">
        <div className="h-4 w-20 bg-gray-200 rounded animate-pulse" />
        <div className="h-4 w-2 bg-gray-200 rounded animate-pulse" />
        <div className="h-4 w-32 bg-gray-200 rounded animate-pulse" />
        <div className="ml-auto h-6 w-16 bg-gray-200 rounded-full animate-pulse" />
      </div>

      {/* Content skeleton — responsive */}
      <div className="flex-1 flex flex-col md:grid md:grid-cols-[320px_minmax(0,1fr)] md:gap-4 md:overflow-hidden p-4">
        {/* Sidebar skeleton */}
        <div className="w-full md:w-[320px] space-y-4">
          <div className="flex items-center gap-3">
            <div className="h-14 w-14 bg-gray-200 rounded-lg animate-pulse" />
            <div className="space-y-2 flex-1">
              <div className="h-4 w-24 bg-gray-200 rounded animate-pulse" />
              <div className="h-3 w-32 bg-gray-200 rounded animate-pulse" />
            </div>
          </div>
          <div className="space-y-3 pt-4 border-t border-gray-100">
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className="flex justify-between">
                <div className="h-3 w-16 bg-gray-200 rounded animate-pulse" />
                <div className="h-3 w-20 bg-gray-200 rounded animate-pulse" />
              </div>
            ))}
          </div>
        </div>

        {/* Overview pane skeleton */}
        <div className="flex-1 mt-4 md:mt-0 min-h-[60vh]">
          <div className="flex gap-4 border-b border-gray-200 pb-2">
            {[1, 2, 3, 4, 5, 6].map((i) => (
              <div key={i} className="h-4 w-16 bg-gray-200 rounded animate-pulse" />
            ))}
          </div>
          <div className="mt-4 space-y-3">
            {[1, 2, 3].map((i) => (
              <div key={i} className="h-12 bg-gray-200 rounded animate-pulse" />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

/* ─── Not Found State ─── */

function AgentNotFound() {
  return (
    <div className="flex flex-col items-center justify-center h-full min-h-[50vh] px-4">
      <h2 className="text-lg font-semibold text-gray-900">Agent not found</h2>
      <p className="mt-2 text-sm text-gray-600">
        The agent you're looking for doesn't exist or has been deleted.
      </p>
      <Link
        to="/agents"
        className="mt-4 text-sm text-blue-600 hover:text-blue-800 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 rounded"
      >
        ← Back to Agents
      </Link>
    </div>
  );
}

/* ─── Error State ─── */

function AgentError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center h-full min-h-[50vh] px-4">
      <h2 className="text-lg font-semibold text-gray-900">Something went wrong</h2>
      <p className="mt-2 text-sm text-red-600">{message}</p>
      <button
        type="button"
        onClick={onRetry}
        className="mt-4 inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
        aria-label="Retry loading agent"
      >
        Retry
      </button>
    </div>
  );
}

/* ─── Main Page Component ─── */

export default function AgentDetail() {
  const { id } = useParams<{ id: string }>();
  const { data: agent, isLoading, error, refetch } = useAgent(id || "");
  const updateAgent = useUpdateAgent(id || "");
  const deleteAgent = useDeleteAgent();

  // Subscribe to WebSocket events for real-time updates
  useAgentWebSocket(id || "");

  // Skeleton timeout: show error after 10s if still loading
  const [timedOut, setTimedOut] = useState(false);
  useEffect(() => {
    if (!isLoading) {
      setTimedOut(false);
      return;
    }
    const timer = setTimeout(() => setTimedOut(true), 10000);
    return () => clearTimeout(timer);
  }, [isLoading]);

  // Determine ownership — in single-user mode, always treat as owner
  const isOwner = true;

  // Handlers
  const handleUpdate = async (data: Partial<Agent>): Promise<void> => {
    await updateAgent.mutateAsync(data);
  };

  const handleDelete = async (): Promise<void> => {
    if (!id) return;
    await deleteAgent.mutateAsync(id);
  };

  const handleRetry = () => {
    setTimedOut(false);
    void refetch();
  };

  /* ─── Loading state ─── */
  if (isLoading && !timedOut) {
    return <AgentDetailSkeleton />;
  }

  /* ─── Timeout state (loading took too long) ─── */
  if (isLoading && timedOut) {
    return (
      <AgentError
        message="Loading is taking longer than expected. The server may be unavailable."
        onRetry={handleRetry}
      />
    );
  }

  /* ─── 404 state ─── */
  if (error && "status" in (error as any) && (error as any).status === 404) {
    return <AgentNotFound />;
  }

  if (!agent && !isLoading) {
    return <AgentNotFound />;
  }

  /* ─── Error state (non-404) ─── */
  if (error) {
    return (
      <AgentError
        message={error instanceof Error ? error.message : "Failed to load agent"}
        onRetry={handleRetry}
      />
    );
  }

  /* ─── Render page ─── */
  if (!agent) return null;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Page Header */}
      <PageHeader agent={agent} isOwner={isOwner} onDelete={handleDelete} />

      {/* Two-column layout: responsive grid on desktop, stacked on mobile */}
      <div className="flex-1 flex flex-col gap-3 overflow-y-auto md:grid md:grid-cols-[320px_minmax(0,1fr)] md:gap-0 md:overflow-hidden">
        {/* Sidebar Inspector */}
        <SidebarInspector agent={agent} isOwner={isOwner} onUpdate={handleUpdate} />

        {/* Overview Pane (tabbed content) */}
        <OverviewPane agent={agent} isOwner={isOwner} onSave={handleUpdate} />
      </div>
    </div>
  );
}
