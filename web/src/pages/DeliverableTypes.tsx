import { useState, useMemo, useCallback, useEffect, useRef } from "react";
import {
  useDeliverableTypes,
  useCreateDeliverableType,
  useUpdateDeliverableType,
  useDeleteDeliverableType,
  type DeliverableType,
  type CreateDeliverableTypeRequest,
  type UpdateDeliverableTypeRequest,
} from "../hooks/useDeliverableTypes";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { useToast } from "../components/Toast";

/**
 * Deliverable Types management page at `/deliverable-types`.
 *
 * Displays all deliverable types (system + user-created) with the ability
 * to create, edit, and delete user-created types. System types are shown
 * but cannot be modified or deleted.
 *
 * Requirements: 4.1, 4.3, 4.4, 4.5, 4.7
 */
export default function DeliverableTypes() {
  const { data: types, isLoading } = useDeliverableTypes();
  const [search, setSearch] = useState("");
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [editingType, setEditingType] = useState<DeliverableType | null>(null);
  const [deletingType, setDeletingType] = useState<DeliverableType | null>(null);

  const filteredTypes = useMemo(() => {
    if (!types) return [];
    if (!search.trim()) return types;
    const term = search.toLowerCase();
    return types.filter(
      (t) =>
        t.name.toLowerCase().includes(term) ||
        t.description.toLowerCase().includes(term)
    );
  }, [types, search]);

  const systemTypes = useMemo(
    () => filteredTypes.filter((t) => t.is_system),
    [filteredTypes]
  );

  const userTypes = useMemo(
    () => filteredTypes.filter((t) => !t.is_system),
    [filteredTypes]
  );

  return (
    <div className="max-w-7xl mx-auto px-6 py-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-xl font-semibold text-gray-900">
            Deliverable Types
          </h2>
          <p className="text-sm text-gray-500 mt-1">
            Define output formats that shape how your agents structure their
            responses.
          </p>
        </div>
        <button
          onClick={() => setShowCreateDialog(true)}
          className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
        >
          + New Type
        </button>
      </div>

      {/* Search */}
      {!isLoading && types && types.length > 0 && (
        <div className="mb-6">
          <input
            type="text"
            placeholder="Search deliverable types by name or description…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full max-w-md px-3 py-2 border border-gray-300 rounded-md text-sm placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
        </div>
      )}

      {/* Loading state */}
      {isLoading && <LoadingSkeleton />}

      {/* Empty state */}
      {!isLoading && (!types || types.length === 0) && <EmptyState />}

      {/* No search matches */}
      {!isLoading &&
        types &&
        types.length > 0 &&
        filteredTypes.length === 0 && (
          <div className="text-center py-12">
            <p className="text-sm text-gray-500">
              No deliverable types match &ldquo;{search}&rdquo;
            </p>
          </div>
        )}

      {/* System Types Section */}
      {!isLoading && systemTypes.length > 0 && (
        <section className="mb-8">
          <h3 className="text-sm font-semibold text-gray-500 uppercase tracking-wide mb-3">
            System Types
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {systemTypes.map((type) => (
              <DeliverableTypeCard
                key={type.id}
                type={type}
                onEdit={() => {}}
                onDelete={() => {}}
              />
            ))}
          </div>
        </section>
      )}

      {/* User Types Section */}
      {!isLoading && userTypes.length > 0 && (
        <section>
          <h3 className="text-sm font-semibold text-gray-500 uppercase tracking-wide mb-3">
            Custom Types
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {userTypes.map((type) => (
              <DeliverableTypeCard
                key={type.id}
                type={type}
                onEdit={() => setEditingType(type)}
                onDelete={() => setDeletingType(type)}
              />
            ))}
          </div>
        </section>
      )}

      {/* Create Dialog */}
      {showCreateDialog && (
        <DeliverableTypeFormDialog
          onClose={() => setShowCreateDialog(false)}
        />
      )}

      {/* Edit Dialog */}
      {editingType && (
        <DeliverableTypeFormDialog
          type={editingType}
          onClose={() => setEditingType(null)}
        />
      )}

      {/* Delete Confirmation */}
      {deletingType && (
        <DeleteDeliverableTypeDialog
          type={deletingType}
          onClose={() => setDeletingType(null)}
        />
      )}
    </div>
  );
}

/* ─── Deliverable Type Card ─── */

function DeliverableTypeCard({
  type,
  onEdit,
  onDelete,
}: {
  type: DeliverableType;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const descriptionPreview = type.description
    ? type.description.length > 100
      ? type.description.slice(0, 100) + "…"
      : type.description
    : null;

  return (
    <div className="bg-white rounded-lg border border-gray-200 p-4 shadow-sm hover:shadow-md transition-shadow">
      <div className="flex items-start justify-between mb-2">
        <h4 className="font-medium text-gray-900 truncate">{type.name}</h4>
        <div className="flex items-center gap-1 flex-shrink-0 ml-2">
          {type.is_system ? (
            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-600">
              System
            </span>
          ) : (
            <>
              <button
                onClick={onEdit}
                className="p-1 text-gray-400 hover:text-blue-600 rounded transition-colors"
                title="Edit"
                aria-label={`Edit ${type.name}`}
              >
                <svg
                  className="w-4 h-4"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                  />
                </svg>
              </button>
              <button
                onClick={onDelete}
                className="p-1 text-gray-400 hover:text-red-600 rounded transition-colors"
                title="Delete"
                aria-label={`Delete ${type.name}`}
              >
                <svg
                  className="w-4 h-4"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                  />
                </svg>
              </button>
            </>
          )}
        </div>
      </div>

      {descriptionPreview && (
        <p className="text-sm text-gray-500 mb-2">{descriptionPreview}</p>
      )}

      {type.output_format && (
        <div className="mt-2 pt-2 border-t border-gray-100">
          <p className="text-xs text-gray-400 font-medium mb-1">
            Output Format
          </p>
          <p className="text-xs text-gray-500 font-mono truncate">
            {type.output_format.slice(0, 80)}
            {type.output_format.length > 80 ? "…" : ""}
          </p>
        </div>
      )}
    </div>
  );
}

/* ─── Form Dialog (Create / Edit) ─── */

function DeliverableTypeFormDialog({
  type,
  onClose,
}: {
  type?: DeliverableType;
  onClose: () => void;
}) {
  const isEdit = !!type;
  const { showToast } = useToast();
  const createMutation = useCreateDeliverableType();
  const updateMutation = useUpdateDeliverableType(type?.id ?? "");

  const [name, setName] = useState(type?.name ?? "");
  const [description, setDescription] = useState(type?.description ?? "");
  const [outputFormat, setOutputFormat] = useState(type?.output_format ?? "");
  const [errors, setErrors] = useState<Record<string, string>>({});

  const dialogRef = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (dialog && !dialog.open) {
      dialog.showModal();
    }
  }, []);

  const validate = useCallback((): boolean => {
    const newErrors: Record<string, string> = {};

    if (!name.trim() || name.length < 1 || name.length > 64) {
      newErrors.name = "Name must be between 1 and 64 characters";
    }
    if (description.length > 255) {
      newErrors.description = "Description must be at most 255 characters";
    }
    if (outputFormat.length > 10000) {
      newErrors.outputFormat = "Output format must be at most 10,000 characters";
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  }, [name, description, outputFormat]);

  const handleSubmit = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault();
      if (!validate()) return;

      if (isEdit) {
        const data: UpdateDeliverableTypeRequest = {
          name: name.trim(),
          description,
          output_format: outputFormat,
        };
        updateMutation.mutate(data, {
          onSuccess: () => {
            showToast("Deliverable type updated", "success");
            onClose();
          },
          onError: (err: Error) => {
            showToast(err.message || "Failed to update deliverable type", "error");
          },
        });
      } else {
        const data: CreateDeliverableTypeRequest = {
          name: name.trim(),
          description,
          output_format: outputFormat,
        };
        createMutation.mutate(data, {
          onSuccess: () => {
            showToast("Deliverable type created", "success");
            onClose();
          },
          onError: (err: Error) => {
            showToast(err.message || "Failed to create deliverable type", "error");
          },
        });
      }
    },
    [
      validate,
      isEdit,
      name,
      description,
      outputFormat,
      createMutation,
      updateMutation,
      showToast,
      onClose,
    ]
  );

  function handleBackdropClick(e: React.MouseEvent<HTMLDialogElement>) {
    if (e.target === dialogRef.current) {
      onClose();
    }
  }

  function handleCancel(e: React.SyntheticEvent<HTMLDialogElement>) {
    e.preventDefault();
    onClose();
  }

  const isPending = createMutation.isPending || updateMutation.isPending;

  return (
    <dialog
      ref={dialogRef}
      onClick={handleBackdropClick}
      onCancel={handleCancel}
      className="fixed inset-0 z-50 m-auto w-full max-w-lg rounded-lg border border-gray-200 bg-white p-0 shadow-xl backdrop:bg-black/30"
      aria-labelledby="deliverable-type-form-title"
    >
      <form onSubmit={handleSubmit} className="p-6">
        <h2
          id="deliverable-type-form-title"
          className="text-lg font-semibold text-gray-900 mb-4"
        >
          {isEdit ? "Edit Deliverable Type" : "Create Deliverable Type"}
        </h2>

        {/* Name */}
        <div className="mb-4">
          <label
            htmlFor="dt-name"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            Name <span className="text-red-500">*</span>
          </label>
          <input
            id="dt-name"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            maxLength={64}
            placeholder="e.g., API Documentation"
            className={`w-full px-3 py-2 border rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 ${
              errors.name ? "border-red-300" : "border-gray-300"
            }`}
          />
          {errors.name && (
            <p className="mt-1 text-xs text-red-600">{errors.name}</p>
          )}
          <p className="mt-1 text-xs text-gray-400">{name.length}/64</p>
        </div>

        {/* Description */}
        <div className="mb-4">
          <label
            htmlFor="dt-description"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            Description
          </label>
          <input
            id="dt-description"
            type="text"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            maxLength={255}
            placeholder="Brief description of this deliverable type"
            className={`w-full px-3 py-2 border rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 ${
              errors.description ? "border-red-300" : "border-gray-300"
            }`}
          />
          {errors.description && (
            <p className="mt-1 text-xs text-red-600">{errors.description}</p>
          )}
          <p className="mt-1 text-xs text-gray-400">
            {description.length}/255
          </p>
        </div>

        {/* Output Format (large textarea) */}
        <div className="mb-6">
          <label
            htmlFor="dt-output-format"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            Output Format
          </label>
          <p className="text-xs text-gray-500 mb-2">
            A markdown template that instructs the AI on the expected structure
            of the output. This is appended to the system prompt during
            execution.
          </p>
          <textarea
            id="dt-output-format"
            value={outputFormat}
            onChange={(e) => setOutputFormat(e.target.value)}
            maxLength={10000}
            rows={10}
            placeholder={"# Document Title\n\n## Section 1\n[Content here]\n\n## Section 2\n[Content here]"}
            className={`w-full px-3 py-2 border rounded-md text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 resize-y ${
              errors.outputFormat ? "border-red-300" : "border-gray-300"
            }`}
          />
          {errors.outputFormat && (
            <p className="mt-1 text-xs text-red-600">{errors.outputFormat}</p>
          )}
          <p className="mt-1 text-xs text-gray-400">
            {outputFormat.length}/10,000
          </p>
        </div>

        {/* Actions */}
        <div className="flex justify-end gap-3">
          <button
            type="button"
            onClick={onClose}
            disabled={isPending}
            className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={isPending}
            className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50"
          >
            {isPending
              ? isEdit
                ? "Saving…"
                : "Creating…"
              : isEdit
                ? "Save Changes"
                : "Create"}
          </button>
        </div>
      </form>
    </dialog>
  );
}

/* ─── Delete Dialog ─── */

function DeleteDeliverableTypeDialog({
  type,
  onClose,
}: {
  type: DeliverableType;
  onClose: () => void;
}) {
  const deleteMutation = useDeleteDeliverableType();
  const { showToast } = useToast();

  const handleConfirm = useCallback(() => {
    deleteMutation.mutate(type.id, {
      onSuccess: () => {
        showToast("Deliverable type deleted", "success");
        onClose();
      },
      onError: (err: Error) => {
        showToast(err.message || "Failed to delete deliverable type", "error");
        onClose();
      },
    });
  }, [deleteMutation, type.id, showToast, onClose]);

  return (
    <ConfirmDialog
      open={true}
      title="Delete Deliverable Type"
      message={`Are you sure you want to delete "${type.name}"? This action cannot be undone.`}
      confirmLabel="Delete"
      confirmVariant="danger"
      onConfirm={handleConfirm}
      onCancel={onClose}
    />
  );
}

/* ─── Loading Skeleton ─── */

function LoadingSkeleton() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {Array.from({ length: 6 }).map((_, i) => (
        <div
          key={i}
          className="bg-white rounded-lg border border-gray-200 p-4 animate-pulse"
        >
          <div className="flex items-start justify-between mb-2">
            <div className="h-5 w-32 bg-gray-200 rounded" />
            <div className="h-5 w-16 bg-gray-100 rounded" />
          </div>
          <div className="space-y-2">
            <div className="h-4 w-full bg-gray-100 rounded" />
            <div className="h-4 w-2/3 bg-gray-100 rounded" />
          </div>
        </div>
      ))}
    </div>
  );
}

/* ─── Empty State ─── */

function EmptyState() {
  return (
    <div className="text-center py-16">
      <div className="text-4xl mb-4">📄</div>
      <h3 className="text-lg font-medium text-gray-900 mb-2">
        No custom deliverable types yet
      </h3>
      <p className="text-sm text-gray-500 mb-6 max-w-md mx-auto">
        Deliverable types define the output format for your agents. System types
        are available by default. Create custom types to define your own output
        structures.
      </p>
    </div>
  );
}
