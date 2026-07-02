import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { AuthLayout } from "@/components/layout/AuthLayout";
import { Spinner } from "@/components/ui/spinner";
import { useAuth } from "@/lib/auth/useAuth";

export function CallbackPage() {
  const { completeLogin } = useAuth();
  const navigate = useNavigate();
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    completeLogin()
      .then(() => navigate("/", { replace: true }))
      .catch(() => setFailed(true));
  }, [completeLogin, navigate]);
  return (
    <AuthLayout
      title={failed ? "Sign-in failed" : "Signing you in…"}
      description={
        failed ? "Something went wrong completing sign-in. Please try again." : undefined
      }
    >
      {failed ? (
        <p className="text-sm text-destructive-foreground">Please return to the login page.</p>
      ) : (
        <div className="flex justify-center text-muted-foreground">
          <Spinner className="size-6" />
        </div>
      )}
    </AuthLayout>
  );
}
