/* ─── Props ─── */

interface RunsCellProps {
  runCount: number | null;
}

/**
 * Table cell for the Runs column.
 * Renders a right-aligned monospace 30-day run count.
 * Shows "—" when data is missing (null).
 *
 * Requirements: 6.7
 */
export function RunsCell({ runCount }: RunsCellProps) {
  if (runCount === null) {
    return (
      <span className="block text-right text-sm text-gray-400">—</span>
    );
  }

  return (
    <span className="block text-right font-mono text-sm text-gray-700">
      {runCount.toLocaleString()}
    </span>
  );
}
