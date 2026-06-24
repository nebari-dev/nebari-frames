import { Navigate, Outlet, useLocation } from "react-router";
import { useAuth } from "@/lib/auth/useAuth";

export function RequireAuth() {
  const { status } = useAuth();
  const location = useLocation();
  if (status === "loading") {
    return <div className="p-8 text-muted-foreground">Loading...</div>;
  }
  if (status === "anonymous") {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  }
  return <Outlet />;
}
