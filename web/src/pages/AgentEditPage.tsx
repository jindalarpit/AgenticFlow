import { useMemo } from "react";
import { useParams, useNavigate, Link, Navigate } from "react-router-dom";
import { useAgent } from "../hooks/useAgentDetail";
import { useAgentSkills } from "../hooks/useAgentSkills";
import { AgentForm } from "../components/agent-form/AgentForm";
import type { AgentFormValues } from "../components/agent-form/types";

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

/**
 * AgentEditPage — wrapper for editing an existing agent.
 *
 * Fetches agent data, checks ownership, and passes to AgentForm in edit mode.
 * Redirects non-owners to the read-only detail page.
 */
export default function AgentEditPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const { data: agent, isLoading, error } = useAgent(id || "");
  const { data: agentSkills } = useAgentSkills(id || "");

  const currentUserId = getCurrentUserId();

  // Convert agent data to form values
  const initialValues: AgentFormValues | null = useMemo(() => {
    if (!agent) return null;
    return {
      name: agent.name,
      description: agent.description || "",
      instructions: agent.instructions || "",
      runtime_id: agent.runtime_id || "",
      model: agent.model || "",
      custom_env: agent.custom_env || {},
      custom_args: agent.custom_args || [],
      max_concurrent_tasks: agent.max_concurrent_tasks || 1,
      visibility: agent.visibility || "private",
      mcp_config: agent.mcp_config || null,
      skill_ids: agentSkills?.map((s) => s.id) ?? agent.skills?.map((s) => s.id) ?? [],
    };
  }, [agent, agentSkills]);

  // Loading state
  if (isLoading) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-6 sm:px-6 lg:px-8">
        <div className="animate-pulse space-y-4">
          <div className="h-4 w-48 bg-gray-200 rounded" />
          <div className="h-8 w-64 bg-gray-200 rounded" />
          <div className="h-64 bg-gray-100 rounded" />
        </div>
      </div>
    );
  }

  // 404 state
  if (error || !agent) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-6 sm:px-6 lg:px-8">
        <div className="text-center py-12">
          <h2 className="text-lg font-semibold text-gray-900">Agent not found</h2>
          <p className="mt-2 text-sm text-gray-600">
            The agent you're looking for doesn't exist or has been deleted.
          </p>
          <Link
            to="/agents"
            className="mt-4 inline-block text-sm text-blue-600 hover:text-blue-800"
          >
            ← Back to Agents
          </Link>
        </div>
      </div>
    );
  }

  // In single-user mode, always allow editing
  const isOwner = true;

  if (!initialValues) return null;

  return (
    <div className="mx-auto max-w-3xl px-4 py-6 sm:px-6 lg:px-8">
      {/* Breadcrumb */}
      <nav className="mb-6 flex items-center gap-2 text-sm" aria-label="Breadcrumb">
        <Link to="/agents" className="text-blue-600 hover:text-blue-800">
          Agents
        </Link>
        <span className="text-gray-400">/</span>
        <Link to={`/agents/${id}`} className="text-blue-600 hover:text-blue-800">
          {agent.name}
        </Link>
        <span className="text-gray-400">/</span>
        <span className="text-gray-700 font-medium">Edit</span>
      </nav>

      {/* Page title */}
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Edit Agent</h1>

      {/* Form */}
      <AgentForm
        mode="edit"
        initialValues={initialValues}
        agentId={id}
        onSuccess={() => {
          navigate(`/agents/${id}`);
        }}
        onCancel={() => navigate(`/agents/${id}`)}
      />
    </div>
  );
}
