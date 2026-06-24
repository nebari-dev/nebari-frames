import { UserManager, WebStorageStateStore } from "oidc-client-ts";
import type { AuthConfig } from "./authConfig";

// Builds a PKCE UserManager. Tokens live in sessionStorage (not localStorage)
// to limit XSS exfiltration and cross-tab persistence.
export function buildUserManager(cfg: AuthConfig, origin: string): UserManager {
  return new UserManager({
    authority: cfg.issuer,
    client_id: cfg.clientId,
    redirect_uri: `${origin}/auth/callback`,
    post_logout_redirect_uri: origin,
    response_type: "code",
    scope: "openid profile email",
    automaticSilentRenew: true,
    userStore: new WebStorageStateStore({ store: window.sessionStorage }),
    stateStore: new WebStorageStateStore({ store: window.sessionStorage }),
  });
}
