import { useEffect, useMemo, useRef, useState } from "react";
import type { User, UserManager } from "oidc-client-ts";
import { fetchAuthConfig } from "./authConfig";
import { buildUserManager } from "./userManager";
import { AuthContext, type AuthContextValue, type AuthStatus } from "./useAuth";

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [status, setStatus] = useState<AuthStatus>("loading");
  const [user, setUser] = useState<User | null>(null);
  const mgrRef = useRef<UserManager | null>(null);

  // Resolve the UserManager once from the backend's auth config.
  const ready = useRef<Promise<UserManager> | null>(null);
  function manager(): Promise<UserManager> {
    if (!ready.current) {
      ready.current = fetchAuthConfig().then((cfg) =>
        buildUserManager(cfg, window.location.origin),
      );
    }
    return ready.current;
  }

  useEffect(() => {
    let cancelled = false;
    manager()
      .then(async (mgr) => {
        mgrRef.current = mgr;
        const existing = await mgr.getUser();
        if (cancelled) return;
        if (existing && !existing.expired) {
          setUser(existing);
          setStatus("authenticated");
        } else {
          setStatus("anonymous");
        }
      })
      .catch(() => !cancelled && setStatus("anonymous"));
    return () => {
      cancelled = true;
    };
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      status,
      user,
      getAccessToken: () => user?.access_token,
      login: async () => {
        (await manager()).signinRedirect();
      },
      completeLogin: async () => {
        const mgr = await manager();
        const u = await mgr.signinRedirectCallback();
        setUser(u);
        setStatus("authenticated");
      },
      logout: async () => {
        const mgr = await manager();
        await mgr.removeUser();
        setUser(null);
        setStatus("anonymous");
      },
    }),
    [status, user],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
