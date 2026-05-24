import { useState, useRef, useEffect } from "react";
import type { Agent, AgentStatus } from "../../lib/agent-detail-types";

/* ─── Props ─── */

interface IdentitySectionProps {
  agent: Agent;
  isOwner: boolean;
  onUpdate: (data: Partial<Agent>) => Promise<void>;
}

/* ─── Validation ─── */

const AGENT_NAME_REGEX = /^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$/;

function validateName(name: string): string | null {
  if (!name.trim()) return "Name is required";
  if (name.length > 64) return "Name must not exceed 64 characters";
  if (!AGENT_NAME_REGEX.test(name))
    return "Name must start with a letter or number and contain only letters, numbers, hyphens, and underscores";
  return null;
}

function validateDescription(desc: string): string | null {
  if (desc.length > 255) return "Description must not exceed 255 characters";
  return null;
}

/* ─── Main Component ─── */

export function IdentitySection({ agent, isOwner, onUpdate }: IdentitySectionProps) {
  return (
    <div className="flex flex-col items-center gap-3 py-4">
      {/* Avatar */}
      <Avatar avatarUrl={agent.avatar_url} name={agent.name} />

      {/* Name */}
      {isOwner ? (
        <EditableName name={agent.name} onSave={(name) => onUpdate({ name })} />
      ) : (
        <h2 className="text-base font-semibold text-gray-900 text-center">
          {agent.name}
        </h2>
      )}

      {/* Description */}
      {isOwner ? (
        <EditableDescription
          description={agent.description}
          onSave={(description) => onUpdate({ description })}
        />
      ) : (
        <p className="text-sm text-gray-500 text-center">
          {agent.description || "No description"}
        </p>
      )}

      {/* Status Badge */}
      <StatusBadge status={agent.status} />
    </div>
  );
}

/* ─── Avatar ─── */

function Avatar({ avatarUrl, name }: { avatarUrl: string | null; name: string }) {
  const initials = name
    .split(/[\s_-]+/)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("");

  if (avatarUrl) {
    return (
      <img
        src={avatarUrl}
        alt={`${name} avatar`}
        className="h-14 w-14 rounded-lg object-cover"
      />
    );
  }

  return (
    <div
      className="flex h-14 w-14 items-center justify-center rounded-lg bg-blue-100 text-blue-700 font-semibold text-lg"
      aria-label={`${name} avatar`}
    >
      {initials || "A"}
    </div>
  );
}

/* ─── Status Badge ─── */

const STATUS_CONFIG: Record<AgentStatus, { dotColor: string; label: string }> = {
  idle: { dotColor: "bg-green-500", label: "Idle" },
  working: { dotColor: "bg-amber-500", label: "Working" },
  offline: { dotColor: "bg-gray-400", label: "Offline" },
};

function StatusBadge({ status }: { status: AgentStatus }) {
  const { dotColor, label } = STATUS_CONFIG[status];

  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-700"
      aria-label={`Status: ${label}`}
    >
      <span className={`h-2 w-2 rounded-full ${dotColor}`} aria-hidden="true" />
      {label}
    </span>
  );
}

/* ─── Editable Name (Popover) ─── */

function EditableName({
  name,
  onSave,
}: {
  name: string;
  onSave: (name: string) => Promise<void>;
}) {
  const [isOpen, setIsOpen] = useState(false);
  const [value, setValue] = useState(name);
  const [error, setError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const popoverRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Sync value when agent name changes externally
  useEffect(() => {
    if (!isOpen) {
      setValue(name);
    }
  }, [name, isOpen]);

  // Focus input when popover opens
  useEffect(() => {
    if (isOpen) {
      inputRef.current?.focus();
      inputRef.current?.select();
    }
  }, [isOpen]);

  // Close on outside click
  useEffect(() => {
    if (!isOpen) return;
    function handleClickOutside(e: MouseEvent) {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        handleCancel();
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [isOpen]);

  function handleOpen() {
    setValue(name);
    setError(null);
    setIsOpen(true);
  }

  function handleCancel() {
    setIsOpen(false);
    setValue(name);
    setError(null);
  }

  async function handleSave() {
    const trimmed = value.trim();
    const validationError = validateName(trimmed);
    if (validationError) {
      setError(validationError);
      return;
    }
    if (trimmed === name) {
      setIsOpen(false);
      return;
    }
    setIsSaving(true);
    try {
      await onSave(trimmed);
      setIsOpen(false);
      setError(null);
    } catch {
      setError("Failed to save name");
    } finally {
      setIsSaving(false);
    }
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === "Enter") {
      e.preventDefault();
      void handleSave();
    } else if (e.key === "Escape") {
      handleCancel();
    }
  }

  return (
    <div className="relative">
      <button
        type="button"
        onClick={handleOpen}
        className="text-base font-semibold text-gray-900 hover:text-blue-600 transition-colors cursor-pointer text-center"
        aria-label="Edit agent name"
      >
        {name}
      </button>

      {isOpen && (
        <div
          ref={popoverRef}
          className="absolute left-1/2 top-full z-10 mt-2 w-64 -translate-x-1/2 rounded-lg border border-gray-200 bg-white p-3 shadow-lg"
          role="dialog"
          aria-label="Edit name"
        >
          <label className="block text-xs font-medium text-gray-600 mb-1">
            Agent Name
          </label>
          <input
            ref={inputRef}
            type="text"
            value={value}
            onChange={(e) => {
              setValue(e.target.value);
              setError(null);
            }}
            onKeyDown={handleKeyDown}
            maxLength={64}
            className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          {error && <p className="mt-1 text-xs text-red-600">{error}</p>}
          <div className="mt-2 flex justify-end gap-2">
            <button
              type="button"
              onClick={handleCancel}
              disabled={isSaving}
              className="rounded-md border border-gray-300 bg-white px-3 py-1 text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={() => void handleSave()}
              disabled={isSaving}
              className="rounded-md bg-blue-600 px-3 py-1 text-xs font-medium text-white hover:bg-blue-700 disabled:opacity-50"
            >
              {isSaving ? "Saving…" : "Save"}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

/* ─── Editable Description (Modal Dialog) ─── */

function EditableDescription({
  description,
  onSave,
}: {
  description: string;
  onSave: (description: string) => Promise<void>;
}) {
  const [isOpen, setIsOpen] = useState(false);
  const [value, setValue] = useState(description);
  const [error, setError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const dialogRef = useRef<HTMLDialogElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Sync value when description changes externally
  useEffect(() => {
    if (!isOpen) {
      setValue(description);
    }
  }, [description, isOpen]);

  // Open/close dialog
  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (isOpen && !dialog.open) {
      dialog.showModal();
      // Focus textarea after dialog opens
      setTimeout(() => textareaRef.current?.focus(), 0);
    } else if (!isOpen && dialog.open) {
      dialog.close();
    }
  }, [isOpen]);

  function handleOpen() {
    setValue(description);
    setError(null);
    setIsOpen(true);
  }

  function handleCancel() {
    setIsOpen(false);
    setValue(description);
    setError(null);
  }

  async function handleSave() {
    const validationError = validateDescription(value);
    if (validationError) {
      setError(validationError);
      return;
    }
    if (value === description) {
      setIsOpen(false);
      return;
    }
    setIsSaving(true);
    try {
      await onSave(value);
      setIsOpen(false);
      setError(null);
    } catch {
      setError("Failed to save description");
    } finally {
      setIsSaving(false);
    }
  }

  function handleDialogCancel(e: React.SyntheticEvent<HTMLDialogElement>) {
    e.preventDefault();
    handleCancel();
  }

  function handleBackdropClick(e: React.MouseEvent<HTMLDialogElement>) {
    if (e.target === dialogRef.current) {
      handleCancel();
    }
  }

  return (
    <>
      <button
        type="button"
        onClick={handleOpen}
        className="text-sm text-gray-500 hover:text-blue-600 transition-colors cursor-pointer text-center"
        aria-label="Edit agent description"
      >
        {description || "No description"}
      </button>

      <dialog
        ref={dialogRef}
        onClick={handleBackdropClick}
        onCancel={handleDialogCancel}
        className="fixed inset-0 z-50 m-auto w-full max-w-md rounded-lg border border-gray-200 bg-white p-0 shadow-xl backdrop:bg-black/30"
        aria-labelledby="edit-description-title"
      >
        <div className="p-6">
          <h2
            id="edit-description-title"
            className="text-lg font-semibold text-gray-900"
          >
            Edit Description
          </h2>
          <div className="mt-4">
            <textarea
              ref={textareaRef}
              value={value}
              onChange={(e) => {
                setValue(e.target.value);
                setError(null);
              }}
              maxLength={255}
              rows={4}
              placeholder="Enter a description for this agent..."
              className="block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 resize-none"
            />
            <p className="mt-1 text-xs text-gray-400 text-right">
              {value.length}/255
            </p>
            {error && <p className="mt-1 text-xs text-red-600">{error}</p>}
          </div>
          <div className="mt-4 flex justify-end gap-3">
            <button
              type="button"
              onClick={handleCancel}
              disabled={isSaving}
              className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={() => void handleSave()}
              disabled={isSaving}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
            >
              {isSaving ? "Saving…" : "Save"}
            </button>
          </div>
        </div>
      </dialog>
    </>
  );
}
