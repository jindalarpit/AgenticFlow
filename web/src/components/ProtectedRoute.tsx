import { Navigate } from "react-router-dom";
import { hasToken } from "../lib/api";

interface ProtectedRouteProps {
  children: React.ReactNode;
}

/**
 * Route wrapper that redirects unauthenticated users to /login.
 */
export function ProtectedRoute({ children }: ProtectedRouteProps) {
  if (!hasToken()) {
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
}
