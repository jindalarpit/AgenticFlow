import type { Availability } from "../../lib/agent-availability";

/* ─── Props ─── */

interface AvailabilityCellProps {
  availability: Availability;
}

/* ─── Config ─── */

const AVAILABILITY_CONFIG: Record<
  Availability,
  { dotColor: string; label: string }
> = {
  online: { dotColor: "bg-green-500", label: "Online" },
  unstable: { dotColor: "bg-amber-500", label: "Unstable" },
  offline: { dotColor: "bg-gray-400", label: "Offline" },
};

/**
 * Table cell for the Status column.
 * Renders a colored dot badge (green/amber/gray) with status text.
 *
 * Requirements: 6.3
 */
export function AvailabilityCell({ availability }: AvailabilityCellProps) {
  const { dotColor, label } = AVAILABILITY_CONFIG[availability];

  return (
    <span
      className="inline-flex items-center gap-1.5 text-sm text-gray-700"
      aria-label={`Availability: ${label}`}
    >
      <span
        className={`h-2 w-2 shrink-0 rounded-full ${dotColor}`}
        aria-hidden="true"
      />
      {label}
    </span>
  );
}
