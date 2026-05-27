import { useMemo } from "react";
import { useNavigate, Link } from "react-router-dom";
import { useDaemons } from "../hooks/useDaemons";
import { AgentForm } from "../components/agent-form/AgentForm";
import { defaultFormValues } from "../components/agent-form/types";
import type { AgentFormValues } from "../components/agent-form/types";

/**
 * AgentCreatePage — wrapper for creating a new agent.
 *
 * Renders breadcrumb and passes default values to AgentForm in create mode.
 * Pre-selects the first available runtime if one exists.
 */
export default function AgentCreatePage() {
  const navigate = useNavigate();
  const { data: daemons } = useDaemons();

  // Pre-select first available runtime
  const initialValues: AgentFormValues = useMemo(() => {
    const defaults = defaultFormValues();

    // Find first runtime from daemons
    if (daemons) {
      for (const daemon of daemons) {
        if (daemon.agent_runtimes && daemon.agent_runtimes.length > 0) {
          defaults.runtime_id = daemon.agent_runtimes[0]!.id;
          break;
        }
      }
    }

    return defaults;
  }, [daemons]);

  return (
    <div className="mx-auto max-w-3xl px-4 py-6 sm:px-6 lg:px-8">
      {/* Breadcrumb */}
      <nav className="mb-6 flex items-center gap-2 text-sm" aria-label="Breadcrumb">
        <Link to="/agents" className="text-blue-600 hover:text-blue-800">
          Agents
        </Link>
        <span className="text-gray-400">/</span>
        <span className="text-gray-700 font-medium">New Agent</span>
      </nav>

      {/* Page title */}
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Create Agent</h1>

      {/* Form */}
      <AgentForm
        mode="create"
        initialValues={initialValues}
        onSuccess={(agent: { id: string }) => {
          navigate(`/agents/${agent.id}`);
        }}
        onCancel={() => navigate("/agents")}
      />
    </div>
  );
}
