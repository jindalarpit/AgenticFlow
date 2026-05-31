import { PROVIDER_TYPE_LABELS, type ProviderType } from "../../hooks/useProviders";

interface ProviderTypeBadgeProps {
  type: ProviderType;
}

/**
 * Displays a styled badge showing the provider type label.
 */
export function ProviderTypeBadge({ type }: ProviderTypeBadgeProps) {
  const label = PROVIDER_TYPE_LABELS[type] ?? type;

  return (
    <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-50 text-blue-700">
      {label}
    </span>
  );
}
