import { useCallback, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { apiFetch, setToken, clearToken, hasToken } from "../lib/api";

interface LoginResponse {
  token: string;
}

/**
 * Auth state management hook.
 * Provides login, logout, and authentication status.
 */
export function useAuth() {
  const navigate = useNavigate();

  const isAuthenticated = useMemo(() => hasToken(), []);

  const login = useCallback(
    async (email: string, password: string) => {
      const data = await apiFetch<LoginResponse>("/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password }),
      });
      setToken(data.token);
      navigate("/");
    },
    [navigate]
  );

  const logout = useCallback(() => {
    clearToken();
    navigate("/login");
  }, [navigate]);

  return { isAuthenticated, login, logout, token: hasToken() ? "stored" : null };
}
