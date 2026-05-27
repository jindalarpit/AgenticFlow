import { useState, useRef, useEffect, useCallback } from "react";
import { Link, useNavigate } from "react-router-dom";
import type { Agent, AgentStatus } from "../../lib/agent-detail-types";
import { ConfirmDialog } from "../ConfirmDialog";
import { useToast } from "../Toast";

/* ─── Props ─── */

interface PageHeaderProps {
  agent: Agent;
  isOwner: boolean;
  onDelete: () => Promise<void>;
}

/* ─── Status Badge Config ─── */

const STATUS_CONFIG: Record<AgentStatus, { dotColor: string; label: string }> = {
  idle: { dotColor: "bg-green-500", label: "Idle" },
  working: { dotColor: "bg-amber-500", label: "Working" },
  offline: { dotColor: "bg-gray-400", label: "Offline" },
};

/* ─── Main Component ─── */

/**
 * Page header for the Agent Detail page.
 *
 * Displays:
 * - Back link ("← Agents") navigating to /agents
 * - "/" separator
 * - Agent name (truncated with ellipsis on overflow)
 * - Status badge (colored dot + label)
 * - Owner-only "..." dropdown menu with "Delete" option
 * - Delete confirmation dialog
 */
export function PageHeader({ agent, isOwner, onDelete }: PageHeaderProps) {
  const navigate = useNavigate();
  const { showToast } = useToast();
  const [menuOpen, setMenuOpen] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const menuButtonRef = useRef<HTMLButtonElement>(null);

  // Close dropdown on outside click
  useEffect(() => {
    if (!menuOpen) return;
    function handleClickOutside(e: MouseEvent) {
      if (
        menuRef.current &&
        !menuRef.current.contains(e.target as Node) &&
        menuButtonRef.current &&
        !menuButtonRef.current.contains(e.target as Node)
      ) {
        setMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [menuOpen]);

  // Close dropdown on Escape
  useEffect(() => {
    if (!menuOpen) return;
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        setMenuOpen(false);
        menuButtonRef.current?.focus();
      }
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [menuOpen]);

  const handleDeleteClick = useCallback(() => {
    setMenuOpen(false);
    setConfirmOpen(true);
  }, []);

  const handleConfirmDelete = useCallback(async () => {
    setIsDeleting(true);
    try {
      await onDelete();
      navigate("/agents");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to delete agent";
      showToast(message, "error");
    } finally {
      setIsDeleting(false);
      setConfirmOpen(false);
    }
  }, [onDelete, navigate, showToast]);

  const handleCancelDelete = useCallback(() => {
    setConfirmOpen(false);
  }, []);

  const { dotColor, label } = STATUS_CONFIG[agent.status];

  return (
    <header className="flex items-center gap-3 px-4 py-3 border-b border-gray-200">
      {/* Breadcrumb: ← Agents / Agent Name */}
      <nav className="flex items-center gap-2 min-w-0 flex-1" aria-label="Breadcrumb">
        <Link
          to="/agents"
          className="text-sm text-blue-600 hover:text-blue-800 whitespace-nowrap flex-shrink-0"
        >
          ← Agents
        </Link>
        <span className="text-sm text-gray-400 flex-shrink-0" aria-hidden="true">
          /
        </span>
        <span
          className="text-sm font-medium text-gray-900 truncate"
          title={agent.name}
        >
          {agent.name}
        </span>
      </nav>

      {/* Status Badge */}
      <span
        className="inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-700 flex-shrink-0"
        aria-label={`Status: ${label}`}
      >
        <span className={`h-2 w-2 rounded-full ${dotColor}`} aria-hidden="true" />
        {label}
      </span>

      {/* Owner-only Edit button */}
      {isOwner && (
        <Link
          to={`/agents/${agent.id}/edit`}
          className="inline-flex items-center rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-1 flex-shrink-0"
        >
          Edit
        </Link>
      )}

      {/* Owner-only dropdown menu */}
      {isOwner && (
        <div className="relative flex-shrink-0">
          <button
            ref={menuButtonRef}
            type="button"
            onClick={() => setMenuOpen((prev) => !prev)}
            className="flex items-center justify-center h-8 w-8 rounded-md text-gray-500 hover:bg-gray-100 hover:text-gray-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-1"
            aria-label="Agent actions menu"
            aria-haspopup="true"
            aria-expanded={menuOpen}
          >
            <span className="text-lg leading-none">⋯</span>
          </button>

          {menuOpen && (
            <div
              ref={menuRef}
              className="absolute right-0 top-full z-20 mt-1 w-40 rounded-md border border-gray-200 bg-white py-1 shadow-lg"
              role="menu"
              aria-label="Agent actions"
            >
              <button
                type="button"
                onClick={handleDeleteClick}
                className="w-full px-4 py-2 text-left text-sm text-red-600 hover:bg-red-50 focus:bg-red-50 focus:outline-none"
                role="menuitem"
              >
                Delete
              </button>
            </div>
          )}
        </div>
      )}

      {/* Delete Confirmation Dialog */}
      <ConfirmDialog
        open={confirmOpen}
        title="Delete Agent"
        message="This will permanently delete the agent. This action cannot be undone."
        confirmLabel={isDeleting ? "Deleting…" : "Delete"}
        cancelLabel="Cancel"
        confirmVariant="danger"
        onConfirm={() => void handleConfirmDelete()}
        onCancel={handleCancelDelete}
      />
    </header>
  );
}
