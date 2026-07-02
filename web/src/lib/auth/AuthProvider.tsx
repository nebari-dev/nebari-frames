import { useEffect, useMemo, useRef, useState } from "react";
import type { User, UserManager } from "oidc-client-ts";
import { fetchAuthConfig } from "./authConfig";
import { buildUserManager } from "./userManager";
import { AuthContext, type AuthContextValue, type AuthStatus } from "./useAuth";

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [status, setStatus] = useState<AuthStatus>("loading");
  const [user, setUser] = useState<User | null>(null);

  // Resolve the UserManager once from the backend's auth config. When auth is
  // disabled (dev mode: the backend injects a fixed identity, so the SPA needs
  // no OIDC), this resolves to null and login/logout become no-ops. Building a
  // UserManager without an issuer would throw "No authority ... configured".
  const ready = useRef<Promise<UserManager | null> | null>(null);
  function manager(): Promise<UserManager | null> {
    if (!ready.current) {
      ready.current = fetchAuthConfig().then((cfg) =>
        cfg.enabled ? buildUserManager(cfg, window.location.origin) : null,
      );
    }
    return ready.current;
  }

  useEffect(() => {
    let cancelled = false;
    manager()
      .then(async (mgr) => {
        if (cancelled) return;
        // Auth disabled: the backend serves every request as the dev user.
        if (!mgr) {
          setStatus("authenticated");
          return;
        }
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
        const mgr = await manager();
        if (mgr) mgr.signinRedirect();
      },
      completeLogin: async () => {
        const mgr = await manager();
        if (!mgr) return;
        const u = await mgr.signinRedirectCallback();
        setUser(u);
        setStatus("authenticated");
      },
      logout: async () => {
        const mgr = await manager();
        if (mgr) await mgr.removeUser();
        setUser(null);
        setStatus("anonymous");
      },
    }),
    [status, user],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
