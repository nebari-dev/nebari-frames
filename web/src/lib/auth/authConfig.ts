import { env } from "@/lib/env";

export interface AuthConfig {
  // False when the backend runs with auth disabled (FRAMES_DEV_MODE): no OIDC
  // provider exists and issuer/clientId are empty. See server.go authConfigResponse.
  enabled: boolean;
  issuer: string;
  clientId: string;
}

// Reads OIDC discovery info from the backend (same endpoint the CLI uses).
export async function fetchAuthConfig(): Promise<AuthConfig> {
  const res = await fetch(`${env.backendBaseUrl}/auth/config`);
  if (!res.ok) {
    throw new Error(`auth config request failed: ${res.status}`);
  }
  // Backend emits snake_case enabled / issuer_url / client_id (server.go
  // authConfigResponse). In dev mode enabled is false and the URLs are omitted.
  const body = (await res.json()) as {
    enabled: boolean;
    issuer_url?: string;
    client_id?: string;
  };
  return {
    enabled: body.enabled,
    issuer: body.issuer_url ?? "",
    clientId: body.client_id ?? "",
  };
}
