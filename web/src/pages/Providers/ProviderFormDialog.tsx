import { useState, useEffect, useRef, useCallback } from "react";
import {
  useCreateProvider,
  useUpdateProvider,
  CREDENTIAL_FIELDS,
  PROVIDER_TYPE_LABELS,
  type Provider,
  type ProviderType,
  type CreateProviderRequest,
  type UpdateProviderRequest,
} from "../../hooks/useProviders";

interface ProviderFormDialogProps {
  provider?: Provider | null;
  onClose: () => void;
  onSuccess: () => void;
}

const PROVIDER_TYPES: ProviderType[] = [
  "openai",
  "azure_openai",
  "aws_bedrock",
  "anthropic",
  "litellm",
];

/**
 * Dialog for creating or editing a provider.
 * Shows dynamic credential fields based on the selected provider_type.
 */
export function ProviderFormDialog({ provider, onClose, onSuccess }: ProviderFormDialogProps) {
  const isEditing = !!provider;
  const dialogRef = useRef<HTMLDialogElement>(null);

  const createProvider = useCreateProvider();
  const updateProvider = useUpdateProvider();

  const [name, setName] = useState(provider?.name ?? "");
  const [providerType, setProviderType] = useState<ProviderType>(
    provider?.provider_type ?? "openai"
  );
  const [credentials, setCredentials] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Open dialog on mount
  useEffect(() => {
    const dialog = dialogRef.current;
    if (dialog && !dialog.open) {
      dialog.showModal();
    }
  }, []);

  // Reset credentials when provider type changes (only for create mode)
  useEffect(() => {
    if (!isEditing) {
      setCredentials({});
    }
  }, [providerType, isEditing]);

  const handleCredentialChange = useCallback((key: string, value: string) => {
    setCredentials((prev) => ({ ...prev, [key]: value }));
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    // Basic client-side validation
    if (!name.trim()) {
      setError("Name is required");
      return;
    }
    if (name.length > 128) {
      setError("Name must be at most 128 characters");
      return;
    }

    // Validate required credential fields (only for create, or if credentials provided for edit)
    const fields = CREDENTIAL_FIELDS[providerType];
    if (!isEditing) {
      for (const field of fields) {
        if (field.required && !credentials[field.key]?.trim()) {
          setError(`${field.label} is required`);
          return;
        }
      }
    }

    setSubmitting(true);

    try {
      if (isEditing) {
        const data: UpdateProviderRequest = { name: name.trim() };
        // Only include credentials if any field was filled
        const hasCredentials = Object.values(credentials).some((v) => v.trim());
        if (hasCredentials) {
          data.credentials = credentials;
        }
        await updateProvider.mutateAsync({ id: provider!.id, data });
      } else {
        const data: CreateProviderRequest = {
          name: name.trim(),
          provider_type: providerType,
          credentials,
        };
        await createProvider.mutateAsync(data);
      }
      onSuccess();
    } catch (err) {
      const message = err instanceof Error ? err.message : "An error occurred";
      setError(message);
    } finally {
      setSubmitting(false);
    }
  };

  const handleBackdropClick = (e: React.MouseEvent<HTMLDialogElement>) => {
    if (e.target === dialogRef.current) {
      onClose();
    }
  };

  const handleCancel = (e: React.SyntheticEvent<HTMLDialogElement>) => {
    e.preventDefault();
    onClose();
  };

  const credentialFields = CREDENTIAL_FIELDS[providerType];

  return (
    <dialog
      ref={dialogRef}
      onClick={handleBackdropClick}
      onCancel={handleCancel}
      className="fixed inset-0 z-50 m-auto w-full max-w-lg rounded-lg border border-gray-200 bg-white p-0 shadow-xl backdrop:bg-black/30"
      aria-labelledby="provider-form-title"
    >
      <form onSubmit={handleSubmit} className="p-6">
        <h2 id="provider-form-title" className="text-lg font-semibold text-gray-900 mb-4">
          {isEditing ? "Edit Provider" : "Add Provider"}
        </h2>

        {error && (
          <div className="mb-4 p-3 rounded-md bg-red-50 border border-red-200">
            <p className="text-sm text-red-700">{error}</p>
          </div>
        )}

        <div className="space-y-4">
          {/* Name */}
          <div>
            <label htmlFor="provider-name" className="block text-sm font-medium text-gray-700 mb-1">
              Name
            </label>
            <input
              id="provider-name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="My OpenAI Provider"
              maxLength={128}
              required
              className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            />
          </div>

          {/* Provider Type (only for create) */}
          {!isEditing && (
            <div>
              <label htmlFor="provider-type" className="block text-sm font-medium text-gray-700 mb-1">
                Provider Type
              </label>
              <select
                id="provider-type"
                value={providerType}
                onChange={(e) => setProviderType(e.target.value as ProviderType)}
                className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
              >
                {PROVIDER_TYPES.map((type) => (
                  <option key={type} value={type}>
                    {PROVIDER_TYPE_LABELS[type]}
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* Dynamic Credential Fields */}
          <fieldset className="border border-gray-200 rounded-md p-4">
            <legend className="text-sm font-medium text-gray-700 px-1">
              {isEditing ? "Credentials (leave blank to keep existing)" : "Credentials"}
            </legend>
            <div className="space-y-3 mt-2">
              {credentialFields.map((field) => (
                <div key={field.key}>
                  <label
                    htmlFor={`cred-${field.key}`}
                    className="block text-sm font-medium text-gray-600 mb-1"
                  >
                    {field.label}
                    {field.required && !isEditing && (
                      <span className="text-red-500 ml-0.5">*</span>
                    )}
                  </label>
                  <input
                    id={`cred-${field.key}`}
                    type={field.type === "password" ? "password" : "text"}
                    value={credentials[field.key] ?? ""}
                    onChange={(e) => handleCredentialChange(field.key, e.target.value)}
                    placeholder={field.placeholder}
                    required={field.required && !isEditing}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                  />
                </div>
              ))}
            </div>
          </fieldset>
        </div>

        {/* Actions */}
        <div className="mt-6 flex justify-end gap-3">
          <button
            type="button"
            onClick={onClose}
            className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={submitting}
            className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {submitting
              ? isEditing
                ? "Saving…"
                : "Creating…"
              : isEditing
                ? "Save Changes"
                : "Create Provider"}
          </button>
        </div>
      </form>
    </dialog>
  );
}
