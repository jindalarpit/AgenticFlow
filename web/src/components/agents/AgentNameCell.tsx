import type { AgentListItem } from "../../hooks/useAgentList";

/* ─── Props ─── */

interface AgentNameCellProps {
  agent: AgentListItem;
  showOwner: boolean;
  ownerAvatarUrl?: string | null;
  ownerName?: string;
}

/**
 * Table cell for the Agent column.
 * Renders avatar, name, truncated description, lock icon for private visibility,
 * and owner avatar when viewing in "All" scope for agents owned by others.
 *
 * Requirements: 6.2
 */
export function AgentNameCell({
  agent,
  showOwner,
  ownerAvatarUrl,
  ownerName,
}: AgentNameCellProps) {
  const initials = agent.name
    .split(/[\s_-]+/)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("");

  return (
    <div className="flex items-center gap-3 min-w-0">
      {/* Agent Avatar */}
      {agent.avatar_url ? (
        <img
          src={agent.avatar_url}
          alt={`${agent.name} avatar`}
          className="h-8 w-8 shrink-0 rounded-md object-cover"
        />
      ) : (
        <div
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-blue-100 text-blue-700 text-xs font-semibold"
          aria-label={`${agent.name} avatar`}
        >
          {initials || "A"}
        </div>
      )}

      {/* Name + Description */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <span className="truncate text-sm font-medium text-gray-900">
            {agent.name}
          </span>
          {agent.visibility === "private" && (
            <LockIcon />
          )}
        </div>
        {agent.description && (
          <p className="truncate text-xs text-gray-500 mt-0.5">
            {agent.description}
          </p>
        )}
      </div>

      {/* Owner Avatar (shown in "All" scope for others' agents) */}
      {showOwner && (
        <div className="shrink-0" title={ownerName ?? "Owner"}>
          {ownerAvatarUrl ? (
            <img
              src={ownerAvatarUrl}
              alt={ownerName ?? "Owner"}
              className="h-5 w-5 rounded-full object-cover ring-1 ring-gray-200"
            />
          ) : (
            <div className="flex h-5 w-5 items-center justify-center rounded-full bg-gray-200 text-[10px] font-medium text-gray-600">
              {ownerName?.[0]?.toUpperCase() ?? "?"}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

/* ─── Icons ─── */

function LockIcon() {
  return (
    <svg
      className="h-3.5 w-3.5 shrink-0 text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
      aria-label="Private agent"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z"
      />
    </svg>
  );
}
