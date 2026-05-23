const API_BASE = "";

/**
 * Get the stored PAT token from localStorage.
 */
function getToken(): string | null {
  return localStorage.getItem("af_token");
}

/**
 * Set the PAT token in localStorage.
 */
export function setToken(token: string): void {
  localStorage.setItem("af_token", token);
}

/**
 * Remove the PAT token from localStorage.
 */
export function clearToken(): void {
  localStorage.removeItem("af_token");
}

/**
 * Check if a token is currently stored.
 */
export function hasToken(): boolean {
  return getToken() !== null;
}

/**
 * API client fetch wrapper that injects the Authorization: Bearer <token> header.
 * Automatically redirects to /login on 401 responses.
 */
export async function apiFetch<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getToken();

  const headers = new Headers(options.headers);
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  if (!headers.has("Content-Type") && options.body) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  if (response.status === 401) {
    clearToken();
    window.location.href = "/login";
    throw new Error("Authentication failed");
  }

  if (!response.ok) {
    const body = await response.text();
    throw new Error(body || `Request failed with status ${response.status}`);
  }

  // Handle 204 No Content
  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}
