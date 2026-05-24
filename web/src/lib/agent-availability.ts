/**
 * Pure functions for deriving agent availability and workload from
 * daemon/runtime state and agent task status.
 *
 * These functions are used by the useAgentPresence hook and the
 * agent list filtering pipeline to compute real-time presence data.
 */

import type { Daemon, AgentRuntime } from "../hooks/useDaemons";
import type { ManagedAgent } from "../hooks/useManagedAgents";

/* ─── Types ─── */

/** Three-state availability derived from daemon + runtime status. */
export type Availability = "online" | "unstable" | "offline";

/** Current task execution state of an agent. */
export type Workload = "idle" | "working" | "queued";

/** Full presence detail for a single agent. */
export interface AgentPresenceDetail {
  availability: Availability;
  workload: Workload;
  runningCount: number;
  queuedCount: number;
  capacity: number;
}

/* ─── Helpers ─── */

/**
 * Finds the daemon that contains the given runtime_id in its agent_runtimes array.
 * Returns the daemon and the matching runtime, or null if not found.
 */
function findDaemonForRuntime(
  runtimeId: string,
  daemons: Daemon[]
): { daemon: Daemon; runtime: AgentRuntime } | null {
  for (const daemon of daemons) {
    const runtime = daemon.agent_runtimes?.find((r) => r.id === runtimeId);
    if (runtime) {
      return { daemon, runtime };
    }
  }
  return null;
}

/**
 * Derives workload state from the agent's status field.
 * - "idle" → idle
 * - "working" → working
 * - "offline" → idle (offline agents have no active work)
 */
function deriveWorkload(agentStatus: string): Workload {
  switch (agentStatus) {
    case "working":
      return "working";
    case "idle":
    case "offline":
    default:
      return "idle";
  }
}

/* ─── Public API ─── */

/**
 * Derives the presence detail for a single agent based on daemon/runtime state.
 *
 * Logic:
 * 1. Find the daemon that contains the agent's runtime_id in its runtimes array
 * 2. If no daemon found OR daemon.status === "offline" → availability = "offline"
 * 3. If daemon online + runtime.status === "available" → availability = "online"
 * 4. If daemon online + runtime.status === "busy" → availability = "unstable"
 * 5. If daemon online + runtime.status === "offline"/"unavailable" → availability = "offline"
 * 6. Workload is derived from agent.status
 */
export function derivePresence(
  agent: ManagedAgent,
  daemons: Daemon[]
): AgentPresenceDetail {
  const capacity = agent.max_concurrent_tasks;
  const workload = deriveWorkload(agent.status);

  // Estimate running/queued counts from status
  // Without detailed task data, we infer from agent status
  const runningCount = workload === "working" ? 1 : 0;
  const queuedCount = 0;

  // Find the daemon hosting this agent's runtime
  const match = findDaemonForRuntime(agent.runtime_id, daemons);

  // No daemon found → offline
  if (!match) {
    return {
      availability: "offline",
      workload,
      runningCount,
      queuedCount,
      capacity,
    };
  }

  const { daemon, runtime } = match;

  // Daemon offline → offline
  if (daemon.status === "offline") {
    return {
      availability: "offline",
      workload,
      runningCount,
      queuedCount,
      capacity,
    };
  }

  // Daemon online — derive availability from runtime status
  let availability: Availability;
  switch (runtime.status) {
    case "available":
      availability = "online";
      break;
    case "busy":
      availability = "unstable";
      break;
    case "unavailable":
    default:
      availability = "offline";
      break;
  }

  return {
    availability,
    workload,
    runningCount,
    queuedCount,
    capacity,
  };
}

/**
 * Derives presence for all agents, returning a Map keyed by agent ID.
 *
 * This is the batch version of derivePresence, used by the agent list page
 * to compute presence for all visible agents in a single pass.
 */
export function derivePresenceMap(
  agents: ManagedAgent[],
  daemons: Daemon[]
): Map<string, AgentPresenceDetail> {
  const map = new Map<string, AgentPresenceDetail>();
  for (const agent of agents) {
    map.set(agent.id, derivePresence(agent, daemons));
  }
  return map;
}
