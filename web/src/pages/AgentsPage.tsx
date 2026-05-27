import { useState, useMemo, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";

import { useAgentList, type AgentListItem } from "../hooks/useAgentList";
import { useAgentPresence } from "../hooks/useAgentPresence";
import { useAgentActivity } from "../hooks/useAgentActivity";
import { useAgentRunCounts } from "../hooks/useAgentRunCounts";
import { useDaemons } from "../hooks/useDaemons";
import { useAgentListWebSocket } from "../hooks/useAgentListWebSocket";
import { useToast } from "../components/Toast";
import { apiFetch } from "../lib/api";

import {
  filterBySearch,
  filterByScope,
  filterByAvailability,
  filterByVisibility,
  type AvailabilityFilter,
} from "../lib/agent-filters";
import { sortAgents, type SortKey } from "../lib/agent-sorting";
import { computeCounts } from "../lib/agent-counts";
import { canArchive } from "../lib/agent-permissions";

import { PageHeaderBar } from "../components/agents/PageHeaderBar";
import { ActiveToolbarRow } from "../components/agents/ActiveToolbarRow";
import { ArchivedToolbarRow } from "../components/agents/ArchivedToolbarRow";
import { AvailabilityFilterRow } from "../components/agents/AvailabilityFilterRow";
import { DataTable, type AgentRow } from "../components/agents/DataTable";
import { CreateAgentDialog } from "../components/agents/CreateAgentDialog";
import { LoadingSkeleton } from "../components/agents/LoadingSkeleton";
import { EmptyState } from "../components/agents/EmptyState";
import { NoMatchesState } from "../components/agents/NoMatchesState";
import { ErrorState } from "../components/agents/ErrorState";
import type { CreateAgentPayload } from "../lib/agent-duplicate";

/* ─── Session Storage Key ─── */

const SCOPE_STORAGE_KEY = "af_agents_scope";

function getPersistedScope(): "mine" | "all" {
  const stored = sessionStorage.getItem(SCOPE_STORAGE_KEY);
  if (stored === "mine" || stored === "all") return stored;
  return "all";
}

function persistScope(scope: "mine" | "all") {
  sessionStorage.setItem(SCOPE_STORAGE_KEY, scope);
}

/* ─── Placeholder User Context ─── */

// Fetches the current user from /api/me to get the real user ID for scope filtering.
function useCurrentUser() {
  const { data } = useQuery<{ id: string; name: string; email: string }>({
    queryKey: ["me"],
    queryFn: () => apiFetch<{ id: string; name: string; email: string }>("/api/me"),
    staleTime: Infinity,
  });

  return {
    id: data?.id ?? "",
    // AgenticFlow uses simple ownership model — single user owns everything.
    // Treat as admin so visibility filtering never hides agents.
    isAdmin: true,
  };
}

/* ─── Page Component ─── */

/**
 * Root page component for the Agent Management UI at `/agents`.
 *
 * Manages page-level state: view, scope (sessionStorage), availabilityFilter, sort, search,
 * showCreate, duplicateTemplate.
 *
 * Wires all hooks: useAgentList, useAgentPresence, useAgentActivity, useAgentRunCounts,
 * useDaemons, useAgentListWebSocket.
 *
 * Implements filtering pipeline via useMemo chain:
 *   inView → visibleInView → inScope → filteredAgents → sortedAgents → agentRows
 *
 * Renders conditionally: LoadingSkeleton | ErrorState | EmptyState | (toolbar + filter + table/NoMatchesState)
 *
 * Requirements: 1.1–1.3, 2.1–2.3, 3.1–3.5, 4.1–4.6, 5.1–5.6, 6.1–6.9, 7.1–7.5,
 *              8.1–8.5, 12.1–12.2, 13.1–13.2, 14.1–14.2, 15.1–15.5, 16.1–16.3,
 *              17.1–17.3, 18.1–18.2
 */
export default function AgentsPage() {
  const navigate = useNavigate();
  const { showToast } = useToast();
  const currentUser = useCurrentUser();

  // ─── Page-Level State ───
  const [view, setView] = useState<"active" | "archived">("active");
  const [scope, setScopeRaw] = useState<"mine" | "all">(getPersistedScope);
  const [availabilityFilter, setAvailabilityFilter] =
    useState<AvailabilityFilter>("all");
  const [sort, setSort] = useState<SortKey>("recent");
  const [search, setSearch] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [duplicateTemplate, setDuplicateTemplate] =
    useState<AgentListItem | null>(null);

  // Persist scope to sessionStorage
  const setScope = useCallback((v: "mine" | "all") => {
    setScopeRaw(v);
    persistScope(v);
  }, []);

  // ─── Data Hooks ───
  const {
    data: agents,
    isLoading,
    isError,
    error,
    refetch,
  } = useAgentList();
  const { data: daemons = [] } = useDaemons();
  const { data: activityMap } = useAgentActivity();
  const { data: runCountMap } = useAgentRunCounts();

  // Derive presence from agents + daemons
  const presenceMap = useAgentPresence(agents ?? [], daemons);

  // Subscribe to WebSocket events for real-time updates
  useAgentListWebSocket();

  // ─── Filtering Pipeline (useMemo chain) ───

  // Step 1: Split by view (active vs archived)
  const inView = useMemo(() => {
    if (!agents) return [];
    if (view === "archived") {
      return agents.filter((a) => a.archived_at !== null);
    }
    return agents.filter((a) => a.archived_at === null);
  }, [agents, view]);

  // Step 2: Visibility filtering — hide others' private agents (unless admin)
  const visibleInView = useMemo(
    () => filterByVisibility(inView, currentUser.id, currentUser.isAdmin),
    [inView, currentUser.id, currentUser.isAdmin]
  );

  // Step 3: Scope filtering (Mine/All)
  const inScope = useMemo(
    () => filterByScope(visibleInView, scope, currentUser.id),
    [visibleInView, scope, currentUser.id]
  );

  // Step 4: Search + availability filtering
  const filteredAgents = useMemo(() => {
    const afterSearch = filterBySearch(inScope, search);
    return filterByAvailability(afterSearch, presenceMap, availabilityFilter);
  }, [inScope, search, presenceMap, availabilityFilter]);

  // Step 5: Sort
  const sortedAgents = useMemo(
    () =>
      sortAgents(
        filteredAgents,
        sort,
        activityMap ?? new Map(),
        runCountMap ?? new Map()
      ),
    [filteredAgents, sort, activityMap, runCountMap]
  );

  // Step 6: Assemble AgentRow objects for the table
  const agentRows: AgentRow[] = useMemo(() => {
    return sortedAgents.map((agent) => {
      // Find the runtime for this agent
      const runtime = findRuntime(agent.runtime_id, daemons);
      const presence = presenceMap.get(agent.id) ?? null;
      const activity = activityMap?.get(agent.id) ?? null;
      const runCount = runCountMap?.get(agent.id) ?? 0;
      const isOwnedByMe = agent.owner_id === currentUser.id;
      const ownerIdToShow =
        scope === "all" && !isOwnedByMe ? agent.owner_id : null;

      return {
        agent,
        runtime,
        presence,
        activity,
        runCount,
        ownerIdToShow,
        isOwnedByMe,
        canManage: canArchive(agent.owner_id, currentUser.id, currentUser.isAdmin),
      };
    });
  }, [
    sortedAgents,
    daemons,
    presenceMap,
    activityMap,
    runCountMap,
    currentUser.id,
    currentUser.isAdmin,
    scope,
  ]);

  // ─── Counts ───
  const counts = useMemo(
    () =>
      computeCounts(
        visibleInView,
        presenceMap,
        scope,
        currentUser.id,
        search,
        availabilityFilter
      ),
    [visibleInView, presenceMap, scope, currentUser.id, search, availabilityFilter]
  );

  // Archived count (for the toolbar link)
  const archivedCount = useMemo(() => {
    if (!agents) return 0;
    const archived = agents.filter((a) => a.archived_at !== null);
    const visibleArchived = filterByVisibility(
      archived,
      currentUser.id,
      currentUser.isAdmin
    );
    return visibleArchived.length;
  }, [agents, currentUser.id, currentUser.isAdmin]);

  // Total active count for the page header
  const totalActiveCount = useMemo(() => {
    if (!agents) return 0;
    const active = agents.filter((a) => a.archived_at === null);
    const visible = filterByVisibility(active, currentUser.id, currentUser.isAdmin);
    return visible.length;
  }, [agents, currentUser.id, currentUser.isAdmin]);

  // ─── Handlers ───

  const handleRowClick = useCallback(
    (agentId: string) => {
      navigate(`/agents/${agentId}`);
    },
    [navigate]
  );

  const handleArchive = useCallback(
    async (agentId: string) => {
      try {
        await apiFetch(`/api/agents/${agentId}/archive`, { method: "POST" });
        showToast("Agent archived successfully", "success");
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Failed to archive agent";
        showToast(message, "error");
      }
    },
    [showToast]
  );

  const handleDuplicate = useCallback((agent: AgentListItem) => {
    setDuplicateTemplate(agent);
    setShowCreate(true);
  }, []);

  const handleCreate = useCallback(
    async (data: CreateAgentPayload) => {
      try {
        const created = await apiFetch<AgentListItem>("/api/agents", {
          method: "POST",
          body: JSON.stringify(data),
        });
        showToast("Agent created successfully", "success");
        setShowCreate(false);
        setDuplicateTemplate(null);
        navigate(`/agents/${created.id}`);
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Failed to create agent";
        showToast(message, "error");
        throw err; // Re-throw so the dialog knows creation failed
      }
    },
    [showToast, navigate]
  );

  const handleShowArchived = useCallback(() => {
    setView("archived");
    setSearch("");
    setAvailabilityFilter("all");
  }, []);

  const handleBackToActive = useCallback(() => {
    setView("active");
    setSearch("");
    setAvailabilityFilter("all");
  }, []);

  const handleOpenCreate = useCallback(() => {
    navigate("/agents/new");
  }, [navigate]);

  const handleCloseCreate = useCallback(() => {
    setShowCreate(false);
    setDuplicateTemplate(null);
  }, []);

  // Auto-switch back to active view when all archived agents are restored
  useMemo(() => {
    if (view === "archived" && archivedCount === 0 && agents && agents.length > 0) {
      setView("active");
    }
  }, [view, archivedCount, agents]);

  // ─── Render ───

  // Loading state
  if (isLoading) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        <LoadingSkeleton />
      </div>
    );
  }

  // Error state
  if (isError) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        <PageHeaderBar totalCount={0} onCreate={handleOpenCreate} />
        <div className="mt-6">
          <ErrorState error={error as Error} onRetry={() => refetch()} />
        </div>
      </div>
    );
  }

  // Empty state — no agents at all (active + archived)
  const hasAnyAgents = (agents?.length ?? 0) > 0;
  if (!hasAnyAgents) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        <PageHeaderBar totalCount={0} onCreate={handleOpenCreate} />
        <div className="mt-6">
          <EmptyState onCreate={handleOpenCreate} />
        </div>
        {showCreate && (
          <CreateAgentDialog
            daemons={daemons}
            daemonsLoading={false}
            currentUserId={currentUser.id}
            template={duplicateTemplate}
            onClose={handleCloseCreate}
            onCreate={handleCreate}
          />
        )}
      </div>
    );
  }

  // Main content
  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
      {/* Page Header */}
      <PageHeaderBar totalCount={totalActiveCount} onCreate={handleOpenCreate} />

      {/* Toolbar */}
      <div className="mt-5 space-y-3">
        {view === "active" ? (
          <>
            <ActiveToolbarRow
              scope={scope}
              setScope={setScope}
              scopeCounts={counts.scopeCounts}
              sort={sort}
              setSort={setSort}
              search={search}
              setSearch={setSearch}
              visibleCount={counts.visible}
              totalCount={counts.total}
              archivedCount={archivedCount}
              onShowArchived={handleShowArchived}
            />
            <AvailabilityFilterRow
              value={availabilityFilter}
              onChange={setAvailabilityFilter}
              counts={counts.availabilityCounts}
              totalCount={counts.total}
            />
          </>
        ) : (
          <ArchivedToolbarRow
            onBack={handleBackToActive}
            archivedCount={archivedCount}
            sort={sort}
            setSort={setSort}
          />
        )}
      </div>

      {/* Table or No Matches */}
      <div className="mt-4">
        {agentRows.length === 0 ? (
          <NoMatchesState view={view} search={search} scope={scope} />
        ) : (
          <DataTable
            data={agentRows}
            onRowClick={handleRowClick}
            onDuplicate={handleDuplicate}
            onArchive={handleArchive}
          />
        )}
      </div>

      {/* Create Agent Dialog */}
      {showCreate && (
        <CreateAgentDialog
          daemons={daemons}
          daemonsLoading={false}
          currentUserId={currentUser.id}
          template={duplicateTemplate}
          onClose={handleCloseCreate}
          onCreate={handleCreate}
        />
      )}
    </div>
  );
}

/* ─── Helpers ─── */

function findRuntime(
  runtimeId: string,
  daemons: { agent_runtimes: { id: string; name: string; provider: string }[] }[]
): { name: string; provider: string; mode: "local" | "cloud" } | null {
  for (const daemon of daemons) {
    const rt = daemon.agent_runtimes?.find((r) => r.id === runtimeId);
    if (rt) {
      return {
        name: rt.name,
        provider: rt.provider,
        mode: "local", // All runtimes are local in AgenticFlow (no cloud fleet)
      };
    }
  }
  return null;
}
