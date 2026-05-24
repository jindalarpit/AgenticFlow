import { useState, useEffect, useMemo } from "react";
import { isCreateDisabled } from "../../lib/agent-create-validation";
import type { AgentListItem } from "../../hooks/useAgentList";
import type { Daemon, AgentRuntime } from "../../hooks/useDaemons";
import type { CreateAgentPayload } from "../../lib/agent-duplicate";

/* ─── Props ─── */

interface CreateAgentDialogProps {
  daemons: Daemon[];
  daemonsLoading: boolean;
  currentUserId: string | null;
  template: AgentListItem | null; // non-null = duplicate mode
  onClose: () => void;
  onCreate: (data: CreateAgentPayload) => Promise<void>;
}

/**
 * Full-height modal dialog for creating or duplicating agents.
 * 85vh height, max-width 672px, scrollable body.
 *
 * Sections:
 * - Identity row: avatar picker, name input, description with counter
 * - Visibility toggle: Workspace/Private card buttons
 * - Runtime picker: grouped by daemon with status indicators
 * - Model dropdown: fetches from selected runtime
 * - Collapsible instructions textarea
 * - Footer: Cancel + Create buttons
 *
 * Requirements: 9.1–9.12, 10.1–10.3, 11.1–11.5
 */
export function CreateAgentDialog({
  daemons,
  daemonsLoading,
  currentUserId: _currentUserId,
  template,
  onClose,
  onCreate,
}: CreateAgentDialogProps) {
  const isDuplicate = template !== null;

  // Form state
  const [name, setName] = useState(isDuplicate ? `${template.name} copy` : "");
  const [description, setDescription] = useState(
    isDuplicate ? template.description : ""
  );
  const [visibility, setVisibility] = useState<"shared" | "private">(
    isDuplicate ? template.visibility : "shared"
  );
  const [runtimeId, setRuntimeId] = useState(
    isDuplicate ? template.runtime_id : ""
  );
  const [model, setModel] = useState(isDuplicate ? template.model : "");
  const [instructions, setInstructions] = useState(
    isDuplicate ? template.instructions : ""
  );
  const [showInstructions, setShowInstructions] = useState(
    isDuplicate ? !!template.instructions : false
  );
  const [avatarUrl, setAvatarUrl] = useState<string | null>(
    isDuplicate ? template.avatar_url : null
  );
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Hidden fields forwarded from template in duplicate mode
  const customEnv = isDuplicate ? template.custom_env : undefined;
  const customArgs = isDuplicate ? template.custom_args : undefined;
  const maxConcurrentTasks = isDuplicate
    ? template.max_concurrent_tasks
    : undefined;

  // Flatten all runtimes from daemons
  const allRuntimes = useMemo(() => {
    return daemons.flatMap((d) =>
      d.agent_runtimes.map((r) => ({ ...r, daemonName: d.device_name, daemonStatus: d.status }))
    );
  }, [daemons]);

  // Close on Escape
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  const disabled = isCreateDisabled(name, runtimeId);

  async function handleSubmit() {
    if (disabled || isSubmitting) return;
    setIsSubmitting(true);

    const payload: CreateAgentPayload = {
      name: name.trim(),
      description,
      runtime_id: runtimeId,
      visibility,
    };

    if (instructions) payload.instructions = instructions;
    if (model) payload.model = model;
    if (avatarUrl) payload.avatar_url = avatarUrl;
    if (customEnv && Object.keys(customEnv).length > 0)
      payload.custom_env = customEnv;
    if (customArgs && customArgs.length > 0) payload.custom_args = customArgs;
    if (maxConcurrentTasks && maxConcurrentTasks > 0)
      payload.max_concurrent_tasks = maxConcurrentTasks;

    try {
      await onCreate(payload);
    } catch {
      // Error is handled by the parent (shows toast) — dialog stays open
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center"
      role="dialog"
      aria-modal="true"
      aria-labelledby="create-agent-title"
    >
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/40"
        onClick={onClose}
        aria-hidden="true"
      />

      {/* Dialog Panel */}
      <div className="relative flex max-h-[85vh] w-full max-w-[672px] flex-col rounded-xl bg-white shadow-2xl">
        {/* Header */}
        <div className="shrink-0 border-b border-gray-200 px-6 py-4">
          <h2
            id="create-agent-title"
            className="text-lg font-semibold text-gray-900"
          >
            {isDuplicate ? "Duplicate Agent" : "Create Agent"}
          </h2>
          <p className="mt-0.5 text-sm text-gray-500">
            {isDuplicate
              ? "Create a copy of an existing agent with the same configuration."
              : "Configure a new AI agent to execute tasks on your behalf."}
          </p>
        </div>

        {/* Scrollable Body */}
        <div className="flex-1 overflow-y-auto px-6 py-5 space-y-6">
          {/* Identity Row */}
          <div className="flex gap-4">
            {/* Avatar Picker */}
            <AvatarPicker avatarUrl={avatarUrl} onSelect={setAvatarUrl} name={name} />

            {/* Name + Description */}
            <div className="flex-1 space-y-3">
              <div>
                <label
                  htmlFor="agent-name"
                  className="block text-sm font-medium text-gray-700"
                >
                  Name <span className="text-red-500">*</span>
                </label>
                <input
                  id="agent-name"
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value.slice(0, 64))}
                  maxLength={64}
                  placeholder="e.g., Nexus"
                  className="mt-1 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:border-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-100"
                  required
                  aria-required="true"
                />
              </div>

              <div>
                <label
                  htmlFor="agent-description"
                  className="block text-sm font-medium text-gray-700"
                >
                  Description
                </label>
                <div className="relative mt-1">
                  <input
                    id="agent-description"
                    type="text"
                    value={description}
                    onChange={(e) =>
                      setDescription(e.target.value.slice(0, 255))
                    }
                    maxLength={255}
                    placeholder="What does this agent do?"
                    className="w-full rounded-lg border border-gray-200 px-3 py-2 pr-14 text-sm text-gray-900 placeholder:text-gray-400 focus:border-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-100"
                  />
                  <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-gray-400">
                    {description.length}/255
                  </span>
                </div>
              </div>
            </div>
          </div>

          {/* Visibility Toggle */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Visibility
            </label>
            <div className="grid grid-cols-2 gap-3">
              <VisibilityCard
                type="shared"
                isSelected={visibility === "shared"}
                onSelect={() => setVisibility("shared")}
              />
              <VisibilityCard
                type="private"
                isSelected={visibility === "private"}
                onSelect={() => setVisibility("private")}
              />
            </div>
          </div>

          {/* Runtime Picker */}
          <div>
            <label
              htmlFor="agent-runtime"
              className="block text-sm font-medium text-gray-700"
            >
              Runtime <span className="text-red-500">*</span>
            </label>
            <RuntimePicker
              daemons={daemons}
              runtimes={allRuntimes}
              loading={daemonsLoading}
              selectedId={runtimeId}
              onSelect={setRuntimeId}
            />
          </div>

          {/* Model Dropdown */}
          <div>
            <label
              htmlFor="agent-model"
              className="block text-sm font-medium text-gray-700"
            >
              Model
            </label>
            <input
              id="agent-model"
              type="text"
              value={model}
              onChange={(e) => setModel(e.target.value)}
              placeholder="e.g., claude-sonnet-4-20250514"
              className="mt-1 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:border-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-100"
              disabled={!runtimeId}
            />
            {!runtimeId && (
              <p className="mt-1 text-xs text-gray-400">
                Select a runtime first to configure the model.
              </p>
            )}
          </div>

          {/* Collapsible Instructions */}
          <div>
            <button
              type="button"
              onClick={() => setShowInstructions((prev) => !prev)}
              className="inline-flex items-center gap-1.5 text-sm font-medium text-gray-700 hover:text-gray-900 transition-colors"
              aria-expanded={showInstructions}
            >
              <ChevronIcon expanded={showInstructions} />
              Instructions
            </button>
            {showInstructions && (
              <textarea
                value={instructions}
                onChange={(e) => setInstructions(e.target.value)}
                placeholder="System prompt for the agent..."
                rows={5}
                className="mt-2 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:border-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-100 resize-y"
                aria-label="Agent instructions"
              />
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="shrink-0 flex items-center justify-end gap-3 border-t border-gray-200 px-6 py-4">
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleSubmit}
            disabled={disabled || isSubmitting}
            className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            aria-label={isDuplicate ? "Duplicate agent" : "Create agent"}
          >
            {isSubmitting ? "Creating..." : isDuplicate ? "Duplicate" : "Create"}
          </button>
        </div>
      </div>
    </div>
  );
}


/* ─── Internal Components ─── */

function AvatarPicker({
  avatarUrl,
  onSelect: _onSelect,
  name,
}: {
  avatarUrl: string | null;
  onSelect: (url: string | null) => void;
  name: string;
}) {
  const initials = name
    .split(/[\s_-]+/)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("");

  return (
    <div className="shrink-0">
      <div className="flex h-16 w-16 items-center justify-center rounded-xl bg-blue-100 text-blue-700 text-lg font-semibold overflow-hidden">
        {avatarUrl ? (
          <img
            src={avatarUrl}
            alt="Agent avatar"
            className="h-full w-full object-cover"
          />
        ) : (
          <span>{initials || "A"}</span>
        )}
      </div>
    </div>
  );
}

function VisibilityCard({
  type,
  isSelected,
  onSelect,
}: {
  type: "shared" | "private";
  isSelected: boolean;
  onSelect: () => void;
}) {
  const config =
    type === "shared"
      ? {
          icon: <GlobeIcon />,
          title: "Workspace",
          description: "All workspace members can see and use this agent",
        }
      : {
          icon: <LockIcon />,
          title: "Private",
          description: "Only you can see and use this agent",
        };

  return (
    <button
      type="button"
      onClick={onSelect}
      className={`flex items-start gap-3 rounded-lg border p-3 text-left transition-colors ${
        isSelected
          ? "border-blue-300 bg-blue-50/50 ring-1 ring-blue-200"
          : "border-gray-200 bg-white hover:bg-gray-50"
      }`}
      aria-pressed={isSelected}
    >
      <div className="shrink-0 mt-0.5">{config.icon}</div>
      <div>
        <span className="block text-sm font-medium text-gray-900">
          {config.title}
        </span>
        <span className="block text-xs text-gray-500 mt-0.5">
          {config.description}
        </span>
      </div>
    </button>
  );
}

interface RuntimeWithDaemon extends AgentRuntime {
  daemonName: string;
  daemonStatus: "online" | "offline";
}

function RuntimePicker({
  daemons,
  runtimes,
  loading,
  selectedId,
  onSelect,
}: {
  daemons: Daemon[];
  runtimes: RuntimeWithDaemon[];
  loading: boolean;
  selectedId: string;
  onSelect: (id: string) => void;
}) {
  if (loading) {
    return (
      <div className="mt-1 rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-400">
        Loading runtimes...
      </div>
    );
  }

  if (runtimes.length === 0) {
    return (
      <div className="mt-1 rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-500">
        No online runtimes detected. Connect a daemon to get started.
      </div>
    );
  }

  // Group runtimes by daemon
  const grouped = daemons
    .filter((d) => d.agent_runtimes.length > 0)
    .map((d) => ({
      daemon: d,
      runtimes: d.agent_runtimes,
    }));

  return (
    <select
      id="agent-runtime"
      value={selectedId}
      onChange={(e) => onSelect(e.target.value)}
      className="mt-1 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-900 focus:border-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-100"
      aria-label="Select runtime"
    >
      <option value="">Select a runtime...</option>
      {grouped.map(({ daemon, runtimes: daemonRuntimes }) => (
        <optgroup
          key={daemon.id}
          label={`${daemon.device_name} (${daemon.status})`}
        >
          {daemonRuntimes.map((rt) => (
            <option
              key={rt.id}
              value={rt.id}
              disabled={rt.status === "unavailable"}
            >
              {rt.name} ({rt.provider}) — {rt.status}
            </option>
          ))}
        </optgroup>
      ))}
    </select>
  );
}

/* ─── Icons ─── */

function ChevronIcon({ expanded }: { expanded: boolean }) {
  return (
    <svg
      className={`h-4 w-4 text-gray-400 transition-transform ${
        expanded ? "rotate-90" : ""
      }`}
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={2}
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="m8.25 4.5 7.5 7.5-7.5 7.5"
      />
    </svg>
  );
}

function GlobeIcon() {
  return (
    <svg
      className="h-5 w-5 text-blue-500"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M12 21a9.004 9.004 0 0 0 8.716-6.747M12 21a9.004 9.004 0 0 1-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 0 1 7.843 4.582M12 3a8.997 8.997 0 0 0-7.843 4.582m15.686 0A11.953 11.953 0 0 1 12 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0 1 21 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0 1 12 16.5a17.92 17.92 0 0 1-8.716-2.247m0 0A8.966 8.966 0 0 1 3 12c0-1.97.633-3.794 1.708-5.282"
      />
    </svg>
  );
}

function LockIcon() {
  return (
    <svg
      className="h-5 w-5 text-gray-500"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z"
      />
    </svg>
  );
}
