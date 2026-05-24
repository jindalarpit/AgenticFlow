import { useState, useRef, useEffect } from "react";
import type { SortKey } from "../../lib/agent-sorting";

/* ─── Props ─── */

interface SortDropdownProps {
  sort: SortKey;
  setSort: (v: SortKey) => void;
}

/* ─── Config ─── */

const SORT_OPTIONS: { key: SortKey; label: string }[] = [
  { key: "recent", label: "Recent activity" },
  { key: "name", label: "Name" },
  { key: "runs", label: "Runs" },
  { key: "created", label: "Created" },
];

/**
 * Dropdown for selecting agent sort order.
 * Defaults to "Recent activity".
 *
 * Requirements: 4.1, 4.6
 */
export function SortDropdown({ sort, setSort }: SortDropdownProps) {
  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const activeLabel =
    SORT_OPTIONS.find((o) => o.key === sort)?.label ?? "Recent activity";

  // Close on outside click
  useEffect(() => {
    if (!isOpen) return;
    function handleClickOutside(e: MouseEvent) {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [isOpen]);

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        type="button"
        onClick={() => setIsOpen((prev) => !prev)}
        className="inline-flex items-center gap-1.5 rounded-md border border-gray-200 bg-white px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50 transition-colors"
        aria-haspopup="listbox"
        aria-expanded={isOpen}
        aria-label={`Sort by: ${activeLabel}`}
      >
        <SortIcon />
        <span>{activeLabel}</span>
        <ChevronIcon />
      </button>

      {isOpen && (
        <div
          className="absolute right-0 top-full z-20 mt-1 w-44 rounded-lg border border-gray-200 bg-white py-1 shadow-lg"
          role="listbox"
          aria-label="Sort options"
        >
          {SORT_OPTIONS.map((option) => (
            <button
              key={option.key}
              type="button"
              role="option"
              aria-selected={sort === option.key}
              onClick={() => {
                setSort(option.key);
                setIsOpen(false);
              }}
              className={`flex w-full items-center px-3 py-2 text-sm transition-colors ${
                sort === option.key
                  ? "bg-blue-50 text-blue-700 font-medium"
                  : "text-gray-700 hover:bg-gray-50"
              }`}
            >
              {option.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

/* ─── Icons ─── */

function SortIcon() {
  return (
    <svg
      className="h-4 w-4 text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M3 7.5 7.5 3m0 0L12 7.5M7.5 3v13.5m13.5-3L16.5 18m0 0L12 13.5m4.5 4.5V4.5"
      />
    </svg>
  );
}

function ChevronIcon() {
  return (
    <svg
      className="h-3.5 w-3.5 text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={2}
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="m19.5 8.25-7.5 7.5-7.5-7.5"
      />
    </svg>
  );
}
