import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api";

export interface AgentRuntime {
  id: string;
  name: string;
  provider: string;
  status: string;
}

export interface Daemon {
  daemon_id: string;
  device_name: string;
  status: "online" | "offline";
  agent_runtimes: AgentRuntime[];
}

/**
 * Fetch all connected daemons with their agent runtimes.
 * GET /api/daemons
 */
export function useDaemons() {
  return useQuery<Daemon[]>({
    queryKey: ["daemons"],
    queryFn: () => apiFetch<Daemon[]>("/api/daemons"),
  });
}
