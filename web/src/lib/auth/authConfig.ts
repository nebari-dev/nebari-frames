import { env } from "@/lib/env";

export interface AuthConfig {
  issuer: string;
  clientId: string;
}

// Reads OIDC discovery info from the backend (same endpoint the CLI uses).
export async function fetchAuthConfig(): Promise<AuthConfig> {
  const res = await fetch(`${env.backendBaseUrl}/auth/config`);
  if (!res.ok) {
    throw new Error(`auth config request failed: ${res.status}`);
  }
  // Backend emits snake_case issuer_url / client_id (server.go authConfigResponse).
  const body = (await res.json()) as { issuer_url: string; client_id: string };
  return { issuer: body.issuer_url, clientId: body.client_id };
}
