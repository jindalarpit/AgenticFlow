import { useMemo } from "react";

import type { ManagedAgent } from "./useManagedAgents";
import type { Daemon } from "./useDaemons";
import {
  derivePresenceMap,
  type AgentPresenceDetail,
} from "../lib/agent-availability";

/**
 * Derives agent presence (availability + workload) for all provided agents
 * based on daemon heartbeats and runtime state.
 *
 * Uses `useMemo` to recompute only when agents or daemons change.
 *
 * @param agents - Array of managed agents (from useAgentList or useManagedAgents)
 * @param daemons - Array of connected daemons (from useDaemons)
 * @returns Map from agent ID to AgentPresenceDetail
 */
export function useAgentPresence(
  agents: ManagedAgent[],
  daemons: Daemon[]
): Map<string, AgentPresenceDetail> {
  return useMemo(
    () => derivePresenceMap(agents, daemons),
    [agents, daemons]
  );
}
