import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
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
    <div className="min-h-screen grid place-items-center text-muted-foreground">
      {failed ? <p className="text-destructive">Sign-in failed. Please try again.</p> : <p>Signing you in...</p>}
    </div>
  );
}
