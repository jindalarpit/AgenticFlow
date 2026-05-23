import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api";

export interface AgentRuntime {
  id: string;
  daemon_id: string;
  provider: string;
  name: string;
  version: string;
  binary_path: string;
  status: "available" | "busy" | "unavailable";
  created_at: string;
  updated_at: string;
}

export interface Daemon {
  id: string;
  daemon_id: string;
  device_name: string;
  status: "online" | "offline";
  last_heartbeat_at: string | null;
  cli_version: string | null;
  agent_runtimes: AgentRuntime[];
  created_at: string;
  updated_at: string;
}

/**
 * Fetch connected daemons for the current user.
 * GET /api/daemons
 */
export function useDaemons() {
  return useQuery<Daemon[]>({
    queryKey: ["daemons"],
    queryFn: () => apiFetch<Daemon[]>("/api/daemons"),
  });
}
