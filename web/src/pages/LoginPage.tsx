import { Navigate } from "react-router";
import { AuthLayout } from "@/components/layout/AuthLayout";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/lib/auth/useAuth";

export function LoginPage() {
  const { status, login } = useAuth();
  if (status === "authenticated") {
    return <Navigate to="/" replace />;
  }
  return (
    <AuthLayout
      title="Sign in to Frames"
      description="Browse, author, and connect your organization's Frames."
    >
      <Button
        className="w-full"
        onClick={() => void login()}
        loading={status === "loading"}
        loadingText="Signing in…"
      >
        Log in
      </Button>
    </AuthLayout>
  );
}
