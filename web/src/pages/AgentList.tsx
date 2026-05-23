import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { useManagedAgents } from "../hooks/useManagedAgents";
import type { ManagedAgent, AgentStatus } from "../hooks/useManagedAgents";
import { wsClient } from "../lib/ws";

export default function AgentList() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data: agents, isLoading } = useManagedAgents();

  // Real-time updates: listen for agent WebSocket events and invalidate cache
  useEffect(() => {
    const unsubStatusChanged = wsClient.on("agent_status_changed", () => {
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    });
    const unsubCreated = wsClient.on("agent_created", () => {
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    });
    const unsubUpdated = wsClient.on("agent_updated", () => {
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    });
    const unsubDeleted = wsClient.on("agent_deleted", () => {
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    });

    return () => {
      unsubStatusChanged();
      unsubCreated();
      unsubUpdated();
      unsubDeleted();
    };
  }, [queryClient]);

  return (
    <div className="max-w-7xl mx-auto px-6 py-8">
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-xl font-semibold text-gray-900">Agents</h2>
        <button
          onClick={() => navigate("/agents/new")}
          className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
        >
          + Create Agent
        </button>
      </div>

      {isLoading ? (
        <p className="text-sm text-gray-500">Loading agents...</p>
      ) : !agents || agents.length === 0 ? (
        <EmptyState onCreateClick={() => navigate("/agents/new")} />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {agents.map((agent) => (
            <AgentCard
              key={agent.id}
              agent={agent}
              onClick={() => navigate(`/agents/${agent.id}`)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

/* ─── Agent Card ─── */

function AgentCard({
  agent,
  onClick,
}: {
  agent: ManagedAgent;
  onClick: () => void;
}) {
  const descriptionPreview = agent.description
    ? agent.description.length > 80
      ? agent.description.slice(0, 80) + "…"
      : agent.description
    : null;

  return (
    <div
      onClick={onClick}
      className="bg-white rounded-lg border border-gray-200 p-4 shadow-sm hover:shadow-md hover:border-gray-300 transition-all cursor-pointer"
    >
      <div className="flex items-start justify-between mb-2">
        <h3 className="font-medium text-gray-900 truncate">{agent.name}</h3>
        <AgentStatusBadge status={agent.status} />
      </div>

      {descriptionPreview && (
        <p className="text-sm text-gray-500 mb-3">{descriptionPreview}</p>
      )}

      <div className="flex flex-wrap items-center gap-2 text-xs text-gray-400">
        {agent.runtime_name && (
          <span className="inline-flex items-center px-2 py-0.5 rounded bg-blue-50 text-blue-700 font-medium">
            {agent.runtime_name}
          </span>
        )}
        {agent.model && (
          <span className="inline-flex items-center px-2 py-0.5 rounded bg-gray-100 text-gray-600">
            {agent.model}
          </span>
        )}
      </div>
    </div>
  );
}

/* ─── Status Badge ─── */

function AgentStatusBadge({ status }: { status: AgentStatus }) {
  const config: Record<AgentStatus, { color: string; label: string }> = {
    idle: { color: "bg-green-100 text-green-700", label: "Idle" },
    working: { color: "bg-amber-100 text-amber-700", label: "Working" },
    offline: { color: "bg-gray-100 text-gray-600", label: "Offline" },
  };

  const { color, label } = config[status];

  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${color}`}
    >
      {label}
    </span>
  );
}

/* ─── Empty State ─── */

function EmptyState({ onCreateClick }: { onCreateClick: () => void }) {
  return (
    <div className="text-center py-16">
      <div className="text-4xl mb-4">🤖</div>
      <h3 className="text-lg font-medium text-gray-900 mb-2">No agents yet</h3>
      <p className="text-sm text-gray-500 mb-6">
        Create your first agent to start delegating tasks.
      </p>
      <button
        onClick={onCreateClick}
        className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
      >
        + Create Agent
      </button>
    </div>
  );
}
