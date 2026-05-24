import { useState, useRef, useEffect } from "react";
import type { AgentListItem } from "../../hooks/useAgentList";

/* ─── Props ─── */

interface AgentRowActionsProps {
  agent: AgentListItem;
  canManage: boolean;
  onDuplicate: (agent: AgentListItem) => void;
  onArchive: (agentId: string) => void;
}

/**
 * Kebab menu for agent row actions.
 * Renders a three-dot button that opens a dropdown with "Duplicate" and "Archive" options.
 * "Archive" is conditionally shown based on `canManage` prop.
 * Stops event propagation to prevent row click navigation.
 *
 * Requirements: 7.1, 7.4
 */
export function AgentRowActions({
  agent,
  canManage,
  onDuplicate,
  onArchive,
}: AgentRowActionsProps) {
  const [isOpen, setIsOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // Close on outside click
  useEffect(() => {
    if (!isOpen) return;
    function handleClickOutside(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [isOpen]);

  // Close on Escape
  useEffect(() => {
    if (!isOpen) return;
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") setIsOpen(false);
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [isOpen]);

  function handleToggle(e: React.MouseEvent) {
    e.stopPropagation();
    setIsOpen((prev) => !prev);
  }

  function handleDuplicate(e: React.MouseEvent) {
    e.stopPropagation();
    setIsOpen(false);
    onDuplicate(agent);
  }

  function handleArchive(e: React.MouseEvent) {
    e.stopPropagation();
    setIsOpen(false);
    onArchive(agent.id);
  }

  return (
    <div className="relative" ref={menuRef}>
      <button
        type="button"
        onClick={handleToggle}
        className="flex h-8 w-8 items-center justify-center rounded-md text-gray-400 hover:bg-gray-100 hover:text-gray-600 transition-colors"
        aria-label="Agent actions"
        aria-haspopup="true"
        aria-expanded={isOpen}
      >
        <KebabIcon />
      </button>

      {isOpen && (
        <div
          className="absolute right-0 top-full z-20 mt-1 w-40 rounded-lg border border-gray-200 bg-white py-1 shadow-lg"
          role="menu"
          aria-label="Agent actions menu"
        >
          <button
            type="button"
            onClick={handleDuplicate}
            className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-50 transition-colors"
            role="menuitem"
          >
            <DuplicateIcon />
            Duplicate
          </button>

          {canManage && (
            <button
              type="button"
              onClick={handleArchive}
              className="flex w-full items-center gap-2 px-3 py-2 text-sm text-red-600 hover:bg-red-50 transition-colors"
              role="menuitem"
            >
              <ArchiveIcon />
              Archive
            </button>
          )}
        </div>
      )}
    </div>
  );
}

/* ─── Icons ─── */

function KebabIcon() {
  return (
    <svg
      className="h-4 w-4"
      fill="currentColor"
      viewBox="0 0 20 20"
      aria-hidden="true"
    >
      <path d="M10 3a1.5 1.5 0 1 1 0 3 1.5 1.5 0 0 1 0-3ZM10 8.5a1.5 1.5 0 1 1 0 3 1.5 1.5 0 0 1 0-3ZM11.5 15.5a1.5 1.5 0 1 0-3 0 1.5 1.5 0 0 0 3 0Z" />
    </svg>
  );
}

function DuplicateIcon() {
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
        d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 0 1-1.125-1.125V7.875c0-.621.504-1.125 1.125-1.125H6.75a9.06 9.06 0 0 1 1.5.124m7.5 10.376h3.375c.621 0 1.125-.504 1.125-1.125V11.25c0-4.46-3.243-8.161-7.5-8.876a9.06 9.06 0 0 0-1.5-.124H9.375c-.621 0-1.125.504-1.125 1.125v3.5m7.5 10.375H9.375a1.125 1.125 0 0 1-1.125-1.125v-9.25m12 6.625v-1.875a3.375 3.375 0 0 0-3.375-3.375h-1.5a1.125 1.125 0 0 1-1.125-1.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H9.75"
      />
    </svg>
  );
}

function ArchiveIcon() {
  return (
    <svg
      className="h-4 w-4"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="m20.25 7.5-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z"
      />
    </svg>
  );
}
