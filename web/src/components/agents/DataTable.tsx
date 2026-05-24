import {
  useReactTable,
  getCoreRowModel,
  flexRender,
  createColumnHelper,
  type ColumnDef,
} from "@tanstack/react-table";
import { useMemo } from "react";

import type { AgentListItem } from "../../hooks/useAgentList";
import type { AgentPresenceDetail } from "../../lib/agent-availability";
import type { AgentActivity } from "../../hooks/useAgentActivity";
import { AgentNameCell } from "./AgentNameCell";
import { AvailabilityCell } from "./AvailabilityCell";
import { WorkloadCell } from "./WorkloadCell";
import { RuntimeCell } from "./RuntimeCell";
import { ActivityCell } from "./ActivityCell";
import { RunsCell } from "./RunsCell";
import { AgentRowActions } from "./AgentRowActions";

/* ─── Types ─── */

export interface AgentRow {
  agent: AgentListItem;
  runtime: { name: string; provider: string; mode: "local" | "cloud" } | null;
  presence: AgentPresenceDetail | null;
  activity: AgentActivity | null;
  runCount: number;
  ownerIdToShow: string | null;
  isOwnedByMe: boolean;
  canManage: boolean;
}

interface DataTableProps {
  data: AgentRow[];
  onRowClick: (agentId: string) => void;
  onDuplicate: (agent: AgentListItem) => void;
  onArchive: (agentId: string) => void;
}

/* ─── Column Widths ─── */

const COL_WIDTHS = {
  agent: 240,
  status: 120,
  workload: 140,
  runtime: 200,
  activity: 100,
  runs: 64,
  actions: 60,
} as const;

/* ─── Column Definitions ─── */

const columnHelper = createColumnHelper<AgentRow>();

function buildColumns(
  onDuplicate: (agent: AgentListItem) => void,
  onArchive: (agentId: string) => void
): ColumnDef<AgentRow, unknown>[] {
  return [
    columnHelper.accessor("agent", {
      id: "agent",
      header: () => "Agent",
      size: COL_WIDTHS.agent,
      cell: ({ row }) => (
        <AgentNameCell
          agent={row.original.agent}
          showOwner={row.original.ownerIdToShow !== null}
        />
      ),
    }),
    columnHelper.accessor("presence", {
      id: "status",
      header: () => "Status",
      size: COL_WIDTHS.status,
      cell: ({ row }) => {
        const availability = row.original.presence?.availability ?? "offline";
        return <AvailabilityCell availability={availability} />;
      },
    }),
    columnHelper.accessor("presence", {
      id: "workload",
      header: () => "Workload",
      size: COL_WIDTHS.workload,
      cell: ({ row }) => {
        const presence = row.original.presence;
        return (
          <WorkloadCell
            workload={presence?.workload ?? "idle"}
            runningCount={presence?.runningCount ?? 0}
            queuedCount={presence?.queuedCount ?? 0}
            capacity={presence?.capacity ?? 1}
          />
        );
      },
    }),
    columnHelper.accessor("runtime", {
      id: "runtime",
      header: () => "Runtime",
      size: COL_WIDTHS.runtime,
      cell: ({ row }) => {
        const rt = row.original.runtime;
        return (
          <RuntimeCell
            runtimeName={rt?.name ?? null}
            provider={rt?.provider ?? null}
            mode={rt?.mode ?? "local"}
          />
        );
      },
    }),
    columnHelper.accessor("activity", {
      id: "activity",
      header: () => "Activity",
      size: COL_WIDTHS.activity,
      cell: ({ row }) => (
        <ActivityCell buckets={row.original.activity?.buckets ?? null} />
      ),
    }),
    columnHelper.accessor("runCount", {
      id: "runs",
      header: () => "Runs",
      size: COL_WIDTHS.runs,
      cell: ({ row }) => <RunsCell runCount={row.original.runCount} />,
    }),
    columnHelper.display({
      id: "actions",
      header: () => "",
      size: COL_WIDTHS.actions,
      cell: ({ row }) => (
        <AgentRowActions
          agent={row.original.agent}
          canManage={row.original.canManage}
          onDuplicate={onDuplicate}
          onArchive={onArchive}
        />
      ),
    }),
  ] as ColumnDef<AgentRow, unknown>[];
}

/* ─── Component ─── */

/**
 * Agent data table using @tanstack/react-table.
 * Configures getCoreRowModel with fixed column widths.
 * Columns: Agent, Status, Workload, Runtime, Activity, Runs, Actions (pinned right).
 * Row click navigates to agent detail (excluding actions column clicks).
 *
 * Requirements: 6.1–6.9
 */
export function DataTable({
  data,
  onRowClick,
  onDuplicate,
  onArchive,
}: DataTableProps) {
  const columns = useMemo(
    () => buildColumns(onDuplicate, onArchive),
    [onDuplicate, onArchive]
  );

  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  return (
    <div className="w-full overflow-x-auto">
      <table className="w-full table-fixed border-collapse">
        {/* Column widths */}
        <colgroup>
          <col style={{ width: COL_WIDTHS.agent, minWidth: COL_WIDTHS.agent }} />
          <col style={{ width: COL_WIDTHS.status }} />
          <col style={{ width: COL_WIDTHS.workload }} />
          <col style={{ width: COL_WIDTHS.runtime, minWidth: COL_WIDTHS.runtime }} />
          <col style={{ width: COL_WIDTHS.activity }} />
          <col style={{ width: COL_WIDTHS.runs }} />
          <col style={{ width: COL_WIDTHS.actions }} />
        </colgroup>

        {/* Header */}
        <thead>
          {table.getHeaderGroups().map((headerGroup) => (
            <tr
              key={headerGroup.id}
              className="border-b border-gray-200"
            >
              {headerGroup.headers.map((header) => (
                <th
                  key={header.id}
                  className={`px-3 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500 ${
                    header.id === "actions" ? "text-right" : ""
                  } ${header.id === "runs" ? "text-right" : ""}`}
                >
                  {header.isPlaceholder
                    ? null
                    : flexRender(
                        header.column.columnDef.header,
                        header.getContext()
                      )}
                </th>
              ))}
            </tr>
          ))}
        </thead>

        {/* Body */}
        <tbody>
          {table.getRowModel().rows.map((row) => (
            <tr
              key={row.id}
              onClick={(e) => {
                // Don't navigate if clicking within the actions column
                const target = e.target as HTMLElement;
                if (target.closest('[data-column="actions"]')) return;
                onRowClick(row.original.agent.id);
              }}
              className="border-b border-gray-100 hover:bg-gray-50 cursor-pointer transition-colors"
              role="row"
              aria-label={`Agent: ${row.original.agent.name}`}
            >
              {row.getVisibleCells().map((cell) => (
                <td
                  key={cell.id}
                  data-column={cell.column.id}
                  className={`px-3 py-3 ${
                    cell.column.id === "actions" ? "text-right" : ""
                  } ${cell.column.id === "runs" ? "text-right" : ""}`}
                >
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
