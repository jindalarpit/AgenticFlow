import { useDeliverableTypes } from "../../hooks/useDeliverableTypes";

interface DeliverableTypeSelectorProps {
  value: string;
  onChange: (deliverableTypeId: string) => void;
}

/**
 * DeliverableTypeSelector — dropdown for selecting a deliverable type in local mode.
 *
 * Shows all deliverable types including "Code Execution" (the default for local agents).
 */
export function DeliverableTypeSelector({
  value,
  onChange,
}: DeliverableTypeSelectorProps) {
  const { data: deliverableTypes, isLoading } = useDeliverableTypes();

  if (isLoading) {
    return (
      <div className="space-y-1">
        <label className="block text-sm font-medium text-gray-700">
          Deliverable Type
        </label>
        <div className="h-10 bg-gray-100 rounded-md animate-pulse" />
      </div>
    );
  }

  return (
    <div className="space-y-1">
      <label
        htmlFor="local-deliverable-type-select"
        className="block text-sm font-medium text-gray-700"
      >
        Deliverable Type
      </label>
      <select
        id="local-deliverable-type-select"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
      >
        <option value="">Default (Code Execution)</option>
        {(deliverableTypes ?? []).map((dt) => (
          <option key={dt.id} value={dt.id}>
            {dt.name}
            {dt.is_system ? " (System)" : ""}
          </option>
        ))}
      </select>
      <p className="text-xs text-gray-500">
        Defines the output format for this agent.
      </p>
    </div>
  );
}
