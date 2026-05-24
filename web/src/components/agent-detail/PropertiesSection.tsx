import { useState } from "react";
import type { Agent } from "../../lib/agent-detail-types";

/* ─── Props ─── */

interface PropertiesSectionProps {
  agent: Agent;
  isOwner: boolean;
  onUpdate: (data: Partial<Agent>) => Promise<void>;
}

/* ─── Component ─── */

/**
 * PropertiesSection displays the "PROPERTIES" section in the sidebar inspector.
 *
 * Shows Runtime (with online/offline dot), Model, Visibility, and Concurrency.
 * Owner mode: interactive controls for editing each property.
 * Non-owner mode: read-only text display.
 */
export function PropertiesSection({ agent, isOwner, onUpdate }: PropertiesSectionProps) {
  return (
    <section aria-labelledby="properties-heading">
      <h3
        id="properties-heading"
        className="text-xs font-semibold uppercase tracking-wider text-gray-500 mb-3"
      >
        Properties
      </h3>
      <div className="space-y-3">
        <RuntimeRow agent={agent} isOwner={isOwner} onUpdate={onUpdate} />
        <ModelRow agent={agent} isOwner={isOwner} onUpdate={onUpdate} />
        <VisibilityRow agent={agent} isOwner={isOwner} onUpdate={onUpdate} />
        <ConcurrencyRow agent={agent} isOwner={isOwner} onUpdate={onUpdate} />
      </div>
    </section>
  );
}

/* ─── Runtime Row ─── */

function RuntimeRow({
  agent,
  isOwner,
  onUpdate,
}: PropertiesSectionProps) {
  const isOnline = agent.status !== "offline";
  const displayName = agent.runtime_name || "Unknown Runtime";

  return (
    <PropertyRow label="Runtime">
      <div className="flex items-center gap-2">
        <span
          className={`inline-block h-2 w-2 rounded-full ${
            isOnline ? "bg-green-500" : "bg-gray-400"
          }`}
          aria-label={isOnline ? "Online" : "Offline"}
        />
        {isOwner ? (
          <RuntimePicker
            currentValue={displayName}
            runtimeId={agent.runtime_id}
            onUpdate={onUpdate}
          />
        ) : (
          <span className="text-sm text-gray-900">{displayName}</span>
        )}
      </div>
    </PropertyRow>
  );
}

/* ─── RuntimePicker (owner mode) ─── */

function RuntimePicker({
  currentValue,
  runtimeId,
  onUpdate,
}: {
  currentValue: string;
  runtimeId: string;
  onUpdate: (data: Partial<Agent>) => Promise<void>;
}) {
  // For now, display the current runtime value as a read-only dropdown
  // since we don't have a runtimes list endpoint in this spec.
  return (
    <select
      value={runtimeId}
      onChange={(e) => {
        if (e.target.value !== runtimeId) {
          void onUpdate({ runtime_id: e.target.value });
        }
      }}
      className="text-sm text-gray-900 bg-transparent border border-gray-200 rounded px-2 py-0.5 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
      aria-label="Select runtime"
    >
      <option value={runtimeId}>{currentValue}</option>
    </select>
  );
}

/* ─── Model Row ─── */

function ModelRow({
  agent,
  isOwner,
  onUpdate,
}: PropertiesSectionProps) {
  const displayValue = agent.model || "Default";

  return (
    <PropertyRow label="Model">
      {isOwner ? (
        <ModelInput currentValue={agent.model} onUpdate={onUpdate} />
      ) : (
        <span className="text-sm text-gray-900">{displayValue}</span>
      )}
    </PropertyRow>
  );
}

/* ─── ModelInput (owner mode) ─── */

function ModelInput({
  currentValue,
  onUpdate,
}: {
  currentValue: string;
  onUpdate: (data: Partial<Agent>) => Promise<void>;
}) {
  const [value, setValue] = useState(currentValue);
  const [isEditing, setIsEditing] = useState(false);

  function handleBlur() {
    setIsEditing(false);
    if (value !== currentValue) {
      void onUpdate({ model: value });
    }
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter") {
      e.currentTarget.blur();
    } else if (e.key === "Escape") {
      setValue(currentValue);
      setIsEditing(false);
    }
  }

  if (!isEditing) {
    return (
      <button
        type="button"
        onClick={() => setIsEditing(true)}
        className="text-sm text-gray-900 hover:text-blue-600 text-left truncate max-w-[180px]"
        aria-label="Edit model"
      >
        {currentValue || "Default"}
      </button>
    );
  }

  return (
    <input
      type="text"
      value={value}
      onChange={(e) => setValue(e.target.value)}
      onBlur={handleBlur}
      onKeyDown={handleKeyDown}
      placeholder="e.g., claude-sonnet-4-20250514"
      className="text-sm text-gray-900 bg-transparent border border-gray-200 rounded px-2 py-0.5 w-full max-w-[180px] focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
      aria-label="Model name"
      autoFocus
    />
  );
}

/* ─── Visibility Row ─── */

function VisibilityRow({
  agent,
  isOwner,
  onUpdate,
}: PropertiesSectionProps) {
  const isShared = agent.visibility === "shared";
  const displayValue = isShared ? "Shared" : "Private";

  return (
    <PropertyRow label="Visibility">
      {isOwner ? (
        <VisibilityToggle
          isShared={isShared}
          onToggle={() => {
            const newVisibility = isShared ? "private" : "shared";
            void onUpdate({ visibility: newVisibility });
          }}
        />
      ) : (
        <span className="text-sm text-gray-900">{displayValue}</span>
      )}
    </PropertyRow>
  );
}

/* ─── VisibilityToggle (owner mode) ─── */

function VisibilityToggle({
  isShared,
  onToggle,
}: {
  isShared: boolean;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={isShared}
      aria-label={`Visibility: ${isShared ? "Shared" : "Private"}`}
      onClick={onToggle}
      className="inline-flex items-center gap-2 text-sm"
    >
      <span
        className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
          isShared ? "bg-blue-600" : "bg-gray-300"
        }`}
      >
        <span
          className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform ${
            isShared ? "translate-x-4" : "translate-x-1"
          }`}
        />
      </span>
      <span className="text-gray-900">{isShared ? "Shared" : "Private"}</span>
    </button>
  );
}

/* ─── Concurrency Row ─── */

function ConcurrencyRow({
  agent,
  isOwner,
  onUpdate,
}: PropertiesSectionProps) {
  return (
    <PropertyRow label="Concurrency">
      {isOwner ? (
        <ConcurrencyPicker
          value={agent.max_concurrent_tasks}
          onUpdate={onUpdate}
        />
      ) : (
        <span className="text-sm text-gray-900">{agent.max_concurrent_tasks}</span>
      )}
    </PropertyRow>
  );
}

/* ─── ConcurrencyPicker (owner mode) ─── */

function ConcurrencyPicker({
  value,
  onUpdate,
}: {
  value: number;
  onUpdate: (data: Partial<Agent>) => Promise<void>;
}) {
  function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
    const raw = parseInt(e.target.value, 10);
    if (isNaN(raw)) return;
    const clamped = Math.min(20, Math.max(1, raw));
    if (clamped !== value) {
      void onUpdate({ max_concurrent_tasks: clamped });
    }
  }

  return (
    <input
      type="number"
      min={1}
      max={20}
      value={value}
      onChange={handleChange}
      className="text-sm text-gray-900 bg-transparent border border-gray-200 rounded px-2 py-0.5 w-16 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
      aria-label="Max concurrent tasks (1-20)"
    />
  );
}

/* ─── Shared PropertyRow Layout ─── */

function PropertyRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-sm text-gray-500">{label}</span>
      <div className="flex items-center">{children}</div>
    </div>
  );
}
