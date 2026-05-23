import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api";
import { TokenCreatedModal } from "./TokenCreatedModal";

/* ─── Types ─── */

interface Token {
  id: string;
  name: string;
  token_prefix: string;
  expires_at: string | null;
  last_used_at: string | null;
  created_at: string;
}

interface CreateTokenResponse extends Token {
  token: string;
}

interface CreateTokenRequest {
  name: string;
  expires_in_days: number | null;
}

/* ─── Expiry Options ─── */

const expiryOptions: { label: string; value: number | null }[] = [
  { label: "30 days", value: 30 },
  { label: "90 days", value: 90 },
  { label: "365 days", value: 365 },
  { label: "Never", value: null },
];

/* ─── Helpers ─── */

function maskPrefix(prefix: string): string {
  const visible = prefix.slice(0, 4);
  return `${visible}••••••••`;
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function isExpired(expiresAt: string | null): boolean {
  if (!expiresAt) return false;
  return new Date(expiresAt) < new Date();
}

/* ─── Main Component ─── */

export function TokenManagement() {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [expiresInDays, setExpiresInDays] = useState<number | null>(90);
  const [createdToken, setCreatedToken] = useState<string | null>(null);

  // Fetch tokens
  const {
    data: tokens,
    isLoading,
  } = useQuery<Token[]>({
    queryKey: ["tokens"],
    queryFn: () => apiFetch<Token[]>("/api/tokens"),
  });

  // Create token mutation
  const createMutation = useMutation<CreateTokenResponse, Error, CreateTokenRequest>({
    mutationFn: (data) =>
      apiFetch<CreateTokenResponse>("/api/tokens", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ["tokens"] });
      setName("");
      setExpiresInDays(90);
      setCreatedToken(data.token);
    },
  });

  // Revoke token mutation
  const revokeMutation = useMutation<void, Error, string>({
    mutationFn: (tokenId) =>
      apiFetch<void>(`/api/tokens/${tokenId}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tokens"] });
    },
  });

  function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) return;
    createMutation.mutate({ name: trimmedName, expires_in_days: expiresInDays });
  }

  return (
    <div className="space-y-6">
      {/* Token Created Modal */}
      {createdToken && (
        <TokenCreatedModal
          token={createdToken}
          onClose={() => setCreatedToken(null)}
        />
      )}

      {/* Create Token Form */}
      <div className="bg-white rounded-lg border border-gray-200 p-6 shadow-sm">
        <h2 className="text-lg font-medium text-gray-900 mb-4">
          Create New Token
        </h2>
        <form onSubmit={handleCreate} className="flex flex-col sm:flex-row gap-3">
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Token name"
            maxLength={64}
            required
            className="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
            aria-label="Token name"
          />
          <select
            value={expiresInDays === null ? "null" : String(expiresInDays)}
            onChange={(e) =>
              setExpiresInDays(
                e.target.value === "null" ? null : Number(e.target.value)
              )
            }
            className="rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
            aria-label="Token expiry"
          >
            {expiryOptions.map((opt) => (
              <option key={String(opt.value)} value={opt.value === null ? "null" : String(opt.value)}>
                {opt.label}
              </option>
            ))}
          </select>
          <button
            type="submit"
            disabled={createMutation.isPending || !name.trim()}
            className="inline-flex items-center justify-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {createMutation.isPending ? "Creating…" : "Create"}
          </button>
        </form>
        {createMutation.isError && (
          <p className="mt-2 text-sm text-red-600">
            {createMutation.error.message}
          </p>
        )}
      </div>

      {/* Token List */}
      <div className="bg-white rounded-lg border border-gray-200 shadow-sm">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-lg font-medium text-gray-900">
            Personal Access Tokens
          </h2>
          <p className="text-sm text-gray-500 mt-1">
            Tokens are used for CLI and API authentication.
          </p>
        </div>

        {isLoading ? (
          <div className="px-6 py-8 text-center text-sm text-gray-500">
            Loading tokens…
          </div>
        ) : !tokens || tokens.length === 0 ? (
          <EmptyState />
        ) : (
          <div className="divide-y divide-gray-100">
            {tokens.map((token) => (
              <TokenRow
                key={token.id}
                token={token}
                onRevoke={() => revokeMutation.mutate(token.id)}
                isRevoking={revokeMutation.isPending}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

/* ─── Token Row ─── */

function TokenRow({
  token,
  onRevoke,
  isRevoking,
}: {
  token: Token;
  onRevoke: () => void;
  isRevoking: boolean;
}) {
  const expired = isExpired(token.expires_at);

  return (
    <div
      className={`px-6 py-4 flex flex-col sm:flex-row sm:items-center gap-3 ${
        expired ? "opacity-60" : ""
      }`}
    >
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span
            className={`font-medium text-sm ${
              expired ? "text-gray-400 line-through" : "text-gray-900"
            }`}
          >
            {token.name}
          </span>
          {expired && (
            <span className="inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium bg-red-100 text-red-700">
              Expired
            </span>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 mt-1 text-xs text-gray-500">
          <span className="font-mono">{maskPrefix(token.token_prefix)}</span>
          <span>Created {formatDate(token.created_at)}</span>
          <span>
            Last used:{" "}
            {token.last_used_at ? formatDate(token.last_used_at) : "Never"}
          </span>
          <span>
            Expires:{" "}
            {token.expires_at ? formatDate(token.expires_at) : "Never"}
          </span>
        </div>
      </div>
      <button
        onClick={onRevoke}
        disabled={isRevoking}
        className="inline-flex items-center px-3 py-1.5 text-sm font-medium text-red-600 bg-red-50 rounded-md hover:bg-red-100 focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        Revoke
      </button>
    </div>
  );
}

/* ─── Empty State ─── */

function EmptyState() {
  return (
    <div className="px-6 py-12 text-center">
      <div className="text-3xl mb-3">🔑</div>
      <h3 className="text-sm font-medium text-gray-900 mb-1">
        No tokens yet
      </h3>
      <p className="text-sm text-gray-500">
        Create a personal access token to authenticate with the CLI or API.
      </p>
    </div>
  );
}
