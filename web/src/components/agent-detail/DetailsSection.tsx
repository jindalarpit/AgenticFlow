import type { Agent } from "../../lib/agent-detail-types";
import { formatRelativeTime } from "../../lib/agent-utils";

interface DetailsSectionProps {
  agent: Agent;
}

/**
 * Read-only details section in the sidebar inspector.
 * Displays Owner, Created (relative time), and Updated (relative time).
 */
export function DetailsSection({ agent }: DetailsSectionProps) {
  return (
    <div className="border-b px-5 py-4">
      <div className="mb-1 -mx-2 px-2 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
        Details
      </div>
      <div className="grid grid-cols-[auto_1fr] gap-x-2 gap-y-0.5">
        {/* Owner row */}
        <span className="py-1 text-xs text-muted-foreground">Owner</span>
        <span className="truncate py-1 text-xs">
          {agent.owner_name ?? "Unknown"}
        </span>

        {/* Created row */}
        <span className="py-1 text-xs text-muted-foreground">Created</span>
        <span className="py-1 text-xs text-muted-foreground">
          {formatRelativeTime(agent.created_at)}
        </span>

        {/* Updated row */}
        <span className="py-1 text-xs text-muted-foreground">Updated</span>
        <span className="py-1 text-xs text-muted-foreground">
          {formatRelativeTime(agent.updated_at)}
        </span>
      </div>
    </div>
  );
}
