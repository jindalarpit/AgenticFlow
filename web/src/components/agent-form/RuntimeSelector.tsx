import { useMemo } from "react";
import { useDaemons } from "../../hooks/useDaemons";
import type { RuntimeSelectorProps, FlatRuntime } from "./types";

/**
 * RuntimeSelector — displays available runtimes flattened from daemon data.
 *
 * Shows each runtime with name, provider, and daemon online/offline status.
 * Displays "No runtimes detected" when the list is empty.
 */
export function RuntimeSelector({ value, onChange, error }: RuntimeSelectorProps) {
  const { data: daemons, isLoading } = useDaemons();

  // Flatten daemon.agent_runtimes[] into a unified list
  const runtimes: FlatRuntime[] = useMemo(() => {
    if (!daemons) return [];
    const flat: FlatRuntime[] = [];
    for (const daemon of daemons) {
      for (const rt of daemon.agent_runtimes ?? []) {
        flat.push({
          id: rt.id,
          name: rt.name,
          provider: rt.provider,
          status: rt.status,
          daemon_id: daemon.daemon_id,
          daemon_status: daemon.status,
          daemon_device_name: daemon.device_name,
        });
      }
    }
    return flat;
  }, [daemons]);

  if (isLoading) {
    return (
      <div className="space-y-1">
        <label className="block text-sm font-medium text-gray-700">
          Runtime <span className="text-red-500">*</span>
        </label>
        <div className="h-10 bg-gray-100 rounded-md animate-pulse" />
      </div>
    );
  }

  if (runtimes.length === 0) {
    return (
      <div className="space-y-1">
        <label className="block text-sm font-medium text-gray-700">
          Runtime <span className="text-red-500">*</span>
        </label>
        <p className="text-sm text-amber-600">
          No runtimes detected. Connect a daemon with an AI CLI runtime to continue.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-1">
      <label
        htmlFor="runtime-select"
        className="block text-sm font-medium text-gray-700"
      >
        Runtime <span className="text-red-500">*</span>
      </label>
      <select
        id="runtime-select"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={`block w-full rounded-md border px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 ${
          error
            ? "border-red-300 focus:ring-red-500"
            : "border-gray-300"
        }`}
        aria-invalid={!!error}
        aria-describedby={error ? "runtime-error" : undefined}
      >
        <option value="">Select a runtime…</option>
        {runtimes.map((rt) => (
          <option key={rt.id} value={rt.id}>
            {rt.name} ({rt.provider}) — {rt.daemon_status === "online" ? "🟢 Online" : "🔴 Offline"}
          </option>
        ))}
      </select>
      {error && (
        <p id="runtime-error" className="text-sm text-red-600">
          {error}
        </p>
      )}
    </div>
  );
}
