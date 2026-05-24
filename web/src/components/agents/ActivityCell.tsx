import type { AgentActivityBucket } from "../../hooks/useAgentActivity";

/* ─── Props ─── */

interface ActivityCellProps {
  buckets: AgentActivityBucket[] | null;
}

/* ─── Constants ─── */

const SPARKLINE_WIDTH = 80;
const SPARKLINE_HEIGHT = 24;
const BAR_GAP = 2;
const NUM_DAYS = 7;
const BAR_WIDTH = (SPARKLINE_WIDTH - BAR_GAP * (NUM_DAYS - 1)) / NUM_DAYS;

/**
 * Table cell for the Activity column.
 * Renders a 7-day sparkline SVG from activity buckets.
 * Shows "—" when data is missing or empty.
 *
 * Requirements: 6.6
 */
export function ActivityCell({ buckets }: ActivityCellProps) {
  if (!buckets || buckets.length === 0) {
    return <span className="text-sm text-gray-400">—</span>;
  }

  // Take the last 7 days (or fewer if less data)
  const days = buckets.slice(-NUM_DAYS);

  // Compute max value for scaling
  const values = days.map((b) => b.completed + b.failed);
  const maxVal = Math.max(...values, 1); // avoid division by zero

  return (
    <svg
      width={SPARKLINE_WIDTH}
      height={SPARKLINE_HEIGHT}
      viewBox={`0 0 ${SPARKLINE_WIDTH} ${SPARKLINE_HEIGHT}`}
      className="block"
      aria-label="7-day activity sparkline"
      role="img"
    >
      {days.map((bucket, i) => {
        const total = bucket.completed + bucket.failed;
        const barHeight = Math.max((total / maxVal) * SPARKLINE_HEIGHT, 1);
        const x = i * (BAR_WIDTH + BAR_GAP);
        const y = SPARKLINE_HEIGHT - barHeight;

        return (
          <rect
            key={i}
            x={x}
            y={y}
            width={BAR_WIDTH}
            height={barHeight}
            rx={1}
            className="fill-blue-400"
          />
        );
      })}
    </svg>
  );
}
