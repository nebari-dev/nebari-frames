import { useAuth } from "@/lib/auth/useAuth";
import { Button } from "@/components/ui/button";
import { Navigate } from "react-router";

export function LoginPage() {
  const { status, login } = useAuth();
  if (status === "authenticated") {
    return <Navigate to="/" replace />;
  }
  return (
    <div className="min-h-screen grid place-items-center">
      <div className="text-center space-y-4">
        <h1 className="text-2xl font-semibold">Nebari Frames</h1>
        <Button onClick={() => void login()} disabled={status === "loading"}>
          Log in
        </Button>
      </div>
    </div>
  );
}
