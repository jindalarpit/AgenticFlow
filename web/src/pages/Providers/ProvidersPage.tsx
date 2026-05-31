import { useState, useCallback } from "react";
import {
  useProviders,
  useDeleteProvider,
  useValidateProvider,
  type Provider,
} from "../../hooks/useProviders";
import { useToast } from "../../components/Toast";
import { ConfirmDialog } from "../../components/ConfirmDialog";
import { ProviderFormDialog } from "./ProviderFormDialog";
import { StatusBadge } from "./StatusBadge";
import { ProviderTypeBadge } from "./ProviderTypeBadge";

/**
 * Provider management page at `/providers`.
 *
 * Displays all registered online AI providers with status badges,
 * model counts, and CRUD actions (create, edit, delete, re-validate).
 *
 * Requirements: 1.1, 1.9, 1.10, 1.11, 1.12
 */
export default function ProvidersPage() {
  const { data: providers, isLoading, isError, refetch } = useProviders();
  const deleteProvider = useDeleteProvider();
  const validateProvider = useValidateProvider();
  const { showToast } = useToast();

  const [showCreate, setShowCreate] = useState(false);
  const [editingProvider, setEditingProvider] = useState<Provider | null>(null);
  const [deletingProvider, setDeletingProvider] = useState<Provider | null>(null);

  const handleDelete = useCallback(async () => {
    if (!deletingProvider) return;
    try {
      await deleteProvider.mutateAsync(deletingProvider.id);
      showToast("Provider deleted successfully", "success");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to delete provider";
      showToast(message, "error");
    } finally {
      setDeletingProvider(null);
    }
  }, [deletingProvider, deleteProvider, showToast]);

  const handleValidate = useCallback(
    async (id: string) => {
      try {
        await validateProvider.mutateAsync(id);
        showToast("Validation triggered", "success");
      } catch (err) {
        const message = err instanceof Error ? err.message : "Failed to validate provider";
        showToast(message, "error");
      }
    },
    [validateProvider, showToast]
  );

  // Loading state
  if (isLoading) {
    return (
      <div className="max-w-7xl mx-auto px-6 py-8">
        <PageHeader onCreate={() => setShowCreate(true)} />
        <LoadingSkeleton />
      </div>
    );
  }

  // Error state
  if (isError) {
    return (
      <div className="max-w-7xl mx-auto px-6 py-8">
        <PageHeader onCreate={() => setShowCreate(true)} />
        <div className="text-center py-16">
          <div className="text-4xl mb-4">⚠️</div>
          <h3 className="text-lg font-medium text-gray-900 mb-2">
            Failed to load providers
          </h3>
          <p className="text-sm text-gray-500 mb-6">
            Something went wrong while fetching your providers.
          </p>
          <button
            onClick={() => refetch()}
            className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
          >
            Retry
          </button>
        </div>
      </div>
    );
  }

  const hasProviders = (providers?.length ?? 0) > 0;

  return (
    <div className="max-w-7xl mx-auto px-6 py-8">
      <PageHeader onCreate={() => setShowCreate(true)} />

      {!hasProviders ? (
        <EmptyState onCreate={() => setShowCreate(true)} />
      ) : (
        <div className="mt-6">
          <ProviderTable
            providers={providers!}
            onEdit={setEditingProvider}
            onDelete={setDeletingProvider}
            onValidate={handleValidate}
          />
        </div>
      )}

      {/* Create Dialog */}
      {showCreate && (
        <ProviderFormDialog
          onClose={() => setShowCreate(false)}
          onSuccess={() => {
            setShowCreate(false);
            showToast("Provider created successfully", "success");
          }}
        />
      )}

      {/* Edit Dialog */}
      {editingProvider && (
        <ProviderFormDialog
          provider={editingProvider}
          onClose={() => setEditingProvider(null)}
          onSuccess={() => {
            setEditingProvider(null);
            showToast("Provider updated successfully", "success");
          }}
        />
      )}

      {/* Delete Confirmation */}
      <ConfirmDialog
        open={deletingProvider !== null}
        title="Delete Provider"
        message={`Are you sure you want to delete "${deletingProvider?.name ?? ""}"? This action cannot be undone.`}
        confirmLabel="Delete"
        confirmVariant="danger"
        onConfirm={handleDelete}
        onCancel={() => setDeletingProvider(null)}
      />
    </div>
  );
}

/* ─── Page Header ─── */

function PageHeader({ onCreate }: { onCreate: () => void }) {
  return (
    <div className="flex items-center justify-between">
      <div>
        <h1 className="text-2xl font-semibold text-gray-900">Providers</h1>
        <p className="mt-1 text-sm text-gray-500">
          Manage your online AI service providers
        </p>
      </div>
      <button
        onClick={onCreate}
        className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
      >
        <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
        </svg>
        Add Provider
      </button>
    </div>
  );
}

/* ─── Provider Table ─── */

interface ProviderTableProps {
  providers: Provider[];
  onEdit: (provider: Provider) => void;
  onDelete: (provider: Provider) => void;
  onValidate: (id: string) => void;
}

function ProviderTable({ providers, onEdit, onDelete, onValidate }: ProviderTableProps) {
  return (
    <div className="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
      <table className="min-w-full divide-y divide-gray-200">
        <thead className="bg-gray-50">
          <tr>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Name
            </th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Type
            </th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Status
            </th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Models
            </th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Created
            </th>
            <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
              Actions
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-200">
          {providers.map((provider) => (
            <ProviderRow
              key={provider.id}
              provider={provider}
              onEdit={() => onEdit(provider)}
              onDelete={() => onDelete(provider)}
              onValidate={() => onValidate(provider.id)}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}

/* ─── Provider Row ─── */

interface ProviderRowProps {
  provider: Provider;
  onEdit: () => void;
  onDelete: () => void;
  onValidate: () => void;
}

function ProviderRow({ provider, onEdit, onDelete, onValidate }: ProviderRowProps) {
  const modelCount = provider.models?.length ?? 0;
  const createdDate = new Date(provider.created_at).toLocaleDateString();

  return (
    <tr className="hover:bg-gray-50 transition-colors">
      <td className="px-6 py-4 whitespace-nowrap">
        <div className="text-sm font-medium text-gray-900">{provider.name}</div>
        {provider.status_message && provider.status === "error" && (
          <div className="text-xs text-red-500 mt-0.5 truncate max-w-xs" title={provider.status_message}>
            {provider.status_message}
          </div>
        )}
      </td>
      <td className="px-6 py-4 whitespace-nowrap">
        <ProviderTypeBadge type={provider.provider_type} />
      </td>
      <td className="px-6 py-4 whitespace-nowrap">
        <StatusBadge status={provider.status} />
      </td>
      <td className="px-6 py-4 whitespace-nowrap">
        <span className="text-sm text-gray-600">
          {modelCount} {modelCount === 1 ? "model" : "models"}
        </span>
      </td>
      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
        {createdDate}
      </td>
      <td className="px-6 py-4 whitespace-nowrap text-right">
        <div className="flex items-center justify-end gap-2">
          {provider.status === "error" && (
            <button
              onClick={onValidate}
              title="Re-validate credentials"
              className="p-1.5 text-gray-400 hover:text-blue-600 rounded-md hover:bg-blue-50 transition-colors"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </button>
          )}
          <button
            onClick={onEdit}
            title="Edit provider"
            className="p-1.5 text-gray-400 hover:text-blue-600 rounded-md hover:bg-blue-50 transition-colors"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
            </svg>
          </button>
          <button
            onClick={onDelete}
            title="Delete provider"
            className="p-1.5 text-gray-400 hover:text-red-600 rounded-md hover:bg-red-50 transition-colors"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
          </button>
        </div>
      </td>
    </tr>
  );
}

/* ─── Empty State ─── */

function EmptyState({ onCreate }: { onCreate: () => void }) {
  return (
    <div className="text-center py-16 mt-6">
      <div className="text-4xl mb-4">🔌</div>
      <h3 className="text-lg font-medium text-gray-900 mb-2">
        No providers registered
      </h3>
      <p className="text-sm text-gray-500 mb-6 max-w-md mx-auto">
        Add an online AI provider to start using cloud-based models for your agents.
        Supported providers include OpenAI, Azure OpenAI, AWS Bedrock, Anthropic, and LiteLLM.
      </p>
      <button
        onClick={onCreate}
        className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
      >
        <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
        </svg>
        Add Your First Provider
      </button>
    </div>
  );
}

/* ─── Loading Skeleton ─── */

function LoadingSkeleton() {
  return (
    <div className="mt-6 space-y-3">
      {Array.from({ length: 3 }).map((_, i) => (
        <div
          key={i}
          className="bg-white rounded-lg border border-gray-200 p-4 animate-pulse"
        >
          <div className="flex items-center gap-4">
            <div className="h-5 w-40 bg-gray-200 rounded" />
            <div className="h-5 w-24 bg-gray-100 rounded" />
            <div className="h-5 w-16 bg-gray-100 rounded" />
            <div className="flex-1" />
            <div className="h-5 w-20 bg-gray-100 rounded" />
          </div>
        </div>
      ))}
    </div>
  );
}
