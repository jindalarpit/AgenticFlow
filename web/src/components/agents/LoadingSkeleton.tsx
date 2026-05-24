/**
 * Loading skeleton for the Agents list page.
 * Renders animated placeholder shapes for header, toolbar, filter row, and 4+ table rows.
 * Shown while agents are being fetched from the server.
 *
 * Requirements: 12.1, 12.2
 */
export function LoadingSkeleton() {
  return (
    <div className="space-y-6 animate-pulse" aria-label="Loading agents" role="status">
      {/* Header Skeleton */}
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-3">
          <div className="h-10 w-10 rounded-lg bg-gray-200" />
          <div className="space-y-2">
            <div className="h-5 w-32 rounded bg-gray-200" />
            <div className="h-4 w-56 rounded bg-gray-100" />
          </div>
        </div>
        <div className="h-9 w-28 rounded-lg bg-gray-200" />
      </div>

      {/* Toolbar Skeleton */}
      <div className="flex items-center gap-3">
        <div className="h-8 w-52 rounded-lg bg-gray-200" />
        <div className="h-8 w-28 rounded-full bg-gray-200" />
        <div className="h-8 w-36 rounded-md bg-gray-200" />
        <div className="flex-1" />
        <div className="h-4 w-16 rounded bg-gray-100" />
      </div>

      {/* Filter Row Skeleton */}
      <div className="flex items-center gap-2">
        <div className="h-7 w-14 rounded-full bg-gray-200" />
        <div className="h-7 w-18 rounded-full bg-gray-200" />
        <div className="h-7 w-20 rounded-full bg-gray-200" />
        <div className="h-7 w-16 rounded-full bg-gray-200" />
      </div>

      {/* Table Rows Skeleton */}
      <div className="space-y-1">
        {/* Table Header */}
        <div className="flex items-center gap-4 border-b border-gray-100 pb-2">
          <div className="h-3 w-16 rounded bg-gray-100" />
          <div className="flex-1" />
          <div className="h-3 w-12 rounded bg-gray-100" />
          <div className="h-3 w-14 rounded bg-gray-100" />
          <div className="h-3 w-16 rounded bg-gray-100" />
          <div className="h-3 w-12 rounded bg-gray-100" />
          <div className="h-3 w-8 rounded bg-gray-100" />
        </div>

        {/* Row Placeholders */}
        {Array.from({ length: 5 }).map((_, i) => (
          <TableRowSkeleton key={i} />
        ))}
      </div>
    </div>
  );
}

/* ─── Internal ─── */

function TableRowSkeleton() {
  return (
    <div className="flex items-center gap-4 py-3 border-b border-gray-50">
      {/* Agent cell: avatar + name + description */}
      <div className="flex items-center gap-3 w-60">
        <div className="h-8 w-8 shrink-0 rounded-md bg-gray-200" />
        <div className="space-y-1.5 flex-1">
          <div className="h-3.5 w-24 rounded bg-gray-200" />
          <div className="h-3 w-36 rounded bg-gray-100" />
        </div>
      </div>

      {/* Status cell */}
      <div className="w-24">
        <div className="h-5 w-16 rounded-full bg-gray-200" />
      </div>

      {/* Workload cell */}
      <div className="w-32">
        <div className="h-4 w-12 rounded bg-gray-100" />
      </div>

      {/* Runtime cell */}
      <div className="flex-1">
        <div className="h-4 w-28 rounded bg-gray-200" />
      </div>

      {/* Activity cell */}
      <div className="w-20">
        <div className="h-5 w-16 rounded bg-gray-100" />
      </div>

      {/* Runs cell */}
      <div className="w-12">
        <div className="h-4 w-8 rounded bg-gray-100" />
      </div>

      {/* Actions cell */}
      <div className="w-10">
        <div className="h-6 w-6 rounded bg-gray-100" />
      </div>
    </div>
  );
}
