import type { ProviderStatus } from "../../hooks/useProviders";

interface StatusBadgeProps {
  status: ProviderStatus;
}

const STATUS_CONFIG: Record<ProviderStatus, { label: string; className: string }> = {
  active: {
    label: "Active",
    className: "bg-green-100 text-green-700",
  },
  error: {
    label: "Error",
    className: "bg-red-100 text-red-700",
  },
  validating: {
    label: "Validating",
    className: "bg-amber-100 text-amber-700",
  },
  inactive: {
    label: "Inactive",
    className: "bg-gray-100 text-gray-600",
  },
};

/**
 * Displays a colored status badge for a provider's current status.
 */
export function StatusBadge({ status }: StatusBadgeProps) {
  const config = STATUS_CONFIG[status] ?? STATUS_CONFIG.inactive;

  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium ${config.className}`}
    >
      <span className="w-1.5 h-1.5 rounded-full bg-current" />
      {config.label}
    </span>
  );
}
